package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// WarmupTotal is a counter tracking total warmup executions
	WarmupTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_booster_warmup_total",
			Help: "Total warmup executions",
		},
		[]string{"namespace", "result"},
	)

	// WarmupRequestsTotal is a counter tracking total HTTP requests sent during warmup
	WarmupRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kube_booster_warmup_requests_total",
			Help: "Total HTTP requests sent during warmup",
		},
		[]string{"namespace"},
	)

	// WarmupDurationSeconds is a histogram tracking time from warmup start to completion
	WarmupDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_booster_warmup_duration_seconds",
			Help:    "Time from warmup start to completion",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace"},
	)

	// PodsPendingWarmup is a gauge tracking pods currently waiting for warmup
	PodsPendingWarmup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_booster_pods_pending_warmup",
			Help: "Pods currently waiting for warmup",
		},
		[]string{"namespace", "node"},
	)

	// WarmupQueueWaitSeconds is a histogram tracking time pods wait for the warmup semaphore
	WarmupQueueWaitSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_booster_warmup_queue_wait_seconds",
			Help:    "Time pods wait for the warmup semaphore before execution begins",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		WarmupTotal,
		WarmupRequestsTotal,
		WarmupDurationSeconds,
		PodsPendingWarmup,
		WarmupQueueWaitSeconds,
	)
}

// RecordWarmupResult records the outcome of a warmup execution
func RecordWarmupResult(namespace string, success bool, durationSeconds float64) {
	result := "failure"
	if success {
		result = "success"
	}
	WarmupTotal.WithLabelValues(namespace, result).Inc()
	WarmupDurationSeconds.WithLabelValues(namespace).Observe(durationSeconds)
}

// RecordWarmupRequests records the number of HTTP requests sent during warmup
func RecordWarmupRequests(namespace string, count int) {
	WarmupRequestsTotal.WithLabelValues(namespace).Add(float64(count))
}

// IncrementPodsPendingWarmup increments the pending pods gauge for a namespace/node
func IncrementPodsPendingWarmup(namespace, node string) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Inc()
}

// DecrementPodsPendingWarmup decrements the pending pods gauge for a namespace/node
func DecrementPodsPendingWarmup(namespace, node string) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Dec()
}

// SetPodsPendingWarmup sets the pending pods gauge to a specific value.
// Exported for use in tests to set up initial gauge values.
func SetPodsPendingWarmup(namespace, node string, count float64) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Set(count)
}

// RecordWarmupQueueWait records the time a pod waited for the warmup semaphore.
func RecordWarmupQueueWait(namespace string, seconds float64) {
	WarmupQueueWaitSeconds.WithLabelValues(namespace).Observe(seconds)
}
