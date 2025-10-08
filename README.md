# AccessGraph - Cloud Native Security Policy Visualizer

AccessGraph is an offline-capable graph-based security policy analyzer for AWS IAM and Kubernetes RBAC. It ingests policy definitions, builds a directed graph of relationships, evaluates policies with OPA, and provides a visual interface for exploring access paths and security findings.

## Features

- **Multi-Cloud Support**: Parse AWS IAM (roles, policies, trust relationships) and Kubernetes RBAC (ServiceAccounts, Roles, Bindings)
- **Graph Analysis**: Build and query a directed graph of principals, roles, policies, permissions, and resources
- **Policy Evaluation**: Detect security issues using OPA (wildcard actions, cross-account trust, cluster-admin bindings)
- **Visual Interface**: React-based UI with Cytoscape.js for graph visualization and path exploration
- **Snapshot Comparison**: Diff snapshots to track policy changes over time
- **Fully Offline**: No network egress required; works with local data sources and local OPA server

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

## Prerequisites

- **Go** 1.22+ (backend)
- **Node.js** 18+ (UI)
- **Docker** & **Docker Compose** (for containerized deployment)
- **golangci-lint** (for linting)
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

# Compare snapshots
make demo-diff
```

### 4. Start Web UI

```bash
# Start all services (OPA, API, UI)
docker compose up --build

# Access the UI:
# - UI: http://localhost:3000
# - API: http://localhost:8080
# - GraphQL Playground: http://localhost:8080/
# - OPA: http://localhost:8181
```

### 5. Use the UI

1. **Search**: Navigate to http://localhost:3000, search for "DevRole"
2. **Graph View**: Click a result to visualize the node and its neighbors
3. **Find Path**: Select a target resource, click "Find Path" to highlight the access path
4. **Findings**: View the Findings page to see all policy violations
5. **Snapshots**: Compare snapshots to see added/removed edges

## Development

### Build Commands

```bash
make build       # Build all binaries
make test        # Run tests with coverage
make lint        # Run linter
make sec         # Run security scanner
make ui          # Build UI
make generate    # Generate GraphQL code
make clean       # Clean build artifacts
```

### Run Tests

```bash
# Run all tests with coverage
make test

# Coverage must be ≥70% to pass CI gate
```

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

Environment variables:

| Variable       | Default                                  | Description                          |
|----------------|------------------------------------------|--------------------------------------|
| `OFFLINE`      | `true`                                   | Enable offline mode (block egress)   |
| `OPA_URL`      | `http://localhost:8181/v1/data/accessgraph` | OPA endpoint                        |
| `SQLITE_PATH`  | `./graph.db`                             | SQLite database path                 |
| `PORT`         | `8080`                                   | API server port                      |

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

## Roadmap (Future Phases)

- Phase 2: GCP IAM, Azure RBAC support
- Phase 3: Real-time live ingestion (AWS SDK, K8s API)
- Phase 4: Advanced analytics (anomaly detection, risk scoring)
- Phase 5: Policy remediation automation

## Documentation

Comprehensive documentation for Phase 1:

- **[Implementation Summary](docs/implementation_summary.md)** - Detailed technical overview of all components
- **[Delivery Checklist](docs/delivery_checklist.md)** - Complete verification of Phase 1 requirements
- **[Phase 1 Status](docs/status_phase1.md)** - Current capabilities, limitations, and next steps
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

**Phase 1 Status**: MVP complete; suitable for offline demos  
**Mode**: OFFLINE (no network egress). Capabilities: read-only.

