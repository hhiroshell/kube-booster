# kube-booster

A Kubernetes custom controller that ensures smooth application launches by sending warmup requests to application endpoints before pods transition to READY state.

## Overview

kube-booster helps reduce cold start issues and improves application readiness by pre-warming application endpoints. This is particularly useful for applications that need time to initialize caches, load data, or perform JIT compilation before serving production traffic efficiently.

## Features

- Sends warmup requests to application endpoints before pod status becomes READY
- Configurable warmup strategies and request patterns
- Integrates seamlessly with Kubernetes pod lifecycle
- Helps reduce cold start latency for production traffic

## How It Works

kube-booster watches for new pods and intercepts the readiness check process. Before marking a pod as READY:

1. Detects new pods matching configured criteria
2. Sends configured warmup requests to the application endpoint
3. Waits for successful warmup completion
4. Allows the pod to transition to READY state

## Installation

_Coming soon_

## Configuration

_Coming soon_

## Usage

_Coming soon_

## Requirements

- Kubernetes cluster (version TBD)
- Appropriate RBAC permissions for the controller

## Development

### Prerequisites

- Go 1.23 or later
- Docker (for building container images)
- kubectl (for deploying to Kubernetes)
- Access to a Kubernetes cluster

### Building

Build the controller binary:
```bash
make build
```

### Running Locally

Run the controller locally:
```bash
make run
```

### Testing

Run tests:
```bash
make test
```

Run linter:
```bash
make lint
```

### Building Docker Image

Build the Docker image:
```bash
make docker-build IMG=your-registry/kube-booster:tag
```

### Project Structure

- `cmd/controller/` - Controller entry point
- `pkg/controller/` - Controller implementation
- `pkg/warmup/` - Warmup request logic
- `config/crd/` - Custom Resource Definitions
- `config/rbac/` - RBAC configurations
- `config/samples/` - Example configurations

## Contributing

_Coming soon_

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
