# Changelog

All notable changes to AccessGraph will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2024-10-08

### Added
- Initial Phase 1 MVP release
- AWS IAM parser (roles, policies, trust relationships, attachments)
- Kubernetes RBAC parser (ServiceAccounts, Roles, RoleBindings, NetworkPolicies)
- Terraform plan parser (optional IaC tagging)
- Directed multigraph engine using gonum
- SQLite persistence for graph snapshots
- OPA policy evaluation with 3 rules:
  - IAM.WildcardAction (MEDIUM)
  - IAM.CrossAccountAssumeRole (HIGH)
  - K8s.ClusterAdminBinding (HIGH)
- GraphQL API with chi router
- React UI with Cytoscape.js visualization
- CLI tools:
  - `accessgraph-ingest`: Ingest data from local sources
  - `accessgraph-api`: Run GraphQL API server
  - `accessgraph-cli`: Query snapshots, findings, paths, diffs
- Docker Compose orchestration (OPA, API, UI)
- Offline mode with network egress blocking
- Log redaction for sensitive data (ARNs, account IDs, secrets)
- GitHub Actions CI with:
  - Multi-version Go testing (1.22.x, 1.23.x)
  - 70% coverage gate
  - golangci-lint integration
  - Trivy container scanning
  - Automated releases
- Comprehensive unit tests (>70% coverage)
- Documentation (README, LICENSE, 5-minute demo)

### Security
- Offline mode prevents accidental data exfiltration
- Read-only capabilities (no modification of source systems)
- Sensitive data redaction in logs

[Unreleased]: https://github.com/jamesolaitan/accessgraph/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/jamesolaitan/accessgraph/releases/tag/v1.0.0

