package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordWarmupResult_Success(t *testing.T) {
	// Reset metrics before test
	WarmupTotal.Reset()
	WarmupDurationSeconds.Reset()

	RecordWarmupResult("default", true, 5.0)

	// Verify counter incremented
	count := testutil.ToFloat64(WarmupTotal.WithLabelValues("default", "success"))
	if count != 1 {
		t.Errorf("expected warmup_total{namespace=default,result=success} = 1, got %f", count)
	}

	// Verify failure counter is 0
	failureCount := testutil.ToFloat64(WarmupTotal.WithLabelValues("default", "failure"))
	if failureCount != 0 {
		t.Errorf("expected warmup_total{namespace=default,result=failure} = 0, got %f", failureCount)
	}
}

func TestRecordWarmupResult_Failure(t *testing.T) {
	WarmupTotal.Reset()
	WarmupDurationSeconds.Reset()

	RecordWarmupResult("kube-system", false, 10.0)

	count := testutil.ToFloat64(WarmupTotal.WithLabelValues("kube-system", "failure"))
	if count != 1 {
		t.Errorf("expected warmup_total{namespace=kube-system,result=failure} = 1, got %f", count)
	}

	// Verify success counter is 0
	successCount := testutil.ToFloat64(WarmupTotal.WithLabelValues("kube-system", "success"))
	if successCount != 0 {
		t.Errorf("expected warmup_total{namespace=kube-system,result=success} = 0, got %f", successCount)
	}
}

func TestRecordWarmupResult_MultipleNamespaces(t *testing.T) {
	WarmupTotal.Reset()
	WarmupDurationSeconds.Reset()

	RecordWarmupResult("ns1", true, 1.0)
	RecordWarmupResult("ns2", true, 2.0)
	RecordWarmupResult("ns1", false, 3.0)

	// ns1 should have 1 success and 1 failure
	ns1Success := testutil.ToFloat64(WarmupTotal.WithLabelValues("ns1", "success"))
	ns1Failure := testutil.ToFloat64(WarmupTotal.WithLabelValues("ns1", "failure"))
	if ns1Success != 1 || ns1Failure != 1 {
		t.Errorf("expected ns1 success=1, failure=1, got success=%f, failure=%f", ns1Success, ns1Failure)
	}

	// ns2 should have 1 success
	ns2Success := testutil.ToFloat64(WarmupTotal.WithLabelValues("ns2", "success"))
	if ns2Success != 1 {
		t.Errorf("expected ns2 success=1, got %f", ns2Success)
	}
}

func TestRecordWarmupRequests(t *testing.T) {
	WarmupRequestsTotal.Reset()

	RecordWarmupRequests("default", 100)

	count := testutil.ToFloat64(WarmupRequestsTotal.WithLabelValues("default"))
	if count != 100 {
		t.Errorf("expected warmup_requests_total = 100, got %f", count)
	}
}

func TestRecordWarmupRequests_Accumulates(t *testing.T) {
	WarmupRequestsTotal.Reset()

	RecordWarmupRequests("default", 50)
	RecordWarmupRequests("default", 30)

	count := testutil.ToFloat64(WarmupRequestsTotal.WithLabelValues("default"))
	if count != 80 {
		t.Errorf("expected warmup_requests_total = 80, got %f", count)
	}
}

func TestRecordRequestLatency(t *testing.T) {
	WarmupRequestLatencySeconds.Reset()

	RecordRequestLatency("default", 0.05)
	RecordRequestLatency("default", 0.1)

	// Verify histogram has observations
	count := testutil.CollectAndCount(WarmupRequestLatencySeconds)
	if count == 0 {
		t.Error("expected histogram to have observations")
	}
}

func TestSetPodsPendingWarmup(t *testing.T) {
	PodsPendingWarmup.Reset()

	SetPodsPendingWarmup("default", "node-1", 5)

	count := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("default", "node-1"))
	if count != 5 {
		t.Errorf("expected pods_pending_warmup = 5, got %f", count)
	}
}

func TestIncrementPodsPendingWarmup(t *testing.T) {
	PodsPendingWarmup.Reset()

	IncrementPodsPendingWarmup("default", "node-1")
	IncrementPodsPendingWarmup("default", "node-1")

	count := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("default", "node-1"))
	if count != 2 {
		t.Errorf("expected pods_pending_warmup = 2, got %f", count)
	}
}

func TestDecrementPodsPendingWarmup(t *testing.T) {
	PodsPendingWarmup.Reset()

	SetPodsPendingWarmup("default", "node-1", 5)
	DecrementPodsPendingWarmup("default", "node-1")

	count := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("default", "node-1"))
	if count != 4 {
		t.Errorf("expected pods_pending_warmup = 4, got %f", count)
	}
}

func TestPodsPendingWarmup_MultipleNodes(t *testing.T) {
	PodsPendingWarmup.Reset()

	SetPodsPendingWarmup("default", "node-1", 3)
	SetPodsPendingWarmup("default", "node-2", 2)
	SetPodsPendingWarmup("kube-system", "node-1", 1)

	node1Default := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("default", "node-1"))
	node2Default := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("default", "node-2"))
	node1KubeSystem := testutil.ToFloat64(PodsPendingWarmup.WithLabelValues("kube-system", "node-1"))

	if node1Default != 3 {
		t.Errorf("expected default/node-1 = 3, got %f", node1Default)
	}
	if node2Default != 2 {
		t.Errorf("expected default/node-2 = 2, got %f", node2Default)
	}
	if node1KubeSystem != 1 {
		t.Errorf("expected kube-system/node-1 = 1, got %f", node1KubeSystem)
	}
}
