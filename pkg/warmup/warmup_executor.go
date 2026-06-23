package warmup

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/go-logr/logr"
)

// Executor executes warmup requests
type Executor interface {
	Execute(ctx context.Context, config *Config) *Result
}

// WarmupExecutorOption is a functional option for WarmupExecutor.
type WarmupExecutorOption func(*WarmupExecutor)

// WithRateLimiter sets the rate limiter on the executor.
// Passing nil (e.g., when --max-warmup-rps <= 0) is safe and disables rate limiting —
// equivalent to not calling this option at all.
func WithRateLimiter(rl *RequestRateLimiter) WarmupExecutorOption {
	return func(e *WarmupExecutor) {
		e.rateLimiter = rl
	}
}

// WarmupExecutor fires warmup requests back-to-back (ASAP model), dispatching to the
// appropriate Sender based on the configured protocol (HTTP or gRPC).
type WarmupExecutor struct {
	logger      logr.Logger
	client      *http.Client
	rateLimiter *RequestRateLimiter // nil = unlimited
}

// NewWarmupExecutor creates a new WarmupExecutor.
func NewWarmupExecutor(logger logr.Logger, opts ...WarmupExecutorOption) *WarmupExecutor {
	e := &WarmupExecutor{
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute performs warmup requests back-to-back as fast as possible.
func (e *WarmupExecutor) Execute(ctx context.Context, config *Config) *Result {
	result := &Result{}

	if config.PodIP == "" {
		result.Error = ErrNoPodIP
		result.Message = "cannot execute warmup: pod IP not set"
		return result
	}

	// Choose sender and build target based on protocol.
	var (
		sender Sender
		target Target
	)
	switch config.Protocol {
	case ProtocolGRPC:
		sender = NewGRPCSender(e.logger)
		target = Target{
			Address: config.BuildGRPCAddress(),
			Method:  config.GRPCMethod,
			Payload: []byte(config.GRPCPayload),
		}
	case ProtocolHTTP:
		sender = &HTTPSender{client: e.client, logger: e.logger}
		target = Target{
			Address: config.BuildEndpointURL(),
			Method:  http.MethodGet,
			Headers: map[string]string{
				"User-Agent":       "kube-booster/1.0",
				"X-Warmup-Request": "true",
			},
		}
	default:
		result.Error = fmt.Errorf("unsupported warmup protocol %q", config.Protocol)
		result.Message = fmt.Sprintf("cannot execute warmup: unsupported protocol %q", config.Protocol)
		return result
	}
	e.logger.V(1).Info("starting warmup",
		"pod", config.PodName,
		"namespace", config.PodNamespace,
		"protocol", config.Protocol,
		"address", target.Address,
		"method", target.Method,
		"requestCount", config.RequestCount,
		"timeout", config.Timeout)
	defer sender.Close() //nolint:errcheck

	// Create context with timeout.
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

		if err := e.rateLimiter.Wait(warmupCtx); err != nil {
			// Count remaining un-attempted requests as failed so the result message and
			// the fail-open decision in the controller reflect the full RequestCount picture.
			failCount += config.RequestCount - successCount - failCount
			break
		}

		resp := sender.Send(warmupCtx, target)

		if resp.Error != nil {
			failCount++
			if warmupCtx.Err() != nil {
				break
			}
			continue
		}

		// Record latency for all completed round-trips regardless of status code.
		// Even error responses exercise the application's request handling path.
		latencies = append(latencies, resp.Duration)

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			successCount++
		} else {
			failCount++
		}
	}

	totalDuration := time.Since(start)

	if warmupCtx.Err() != nil {
		result.Error = warmupCtx.Err()
	}

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
		"protocol", config.Protocol,
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

// Ensure WarmupExecutor implements Executor
var _ Executor = (*WarmupExecutor)(nil)
