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

	// WarmupDurationSeconds is a histogram tracking time from warmup start to completion.
	// Buckets extend to 120s so histogram_quantile gives accurate results up to and beyond
	// the KubeBoosterSlowWarmup alert threshold of 60s.
	WarmupDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_booster_warmup_duration_seconds",
			Help:    "Time from warmup start to completion",
			Buckets: []float64{.5, 1, 2.5, 5, 10, 15, 30, 45, 60, 90, 120},
		},
		[]string{"namespace"},
	)

	// WarmupActivePods is a gauge tracking pods currently executing warmup requests
	WarmupActivePods = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kube_booster_warmup_active_pods",
			Help: "Pods currently executing warmup requests",
		},
		[]string{"namespace", "node"},
	)

	// WarmupQueueWaitSeconds is a histogram tracking time pods wait for the warmup semaphore.
	// Custom buckets extend up to 300s (5 minutes) to cover high-contention scenarios where
	// a pod waits behind multiple long-running warmups at a rate-limited cluster.
	WarmupQueueWaitSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_booster_warmup_queue_wait_seconds",
			Help:    "Time pods wait for the warmup semaphore before execution begins",
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 20, 30, 60, 120, 300},
		},
		[]string{"namespace"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		WarmupTotal,
		WarmupRequestsTotal,
		WarmupDurationSeconds,
		WarmupActivePods,
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

// IncrementWarmupActivePods increments the active warmup pods gauge for a namespace/node
func IncrementWarmupActivePods(namespace, node string) {
	WarmupActivePods.WithLabelValues(namespace, node).Inc()
}

// DecrementWarmupActivePods decrements the active warmup pods gauge for a namespace/node
func DecrementWarmupActivePods(namespace, node string) {
	WarmupActivePods.WithLabelValues(namespace, node).Dec()
}

// SetWarmupActivePods sets the active warmup pods gauge to a specific value.
// Exported for use in tests to set up initial gauge values.
func SetWarmupActivePods(namespace, node string, count float64) {
	WarmupActivePods.WithLabelValues(namespace, node).Set(count)
}

// RecordWarmupQueueWait records the time a pod waited for the warmup semaphore.
func RecordWarmupQueueWait(namespace string, seconds float64) {
	WarmupQueueWaitSeconds.WithLabelValues(namespace).Observe(seconds)
}
