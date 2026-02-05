# AccessGraph Architecture

This document provides a deep dive into AccessGraph's architecture, design decisions, and implementation details.

## System Overview

AccessGraph is a graph-based security policy analyzer designed for offline environments. It follows a classic three-tier architecture:

1. **Data Layer**: SQLite for persistence, gonum for in-memory graph operations
2. **Application Layer**: Go backend with GraphQL API, OPA policy engine
3. **Presentation Layer**: React SPA with Cytoscape.js for graph visualization

## Core Components

### 1. Ingestion Pipeline (`internal/ingest/`)

**Purpose**: Parse multi-cloud policy files into unified graph representation.

**Parsers**:
- `awsjson.go` - AWS IAM roles, policies, trust relationships
- `k8srbac.go` - Kubernetes ServiceAccounts, Roles, RoleBindings, ClusterRoles
- `tfplan.go` - Terraform plan JSON for IaC tagging

**Design Pattern**: Builder pattern with error accumulation. Each parser returns `([]Node, []Edge, error)`. Errors don't halt parsing - they accumulate and report all issues.

**Key Decision**: Unified `Node` and `Edge` types with `Kind` discriminators instead of polymorphic types. Simplifies serialization and enables SQL queries by kind.

### 2. Graph Engine (`internal/graph/`)

**Purpose**: In-memory directed multigraph with traversal algorithms.

**Implementation**:
```go
type Graph struct {
    g         *simple.DirectedGraph  // gonum graph
    nodeMap   map[string]graph.Node  // ID -> gonum node
    edgeMap   map[string]*Edge       // composite key -> edge
    edgeIndex map[int64][]Edge       // node ID -> outbound edges (O(1) lookup)
    mu        sync.RWMutex           // protects concurrent access
}
```

**Key Algorithms**:
- **BFS Shortest Path** (`ShortestPath`): Classic BFS with depth tracking. Visited map prevents cycles. Early termination on sensitive resource match.
- **Attack Path Enumeration** (`attackpath.go`): BFS with hop limit and sensitivity awareness. Returns path with risk annotations.
- **Neighbor Queries** (`Neighbors`): O(1) via edge index instead of O(E) iteration.

**Concurrency Model**: RWMutex enables multiple concurrent readers (GraphQL queries) with exclusive writes (graph loading). Read-heavy workload (90%+ queries vs 10% writes) benefits from this.

**Trade-off**: In-memory graph limits scale to available RAM. For 1M+ nodes, export to Neo4j.

### 3. Persistence Layer (`internal/store/`)

**Purpose**: SQLite-based snapshot storage for point-in-time graph captures.

**Schema** (`models.sql`):
```sql
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    label TEXT
);

CREATE TABLE nodes (
    snapshot_id TEXT NOT NULL,
    id TEXT NOT NULL,
    kind TEXT NOT NULL,
    labels TEXT NOT NULL,  -- JSON array
    props TEXT NOT NULL,   -- JSON object
    PRIMARY KEY (snapshot_id, id)
);

CREATE TABLE edges (
    snapshot_id TEXT NOT NULL,
    src TEXT NOT NULL,
    dst TEXT NOT NULL,
    kind TEXT NOT NULL,
    props TEXT NOT NULL,   -- JSON object
    PRIMARY KEY (snapshot_id, src, dst, kind)
);

-- Indices for query performance
CREATE INDEX idx_nodes_snapshot_kind ON nodes(snapshot_id, kind);
CREATE INDEX idx_nodes_id ON nodes(id);
CREATE INDEX idx_edges_snapshot ON edges(snapshot_id);
```

**Key Decision**: Composite primary keys (snapshot_id + node/edge identifiers) enable multiple snapshots in one database. Alternative (separate DB per snapshot) complicates queries across snapshots.

**Indices**: Added in Phase 2 after profiling showed O(N) scans on `kind` filters. 3-5x speedup on large graphs.

**SQLite Driver**: `modernc.org/sqlite` (pure Go, CGO-free) enables static binary compilation. Alternative `mattn/go-sqlite3` requires CGO, breaking cross-compilation.

### 4. Policy Engine (`internal/policy/`)

**Purpose**: OPA integration for rule-based security checks.

**Architecture**:
```
Graph → InputBuilder → OPA Input (JSON) → OPA Server → Findings
```

**Input Format** (`input_builder.go`):
```json
{
  "nodes": [{"id": "...", "kind": "PRINCIPAL", "labels": [...]}],
  "edges": [{"src": "...", "dst": "...", "kind": "ASSUMES_ROLE"}],
  "nodeIndex": {"node-id": {"id": "...", "kind": "..."}}
}
```

**Key Decision**: Compact input format with pre-built index. OPA rules query `input.nodeIndex[edge.dst]` for O(1) lookups instead of iterating `input.nodes`.

**OPA Rules** (`policy/*.rego`):
- `wildcards.rego` - Detects `*` in actions/resources
- `cross_account.rego` - Finds trust relationships to external accounts
- `k8s_clusteradmin.rego` - Identifies cluster-admin bindings

**Trade-off**: External OPA server adds operational complexity but enables policy updates without recompiling application code.

### 5. GraphQL API (`internal/api/graphql/`)

**Purpose**: Flexible query interface for graph data.

**Schema Highlights**:
```graphql
type Query {
  snapshots: [Snapshot!]!
  node(snapshotId: String!, id: String!): Node
  searchPrincipals(snapshotId: String!, query: String!, limit: Int): [Node!]!
  findings(snapshotId: String!): [Finding!]!
  attackPath(snapshotId: String!, from: String!, to: String!, maxHops: Int): AttackPathResult!
  recommend(snapshotId: String!, policyId: String!, target: String, cap: Int): Recommendation!
}
```

**Testability Refactor (Phase 2)**:
- Original: Resolvers directly instantiated SQLite stores and OPA clients
- Improved: Introduced `DataStore` and `PolicyEvaluator` interfaces
- Benefit: 5 resolver tests with mocks, zero I/O

**Design Pattern**: Resolver struct holds dependencies:
```go
type Resolver struct {
    store     DataStore          // interface
    evaluator PolicyEvaluator    // interface
    cache     *graphCache        // in-memory graph cache
}
```

**Caching Strategy**: `graphCache` (map[snapshotID]*graph.Graph) with RWMutex. Cache miss loads from SQLite. No eviction policy (bounded by snapshot count, not unbounded user queries).

### 6. Offline Mode (`internal/config/offline.go`)

**Purpose**: Block network egress to prevent data exfiltration.

**Implementation**: Custom `http.RoundTripper` that intercepts requests:
```go
func (t *OfflineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    host := req.URL.Hostname()

    // Always block IMDS (even when OFFLINE=false)
    if host == "169.254.169.254" {
        return nil, ErrIMDSBlocked
    }

    // Block external hosts when offline
    if t.offline && !isLocalhost(host) && !isRFC1918(host) {
        return nil, ErrOfflineMode
    }

    return t.base.RoundTrip(req)
}
```

**Key Decision**: IMDS blocking is unconditional. Cloud metadata services are a common attack vector in containerized environments.

**Allowlist**: Localhost (127.0.0.1, ::1, localhost), RFC1918 (10.x, 172.16-31.x, 192.168.x). OPA server at `http://opa:8181` allowed via Docker internal network.

### 7. Export Formats (`internal/graph/export_*.go`)

**Markdown** (`export_markdown.go`):
- Human-readable attack path reports
- Risk annotations (wildcard permissions, cross-account trust)
- Includes remediation guidance

**SARIF** (`export_sarif.go`):
- SARIF v2.1.0 compliant
- CI/CD integration (GitHub Security tab, GitLab SAST)
- Machine-readable findings with locations and severities

**Cypher** (`export_cypher.go`):
- Neo4j-compatible MERGE statements
- Deterministic output (sorted nodes/edges)
- Sample queries for common analysis patterns

**Golden Tests**: All exporters have golden file tests. Ensures deterministic output for same snapshot.

## Data Flow Examples

### Ingestion Flow

```
1. User runs: accessgraph-ingest --aws sample/aws --k8s sample/k8s --snapshot demo1
2. Parse AWS JSON → []Node, []Edge (roles, policies, trust relationships)
3. Parse K8s YAML → []Node, []Edge (ServiceAccounts, Roles, Bindings)
4. Merge nodes/edges → Graph
5. Graph.SaveSnapshot(ctx, "demo1", "Demo snapshot", graph) → SQLite
```

### Query Flow

```
1. User queries: {node(snapshotId: "demo1", id: "arn:...")}
2. Resolver.Node() → cache.Get("demo1")
3. Cache miss → store.LoadSnapshot(ctx, "demo1") → Graph
4. cache.Set("demo1", graph)
5. graph.GetNode("arn:...") → Node
6. Return GraphQL Node type
```

### Attack Path Flow

```
1. CLI: accessgraph-cli attack-path --from X --to Y --sarif out.sarif
2. GraphQL query: attackPath(snapshotId: "demo1", from: X, to: Y, maxHops: 8)
3. Resolver loads graph from cache/store
4. graph.AttackPath(from, to, maxHops) → BFS traversal
5. Early termination on sensitive resource match
6. Return path with hop count and risk annotations
7. ExportSARIF(path) → SARIF v2.1.0 JSON
```

## Security Architecture

### Threat Model

**In Scope**:
- Data exfiltration via network egress (mitigated by offline mode)
- Secrets leakage in logs (mitigated by redaction)
- Container escape (mitigated by security contexts)
- IMDS attacks (mitigated by unconditional blocking)
- Supply chain attacks (mitigated by Trivy scanning)

**Out of Scope**:
- Malicious policy files (input validation only)
- Privilege escalation in orchestrator (relies on Kubernetes RBAC)
- Physical access to deployment environment

### Defense in Depth

**Layer 1: Network**
- Offline mode blocks egress by default
- IMDS always blocked (169.254.169.254)
- Kubernetes NetworkPolicy (Helm chart) enforces default-deny

**Layer 2: Container**
- Non-root user (UID 65534 for API, 101 for UI)
- Read-only root filesystem
- All capabilities dropped
- No privilege escalation allowed

**Layer 3: Application**
- Request body size limit (10MB)
- Timeouts (read: 15s, write: 15s, idle: 60s)
- Log redaction (ARNs, account IDs, secrets)
- Context-aware request handling (cancellation support)

**Layer 4: Data**
- SQLite database file permissions (0600)
- No plaintext secrets in configuration
- Snapshots isolated by ID (multi-tenancy at data layer)

## Performance Characteristics

### Bottlenecks

**Ingestion**: O(N) where N = files to parse. CPU-bound (JSON/YAML parsing). Parallelizable per file.

**Graph Loading**: O(N + E) where N = nodes, E = edges. I/O-bound (SQLite read). Cached after first load.

**BFS Traversal**: O(V + E) worst case. Early termination on sensitive resource match reduces average case to O(V/2 + E/4) empirically.

**GraphQL Queries**: Resolver overhead ~1ms. Graph operations <100ms for <10K nodes.

### Optimizations

**Indices**: `idx_nodes_snapshot_kind`, `idx_nodes_id`, `idx_edges_snapshot` (added Phase 2). 3-5x speedup on filtered queries.

**Edge Index**: In-memory `map[nodeID][]Edge` for O(1) neighbor lookups vs O(E) iteration.

**RWMutex**: Concurrent reads scale linearly with CPU cores. Single-writer constraint only on cache updates.

**Cytoscape Layout**: Dagre layout algorithm (O(V log V)) for DAGs. Cola layout (O(V²)) for general graphs. UI limits to 500 nodes for 60fps rendering.

### Scalability Limits

| Metric | Limit | Reason |
|--------|-------|--------|
| Nodes | ~1M | RAM (100 bytes/node → 100MB @ 1M nodes) |
| Edges | ~10M | RAM (50 bytes/edge → 500MB @ 10M edges) |
| Snapshots | ~1000 | SQLite file size (100MB/snapshot → 100GB @ 1000) |
| Concurrent Users | ~100 | RWMutex contention, goroutine overhead |

**Mitigation**: For larger graphs, export to Neo4j. For high concurrency, deploy multiple replicas with read-only databases.

## Design Decisions

### Why Go?

**Pros**:
- Native concurrency (goroutines, channels)
- Static binaries (no runtime dependencies)
- Cross-compilation (darwin/linux/windows × amd64/arm64)
- Excellent HTTP/2 and gRPC support
- Fast compile times

**Cons**:
- Verbose error handling
- No generics (until Go 1.18)
- GC pauses (mitigated by GOGC tuning)

**Alternative Considered**: Rust (rejected due to slower compile times, steeper learning curve, and less mature graph libraries).

### Why SQLite?

**Pros**:
- Zero-configuration (embedded)
- Excellent for read-heavy workloads
- ACID transactions
- Snapshot isolation via separate DBs

**Cons**:
- No concurrent writes across processes
- Not suitable for extreme scale (>1M nodes)
- No built-in graph queries

**Alternative Considered**: PostgreSQL (rejected due to deployment complexity and unnecessary for MVP scale).

### Why OPA?

**Pros**:
- Industry standard (CNCF project)
- Declarative policy language (Rego)
- Testable policies (`opa test`)
- Version-controlled policies

**Cons**:
- Learning curve for Rego
- External service dependency
- Policy debugging can be opaque

**Alternative Considered**: Embedded policy engine in Go (rejected due to inflexibility - policy updates require recompilation).

### Why GraphQL?

**Pros**:
- Single endpoint (vs 20+ REST endpoints)
- Strongly typed schema
- Client-specified queries (no over-fetching)
- Perfect for graph data (nested queries)

**Cons**:
- Complexity overhead for simple queries
- N+1 query problem (mitigated by DataLoader pattern)
- Tooling immaturity compared to REST

**Alternative Considered**: REST API (rejected due to proliferation of endpoints for different traversal patterns).

### Why Not Stream Processing?

**Current**: Batch ingestion of snapshot files.

**Alternative**: Real-time streaming from AWS CloudTrail, K8s audit logs.

**Reason for Current Approach**: MVP targets offline/air-gapped environments where real-time streams don't exist. Streaming adds significant complexity (Kafka, state management, exactly-once semantics).

**Future**: Phase 3 may add streaming connectors as optional feature.

## Testing Strategy

### Unit Tests

**Coverage Target**: ≥70% (enforced by CI gate).

**Focus Areas**:
- Parsers (AWS, K8s, Terraform)
- Graph algorithms (BFS, shortest path)
- Store operations (save/load round-trip)
- Policy input building
- Offline mode enforcement

**Pattern**: Table-driven tests with `t.Run` subtests. Golden files for complex outputs (SARIF, Cypher, Markdown).

### Integration Tests

**Script**: `scripts/test-integration.sh`

**Flow**:
1. Ingest sample data → snapshot
2. Query via CLI (snapshots, findings, paths)
3. Assert expected output

**Environment**: Requires Docker (OPA server), SQLite, Go toolchain.

### Resolver Tests

**Approach**: Mock `DataStore` and `PolicyEvaluator` interfaces. Zero I/O, pure logic tests.

**Example**:
```go
func TestResolver_Snapshots(t *testing.T) {
    store := newMockStore()
    store.snapshots = []store.Snapshot{{ID: "s1", Label: "Test"}}

    resolver := newTestResolver(store, nil)
    snapshots, err := resolver.Query().Snapshots(context.Background())

    assert.NoError(t, err)
    assert.Len(t, snapshots, 1)
    assert.Equal(t, "s1", snapshots[0].ID)
}
```

**Coverage**: 5 resolver tests covering critical paths (snapshots, search, findings, node, path).

### Golden File Tests

**Purpose**: Ensure deterministic exports.

**Process**:
1. Generate export (Markdown, SARIF, Cypher)
2. Compare to golden file (`testdata/*.golden`)
3. Update golden file on intentional changes

**Benefit**: Catches unintended format changes. Forces explicit acknowledgment of breaking changes.

## Deployment Patterns

### Local Development

```bash
make build && make demo && docker compose up
```

**Use Case**: Feature development, testing, demos.

### Kubernetes (Helm)

```bash
helm install accessgraph ./deployments/helm/accessgraph \
  --namespace accessgraph \
  --create-namespace
```

**Use Case**: Production deployments, multi-user environments.

**Features**:
- Network policies (default-deny egress)
- Security contexts (non-root, read-only FS)
- Health probes (liveness, readiness, startup)
- Resource limits (CPU/memory)
- Persistent volumes (snapshot storage)
- Ingress (TLS support)

### Air-Gapped Environments

**Requirements**:
- Pre-pulled Docker images
- Local Helm chart
- No external network access

**Process**:
1. Export images: `docker save accessgraph-api:latest | gzip > api.tar.gz`
2. Transfer to air-gapped environment
3. Import: `docker load < api.tar.gz`
4. Deploy with `offline=true`

## Future Architecture Considerations

### Horizontal Scaling

**Challenge**: SQLite doesn't support concurrent writes across replicas.

**Solution**:
- Option 1: Migrate to PostgreSQL (concurrent writes, replication)
- Option 2: Read replicas with write leader (SQLite replication via Litestream)
- Option 3: Sharding by snapshot ID (each replica owns subset of snapshots)

**Trade-off**: Complexity vs scale. Current single-replica design suits 90% of use cases.

### Real-Time Updates

**Challenge**: Batch ingestion requires manual re-ingestion for policy changes.

**Solution**:
- Option 1: GraphQL subscriptions (WebSocket push)
- Option 2: Webhook ingestion endpoint (POST new policy files)
- Option 3: Cloud API connectors (poll AWS/K8s APIs)

**Trade-off**: Real-time adds state management complexity and breaks offline-first design.

### Multi-Tenancy

**Challenge**: Single database for all users/organizations.

**Solution**:
- Option 1: Separate SQLite DBs per tenant (simple but limits queries across tenants)
- Option 2: Tenant ID in all tables (complex row-level security)
- Option 3: Separate deployments per tenant (isolation but higher overhead)

**Trade-off**: Security vs efficiency. Current single-tenant design prioritizes simplicity.

## Conclusion

AccessGraph's architecture prioritizes offline operation, testability, and simplicity over massive scale. Key decisions (SQLite, OPA, GraphQL, Go) optimize for the 90% use case: security teams analyzing <100K node graphs in air-gapped environments.

Future phases can add scale (PostgreSQL, Neo4j export), real-time updates (streaming), and multi-tenancy without fundamental redesign. The interface-driven design (DataStore, PolicyEvaluator) enables incremental evolution.
