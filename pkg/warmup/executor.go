package warmup

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// Executor executes warmup requests
type Executor interface {
	Execute(ctx context.Context, config *Config) *Result
}

// VegetaExecutor implements Executor using the Vegeta load testing library
type VegetaExecutor struct {
	logger logr.Logger
}

// NewVegetaExecutor creates a new VegetaExecutor
func NewVegetaExecutor(logger logr.Logger) *VegetaExecutor {
	return &VegetaExecutor{logger: logger}
}

// Execute performs warmup requests using Vegeta
func (e *VegetaExecutor) Execute(ctx context.Context, config *Config) *Result {
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
		"duration", config.Duration)

	// Create target
	target := vegeta.Target{
		Method: "GET",
		URL:    endpoint,
		Header: map[string][]string{
			"User-Agent":       {"kube-booster/1.0"},
			"X-Warmup-Request": {"true"},
		},
	}
	targeter := vegeta.NewStaticTargeter(target)

	// Calculate rate: spread RequestCount requests over Duration
	// Using a steady rate to avoid overwhelming the application
	rate := vegeta.Rate{Freq: config.RequestCount, Per: config.Duration}

	// Calculate per-request timeout
	// Give each request enough time, but cap at a reasonable value
	perRequestTimeout := config.Duration / time.Duration(config.RequestCount)
	if perRequestTimeout < time.Second {
		perRequestTimeout = time.Second
	}
	if perRequestTimeout > 10*time.Second {
		perRequestTimeout = 10 * time.Second
	}

	// Create attacker with timeout
	attacker := vegeta.NewAttacker(vegeta.Timeout(perRequestTimeout))

	// Execute attack and collect metrics
	var metrics vegeta.Metrics
	attackDone := make(chan struct{})

	go func() {
		defer close(attackDone)
		for res := range attacker.Attack(targeter, rate, config.Duration, "warmup") {
			metrics.Add(res)
		}
	}()

	// Wait for completion or context cancellation
	select {
	case <-ctx.Done():
		attacker.Stop()
		<-attackDone // Wait for attack goroutine to finish
		metrics.Close()
		result.Error = ctx.Err()
		result.Message = "warmup cancelled"
		e.logger.V(1).Info("warmup cancelled",
			"pod", config.PodName,
			"reason", ctx.Err())
		return result
	case <-attackDone:
		// Attack completed normally
	}

	metrics.Close()

	// Populate result from metrics
	totalRequests := int(metrics.Requests)
	successfulRequests := int(float64(metrics.Requests) * metrics.Success)
	failedRequests := totalRequests - successfulRequests

	result.RequestsCompleted = successfulRequests
	result.RequestsFailed = failedRequests
	result.TotalDuration = metrics.Duration
	result.LatencyP50 = metrics.Latencies.P50
	result.LatencyP99 = metrics.Latencies.P99
	result.Success = metrics.Success > 0 // At least some requests succeeded
	result.Message = result.BuildMessage()

	e.logger.V(1).Info("warmup completed",
		"pod", config.PodName,
		"namespace", config.PodNamespace,
		"success", result.Success,
		"requests", totalRequests,
		"successRate", metrics.Success,
		"latencyP50", metrics.Latencies.P50,
		"latencyP99", metrics.Latencies.P99,
		"duration", metrics.Duration)

	return result
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
