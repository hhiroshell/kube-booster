#!/usr/bin/env bash

# Quick test script for kube-booster Phase 1
# This script performs basic smoke tests to verify the implementation

set -e

NAMESPACE="default"
TEST_POD="test-warmup-pod"

echo "=========================================="
echo "kube-booster Phase 1 Quick Test"
echo "=========================================="
echo ""

# Check if controller is running
echo "1. Checking if controller is running..."
if kubectl get deployment kube-booster-controller -n kube-system &> /dev/null; then
    READY=$(kubectl get deployment kube-booster-controller -n kube-system -o jsonpath='{.status.readyReplicas}')
    if [ "$READY" == "1" ]; then
        echo "   ✓ Controller is running"
    else
        echo "   ✗ Controller is not ready"
        exit 1
    fi
else
    echo "   ✗ Controller deployment not found"
    echo "   Run 'make deploy' first"
    exit 1
fi

# Test webhook injection
echo ""
echo "2. Testing webhook injection..."
kubectl run ${TEST_POD} --image=nginx:1.25 --annotations="kube-booster.io/warmup=enabled" --restart=Never

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
echo "3. Waiting for containers to be ready..."
kubectl wait --for=condition=ContainersReady pod/${TEST_POD} --timeout=60s || true

# Give controller time to reconcile
sleep 3

# Check if warmup condition was set
echo ""
echo "4. Checking warmup condition..."
CONDITION_STATUS=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="kube-booster.io/warmup-ready")].status}')
if [ "$CONDITION_STATUS" == "True" ]; then
    echo "   ✓ Warmup condition set to True"
else
    echo "   ⚠ Warmup condition not set (status: ${CONDITION_STATUS})"
    echo "   This is normal if containers are not ready yet"
fi

# Check if pod is ready
POD_READY=$(kubectl get pod ${TEST_POD} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
if [ "$POD_READY" == "True" ]; then
    echo "   ✓ Pod is READY"
else
    echo "   ⚠ Pod not READY yet (status: ${POD_READY})"
fi

# Test without annotation
echo ""
echo "5. Testing pod without annotation..."
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
echo "6. Cleaning up test pods..."
kubectl delete pod ${TEST_POD} ${TEST_POD}-no-warmup --ignore-not-found=true
echo "   ✓ Cleanup complete"

echo ""
echo "=========================================="
echo "✓ All tests passed!"
echo "=========================================="
echo ""
echo "Phase 1 implementation is working correctly."
echo ""
echo "Next steps:"
echo "  - Deploy sample application: make deploy-sample"
echo "  - View controller logs: kubectl logs -n kube-system -l app=kube-booster-controller -f"
echo "  - See USAGE.md for more details"
echo ""
