# AccessGraph - Cloud Native Security Policy Visualizer

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.19+-326CE5?logo=kubernetes)](https://kubernetes.io)

AccessGraph is a production-grade, offline-first graph-based security policy analyzer for AWS IAM and Kubernetes RBAC. It ingests policy definitions, builds a directed graph of relationships, evaluates policies with OPA, and provides attack path analysis with least-privilege recommendations.

**Latest Release**: v1.1.0

## Why This Project Exists

Organizations struggle to visualize complex cloud access relationships. When an AWS role trusts another account, assumes multiple roles, and those roles have wildcard permissions across hundreds of resources, understanding "who can access what" becomes impossible with spreadsheets or static analysis.

AccessGraph solves this by treating access control as a graph problem. It answers critical security questions:
- Can this compromised service account reach our production database?
- Which policies grant wildcard permissions that violate least privilege?
- How would removing this trust relationship impact access paths?

Built for **air-gapped environments** where data sovereignty and offline operation are non-negotiable. No cloud API calls, no telemetry, no external dependencies beyond localhost.

## Features

### Core Capabilities
- **Multi-Cloud Support**: Parse AWS IAM (roles, policies, trust relationships) and Kubernetes RBAC (ServiceAccounts, Roles, Bindings)
- **Graph Analysis**: Build and query a directed graph of principals, roles, policies, permissions, and resources
- **Policy Evaluation**: Detect security issues using OPA (wildcard actions, cross-account trust, cluster-admin bindings)
- **Visual Interface**: React-based UI with Cytoscape.js for graph visualization and path exploration
- **Snapshot Comparison**: Diff snapshots to track policy changes over time
- **Fully Offline**: No network egress required; works with local data sources and local OPA server

### Phase 2: Production Features ✨
- **Attack Path Enumeration**: Find shortest paths from principals to sensitive resources with markdown/SARIF export
- **Least-Privilege Recommender**: AI-powered wildcard policy tightening with RFC 6902 JSON Patch
- **Neo4j Export**: Generate Cypher scripts for graph database analysis
- **Kubernetes-Ready**: Production Helm chart with security hardening
- **Observability**: Health probes, Prometheus metrics, graceful shutdown
- **Enhanced Security**: IMDS blocking, network policies, non-root containers

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Sample    │────▶│   Ingest     │────▶│   SQLite    │
│ AWS/K8s/TF  │     │   (Go)       │     │  Snapshot   │
└─────────────┘     └──────────────┘     └─────────────┘
                                                 │
                                                 ▼
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  React UI   │◀───▶│  GraphQL API │────▶│    Graph    │
│ Cytoscape.js│     │   (Go/chi)   │     │   Engine    │
└─────────────┘     └──────────────┘     └─────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  OPA Server  │
                    │ (Rego rules) │
                    └──────────────┘
```

### Technology Choices

**Go Backend** - Chosen for strong concurrency primitives, excellent HTTP/2 support, and native cross-compilation. The graph traversal workload benefits from goroutines for parallel BFS operations.

**gonum/graph** - Industry-standard graph algorithms library. Provides battle-tested BFS, shortest path, and traversal primitives. More reliable than rolling custom graph code.

**SQLite (modernc.org/sqlite)** - Embedded database eliminates deployment complexity. CGO-free pure Go driver enables static binary compilation. Handles 100K+ nodes efficiently with proper indexing. For larger graphs, export to Neo4j.

**OPA (Open Policy Agent)** - Industry-standard policy engine. Rego policies are testable, version-controlled, and portable. Separates policy logic from application code.

**GraphQL API** - Graph data naturally maps to GraphQL's nested query model. Single endpoint reduces API surface area. Strongly typed schema prevents client-server drift.

**React + Cytoscape.js** - Cytoscape.js specializes in graph visualization with layout algorithms tuned for directed graphs. React provides component reusability for UI controls.

**Offline-First Architecture** - HTTP transport wrapper blocks non-localhost requests by default. Enables deterministic testing, prevents data exfiltration, and meets air-gap requirements.

## Prerequisites

- **Go** 1.24+ (backend) - required for building from source
- **Node.js** 18+ (UI) - only needed for local UI development
- **Docker Desktop** - must be running before `docker compose` commands
- **Docker Compose** v2+ (included with Docker Desktop)
- **golangci-lint** (optional, for linting)
- **gosec** (optional, for security scanning)

## Quick Start (5-Minute Demo)

### 1. Clone and Build

```bash
git clone <repo-url>
cd Cloud-native-security-policy-visualizer

# Install Go dependencies
go mod download

# Build binaries
make build
```

### 2. Run Demo Ingestion

```bash
# Ingest sample data into two snapshots
make demo

# Expected output:
# - demo1: baseline snapshot with AWS + K8s data
# - demo2: snapshot with Terraform plan showing permission expansion
```

### 3. Explore CLI

```bash
# List snapshots
./bin/accessgraph-cli snapshots ls

# View findings
./bin/accessgraph-cli findings --snapshot demo1

# Expected findings:
# - IAM.WildcardAction (MEDIUM)
# - IAM.CrossAccountAssumeRole (HIGH)
# - K8s.ClusterAdminBinding (HIGH)

# Find path from principal to resource
./bin/accessgraph-cli graph path \
  --from "arn:aws:iam::111111111111:role/DevRole" \
  --to "arn:aws:s3:::data-bkt"

# 🆕 Phase 2: Attack path analysis with exports
./bin/accessgraph-cli attack-path \
  --from "arn:aws:iam::111111111111:role/DevRole" \
  --to "arn:aws:s3:::data-bkt" \
  --out attack-path.md \
  --sarif findings.sarif

# 🆕 Phase 2: Get least-privilege recommendations
./bin/accessgraph-cli recommend \
  --snapshot demo1 \
  --policy "arn:aws:iam::aws:policy/PowerUserAccess" \
  --out recommendations.json

# 🆕 Phase 2: Export to Neo4j
./bin/accessgraph-cli graph export \
  --snapshot demo1 \
  --format cypher \
  --out graph.cypher

# Compare snapshots
make demo-diff
```

### 4. Start Web UI

> **Important**: Run `make demo` first to populate the database. The Docker containers read from `data/graph.db` which is created by the demo target.

> **Note**: Ensure Docker Desktop is running before executing `docker compose`.

```bash
# Start all services (OPA, API, UI)
docker compose up --build

# Access the UI:
# - UI: http://localhost:3000
# - API: http://localhost:8080
# - GraphQL Playground: http://localhost:8080/ (only in DEV mode)
# - OPA: http://localhost:8181
```

**Verify Everything Works:**

Test the API with curl:

```bash
# Check API health
curl http://localhost:8080/healthz
# Expected: OK

# Query snapshots via GraphQL
curl -s -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "{ snapshots { id createdAt } }"}' | jq
```

Expected response: Two snapshots (`demo1`, `demo2`) with timestamps.

> **Note**: GraphQL Playground is disabled by default for security. To enable it for development, set `DEV=true` in `docker-compose.yml` and restart.

### 5. Use the UI

1. **Search**: Navigate to http://localhost:3000, type "DevRole" in the search box and press Enter
2. **Graph View**: Click a search result to visualize the node and its neighbors
3. **Find Attack Path**: 
   - In the Graph View, enter a target resource ID in the "Find Path" text box
   - Try: `arn:aws:s3:::data-bkt`
   - Click "Find Path" to highlight the attack path in purple
4. **Findings**: Click "Findings" in the nav bar to see all policy violations
5. **Snapshots**: Click "Snapshots" to compare snapshots and see added/removed edges

**Quick Attack Path Demo:**

Navigate directly to this URL to see the DevRole graph:
```
http://localhost:3000/graph/arn%3Aaws%3Aiam%3A%3A111111111111%3Arole%2FDevRole
```

Then enter `arn:aws:s3:::data-bkt` in the "Find Path" input and click the button to see the highlighted attack path showing how DevRole can access the S3 bucket.

**Screenshot Guidance**: For portfolio presentations, capture:
- Search page with results (shows data ingestion works)
- Graph view with path highlighted in purple (shows attack path visualization)
- Findings page with severity badges (shows OPA integration)
- Snapshot diff view with added/removed edges (shows temporal analysis)

### 6. Deploy to Kubernetes (Optional)

For production deployments with Kubernetes:

```bash
# Install via Helm
helm install accessgraph ./deployments/helm/accessgraph \
  --namespace accessgraph \
  --create-namespace \
  --set offline=true

# Access via port-forward
kubectl port-forward -n accessgraph svc/accessgraph-ui 8081:80

# Check health
curl http://localhost:8081/healthz
```

See [Helm Chart README](deployments/helm/accessgraph/README.md) for full deployment options.

## Development

### Build Commands

```bash
make build       # Build all binaries
make test        # Run tests with coverage
make lint        # Run linter
make sec         # Run security scanner
make ui          # Build UI
make generate    # Generate GraphQL code
make fix         # Fix dependency issues
make clean       # Clean build artifacts
```

### Troubleshooting

If you encounter build or linter errors, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for solutions.

### Run Tests

```bash
# Run all tests with coverage
make test

# Coverage must be ≥70% to pass CI gate
```

**What's Tested:**
- Parsers: AWS IAM JSON, Kubernetes RBAC YAML, Terraform plan JSON
- Graph algorithms: BFS shortest path, neighbor queries, attack path enumeration
- SQLite persistence: Round-trip snapshot save/load, edge comparison for diffs
- Policy evaluation: OPA input building, finding generation
- Offline mode: Network egress blocking for non-localhost requests
- Log redaction: ARN and secret pattern masking
- GraphQL resolvers: 5 resolver tests with mock DataStore and PolicyEvaluator interfaces
- Export formats: Markdown, SARIF v2.1.0, Neo4j Cypher (golden file tests)

### Offline Mode

By default, `OFFLINE=true` is set, which blocks all non-localhost HTTP requests. This ensures:

- No accidental network egress
- Deterministic behavior
- Safe for air-gapped environments

To test offline enforcement:

```bash
OFFLINE=true go test ./internal/config/...
```

## Configuration

### Environment Variables

| Variable              | Default                                  | Description                               |
|-----------------------|------------------------------------------|-------------------------------------------|
| `OFFLINE`             | `true`                                   | Enable offline mode (block network egress)|
| `OPA_URL`             | `http://localhost:8181/v1/data/accessgraph` | OPA endpoint                           |
| `SQLITE_PATH`         | `./graph.db`                             | SQLite database path                      |
| `PORT`                | `8080`                                   | API server port                           |
| **Phase 2 Additions** |                                          |                                           |
| `LOG_FORMAT`          | `text`                                   | Log format (`text` or `json`)             |
| `READ_TIMEOUT`        | `15s`                                    | HTTP read timeout                         |
| `WRITE_TIMEOUT`       | `15s`                                    | HTTP write timeout                        |
| `IDLE_TIMEOUT`        | `60s`                                    | HTTP idle timeout                         |
| `DEV`                 | `false`                                  | Enable dev mode (CORS for localhost)      |
| `CORS_ALLOWED_ORIGINS`| `""`                                     | Comma-separated allowed CORS origins      |

### Security Hardening (Phase 2)

AccessGraph now includes enhanced security features:

- **IMDS Blocking**: AWS metadata service (169.254.169.254) is **always** blocked, even when `OFFLINE=false`
- **RFC1918 Egress Control**: External network access to private ranges blocked when `OFFLINE=true`
- **Request Limits**: 10MB max request body size
- **Graceful Shutdown**: 30-second grace period on SIGTERM/SIGINT

## GraphQL API Examples

Access GraphQL Playground at http://localhost:8080/ when API is running. Here are common queries:

### Find Attack Paths

```graphql
query AttackPath {
  attackPath(
    from: "arn:aws:iam::111111111111:role/DevRole"
    to: "arn:aws:s3:::data-bkt"
    maxHops: 8
  ) {
    nodes {
      id
      kind
      labels
    }
    edges {
      from
      to
      kind
    }
  }
}
```

### Search for Principals

```graphql
query SearchPrincipals {
  searchPrincipals(query: "DevRole", limit: 10) {
    id
    kind
    labels
    props {
      key
      value
    }
  }
}
```

### Get Policy Findings

```graphql
query Findings {
  findings(snapshotId: "demo1") {
    id
    ruleId
    severity
    entityRef
    reason
    remediation
  }
}
```

### Get Node with Neighbors

```graphql
query NodeDetails {
  node(id: "arn:aws:iam::111111111111:role/DevRole") {
    id
    kind
    labels
    props {
      key
      value
    }
    neighbors(kinds: ["ATTACHED_POLICY", "ASSUMES_ROLE"]) {
      id
      kind
      labels
      edgeKind
    }
  }
}
```

### Get Least-Privilege Recommendations

```graphql
query Recommendations {
  recommend(
    snapshotId: "demo1"
    policyId: "arn:aws:iam::aws:policy/PowerUserAccess"
    cap: 20
  ) {
    policyId
    suggestedActions
    suggestedResources
    patchJson
    rationale
  }
}
```

### Compare Snapshots

```graphql
query SnapshotDiff {
  snapshotDiff(a: "demo1", b: "demo2") {
    addedEdges {
      from
      to
      kind
    }
    removedEdges {
      from
      to
      kind
    }
    summary {
      added
      removed
      changed
    }
  }
}
```

## Data Contracts

### Node Types

- **PRINCIPAL**: AWS IAM Role/User, K8s ServiceAccount
- **ROLE**: K8s Role/ClusterRole
- **POLICY**: AWS IAM Policy
- **PERMISSION**: Specific action (e.g., `s3:GetObject`)
- **RESOURCE**: AWS resource (e.g., S3 bucket)
- **NAMESPACE**: Kubernetes namespace
- **ACCOUNT**: AWS account

### Edge Types

- **ASSUMES_ROLE**: Principal → Role
- **TRUSTS_CROSS_ACCOUNT**: Role → Account
- **ATTACHED_POLICY**: Role → Policy
- **ALLOWS_ACTION**: Policy → Permission
- **APPLIES_TO**: Permission → Resource
- **BINDS_TO**: Role → Principal (K8s)
- **IN_NAMESPACE**: Principal/Resource → Namespace

## OPA Policy Rules

1. **IAM.WildcardAction** (MEDIUM): Detects policies with wildcard (`*`) actions
2. **IAM.CrossAccountAssumeRole** (HIGH): Detects cross-account trust relationships
3. **K8s.ClusterAdminBinding** (HIGH): Detects cluster-admin role bindings

## CI/CD

GitHub Actions workflow includes:

- **Build/Test**: Go 1.22.x and 1.23.x matrix
- **Linting**: golangci-lint with strict rules
- **Coverage Gate**: Fails if coverage < 70%
- **Frontend Build**: Node.js build and artifact upload
- **Container Build**: Docker image build with caching
- **Security Scan**: Trivy scan on API image
- **Release**: Automatic binary builds and GitHub releases on tags

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.

## Roadmap

### Phase 1 (Completed)
- Multi-cloud ingestion (AWS IAM, K8s RBAC, Terraform)
- Graph-based analysis with SQLite persistence
- OPA policy evaluation
- React UI with Cytoscape.js visualization
- Offline-first architecture

### Phase 2 (Completed - v1.1.0)
- Attack path enumeration with markdown/SARIF export
- Least-privilege recommender with JSON Patch
- Neo4j Cypher export
- Production Helm chart
- Observability (metrics, health checks)
- Enhanced security hardening

### 🔜 Phase 3 (Planned)
- GCP IAM and Azure RBAC support
- Real-time policy change monitoring
- Advanced UI with interactive attack path visualization
- ML-based anomaly detection

### 🔮 Future
- Policy remediation automation
- Multi-tenant SaaS deployment
- Cloud-native connectors (AWS/Azure/GCP APIs)
- Risk scoring and compliance frameworks

## Future Improvements & Known Trade-Offs

**What I'd Do Differently with More Time:**

1. **Replace SQLite with Native Graph Database for Scale** - SQLite works well for <100K nodes, but larger enterprise graphs (1M+ nodes) would benefit from Neo4j or Dgraph. Trade-off: Deployment complexity increases (external database vs embedded). Current approach prioritizes easy demos over massive scale.

2. **Add GraphQL Subscriptions for Real-Time Policy Changes** - Current snapshot-based model requires manual re-ingestion. WebSocket subscriptions could push updates when policies change. Trade-off: Real-time monitoring requires persistent connections and complicates offline-first architecture.

3. **Implement Policy Simulation Engine** - "What-if" analysis to preview impact of policy changes before applying them. Would require symbolic execution of Rego rules. Trade-off: Significant complexity increase, diminishing returns for MVP use case.

4. **Add Horizontal Pod Autoscaling to Helm Chart** - Current chart supports single-replica deployment. HPA based on CPU/memory or custom metrics (GraphQL query rate) would enable auto-scaling. Trade-off: SQLite doesn't support concurrent writes from multiple replicas. Would need to migrate to PostgreSQL or add read replicas.

5. **Build Interactive Attack Path Modal in React UI** - CLI and GraphQL API support attack path analysis, but UI only shows basic graph view. Modal with step-by-step path visualization would improve UX. Trade-off: UI development time vs backend feature development.

6. **Add Caching Layer for Expensive Graph Queries** - Repeated BFS traversals on large graphs are CPU-intensive. Redis cache for common queries (top 10 attack paths, sensitive resource inventory) would reduce latency. Trade-off: Cache invalidation complexity when snapshots change.

**Architectural Decisions I'm Confident About:**

- **Offline-first design** - Enables deterministic testing, meets compliance requirements, simplifies deployment
- **Interface-driven testing** - Testability improvements directly correlated with interface adoption
- **OPA for policies** - Externalizing policy logic enables security teams to update rules without code changes
- **GraphQL over REST** - Graph queries map naturally to GraphQL's nested model
- **Go for backend** - Concurrency primitives, static binaries, and cross-compilation justified the choice

## Key Technical Challenges & Solutions

### Challenge 1: BFS Depth Tracking Bug in Multi-Path Graphs

**Problem**: Initial BFS implementation failed to track depth correctly when multiple paths existed to the same node. This caused incorrect hop counts in attack path analysis, leading to false positives in shortest path calculations.

**Solution**: Rewrote BFS to use a visited map that stores depth at first encounter. Added early termination when reaching sensitive resources. Validated with golden file tests covering 8-hop paths with multiple routes.

**What I Learned**: Graph algorithm correctness is subtle. Integration tests with real-world graph topologies (AWS trust chains, K8s namespace boundaries) caught edge cases unit tests missed.

### Challenge 2: Making GraphQL Resolvers Testable

**Problem**: Original resolver code directly instantiated SQLite stores and OPA HTTP clients, making unit testing impossible without spinning up external services.

**Solution**: Introduced `DataStore` and `PolicyEvaluator` interfaces. Resolvers now accept interfaces, enabling mock implementations in tests. Added 5 resolver tests with in-memory mocks that verify query logic without I/O.

**What I Learned**: Dependency injection isn't just for Java. Go interfaces make testing orders of magnitude easier. The refactor took 2 hours but saved days of flaky integration test debugging.

### Challenge 3: Race Conditions in In-Memory Graph Cache

**Problem**: Concurrent GraphQL requests caused data races when loading graphs from SQLite into memory. `-race` detector caught panics under load testing.

**Solution**: Added `sync.RWMutex` to protect the graph cache map. Read-heavy workload benefits from RWMutex over Mutex (multiple readers allowed). Added edge index for O(1) neighbor lookups instead of O(E) scans.

**What I Learned**: Concurrency bugs are silent killers. Always run `go test -race` in CI. The performance win from parallel reads justified the added complexity.

### Challenge 4: TypeScript `any` Types Hiding Frontend Bugs

**Problem**: React frontend used `any` types everywhere, hiding null pointer bugs until runtime. Cytoscape.js integration was particularly fragile.

**Solution**: Replaced all `any` with proper interfaces. Created `CytoscapeNode`, `CytoscapeEdge`, and `GraphData` types. Added null checks and type guards. TypeScript compiler now catches errors at build time.

**What I Learned**: TypeScript's value is proportional to how strictly you use it. Incrementally typing a codebase is tedious but pays off immediately in reduced runtime errors.

### Challenge 5: Docker Compose Platform-Specific Build Failures

**Problem**: Docker Compose builds failed on Apple Silicon with vague errors. Deprecated `version` key caused warnings. Invalid `platform` flags on services broke builds.

**Solution**: Removed deprecated `version: "3.8"` key. Removed invalid `platform` flags from service definitions (only valid in `build` context). Builds now succeed on both amd64 and arm64.

**What I Learned**: Docker Compose v2 semantics differ from v1. Read deprecation warnings carefully. Platform-specific flags belong in build context, not service definitions.

## What I Learned Building This

**Graph Algorithms in Practice**: Textbook BFS is straightforward, but production graph analysis requires sensitivity-aware traversal, hop limits, and deterministic output for reproducible audits. Real-world access graphs have cycles (role assumption loops) that break naive implementations.

**OPA Policy Testing**: Rego policies need unit tests just like application code. `opa test` catches logic bugs early. Structured policy input (nodes, edges) enables complex relationship queries without embedding graph logic in Rego.

**Offline-First Architecture**: Blocking network egress by default forces you to think about dependencies upfront. Custom HTTP transport wrappers are surprisingly simple (20 LOC) and enable deterministic testing. Air-gap compliance becomes a feature, not an afterthought.

**GraphQL for Graph Data**: GraphQL's nested query model naturally represents graph traversal. A single `node { neighbors { node { ... } } }` query replaces multiple REST calls. But schema design matters - polymorphic nodes (PRINCIPAL vs RESOURCE) need union types.

**Interface-Driven Testing**: Go's implicit interfaces enable incremental testability improvements. Adding interfaces to existing code doesn't require changing callers. Mock implementations are trivial (20-30 LOC for a store mock). Coverage jumped from 60% to 75% after refactoring to interfaces.

**TypeScript Adoption Strategy**: Typing an existing JavaScript codebase incrementally works. Start with data layer (API types), then components (props), then event handlers (callbacks). The compiler finds bugs you didn't know existed.

## Documentation

Comprehensive documentation for Phase 1:

- **[Implementation Summary](docs/implementation_summary.md)** - Detailed technical overview of all components
- **[Delivery Checklist](docs/delivery_checklist.md)** - Complete verification of Phase 1 requirements
- **[Phase 1 Status](docs/status_phase1.md)** - Current capabilities, limitations, and next steps
- **[Troubleshooting Guide](TROUBLESHOOTING.md)** - Solutions for common build and runtime issues
- **[Contributing Guide](CONTRIBUTING.md)** - How to contribute to the project
- **[Changelog](CHANGELOG.md)** - Version history and release notes

These documents are maintained in the repository to support onboarding, audits, and reviews.

## Contributing

This is a Phase 1 MVP. Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Ensure tests pass (`make test`)
4. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.

## Support

For issues, questions, or feature requests, please open a GitHub issue.

---

**Current Version**: v1.1.0 (Phase 2 - Production Ready)  
**Status**: Production-grade implementation with Kubernetes support  
**Mode**: OFFLINE by default (configurable). Full attack path analysis and policy recommendations available.

