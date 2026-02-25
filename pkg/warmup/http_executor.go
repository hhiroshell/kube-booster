package warmup

import (
	"context"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/go-logr/logr"
)

// Executor executes warmup requests
type Executor interface {
	Execute(ctx context.Context, config *Config) *Result
}

// HTTPExecutor fires warmup requests back-to-back (ASAP model) using net/http.
type HTTPExecutor struct {
	logger logr.Logger
	client *http.Client
}

// NewHTTPExecutor creates a new HTTPExecutor
func NewHTTPExecutor(logger logr.Logger) *HTTPExecutor {
	return &HTTPExecutor{
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Execute performs warmup requests back-to-back as fast as possible
func (e *HTTPExecutor) Execute(ctx context.Context, config *Config) *Result {
	result := &Result{}

	// Validate config
	if config.PodIP == "" {
		result.Error = ErrNoPodIP
		result.Message = "cannot execute warmup: pod IP not set"
		return result
	}

	// Build endpoint URL
	endpoint := config.BuildEndpointURL()

	e.logger.V(1).Info("starting warmup",
		"pod", config.PodName,
		"namespace", config.PodNamespace,
		"endpoint", endpoint,
		"requestCount", config.RequestCount,
		"timeout", config.Timeout)

	// Create context with timeout
	warmupCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	start := time.Now()
	latencies := make([]time.Duration, 0, config.RequestCount)
	successCount := 0
	failCount := 0

	for i := 0; i < config.RequestCount; i++ {
		if warmupCtx.Err() != nil {
			break
		}

		reqStart := time.Now()
		req, err := http.NewRequestWithContext(warmupCtx, http.MethodGet, endpoint, nil)
		if err != nil {
			failCount++
			continue
		}
		req.Header.Set("User-Agent", "kube-booster/1.0")
		req.Header.Set("X-Warmup-Request", "true")

		resp, err := e.client.Do(req)
		latency := time.Since(reqStart)

		if err != nil {
			failCount++
			// If context was cancelled/timed out, stop sending requests
			if warmupCtx.Err() != nil {
				break
			}
			continue
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			e.logger.V(2).Info("failed to drain response body", "error", err)
		}
		if err := resp.Body.Close(); err != nil {
			e.logger.V(2).Info("failed to close response body", "error", err)
		}

		// Record latency for all completed HTTP round-trips regardless of status code.
		// Even 4xx/5xx responses exercise the application's request handling path
		// (class loading, cache warming, JIT compilation), which is the goal of warmup.
		latencies = append(latencies, latency)

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			successCount++
		} else {
			failCount++
		}
	}

	totalDuration := time.Since(start)

	// Record context error if warmup was cancelled or timed out
	if warmupCtx.Err() != nil {
		result.Error = warmupCtx.Err()
	}

	// Calculate percentiles
	p50, p99 := calculatePercentiles(latencies)

	result.RequestsCompleted = successCount
	result.RequestsFailed = failCount
	result.TotalDuration = totalDuration
	result.LatencyP50 = p50
	result.LatencyP99 = p99
	result.Success = successCount > 0
	result.Message = result.BuildMessage()

	e.logger.V(1).Info("warmup completed",
		"pod", config.PodName,
		"namespace", config.PodNamespace,
		"success", result.Success,
		"requestsCompleted", successCount,
		"requestsFailed", failCount,
		"latencyP50", p50,
		"latencyP99", p99,
		"duration", totalDuration)

	return result
}

// calculatePercentiles computes P50 and P99 from a slice of latencies.
func calculatePercentiles(latencies []time.Duration) (p50, p99 time.Duration) {
	n := len(latencies)
	if n == 0 {
		return 0, 0
	}
	slices.Sort(latencies)

	// Nearest-rank method: index = max(0, ceil(n*p/100) - 1)
	p50idx := (n*50 + 99) / 100
	if p50idx > 0 {
		p50idx--
	}
	p50 = latencies[p50idx]

	p99idx := (n*99 + 99) / 100
	if p99idx > 0 {
		p99idx--
	}
	if p99idx >= n {
		p99idx = n - 1
	}
	p99 = latencies[p99idx]
	return
}

// Error definitions
var (
	ErrNoPodIP = &WarmupError{msg: "pod IP not set"}
)

// WarmupError represents a warmup-specific error
type WarmupError struct {
	msg string
}

func (e *WarmupError) Error() string {
	return e.msg
}

// Ensure HTTPExecutor implements Executor
var _ Executor = (*HTTPExecutor)(nil)
