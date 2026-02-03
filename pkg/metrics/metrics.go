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

	// WarmupRequestLatencySeconds is a histogram tracking individual request latency during warmup
	WarmupRequestLatencySeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kube_booster_warmup_request_latency_seconds",
			Help:    "Individual request latency during warmup",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
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
)

func init() {
	metrics.Registry.MustRegister(
		WarmupTotal,
		WarmupRequestsTotal,
		WarmupDurationSeconds,
		WarmupRequestLatencySeconds,
		PodsPendingWarmup,
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

// RecordRequestLatency records an individual request latency observation
func RecordRequestLatency(namespace string, latencySeconds float64) {
	WarmupRequestLatencySeconds.WithLabelValues(namespace).Observe(latencySeconds)
}

// IncrementPodsPendingWarmup increments the pending pods gauge for a namespace/node
func IncrementPodsPendingWarmup(namespace, node string) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Inc()
}

// DecrementPodsPendingWarmup decrements the pending pods gauge for a namespace/node
func DecrementPodsPendingWarmup(namespace, node string) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Dec()
}

// SetPodsPendingWarmup sets the pending pods gauge to a specific value
func SetPodsPendingWarmup(namespace, node string, count float64) {
	PodsPendingWarmup.WithLabelValues(namespace, node).Set(count)
}
