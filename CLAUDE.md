# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Important: Always Reference Documentation

**ALWAYS read and reference these files when working on this codebase:**

1. **docs/DEVELOPMENT.md** - Required reading for all development tasks
   - Contains project structure, architecture details, and development workflow
   - Explains how components work together
   - Provides debugging tips and best practices
   - **Read this file before making any code changes**

2. **docs/USAGE.md** - Reference when changes affect user-facing features
   - Understand how users interact with the project
   - Ensure changes don't break documented behavior

3. **docs/IMPLEMENTATION_SUMMARY.md** - Technical implementation details
   - Phase 1 implementation status
   - Architecture flow and component interactions
   - Test coverage and success criteria

When asked to implement features, fix bugs, or make changes:
1. First read docs/DEVELOPMENT.md to understand the codebase structure
2. Review relevant sections based on the task
3. Follow the established patterns and conventions
4. Update documentation when making user-facing changes

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

- **Language**: Go 1.25+
- **Framework**: Standard Kubernetes client-go and controller-runtime (to be added)
- **Build**: Multi-stage Dockerfile with distroless base image
- **Deployment**: Standard Kubernetes manifests in config/

## Related Projects

When implementing features, be aware of these similar projects for reference:

- **Mittens (Expedia)**: Sidecar-based warmup tool for HTTP/gRPC applications
- **AWS Load Balancer Controller**: Example of readiness gate + mutating webhook pattern
- **BlaBlaCar's warmup solution**: Startup probe-based warmup (internal, not open source)

Key differentiator: kube-booster aims to be a cluster-wide controller solution rather than requiring per-deployment sidecar configuration.

## Working with This Codebase

### Before Starting Any Task

1. **Read docs/DEVELOPMENT.md** - Understand the project structure and development workflow
2. Review the relevant package documentation:
   - Working on webhook? See `pkg/webhook/` section in docs/DEVELOPMENT.md
   - Working on controller? See `pkg/controller/` section in docs/DEVELOPMENT.md
   - Working on main entry point? See `cmd/controller/` section in docs/DEVELOPMENT.md

### When Making Changes

**For new features:**
- Check docs/DEVELOPMENT.md → "Adding New Features" section
- Follow established code patterns
- Add tests with good coverage
- Update docs/USAGE.md if user-facing

**For bug fixes:**
- Check docs/DEVELOPMENT.md → "Debugging" section
- Understand the component architecture first
- Add regression tests
- Update troubleshooting docs if applicable

**For refactoring:**
- Review docs/DEVELOPMENT.md → "Architecture Overview"
- Ensure changes maintain existing behavior
- Run full test suite
- Update architecture docs if needed

### Testing Your Changes

Always follow the workflow in docs/DEVELOPMENT.md:
1. Run unit tests: `make test`
2. Run linter: `make lint`
3. Test in kind cluster (see docs/DEVELOPMENT.md → "Local Development with kind")
4. Run smoke tests: `./hack/quick_test.sh`

### Documentation Updates

When you make changes that affect:
- **User behavior** → Update docs/USAGE.md
- **Development workflow** → Update docs/DEVELOPMENT.md
- **Architecture** → Update CLAUDE.md and docs/IMPLEMENTATION_SUMMARY.md
- **New annotations/config** → Update all three docs

### Key Principles

1. **Follow existing patterns**: This codebase uses controller-runtime patterns extensively
2. **Test thoroughly**: Maintain >80% coverage for new code
3. **Document changes**: Keep docs in sync with code
4. **Security first**: Follow security guidelines in DEVELOPMENT.md
5. **Performance matters**: Keep webhook <100ms, controller efficient

### Quick Reference

| Task | Primary Reference |
|------|------------------|
| Setting up dev environment | docs/DEVELOPMENT.md → "Development Environment Setup" |
| Understanding architecture | docs/DEVELOPMENT.md → "Architecture Overview" |
| Adding new features | docs/DEVELOPMENT.md → "Adding New Features" |
| Running tests | docs/DEVELOPMENT.md → "Running Tests" |
| Debugging issues | docs/DEVELOPMENT.md → "Debugging" |
| User-facing changes | docs/USAGE.md |
| Phase 2 planning | CLAUDE.md (this file) + docs/IMPLEMENTATION_SUMMARY.md |
