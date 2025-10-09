# AccessGraph Helm Chart

Official Helm chart for deploying AccessGraph on Kubernetes.

## Features

- **Offline-first by default** - Network egress blocked, IMDS protection
- **Security hardened** - Non-root containers, read-only filesystems, network policies
- **Production-ready** - Health probes, resource limits, graceful shutdown
- **Observability** - Prometheus metrics, structured logging
- **Flexible deployment** - Ingress support, configurable resources

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PV provisioner support (optional, for persistence)

## Quick Start

### Install with default settings (offline mode)

```bash
helm install accessgraph ./deployments/helm/accessgraph \
  --namespace accessgraph \
  --create-namespace
```

### Install with custom values

```bash
helm install accessgraph ./deployments/helm/accessgraph \
  --namespace accessgraph \
  --create-namespace \
  --set offline=false \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=accessgraph.example.com
```

### Access the application

```bash
# Port forward to UI
kubectl port-forward -n accessgraph svc/accessgraph-ui 8081:80

# Port forward to API
kubectl port-forward -n accessgraph svc/accessgraph-api 8080:8080

# Visit http://localhost:8081 for UI
# Visit http://localhost:8080 for GraphQL Playground
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `offline` | Enable offline mode (blocks network egress) | `true` |
| `opa.enabled` | Enable OPA policy engine | `true` |
| `networkPolicy.enabled` | Enable Kubernetes network policies | `true` |
| `persistence.enabled` | Use persistent volume for data | `false` |
| `ingress.enabled` | Enable ingress controller | `false` |
| `api.resources.requests.cpu` | API CPU request | `100m` |
| `api.resources.requests.memory` | API memory request | `128Mi` |
| `api.resources.limits.cpu` | API CPU limit | `500m` |
| `api.resources.limits.memory` | API memory limit | `512Mi` |

### Example: Production Deployment

```yaml
# values-production.yaml
replicaCount: 2

offline: true

api:
  env:
    LOG_FORMAT: "json"
    CORS_ALLOWED_ORIGINS: "https://dashboard.example.com"
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "1000m"
      memory: "1Gi"

persistence:
  enabled: true
  storageClass: "fast-ssd"
  size: 10Gi

ingress:
  enabled: true
  className: "nginx"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
  hosts:
    - host: accessgraph.example.com
      paths:
        - path: /
          pathType: Prefix
          backend: ui
        - path: /query
          pathType: Prefix
          backend: api
  tls:
    - secretName: accessgraph-tls
      hosts:
        - accessgraph.example.com

nodeSelector:
  workload: compute

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: accessgraph
          topologyKey: kubernetes.io/hostname
```

Deploy with:

```bash
helm upgrade --install accessgraph ./deployments/helm/accessgraph \
  -n accessgraph \
  -f values-production.yaml
```

## Security Features

### Network Security

- **IMDS blocking**: AWS metadata service (169.254.169.254) always blocked
- **Offline mode**: External network egress blocked by default
- **Network policies**: Strict pod-to-pod communication rules
- **TLS support**: Via ingress controller

### Container Security

- **Non-root containers**: All containers run as non-root users
- **Read-only filesystem**: Root filesystems are read-only
- **No privilege escalation**: `allowPrivilegeEscalation: false`
- **Capability dropping**: All capabilities dropped
- **Security contexts**: Full security context configuration

### Runtime Security

- **Resource limits**: CPU and memory limits enforced
- **Health probes**: Liveness, readiness, and startup probes
- **Graceful shutdown**: 30-second grace period for clean termination

## Monitoring

### Prometheus Metrics

Metrics are exposed on `/metrics` endpoint:

```yaml
# ServiceMonitor example (if using Prometheus Operator)
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: accessgraph
spec:
  selector:
    matchLabels:
      app: accessgraph
      component: api
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Available Metrics

- `accessgraph_healthy` - Service health status (1 = healthy)
- `accessgraph_ready` - Service readiness status (1 = ready)
- `accessgraph_info` - Service version information

### Health Endpoints

- `/healthz` - Combined health check
- `/healthz/live` - Liveness probe
- `/healthz/ready` - Readiness probe

## Upgrading

### Upgrade to latest version

```bash
helm upgrade accessgraph ./deployments/helm/accessgraph \
  -n accessgraph \
  --reuse-values
```

### Rollback

```bash
helm rollback accessgraph -n accessgraph
```

## Uninstalling

```bash
helm uninstall accessgraph -n accessgraph
```

## Troubleshooting

### Check pod status

```bash
kubectl get pods -n accessgraph
```

### View logs

```bash
# API logs
kubectl logs -f -n accessgraph -l component=api

# UI logs
kubectl logs -f -n accessgraph -l component=ui

# OPA logs
kubectl logs -f -n accessgraph -l component=opa
```

### Check health

```bash
kubectl port-forward -n accessgraph svc/accessgraph-api 8080:8080
curl http://localhost:8080/healthz
```

### Debug network policies

```bash
# Describe network policies
kubectl describe networkpolicy -n accessgraph

# Test connectivity
kubectl run -it --rm debug --image=busybox -n accessgraph -- wget -O- http://accessgraph-api:8080/healthz
```

## Development

### Template rendering

```bash
# Render templates locally
helm template accessgraph ./deployments/helm/accessgraph \
  --namespace accessgraph \
  --set offline=true \
  --debug

# Lint chart
helm lint ./deployments/helm/accessgraph
```

### Local testing with kind

```bash
# Create kind cluster
kind create cluster --name accessgraph-test

# Install chart
helm install accessgraph ./deployments/helm/accessgraph \
  -n accessgraph \
  --create-namespace \
  --wait

# Test
kubectl port-forward -n accessgraph svc/accessgraph-ui 8081:80
```

## Support

- **Documentation**: https://github.com/jamesolaitan/accessgraph
- **Issues**: https://github.com/jamesolaitan/accessgraph/issues
- **License**: Apache 2.0

## Chart Versioning

This chart follows semantic versioning:
- Chart version: 1.1.0
- App version: 1.1.0

For changes and upgrade notes, see [CHANGELOG](../../../CHANGELOG.md).

