# kube-booster Development Guide

This guide is for developers who want to contribute to kube-booster or build it from source.

## Development Environment Setup

### Prerequisites

- Go 1.25 or later
- Docker
- kubectl
- kind or minikube (for local testing)
- golangci-lint (for linting)

### Clone the Repository

```bash
git clone https://github.com/hhiroshell/kube-booster.git
cd kube-booster
```

### Install Dependencies

```bash
go mod download
go mod tidy
```

## Building from Source

### Build the Controller Binary

```bash
# Build with make
make build

# Or build directly with go
go build -o bin/controller cmd/controller/main.go
```

The binary will be created at `bin/controller`.

### Build the Docker Image

```bash
# Build with default tag (controller:latest)
make docker-build

# Build with custom tag
make docker-build IMG=myregistry/kube-booster:v1.0.0
```

### Push to Registry

```bash
make docker-push IMG=myregistry/kube-booster:v1.0.0
```

## Local Development with kind

This is the recommended way to test changes locally.

### 1. Create a kind Cluster

```bash
kind create cluster --name kube-booster-dev
```

### 2. Build and Load Image

```bash
# Build the image
make docker-build

# Load into kind cluster
kind load docker-image controller:latest --name kube-booster-dev
```

### 3. Deploy to kind Cluster

```bash
# Generate certificates
make generate-certs

# Deploy all components
make deploy

# Verify deployment
kubectl get pods -n kube-system -l app=kube-booster-controller
kubectl logs -n kube-system -l app=kube-booster-controller -f
```

### 4. Test Your Changes

```bash
# Deploy sample application
make deploy-sample

# Watch pods
kubectl get pods -l app=nginx -w

# Run smoke tests
./hack/quick_test.sh
```

### 5. Iterate on Changes

```bash
# Make code changes
vim pkg/controller/pod_controller.go

# Rebuild and reload
make docker-build
kind load docker-image controller:latest --name kube-booster-dev

# Restart deployment
kubectl rollout restart deployment kube-booster-controller -n kube-system

# Watch logs
kubectl logs -n kube-system -l app=kube-booster-controller -f
```

### 6. Cleanup

```bash
# Remove deployment
make undeploy

# Delete kind cluster
kind delete cluster --name kube-booster-dev
```

## Running Tests

### Unit Tests

```bash
# Run all tests
make test

# Run tests with verbose output
go test ./... -v

# Run tests for specific package
go test ./pkg/webhook/... -v
go test ./pkg/controller/... -v

# Run specific test
go test ./pkg/webhook/... -run TestPodMutator_Handle -v
```

### Test Coverage

```bash
# Generate coverage report
make test

# View coverage in browser
go tool cover -html=coverage.out
```

Current coverage:
- `pkg/webhook`: 84.2%
- `pkg/controller`: 68.4%
- `pkg/warmup`: 92.9%

### Integration Tests with envtest

The tests use controller-runtime's envtest which provides a real Kubernetes API server:

```bash
# Install setup-envtest tool
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Download test binaries
setup-envtest use

# Run tests
make test
```

## Code Quality

### Format Code

```bash
# Format all code
make fmt

# Or use go directly
go fmt ./...
```

### Run Linter

```bash
# Run golangci-lint
make lint

# Install golangci-lint if needed
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

### Vet Code

```bash
# Run go vet
make vet

# Or directly
go vet ./...
```

## Project Structure

```
kube-booster/
├── cmd/
│   └── controller/
│       └── main.go              # Main entry point
├── pkg/
│   ├── controller/
│   │   ├── pod_controller.go     # Reconciler implementation
│   │   ├── pod_controller_test.go
│   │   └── predicates.go         # Event filters
│   ├── warmup/
│   │   ├── config.go             # Configuration parsing
│   │   ├── config_test.go
│   │   ├── executor.go           # Vegeta executor implementation
│   │   ├── executor_test.go
│   │   └── result.go             # Warmup result structure
│   └── webhook/
│       ├── constants.go          # Shared constants
│       ├── pod_mutator.go        # Webhook handler
│       └── pod_mutator_test.go
├── config/
│   ├── rbac/                    # RBAC manifests
│   │   ├── service_account.yaml
│   │   ├── role.yaml
│   │   └── role_binding.yaml
│   ├── webhook/                 # Webhook manifests
│   │   ├── service.yaml
│   │   └── mutating_webhook.yaml
│   ├── samples/                 # Sample applications
│   │   └── sample_deployment.yaml
│   ├── deployment.yaml          # Controller deployment
│   └── kustomization.yaml       # Kustomize config
├── hack/
│   ├── generate_certs.sh        # Certificate generation
│   └── quick_test.sh            # Smoke tests
├── Dockerfile                   # Multi-stage build
├── Makefile                     # Build automation
├── go.mod                       # Go module definition
└── go.sum                       # Dependency checksums
```

## Architecture Overview

### Components

#### Webhook (pkg/webhook/)

**pod_mutator.go**
- Implements `admission.Handler` interface
- Decodes pod from admission request
- Checks for `kube-booster.io/warmup: "enabled"` annotation
- Injects readiness gate if annotation present
- Returns JSON patch response

**Key methods:**
- `Handle(ctx, req)` - Main webhook handler
- `InjectDecoder(decoder)` - Sets up decoder

#### Controller (pkg/controller/)

**pod_controller.go**
- Implements `reconcile.Reconciler` interface
- Watches pods with our readiness gate
- Checks if containers are ready
- Executes warmup requests before marking ready
- Updates pod condition when warmup completes

**Key methods:**
- `Reconcile(ctx, req)` - Main reconciliation loop
- `SetupWithManager(mgr)` - Registers controller
- `isConditionTrue(pod, type)` - Checks condition status
- `areContainersReady(pod)` - Checks container readiness
- `setConditionTrue(ctx, pod)` - Updates pod condition

**predicates.go**
- `HasReadinessGatePredicate()` - Filters events for relevant pods
- Only reconciles pods with our readiness gate

#### Warmup Package (pkg/warmup/)

**config.go**
- `Config` struct holds parsed warmup configuration
- `ParseConfig(pod)` parses annotations into Config:
  - `kube-booster.io/warmup-endpoint` → Endpoint path (default: `/`)
  - `kube-booster.io/warmup-requests` → Request count (default: `3`)
  - `kube-booster.io/warmup-duration` → Duration (default: `30s`)
  - `kube-booster.io/warmup-port` → Port (auto-detected if possible)
- Auto-detects port from container spec (single container, single port)
- Validates numeric values and duration format
- `BuildEndpointURL()` constructs full URL for requests

**executor.go**
- `Executor` interface for warmup implementations
- `VegetaExecutor` uses Vegeta load testing library
- Calculates request rate: `RequestCount / Duration`
- Per-request timeout: 1s minimum, scales with rate
- Adds custom headers:
  - `User-Agent: kube-booster/1.0`
  - `X-Warmup-Request: true`
- Context-aware cancellation support

**result.go**
- `Result` struct tracks warmup outcome:
  - `Success` - Whether warmup met success threshold
  - `RequestCount` - Completed requests
  - `FailedCount` - Failed requests
  - `LatencyP50` / `LatencyP99` - Latency percentiles
  - `SuccessRate` - Percentage of successful requests
  - `Message` - Human-readable summary
- `String()` method for formatted logging

#### Main Entry Point (cmd/controller/main.go)

- Initializes controller-runtime manager
- Registers webhook at `/mutate-v1-pod`
- Registers pod controller with predicates
- Provides endpoints:
  - `:9443` - Webhook server
  - `:8080` - Metrics
  - `:8081/healthz` - Health check
  - `:8081/readyz` - Readiness check

### Data Flow

```
Admission Request
    ↓
PodMutator.Handle()
    ↓
Check annotation
    ↓
Inject readiness gate (if enabled)
    ↓
Return patch response
    ↓
Pod created with readiness gate
    ↓
Controller watch event (filtered by predicate)
    ↓
PodReconciler.Reconcile()
    ↓
Check containers ready
    ↓
Update pod condition
    ↓
Pod becomes READY
```

## Development Workflow

### Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes**
   - Follow existing code patterns
   - Add tests for new functionality
   - Update documentation

3. **Test locally**
   ```bash
   # Run tests
   make test

   # Run linter
   make lint

   # Build binary
   make build

   # Test in kind cluster
   kind create cluster --name test
   make docker-build
   kind load docker-image controller:latest --name test
   make generate-certs
   make deploy
   ./hack/quick_test.sh
   ```

4. **Commit your changes**
   ```bash
   git add .
   git commit -m "Add feature: description"
   ```

5. **Push and create PR**
   ```bash
   git push origin feature/my-feature
   ```

### Adding New Features

#### Example: Adding a New Annotation

1. **Add constant** to `pkg/webhook/constants.go`:
   ```go
   const AnnotationMyFeature = "kube-booster.io/my-feature"
   ```

2. **Update webhook** in `pkg/webhook/pod_mutator.go`:
   ```go
   func (pm *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
       // ... existing code ...

       // Check new annotation
       if pod.Annotations[AnnotationMyFeature] != "" {
           // Handle feature
       }
   }
   ```

3. **Add tests** in `pkg/webhook/pod_mutator_test.go`:
   ```go
   func TestPodMutator_MyFeature(t *testing.T) {
       // Test cases
   }
   ```

4. **Update documentation** in `USAGE.md` and `CLAUDE.md`

## Debugging

### Debugging in kind

```bash
# View controller logs
kubectl logs -n kube-system -l app=kube-booster-controller -f

# Increase verbosity
kubectl set env deployment/kube-booster-controller -n kube-system VERBOSITY=5

# Check webhook requests
kubectl logs -n kube-system -l app=kube-booster-controller | grep "mutate-v1-pod"

# Inspect pod with issues
kubectl describe pod <pod-name>
kubectl get pod <pod-name> -o yaml
```

### Debugging Locally

You can run the controller locally (outside cluster) for faster iteration:

```bash
# Ensure you have kubeconfig set up
export KUBECONFIG=~/.kube/config

# Run controller locally
make run

# Or with custom flags
go run cmd/controller/main.go --webhook-port=9443 --metrics-bind-address=:8080
```

**Note**: Webhook functionality requires valid certificates and proper network setup when running locally.

### Common Issues

**Webhook not called:**
- Check MutatingWebhookConfiguration is registered
- Verify service is routing to correct pod
- Check certificate is valid and matches service DNS

**Controller not reconciling:**
- Check RBAC permissions
- Verify predicates are not filtering out pods
- Check pod has our readiness gate in spec

**Warmup failing:**
- Check pod has IP assigned (`kubectl get pod -o wide`)
- Verify warmup endpoint responds (`kubectl exec` to curl)
- Check port annotation if container has multiple ports
- Review controller logs for warmup result details
- Verify the endpoint path is correct (default is `/`)

**Warmup port not detected:**
- Error: "cannot determine warmup port"
- Solution: Add `kube-booster.io/warmup-port` annotation
- Auto-detection only works with single container having single port

**Tests failing:**
- Run `go mod tidy` to sync dependencies
- Check envtest binaries are installed
- Verify Go version is 1.25+

## CI/CD

### GitHub Actions (to be added)

Example workflow structure:

```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - run: make test
      - run: make lint
      - run: make build
```

## Release Process

### Versioning

Follow semantic versioning (SemVer):
- MAJOR: Breaking changes
- MINOR: New features (backwards compatible)
- PATCH: Bug fixes

### Creating a Release

1. **Update version**
   ```bash
   # Tag the release
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Build release image**
   ```bash
   make docker-build IMG=ghcr.io/hhiroshell/kube-booster:v1.0.0
   make docker-push IMG=ghcr.io/hhiroshell/kube-booster:v1.0.0
   ```

3. **Update documentation**
   - Update CHANGELOG.md
   - Update installation instructions with new version

## Contributing Guidelines

### Code Style

- Follow standard Go conventions
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and small

### Testing Requirements

- Add unit tests for new functions
- Maintain or improve test coverage
- Integration tests for new features
- Update test documentation

### Documentation Requirements

- Update USAGE.md for user-facing changes
- Update DEVELOPMENT.md for developer changes
- Update CLAUDE.md for architecture changes
- Add inline code comments for complex logic

### Pull Request Process

1. Create feature branch from `main`
2. Make changes with tests
3. Ensure all tests pass
4. Update documentation
5. Submit PR with clear description
6. Address review feedback
7. Squash commits before merge

## Performance Considerations

### Webhook Performance

- Webhook should respond in <100ms
- Use efficient JSON patching
- Avoid expensive operations in webhook path

### Controller Performance

- Use predicates to filter events
- Implement exponential backoff for retries
- Batch status updates when possible
- Use informers efficiently

### Resource Usage

Target resource limits:
- CPU: 100m request, 500m limit
- Memory: 64Mi request, 256Mi limit

## Security Considerations

### Security Best Practices

- Run as non-root user
- Use read-only root filesystem
- Drop all capabilities
- Enable seccomp profile
- Validate all inputs
- Sanitize logs (no sensitive data)

### Certificate Management

For production:
- Use cert-manager for automatic certificate rotation
- Monitor certificate expiration
- Use proper CA for production clusters

## Troubleshooting Development Issues

### go mod issues

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
rm go.sum
go mod tidy
```

### Docker build issues

```bash
# Clean Docker cache
docker system prune -a

# Build without cache
docker build --no-cache -t controller:latest .
```

### kind issues

```bash
# Reset kind cluster
kind delete cluster --name kube-booster-dev
kind create cluster --name kube-booster-dev

# Check kind cluster
kind get clusters
kubectl cluster-info --context kind-kube-booster-dev
```

## Resources

- [controller-runtime documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [Kubernetes readiness gates](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-gate)
- [Kubebuilder book](https://book.kubebuilder.io/)

## Future Development (Phase 3+)

Phase 2 HTTP warmup execution is complete. See [CLAUDE.md](../CLAUDE.md) for the complete roadmap. Future areas:

1. **gRPC Support**
   - Add gRPC warmup capability
   - Protocol detection from annotations

2. **Observability Enhancements**
   - Prometheus metrics export
   - Kubernetes events for warmup results
   - Distributed tracing integration

3. **Advanced Features**
   - `WarmupConfig` CRD for complex scenarios
   - Multiple sequential warmup endpoints
   - Custom request bodies (POST support)
   - Retry logic with exponential backoff

4. **Production Hardening**
   - High availability (leader election improvements)
   - Automated certificate rotation
   - Rate limiting for warmup requests

## Getting Help

- Check [USAGE.md](USAGE.md) for user documentation
- Review [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for technical details
- Open an issue on GitHub for bugs or feature requests
- Join discussions for questions and ideas
