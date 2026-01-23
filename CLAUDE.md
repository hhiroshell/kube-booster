# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

kube-booster is a Kubernetes custom controller that ensures smooth application launches by sending warmup requests to application endpoints before pods transition to READY state. The project helps reduce cold start issues and improves application readiness by pre-warming endpoints.

## Architecture

This controller is designed to intercept the Kubernetes pod readiness process using the following approach:

1. **Watch pods** with warmup annotations
2. **Send warmup requests** to application endpoints before marking pods as READY
3. **Use Kubernetes readiness gates** to control when pods become READY
4. **Implement a mutating webhook** to inject readiness gate configuration into pods

### Opt-In Mechanism

Application owners control warmup behavior through **pod annotations**. This provides a simple, declarative way to enable/disable warmup without requiring additional resources.

#### Phase 1: Annotation-Based Configuration (Current Implementation Target)

**Enable warmup with annotations:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        # Enable warmup
        kube-booster.io/warmup: "enabled"

        # Configuration (optional, uses defaults if not specified)
        kube-booster.io/warmup-endpoint: "http://localhost:8080/warmup"
        kube-booster.io/warmup-requests: "5"
        kube-booster.io/warmup-timeout: "30s"
```

**Safety Defaults (Phase 1):**
- Default timeout: 30s
- Default requests: 3
- Default endpoint: Falls back to readiness probe path if not specified
- Fail-open behavior: If warmup fails after retries, still mark pod as ready (with warning event/log)

**Opt-out:**
Pods without the `kube-booster.io/warmup: "enabled"` annotation are not affected by the controller.

#### Phase 2: CRD for Complex Configuration (Future)

For advanced warmup scenarios, a `WarmupConfig` CRD will be introduced:
```yaml
apiVersion: kube-booster.io/v1alpha1
kind: WarmupConfig
metadata:
  name: my-app-warmup
spec:
  selector:
    matchLabels:
      app: my-app
  warmup:
    requests:
      - endpoint: /api/cache/load
        method: POST
        body: '{"action": "preload"}'
      - endpoint: /api/health
        method: GET
        expectedStatus: 200
    successThreshold: 3
    timeout: 60s
```

Reference it via annotation:
```yaml
annotations:
  kube-booster.io/warmup-config: "my-app-warmup"
```

### Key Components

- `cmd/controller/`: Controller entry point with command-line flags
- `pkg/controller/`: Core controller logic for watching pods and managing readiness gates
- `pkg/warmup/`: Warmup request execution logic (HTTP/gRPC)
- `pkg/webhook/`: Mutating webhook for injecting readiness gates
- `config/crd/`: Custom Resource Definitions (Phase 2)
- `config/rbac/`: RBAC roles and bindings for controller permissions
- `config/webhook/`: Webhook configuration and certificates
- `config/samples/`: Example configurations for users

### Implementation Pattern

The controller follows the standard Kubernetes controller pattern with readiness gates, similar to AWS Load Balancer Controller and GKE Ingress Controller:

1. **Mutating webhook** inspects pod creation events
2. If pod has `kube-booster.io/warmup: "enabled"`, inject readiness gate: `kube-booster.io/warmup-ready`
3. **Controller** watches pods with the injected readiness gate
4. Controller sends warmup requests based on annotations (Phase 1) or WarmupConfig (Phase 2)
5. Controller updates pod condition `kube-booster.io/warmup-ready` to `True` when warmup completes
6. Kubernetes marks pod as READY only after all readiness gates are satisfied

## Development Commands

### Building and Running

```bash
# Build controller binary
make build

# Run locally (requires kubeconfig)
make run

# Format and vet code before building
make fmt vet
```

### Testing

```bash
# Run all tests with coverage
make test

# Run linter (golangci-lint)
make lint

# Run a single test package
go test ./pkg/controller/... -v

# Run specific test
go test ./pkg/controller/... -run TestSpecificFunction -v
```

### Docker

```bash
# Build Docker image (default tag: controller:latest)
make docker-build

# Build with custom image tag
make docker-build IMG=your-registry/kube-booster:v1.0.0

# Push image
make docker-push IMG=your-registry/kube-booster:v1.0.0
```

### Kubernetes Deployment

```bash
# Deploy to cluster (uses ~/.kube/config)
make deploy

# Remove from cluster
make undeploy
```

### Cleanup

```bash
# Remove build artifacts
make clean
```

## Technology Stack

- **Language**: Go 1.23+
- **Framework**: Standard Kubernetes client-go and controller-runtime (to be added)
- **Build**: Multi-stage Dockerfile with distroless base image
- **Deployment**: Standard Kubernetes manifests in config/

## Related Projects

When implementing features, be aware of these similar projects for reference:

- **Mittens (Expedia)**: Sidecar-based warmup tool for HTTP/gRPC applications
- **AWS Load Balancer Controller**: Example of readiness gate + mutating webhook pattern
- **BlaBlaCar's warmup solution**: Startup probe-based warmup (internal, not open source)

Key differentiator: kube-booster aims to be a cluster-wide controller solution rather than requiring per-deployment sidecar configuration.
