# Changelog

All notable changes to AccessGraph will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2025-10-09

### Added - Phase 2: Production-Grade Features

#### Attack Path Analysis
- **Attack path enumeration**: Find shortest paths from principals to sensitive resources
- **Sensitive resource tagging**: Mark high-value targets for prioritized analysis
- **Markdown export**: Professional attack path reports with risk assessment
- **SARIF v2.1.0 export**: Standards-compliant security findings for CI/CD integration
- **CLI commands**: `attack-path` with `--out` and `--sarif` flags
- **GraphQL queries**: `attackPath()`, `exportMarkdownAttackPath()`, `exportSarifAttackPath()`

#### Least-Privilege Recommender
- **Wildcard policy detection**: Identify overly permissive IAM/RBAC rules
- **Usage-based recommendations**: Suggest specific actions/resources based on observed paths
- **RFC 6902 JSON Patch**: Machine-readable policy updates
- **CLI command**: `recommend` with JSON output
- **GraphQL query**: `recommend()` with rationale and patch generation

#### Neo4j Integration
- **Cypher export**: Generate `.cypher` files with MERGE statements
- **Sample queries**: 15 pre-built queries for common analysis patterns
- **Constraints and indices**: Optimized for graph database import
- **CLI command**: `graph export --format cypher`
- **GraphQL query**: `exportCypher()`

#### Observability & Monitoring
- **Health endpoints**: `/healthz`, `/healthz/live`, `/healthz/ready`
- **Prometheus metrics**: `/metrics` with service health and readiness gauges
- **Graceful shutdown**: 30-second grace period with SIGTERM/SIGINT handling
- **Structured logging**: JSON format support via `LOG_FORMAT=json`
- **Request instrumentation**: Size limits (10MB), timeouts, CORS configuration

#### Security Enhancements
- **IMDS blocking**: AWS metadata service (169.254.169.254) always blocked
- **Enhanced offline mode**: RFC1918 egress blocking when `OFFLINE=true`
- **Configurable timeouts**: `READ_TIMEOUT`, `WRITE_TIMEOUT`, `IDLE_TIMEOUT`
- **HTTP hardening**: Request body size limits, graceful degradation
- **CORS security**: Dev-mode support, production lockdown

#### Kubernetes Deployment
- **Helm chart**: Production-ready chart with secure defaults
- **Network policies**: Strict pod-to-pod communication rules
- **Security contexts**: Non-root containers, read-only filesystems
- **Resource limits**: CPU/memory requests and limits
- **Health probes**: Liveness, readiness, and startup probes
- **Configurable ingress**: TLS support, path-based routing

#### Developer Experience
- **CLI enhancements**: `--format json|table` for all commands
- **Deterministic outputs**: Sorted results for reproducible analyses
- **Database indices**: Performance optimizations for large graphs
- **Sample data**: `sample/metadata/sensitive.yaml` for testing
- **Helm chart README**: Comprehensive deployment guide

### Changed

- **CLI output format**: Added `--format` flag for structured output
- **API endpoints**: Reorganized health checks under `/healthz/*`
- **Docker images**: Added healthchecks to API container
- **Query results**: All results now sorted for determinism

### Fixed

- **Database performance**: Added indices on `snapshot_id`, `kind`, and `id` columns
- **Memory usage**: Read-only root filesystems reduce attack surface
- **Network security**: Default-deny egress with explicit allow lists

### Security

- **IMDS protection**: Always blocks AWS metadata service, even in online mode
- **Offline enforcement**: Network egress strictly controlled
- **Container hardening**: All capabilities dropped, privilege escalation blocked
- **Network isolation**: Zero-trust networking with explicit policies

## [1.0.0] - 2025-10-01

### Added - Phase 1: MVP

#### Core Features
- **Multi-cloud ingestion**: AWS IAM (JSON), Kubernetes RBAC (YAML), Terraform (JSON)
- **Directed multigraph**: Gonum-based graph with SQLite persistence
- **Policy engine**: OPA integration with 3 pre-built rules
  - IAM wildcard actions
  - Cross-account assume role
  - Kubernetes cluster-admin bindings
- **GraphQL API**: Query principals, paths, findings, snapshots
- **React UI**: Search, graph visualization (Cytoscape.js), findings, snapshot diff
- **CLI tools**: Ingest, API server, query interface
- **Offline-first**: Network egress blocked by default
- **Docker Compose**: OPA, API, UI with proper networking

#### Development Infrastructure
- **GitHub Actions CI**: Lint, test, coverage (70%+ gate), Trivy security scan
- **Makefile**: Build, test, lint, demo, Docker targets
- **Documentation**: README, architecture notes, contribution guide
- **Sample data**: AWS roles/policies, K8s RBAC, Terraform plan

#### Security
- **Log redaction**: Sensitive data masked in logs
- **Offline mode**: External network access blocked
- **SQLite snapshots**: Point-in-time graph captures
- **Zero external dependencies**: Fully offline-capable

## [Unreleased]

### Planned - Future Enhancements
- **Real-time monitoring**: Live policy change detection
- **Multi-tenant support**: Isolated environments per organization
- **Advanced UI**: Interactive attack path visualization, remediation workflows
- **Cloud connectors**: Direct AWS/Azure/GCP API integration
- **ML-based anomaly detection**: Unusual access pattern identification
- **Export formats**: DOT, GEXF, JSON-LD for additional tooling

---

## Version Compatibility

| Version | Kubernetes | Helm | Go  | OPA   |
|---------|-----------|------|-----|-------|
| 1.1.0   | 1.19+     | 3.0+ | 1.22| 0.60+ |
| 1.0.0   | 1.19+     | N/A  | 1.22| 0.60+ |

## Upgrade Guide

### From 1.0.0 to 1.1.0

**Breaking Changes**: None - fully backward compatible

**New Features Available**:
1. Use `attack-path` CLI for attack path analysis
2. Deploy with Helm for production Kubernetes environments
3. Enable structured logging with `LOG_FORMAT=json`
4. Access Prometheus metrics at `/metrics`

**Recommended Actions**:
1. Update to Helm-based deployment for production
2. Enable network policies for enhanced security
3. Configure health probes in orchestration platforms
4. Review and apply least-privilege recommendations

**Migration Steps**:
```bash
# 1. Build new binaries
make build

# 2. Update Docker images
docker-compose pull

# 3. Restart services (graceful shutdown supported)
docker-compose restart

# 4. Verify health
curl http://localhost:8080/healthz
```

## Support

- **Issues**: https://github.com/jamesolaitan/accessgraph/issues
- **Documentation**: https://github.com/jamesolaitan/accessgraph
- **License**: Apache 2.0
