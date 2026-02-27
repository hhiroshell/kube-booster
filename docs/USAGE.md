# kube-booster Usage Guide

This guide shows you how to deploy and use kube-booster in your Kubernetes cluster.

## What is kube-booster?

kube-booster is a Kubernetes controller that ensures smooth application launches by managing warmup readiness gates. It prevents pods from receiving traffic until they are fully warmed up.

**Current Features**:
- Mutating webhook injects readiness gates for annotated pods
- Controller sends HTTP warmup requests back-to-back before marking pods ready
- Fail-open behavior ensures pods become ready even if warmup fails
- Kubernetes Events emitted for warmup lifecycle visibility via `kubectl describe pod`
- Prometheus metrics exported for monitoring warmup performance and alerting

## Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured to access your cluster
- Cluster admin permissions (for installing CRDs and webhooks)

## Installation

### Option 1: Using Pre-built Image (Recommended)

If you have a pre-built image available:

```bash
# Download the manifests
git clone https://github.com/hhiroshell/kube-booster.git
cd kube-booster

# Generate certificates for webhook
make generate-certs

# Deploy to cluster
make deploy
```

### Option 2: Build from Source

See [DEVELOPMENT.md](DEVELOPMENT.md) for instructions on building from source.

### Deployment Architecture

kube-booster deploys as two separate workloads:

```
┌─────────────────────────────────────────────────────────────────────┐
│                     kube-system namespace                           │
│                                                                     │
│  ┌──────────────────────────┐    ┌──────────────────────────────┐  │
│  │   Webhook (Deployment)   │    │   Controller (DaemonSet)     │  │
│  │   • 1 replica            │    │   • 1 pod per node           │  │
│  │   • Handles admission    │    │   • Executes warmup locally  │  │
│  │   • Injects readiness    │    │   • Watches node-local pods  │  │
│  │     gates                │    │                              │  │
│  └──────────────────────────┘    └──────────────────────────────┘  │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

This architecture ensures:
- **Scalability**: Controller pods scale with cluster nodes
- **Efficiency**: Warmup requests are sent from the same node as the target pod
- **Resilience**: Node-local failures don't affect other nodes

See [DEVELOPMENT.md](DEVELOPMENT.md#deployment-architectures) for detailed architecture diagrams and alternative deployment modes.

### Verify Installation

Check that the webhook and controller are running:

```bash
# List all kube-booster components at once
kubectl get pods -n kube-system -l app.kubernetes.io/name=kube-booster

# Or check each component separately
kubectl get deployment -n kube-system kube-booster-webhook
kubectl get daemonset -n kube-system kube-booster-controller

# View logs from all kube-booster components
kubectl logs -n kube-system -l app.kubernetes.io/name=kube-booster --prefix

# Or view logs from specific components
kubectl logs -n kube-system -l app.kubernetes.io/component=webhook -f
kubectl logs -n kube-system -l app.kubernetes.io/component=controller -f
```

Expected output:
```
NAME                     READY   UP-TO-DATE   AVAILABLE
kube-booster-webhook     1/1     1            1

NAME                      DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
kube-booster-controller   1         1         1       1            1
```

## Usage

### Enabling Warmup for Your Application

Add the `kube-booster.io/warmup: "enabled"` annotation to your pod template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
      annotations:
        # Enable warmup readiness gate
        kube-booster.io/warmup: "enabled"
    spec:
      containers:
      - name: my-app
        image: my-app:latest
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
```

### Configuration Annotations

| Annotation | Description | Default |
|------------|-------------|---------|
| `kube-booster.io/warmup` | Enable/disable warmup (`enabled`/`disabled`) | `disabled` |
| `kube-booster.io/warmup-endpoint` | HTTP endpoint path for warmup requests | `/` |
| `kube-booster.io/warmup-requests` | Number of warmup requests to send (1-12000) | `3` |
| `kube-booster.io/warmup-timeout` | Maximum timeout for warmup (1s-5m, e.g., `30s`, `1m`) | `30s` |
| `kube-booster.io/warmup-port` | Container port for warmup requests | Auto-detected |

### Example: Complete Application

Deploy the sample nginx application:

```bash
make deploy-sample
```

Or create your own:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-with-warmup
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        kube-booster.io/warmup: "enabled"
    spec:
      containers:
      - name: nginx
        image: nginx:1.25
        ports:
        - containerPort: 80
        readinessProbe:
          httpGet:
            path: /
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Advanced Configuration

For applications that need customized warmup behavior, use all available annotations:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-api-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-api
  template:
    metadata:
      labels:
        app: my-api
      annotations:
        # Enable warmup
        kube-booster.io/warmup: "enabled"
        # Send warmup requests to the /warmup endpoint
        kube-booster.io/warmup-endpoint: "/warmup"
        # Send 10 warmup requests
        kube-booster.io/warmup-requests: "10"
        # Maximum timeout for warmup (60 seconds)
        kube-booster.io/warmup-timeout: "60s"
        # Use port 8080 (required for multi-port containers)
        kube-booster.io/warmup-port: "8080"
    spec:
      containers:
      - name: api-server
        image: my-api:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
```

**Notes:**
- **Port auto-detection**: If your container has exactly one port, kube-booster will automatically detect it. Specify `warmup-port` explicitly when containers have multiple ports.
- **Request execution**: Requests are sent back-to-back as fast as possible (ASAP model). The `warmup-timeout` sets the maximum wall-clock time for the entire warmup phase. Warmup typically completes much faster than the timeout.
- **Custom headers**: All warmup requests include `User-Agent: kube-booster/1.0` and `X-Warmup-Request: true` headers.

### Controller Flags

The controller binary accepts the following flags for tuning concurrency and rate limiting:

| Flag | Default | Description |
|------|---------|-------------|
| `--max-concurrent-warmups` | `10` | Maximum concurrent warmup executions per controller instance. `0` disables the limit (unlimited). |
| `--max-warmup-rps` | `100` | Maximum aggregate warmup HTTP request rate (requests per second) across all concurrent warmups. `0` disables rate limiting (unlimited). |

These flags are set in the DaemonSet spec for the controller. For example, to allow 20 concurrent warmups and cap the aggregate HTTP request rate at 200 RPS:

```yaml
args:
  - --max-concurrent-warmups=20
  - --max-warmup-rps=200
```

**When to tune these values:**
- **Large-scale rollouts** (many pods starting simultaneously): lower `--max-concurrent-warmups` to prevent overwhelming the controller or target applications.
- **Rate-sensitive applications**: use `--max-warmup-rps` to smooth out the warmup HTTP traffic across concurrent executions.
- The semaphore is per-controller-instance (one per node in DaemonSet mode), so effective limits scale with node count.

### Concurrency and Rate Limiting Interaction

The two controls are complementary but interact in a way that affects pod warmup throughput:

- **`--max-concurrent-warmups`** caps the number of pods being warmed up simultaneously. Each slot is held for the full duration of one pod's warmup.
- **`--max-warmup-rps`** caps the total HTTP request rate across all concurrent warmups. The token bucket is shared globally — all concurrent goroutines draw from the same pool.

When both are set, `--max-concurrent-warmups` determines how many pods warm up in parallel while `--max-warmup-rps` controls how long each slot is held. A pod doing 1,000 warmup requests at a shared rate limit of 100 RPS holds its slot for ~10 seconds regardless of how many other pods are competing. Operators running high request-count warmups should account for this when sizing `--max-warmup-rps`.

### Safety Defaults and Risks

The default values (`--max-concurrent-warmups=10`, `--max-warmup-rps=100`) protect the controller and target applications from unbounded load in most deployments. Be aware of these risks when overriding them:

> **Warning:** Setting `--max-concurrent-warmups=0` disables the concurrency limit. Combined with `--max-warmup-rps=0`, a large-scale HPA scale-out can create unbounded concurrent goroutines, each issuing up to 12,000 HTTP requests. Only opt out of both limits if your network and application infrastructure can handle the resulting load.

### Multi-Tenancy Considerations

kube-booster uses a **single global token bucket** shared across all namespaces. This means:

- A large rollout in one namespace can consume all available RPS tokens and delay warmup for pods in other namespaces.
- There is no per-namespace rate isolation in the current implementation.

To mitigate this in multi-tenant clusters:
- Set `--max-warmup-rps` proportional to the expected number of namespaces doing concurrent rollouts.
- Consider staggering large deployments across namespaces when possible.

## Verification

### Check Readiness Gate Injection

Verify that the readiness gate was injected into your pods:

```bash
# View readiness gates
kubectl get pod -l app=my-app -o jsonpath='{.items[0].spec.readinessGates}'

# Expected output:
# [{"conditionType":"kube-booster.io/warmup-ready"}]
```

### Check Warmup Condition

Monitor the warmup condition status:

```bash
# Watch pod status
kubectl get pods -l app=my-app -w

# View detailed conditions
kubectl describe pod -l app=my-app

# Check warmup condition specifically
kubectl get pod -l app=my-app -o jsonpath='{.items[0].status.conditions[?(@.type=="kube-booster.io/warmup-ready")]}'
```

Expected flow:
1. Pod created with readiness gate injected
2. Containers start and become ready
3. Controller sets `kube-booster.io/warmup-ready` condition to `True`
4. Pod transitions to READY state

### View Warmup Events

kube-booster emits Kubernetes Events during the warmup lifecycle, visible via `kubectl describe pod`:

```bash
kubectl describe pod -l app=my-app | grep -A 20 Events
```

Expected events:
```
Events:
  Type     Reason             Age   From                       Message
  ----     ------             ----  ----                       -------
  Normal   WarmupStarted      10s   kube-booster-controller    Starting warmup execution
  Normal   WarmupCompleted    5s    kube-booster-controller    warmup completed: 5/5 requests succeeded (100.0%), P50=12ms, P99=45ms
  Normal   ConditionUpdated   5s    kube-booster-controller    Pod condition kube-booster.io/warmup-ready set to True
```

| Event | Type | Description |
|-------|------|-------------|
| `WarmupQueued` | Normal | Pod is waiting for a concurrency slot (when `--max-concurrent-warmups` is set) |
| `WarmupStarted` | Normal | Warmup execution begins |
| `WarmupCompleted` | Normal | Warmup completed successfully |
| `WarmupFailed` | Warning | Warmup failed (config error or request failures) |
| `ConditionUpdated` | Normal/Warning | Pod condition set to True (Warning if fail-open) |

### Quick Test

Run a quick smoke test:

```bash
./hack/quick_test.sh
```

This script verifies:
- Controller is running
- Webhook injects readiness gates
- Controller sets conditions
- Pods become READY
- Pods without annotation are unaffected

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│  User creates pod with kube-booster.io/warmup: "enabled"   │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Mutating Webhook intercepts CREATE operation              │
│  → Injects readiness gate: kube-booster.io/warmup-ready   │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Pod created with readiness gate in spec                   │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Controller watches pod (via event filters)                │
│  → Waits for containers to be ready                       │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Controller executes warmup requests back-to-back          │
│  → Emits WarmupStarted event                              │
│  → Parses configuration from annotations                  │
│  → Sends HTTP requests to pod endpoint                    │
│  → Records Prometheus metrics (duration, latency, counts) │
│  → Emits WarmupCompleted or WarmupFailed event            │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Controller sets condition kube-booster.io/warmup-ready    │
│  → Success: condition = True                              │
│  → Failure: condition = True (fail-open) with warning log │
│  → Emits ConditionUpdated event                           │
└──────────────────────┬──────────────────────────────────────┘
                       ↓
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes marks pod as READY                             │
│  (all readiness gates satisfied)                           │
└─────────────────────────────────────────────────────────────┘
```

## Troubleshooting

### Pods Not Getting Readiness Gate

**Symptoms**: Pods created but no readiness gate appears in `spec.readinessGates`

**Possible Causes**:
1. Webhook not running
2. Annotation typo or wrong value
3. Webhook configuration issue

**Solutions**:

Check webhook is running:
```bash
kubectl get deployment -n kube-system kube-booster-webhook
kubectl get pods -n kube-system -l app.kubernetes.io/component=webhook
```

Check webhook logs:
```bash
kubectl logs -n kube-system -l app.kubernetes.io/component=webhook
```

Verify webhook configuration:
```bash
kubectl get mutatingwebhookconfiguration kube-booster-mutating-webhook -o yaml
```

Verify annotation is correct:
```bash
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'
```

### Pods Stuck Not READY

**Symptoms**: Pods have readiness gate but never become READY

**Possible Causes**:
1. Controller not running
2. Containers not becoming ready
3. RBAC permission issues

**Solutions**:

Check controller logs:
```bash
kubectl logs -n kube-system daemonset/kube-booster-controller -f
```

Check container status:
```bash
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[*].ready}'
```

Verify RBAC permissions:
```bash
kubectl auth can-i update pods/status --as=system:serviceaccount:kube-system:kube-booster-controller
```

Check pod conditions:
```bash
kubectl describe pod <pod-name> | grep -A 10 Conditions
```

### Certificate Errors

**Symptoms**: Webhook returns certificate errors, pods fail to create

**Solutions**:

Regenerate certificates:
```bash
kubectl delete secret kube-booster-webhook-cert -n kube-system
make generate-certs
kubectl rollout restart deployment kube-booster-webhook -n kube-system
```

Check certificate expiration:
```bash
kubectl get secret kube-booster-webhook-cert -n kube-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -dates
```

### Controller Logs Show Errors

View detailed logs:
```bash
# View logs from all kube-booster components (with pod name prefix)
kubectl logs -n kube-system -l app.kubernetes.io/name=kube-booster --prefix

# Follow webhook logs in real-time
kubectl logs -n kube-system -l app.kubernetes.io/component=webhook -f

# Follow controller logs in real-time
kubectl logs -n kube-system -l app.kubernetes.io/component=controller -f

# Get recent controller logs
kubectl logs -n kube-system daemonset/kube-booster-controller --tail=100

# Check previous instance (if pod restarted)
kubectl logs -n kube-system daemonset/kube-booster-controller --previous
```

## Uninstallation

Remove kube-booster from your cluster:

```bash
# Remove the controller and webhook
make undeploy

# Clean up sample application (if deployed)
make undeploy-sample

# Delete certificate secret
kubectl delete secret kube-booster-webhook-cert -n kube-system
```

Verify removal:
```bash
kubectl get deployment -n kube-system kube-booster-webhook
kubectl get daemonset -n kube-system kube-booster-controller
# Both should return: Error from server (NotFound)
```

## FAQ

### Does kube-booster affect pods without the annotation?

No. Only pods with `kube-booster.io/warmup: "enabled"` annotation are affected.

### What happens if the webhook is down?

The webhook has `failurePolicy: Ignore`, so pods will be created normally without the readiness gate if the webhook is unavailable.

### Can I use kube-booster with other admission controllers?

Yes. kube-booster is designed to work alongside other admission controllers and webhooks.

### What Kubernetes versions are supported?

Kubernetes 1.19+ is required for readiness gate support.

### Does this work with StatefulSets, DaemonSets, Jobs?

Yes. Any pod with the annotation will have the readiness gate injected, regardless of the controller type.

### How do I disable warmup for a specific pod?

Simply remove the annotation or set it to a value other than `"enabled"`:
```yaml
kube-booster.io/warmup: "disabled"
```

### What happens if warmup fails?

kube-booster uses **fail-open behavior**: if warmup fails (e.g., connection errors, non-200 responses), the pod is still marked as ready. A `WarmupFailed` warning event is emitted with details about the failure, and a `ConditionUpdated` warning event indicates the fail-open behavior. This ensures warmup issues don't prevent pods from becoming ready.

### How can I see warmup progress and results?

Use `kubectl describe pod <pod-name>` to view warmup events:
```bash
kubectl describe pod my-app-pod | grep -A 10 Events
```

Events show the complete warmup lifecycle:
- `WarmupStarted` - When warmup begins
- `WarmupCompleted` or `WarmupFailed` - Warmup result with latency metrics
- `ConditionUpdated` - When the pod condition is set to True

### How does port auto-detection work?

If your pod has exactly one container with exactly one port defined, kube-booster automatically uses that port for warmup requests. If your pod has multiple containers or multiple ports, you must specify the port using the `kube-booster.io/warmup-port` annotation.

### How are warmup requests distributed?

Requests are sent back-to-back as fast as possible (ASAP model). The `warmup-timeout` annotation sets the maximum wall-clock time for the entire warmup phase. For example:
- 3 requests with timeout 30s: completes in milliseconds (much faster than the timeout)
- 10 requests with timeout 60s: completes as fast as the server can respond

### What metrics are logged during warmup?

After warmup completes, the controller logs:
- Total requests sent
- Success/failure count
- P50 and P99 latencies
- Overall success rate

Additionally, Prometheus metrics are recorded for each warmup execution including duration, request count, latency, and success/failure counters. See the next FAQ for details.

### Are Prometheus metrics available?

Yes! kube-booster exports Prometheus metrics on the controller's metrics endpoint (`:8080/metrics`). See [OBSERVABILITY.md](OBSERVABILITY.md) for available metrics, PromQL examples, alerting rules, and a sample Grafana dashboard.

## Next Steps

- Review [OBSERVABILITY.md](OBSERVABILITY.md) for Prometheus metrics and Grafana dashboards
- Review [DEVELOPMENT.md](DEVELOPMENT.md) for building from source and contributing
- Check [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for technical details
- See [CLAUDE.md](../CLAUDE.md) for architecture and future roadmap

## Support

- GitHub Issues: https://github.com/hhiroshell/kube-booster/issues
- Documentation: Check the `docs/` directory for more guides
