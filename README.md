# 🚀 Kube Booster

[![CI](https://github.com/hhiroshell/kube-booster/actions/workflows/ci.yml/badge.svg)](https://github.com/hhiroshell/kube-booster/actions/workflows/ci.yml)

A Kubernetes custom controller that ensures smooth application launches by sending warmup requests to application endpoints before pods transition to READY state.

## 📖 Overview

Kube Booster helps reduce cold start issues and improves application readiness by pre-warming application endpoints. This is particularly useful for applications that need time to initialize caches, load data, or perform JIT compilation before serving production traffic efficiently.

## ✨ Features

- **Mutating Webhook**: Automatically injects readiness gates for annotated pods
- **HTTP Warmup Requests**: Sends configurable warmup requests using Vegeta load testing library
- **Annotation-Based Configuration**: Simple opt-in via pod annotations
- **Fail-Open Behavior**: Pods become ready even if warmup fails (with warning logs)
- **Port Auto-Detection**: Automatically detects container port for single-port containers
- **Metrics Logging**: Logs P50/P99 latencies and success rates after warmup

## 🔧 How It Works

Kube Booster watches for new pods and intercepts the readiness check process using Kubernetes readiness gates:

1. **Webhook Injection**: Mutating webhook injects `kube-booster.io/warmup-ready` readiness gate
2. **Container Readiness**: Controller waits for containers to become ready
3. **Warmup Execution**: Sends HTTP warmup requests to the configured endpoint
4. **Condition Update**: Sets the warmup condition to True, allowing pod to become READY

```
Pod Created → Webhook Injects Gate → Containers Ready → Warmup Requests → Pod READY
```

## 📦 Installation

### Quick Start

```bash
# Clone the repository
git clone https://github.com/hhiroshell/kube-booster.git
cd kube-booster

# Generate certificates for webhook
make generate-certs

# Deploy to cluster
make deploy

# Verify all kube-booster components are running
kubectl get pods -n kube-system -l app.kubernetes.io/name=kube-booster
```

See [docs/USAGE.md](docs/USAGE.md) for detailed installation instructions.

## ⚙️ Configuration

Enable warmup for your application by adding annotations to your pod template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        # Enable warmup (required)
        kube-booster.io/warmup: "enabled"

        # Optional configuration
        kube-booster.io/warmup-endpoint: "/warmup"  # Default: /
        kube-booster.io/warmup-requests: "5"        # Default: 3
        kube-booster.io/warmup-duration: "30s"      # Default: 30s
        kube-booster.io/warmup-port: "8080"         # Auto-detected if single port
    spec:
      containers:
      - name: my-app
        image: my-app:latest
        ports:
        - containerPort: 8080
```

## 📋 Requirements

- Kubernetes cluster 1.19+
- kubectl configured to access your cluster
- Cluster admin permissions (for webhooks and RBAC)

## 🛠️ Development

### Prerequisites

- Go 1.25 or later
- Docker (for building container images)
- kubectl (for deploying to Kubernetes)
- kind or minikube (for local testing)

### Building

```bash
# Build the controller binary
make build

# Build Docker image
make docker-build IMG=your-registry/kube-booster:tag
```

### Testing

```bash
# Run unit tests
make test

# Run linter
make lint

# Run smoke tests (requires deployed controller)
./hack/quick_test.sh
```

### Local Development

```bash
# Create kind cluster
kind create cluster --name kube-booster-dev

# Build and load image
make docker-build
kind load docker-image controller:latest --name kube-booster-dev

# Deploy
make generate-certs
make deploy

# Test with sample app
make deploy-sample
```

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed development instructions.

### 📁 Project Structure

```
kube-booster/
├── cmd/controller/     # Controller entry point
├── pkg/
│   ├── controller/     # Pod reconciler implementation
│   ├── warmup/         # Warmup execution (Vegeta)
│   └── webhook/        # Mutating admission webhook
├── config/
│   ├── rbac/           # RBAC configurations
│   ├── webhook/        # Webhook configurations
│   └── samples/        # Example deployments
├── hack/               # Scripts (certs, testing)
└── docs/               # Documentation
```

## 📚 Documentation

- [Usage Guide](docs/USAGE.md) - Installation and configuration
- [Development Guide](docs/DEVELOPMENT.md) - Building and contributing
- [Implementation Summary](docs/IMPLEMENTATION_SUMMARY.md) - Technical details

## 🤝 Contributing

Contributions are welcome! Please read the [Development Guide](docs/DEVELOPMENT.md) for guidelines.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
