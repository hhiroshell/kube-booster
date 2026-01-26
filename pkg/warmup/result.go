package warmup

import (
	"fmt"
	"time"
)

// Result contains the outcome of a warmup execution
type Result struct {
	// Success indicates whether the warmup was successful (at least some requests succeeded)
	Success bool

	// RequestsCompleted is the number of requests that completed successfully
	RequestsCompleted int

	// RequestsFailed is the number of requests that failed
	RequestsFailed int

	// TotalDuration is the total time taken for the warmup
	TotalDuration time.Duration

	// LatencyP50 is the 50th percentile latency
	LatencyP50 time.Duration

	// LatencyP99 is the 99th percentile latency
	LatencyP99 time.Duration

	// Error contains any error that occurred during warmup
	Error error

	// Message is a human-readable summary of the warmup result
	Message string
}

// BuildMessage creates a human-readable summary of the warmup result
func (r *Result) BuildMessage() string {
	if r.Error != nil {
		return fmt.Sprintf("warmup failed: %v", r.Error)
	}

	if r.RequestsCompleted == 0 && r.RequestsFailed == 0 {
		return "warmup skipped: no requests executed"
	}

	successRate := float64(r.RequestsCompleted) / float64(r.RequestsCompleted+r.RequestsFailed) * 100

	if r.Success {
		return fmt.Sprintf("warmup completed: %d/%d requests succeeded (%.1f%%), P50=%v, P99=%v",
			r.RequestsCompleted,
			r.RequestsCompleted+r.RequestsFailed,
			successRate,
			r.LatencyP50,
			r.LatencyP99)
	}

	return fmt.Sprintf("warmup completed with failures: %d/%d requests succeeded (%.1f%%)",
		r.RequestsCompleted,
		r.RequestsCompleted+r.RequestsFailed,
		successRate)
}
