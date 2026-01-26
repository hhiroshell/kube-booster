# kube-booster Usage Guide

This guide shows you how to deploy and use kube-booster in your Kubernetes cluster.

## What is kube-booster?

kube-booster is a Kubernetes controller that ensures smooth application launches by managing warmup readiness gates. It prevents pods from receiving traffic until they are fully warmed up.

**Phase 1 Status**: Currently establishes the infrastructure for warmup functionality:
- Mutating webhook injects readiness gates for annotated pods
- Controller sets readiness gate condition to True when containers are ready
- Actual warmup request execution will be added in Phase 2

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

### Verify Installation

Check that the controller is running:

```bash
# Check controller deployment
kubectl get deployment -n kube-system kube-booster-controller

# View controller logs
kubectl logs -n kube-system -l app=kube-booster-controller -f
```

Expected output:
```
NAME                       READY   UP-TO-DATE   AVAILABLE
kube-booster-controller    1/1     1            1
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

| Annotation | Description | Default | Status |
|------------|-------------|---------|--------|
| `kube-booster.io/warmup` | Enable/disable warmup | `disabled` | ✅ Phase 1 |
| `kube-booster.io/warmup-endpoint` | Warmup endpoint URL | Readiness probe path | 🔜 Phase 2 |
| `kube-booster.io/warmup-requests` | Number of warmup requests | `3` | 🔜 Phase 2 |
| `kube-booster.io/warmup-timeout` | Warmup timeout duration | `30s` | 🔜 Phase 2 |

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
│  → Sets condition kube-booster.io/warmup-ready = True     │
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
kubectl get pods -n kube-system -l app=kube-booster-controller
```

Check webhook logs:
```bash
kubectl logs -n kube-system -l app=kube-booster-controller
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
kubectl logs -n kube-system -l app=kube-booster-controller -f
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
kubectl rollout restart deployment kube-booster-controller -n kube-system
```

Check certificate expiration:
```bash
kubectl get secret kube-booster-webhook-cert -n kube-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -dates
```

### Controller Logs Show Errors

View detailed logs:
```bash
# Follow logs in real-time
kubectl logs -n kube-system -l app=kube-booster-controller -f

# Get recent logs
kubectl logs -n kube-system -l app=kube-booster-controller --tail=100

# Check previous instance (if pod restarted)
kubectl logs -n kube-system -l app=kube-booster-controller --previous
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
kubectl get deployment -n kube-system kube-booster-controller
# Should return: Error from server (NotFound)
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

## Next Steps

- Review [DEVELOPMENT.md](DEVELOPMENT.md) for building from source and contributing
- Check [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) for technical details
- See Phase 2 roadmap in [CLAUDE.md](CLAUDE.md) for upcoming features

## Support

- GitHub Issues: https://github.com/hhiroshell/kube-booster/issues
- Documentation: Check the `docs/` directory for more guides
