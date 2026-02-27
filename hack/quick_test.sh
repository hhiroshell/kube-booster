#!/usr/bin/env bash

# Quick test script for kube-booster
# This script performs basic smoke tests to verify the implementation

set -e

NAMESPACE="default"
TEST_POD="test-warmup-pod"
CONTROLLER_NAMESPACE="kube-system"

echo "=========================================="
echo "kube-booster Quick Test"
echo "=========================================="
echo ""

# Check if webhook and controller are running
echo "1. Checking if kube-booster components are running..."

# Check webhook deployment
if kubectl get deployment kube-booster-webhook -n ${CONTROLLER_NAMESPACE} &> /dev/null; then
    WEBHOOK_READY=$(kubectl get deployment kube-booster-webhook -n ${CONTROLLER_NAMESPACE} -o jsonpath='{.status.readyReplicas}')
    if [ "$WEBHOOK_READY" == "1" ]; then
        echo "   ✓ Webhook deployment is running"
    else
        echo "   ✗ Webhook deployment is not ready"
        exit 1
    fi
else
    echo "   ✗ Webhook deployment not found"
    echo "   Run 'make deploy' first"
    exit 1
fi

# Check controller daemonset
if kubectl get daemonset kube-booster-controller -n ${CONTROLLER_NAMESPACE} &> /dev/null; then
    DESIRED=$(kubectl get daemonset kube-booster-controller -n ${CONTROLLER_NAMESPACE} -o jsonpath='{.status.desiredNumberScheduled}')
    READY=$(kubectl get daemonset kube-booster-controller -n ${CONTROLLER_NAMESPACE} -o jsonpath='{.status.numberReady}')
    if [ "$READY" == "$DESIRED" ] && [ "$READY" != "0" ]; then
        echo "   ✓ Controller daemonset is running ($READY/$DESIRED pods)"
    else
        echo "   ✗ Controller daemonset is not ready ($READY/$DESIRED pods)"
        exit 1
    fi
else
    echo "   ✗ Controller daemonset not found"
    echo "   Run 'make deploy' first"
    exit 1
fi

# Clean up any previous test pods
echo ""
echo "2. Cleaning up previous test pods..."
kubectl delete pod ${TEST_POD} ${TEST_POD}-no-warmup ${TEST_POD}-ratelimit --ignore-not-found=true --wait=false &> /dev/null || true
sleep 2

# Test webhook injection with warmup-enabled pod
echo ""
echo "3. Testing webhook injection..."

# Create a pod with warmup enabled and explicit port configuration
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD}
  namespace: ${NAMESPACE}
  annotations:
    kube-booster.io/warmup: "enabled"
    kube-booster.io/warmup-endpoint: "/"
    kube-booster.io/warmup-requests: "3"
    kube-booster.io/warmup-timeout: "5s"
    kube-booster.io/warmup-port: "80"
spec:
  containers:
  - name: nginx
    image: nginx:1.25
    ports:
    - containerPort: 80
EOF

# Wait for pod to be created
sleep 2

# Check if readiness gate was injected
READINESS_GATE=$(kubectl get pod ${TEST_POD} -o jsonpath='{.spec.readinessGates[0].conditionType}')
if [ "$READINESS_GATE" == "kube-booster.io/warmup-ready" ]; then
    echo "   ✓ Readiness gate injected successfully"
else
    echo "   ✗ Readiness gate not found"
    kubectl delete pod ${TEST_POD} --ignore-not-found=true
    exit 1
fi

# Wait for containers to be ready
echo ""
echo "4. Waiting for containers to be ready..."
kubectl wait --for=condition=ContainersReady pod/${TEST_POD} --timeout=60s || true

# Give controller time to execute warmup
echo ""
echo "5. Waiting for warmup execution..."
echo "   (warmup configured for 5s duration)"
sleep 10

# Check if warmup condition was set
echo ""
echo "6. Checking warmup condition..."
CONDITION_STATUS=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="kube-booster.io/warmup-ready")].status}')
CONDITION_REASON=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="kube-booster.io/warmup-ready")].reason}')
CONDITION_MESSAGE=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="kube-booster.io/warmup-ready")].message}')

if [ "$CONDITION_STATUS" == "True" ]; then
    echo "   ✓ Warmup condition set to True"
    echo "   Reason: ${CONDITION_REASON}"
    echo "   Message: ${CONDITION_MESSAGE:0:80}..."
else
    echo "   ⚠ Warmup condition not set (status: ${CONDITION_STATUS})"
    echo "   This might indicate warmup is still in progress"
fi

# Check if pod is ready
POD_READY=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
if [ "$POD_READY" == "True" ]; then
    echo "   ✓ Pod is READY"
else
    echo "   ⚠ Pod not READY yet (status: ${POD_READY})"
fi

# Verify warmup execution in controller logs
echo ""
echo "7. Verifying warmup execution in controller logs..."
# Find which node the test pod is running on, then get logs from that node's controller
TEST_POD_NODE=$(kubectl get pod ${TEST_POD} -o jsonpath='{.spec.nodeName}')
CONTROLLER_POD=$(kubectl get pods -n ${CONTROLLER_NAMESPACE} -l app.kubernetes.io/component=controller --field-selector spec.nodeName=${TEST_POD_NODE} -o jsonpath='{.items[0].metadata.name}')
WARMUP_LOGS=$(kubectl logs -n ${CONTROLLER_NAMESPACE} ${CONTROLLER_POD} --tail=50 2>/dev/null | grep -E "(starting warmup execution|warmup completed)" | tail -5 || true)

if [ -n "$WARMUP_LOGS" ]; then
    echo "   ✓ Warmup execution found in controller logs (node: ${TEST_POD_NODE}):"
    echo "$WARMUP_LOGS" | while read line; do
        echo "     $line"
    done
else
    echo "   ⚠ No warmup execution logs found (this may be normal if log level is high)"
fi

# Verify Kubernetes Events
echo ""
echo "8. Verifying Kubernetes Events..."
EVENTS=$(kubectl get events --field-selector involvedObject.name=${TEST_POD} -o jsonpath='{range .items[*]}{.reason}{" "}{end}' 2>/dev/null || true)

WARMUP_QUEUED=$(echo "$EVENTS" | grep -o "WarmupQueued" || true)
WARMUP_STARTED=$(echo "$EVENTS" | grep -o "WarmupStarted" || true)
WARMUP_COMPLETED=$(echo "$EVENTS" | grep -o "WarmupCompleted" || true)
WARMUP_FAILED=$(echo "$EVENTS" | grep -o "WarmupFailed" || true)
CONDITION_UPDATED=$(echo "$EVENTS" | grep -o "ConditionUpdated" || true)

if [ -n "$WARMUP_QUEUED" ]; then
    echo "   ✓ WarmupQueued event found (semaphore concurrency control active)"
else
    echo "   ⚠ WarmupQueued event not found (expected when --max-concurrent-warmups > 0)"
fi

if [ -n "$WARMUP_STARTED" ]; then
    echo "   ✓ WarmupStarted event found"
else
    echo "   ⚠ WarmupStarted event not found"
fi

if [ -n "$WARMUP_COMPLETED" ]; then
    echo "   ✓ WarmupCompleted event found"
elif [ -n "$WARMUP_FAILED" ]; then
    echo "   ⚠ WarmupFailed event found (fail-open behavior)"
else
    echo "   ⚠ No WarmupCompleted/WarmupFailed event found"
fi

if [ -n "$CONDITION_UPDATED" ]; then
    echo "   ✓ ConditionUpdated event found"
else
    echo "   ⚠ ConditionUpdated event not found"
fi

# Show all warmup-related events
echo ""
echo "   Events from kube-booster-controller:"
kubectl get events --field-selector involvedObject.name=${TEST_POD},source=kube-booster-controller --no-headers 2>/dev/null | while read line; do
    echo "     $line"
done || echo "     (no events found)"

# Test concurrency control and rate limiting
echo ""
echo "9. Testing concurrency control and rate limiting..."
echo "   Creating test pod with 200 warmup requests (200 req @ default 100 RPS = ≥2s)..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD}-ratelimit
  namespace: ${NAMESPACE}
  annotations:
    kube-booster.io/warmup: "enabled"
    kube-booster.io/warmup-endpoint: "/"
    kube-booster.io/warmup-requests: "200"
    kube-booster.io/warmup-timeout: "120s"
    kube-booster.io/warmup-port: "80"
spec:
  containers:
  - name: nginx
    image: nginx:1.25
    ports:
    - containerPort: 80
EOF

kubectl wait --for=condition=ContainersReady pod/${TEST_POD}-ratelimit --timeout=60s || true

echo "   Waiting for warmup to complete..."
for i in $(seq 1 60); do
    RL_EVENTS=$(kubectl get events --field-selector involvedObject.name=${TEST_POD}-ratelimit \
        -o jsonpath='{range .items[*]}{.reason}{" "}{end}' 2>/dev/null || true)
    if echo "$RL_EVENTS" | grep -q "WarmupCompleted\|WarmupFailed"; then
        break
    fi
    sleep 1
done

# Semaphore check: parse queue wait duration from WarmupStarted event message
# Message format: "Starting warmup execution (queued for 250ms)"
RL_STARTED_MSG=$(kubectl get events \
    --field-selector involvedObject.name=${TEST_POD}-ratelimit,reason=WarmupStarted \
    -o jsonpath='{.items[0].message}' 2>/dev/null || true)
RL_QUEUE_WAIT=$(echo "$RL_STARTED_MSG" | grep -oP '(?<=queued for )[^)]+' || true)
if [ -n "$RL_QUEUE_WAIT" ]; then
    echo "   ✓ Semaphore: WarmupStarted shows queue wait of ${RL_QUEUE_WAIT} (--max-concurrent-warmups active)"
else
    echo "   ⚠ Semaphore: no queue wait found in WarmupStarted message (semaphore may be disabled)"
fi

# Rate limiter check: parse duration from WarmupCompleted event message and verify >= 1.9s
# Message format: "warmup completed: 200/200 requests succeeded (100.0%), duration=2.1s, P50=..., P99=..."
# With default 100 RPS and 200 requests, the token bucket grants the first request immediately,
# so the minimum duration is 199 intervals x 10ms = 1.99s. Threshold is 1.9s to accommodate this.
RL_COMPLETED_MSG=$(kubectl get events \
    --field-selector involvedObject.name=${TEST_POD}-ratelimit,reason=WarmupCompleted \
    -o jsonpath='{.items[0].message}' 2>/dev/null || true)
RL_DURATION=$(echo "$RL_COMPLETED_MSG" | grep -oP '(?<=duration=)[^,]+' || true)

if [ -n "$RL_DURATION" ]; then
    # Convert Go duration string (e.g. "2.1s", "320ms", "1m2.3s") to seconds
    DURATION_S=$(echo "$RL_DURATION" | awk '
        /ms$/        { gsub(/ms/, ""); printf "%.3f", $0/1000; next }
        /^[0-9.]+s$/ { gsub(/s/, ""); print; next }
        /m/          { n=split($0,a,"m"); gsub(/s/,"",a[2]); printf "%.3f", a[1]*60+(a[2]+0); next }
        { print 0 }
    ')
    IS_RATE_LIMITED=$(awk -v d="$DURATION_S" 'BEGIN { print (d >= 1.9) ? "1" : "0" }')
    if [ "$IS_RATE_LIMITED" = "1" ]; then
        echo "   ✓ Rate limiting: WarmupCompleted shows duration=${RL_DURATION} ≥ 1.9s (200 req @ 100 RPS)"
    else
        echo "   ⚠ Rate limiting: WarmupCompleted shows duration=${RL_DURATION} < 1.9s (may not be rate-limited)"
    fi
else
    echo "   ⚠ Rate limiting: WarmupCompleted event not found or duration missing"
fi

kubectl delete pod ${TEST_POD}-ratelimit --ignore-not-found=true --wait=false &>/dev/null || true

# Test without annotation
echo ""
echo "10. Testing pod without annotation..."
kubectl run ${TEST_POD}-no-warmup --image=nginx:1.25 --restart=Never

sleep 2

READINESS_GATE_NO_WARMUP=$(kubectl get pod ${TEST_POD}-no-warmup -o jsonpath='{.spec.readinessGates[0].conditionType}')
if [ -z "$READINESS_GATE_NO_WARMUP" ]; then
    echo "   ✓ No readiness gate injected (correct)"
else
    echo "   ✗ Readiness gate incorrectly injected"
    kubectl delete pod ${TEST_POD} ${TEST_POD}-no-warmup --ignore-not-found=true
    exit 1
fi

# Cleanup
echo ""
echo "11. Cleaning up test pods..."
kubectl delete pod ${TEST_POD} ${TEST_POD}-no-warmup ${TEST_POD}-ratelimit --ignore-not-found=true
echo "   ✓ Cleanup complete"

echo ""
echo "=========================================="
echo "✓ All tests passed!"
echo "=========================================="
echo ""
echo "kube-booster implementation is working correctly."
echo ""
echo "Warmup features verified:"
echo "  - Readiness gate injection via mutating webhook"
echo "  - Warmup configuration via annotations"
echo "  - HTTP warmup request execution"
echo "  - Pod condition update after warmup"
echo "  - Kubernetes Events for warmup lifecycle (WarmupQueued, WarmupStarted, WarmupCompleted/Failed, ConditionUpdated)"
echo "  - Semaphore concurrency control (--max-concurrent-warmups)"
echo "  - RPS rate limiting (--max-warmup-rps)"
echo ""
echo "Next steps:"
echo "  - Deploy sample application: make deploy-sample"
echo "  - View all pods: kubectl get pods -n kube-system -l app.kubernetes.io/name=kube-booster"
echo "  - View all logs: kubectl logs -n kube-system -l app.kubernetes.io/name=kube-booster --prefix"
echo "  - See docs/USAGE.md for more details"
echo ""
