# AccessGraph - Cloud Native Security Policy Visualizer

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.19+-326CE5?logo=kubernetes)](https://kubernetes.io)

AccessGraph is a production-grade, offline-first graph-based security policy analyzer for AWS IAM and Kubernetes RBAC. It ingests policy definitions, builds a directed graph of relationships, evaluates policies with OPA, and provides attack path analysis with least-privilege recommendations.

**Latest Release**: v1.1.0 (Phase 2 - Production Ready)

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

### ✅ Phase 1 (Completed)
- Multi-cloud ingestion (AWS IAM, K8s RBAC, Terraform)
- Graph-based analysis with SQLite persistence
- OPA policy evaluation
- React UI with Cytoscape.js visualization
- Offline-first architecture

### ✅ Phase 2 (Completed - v1.1.0)
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

