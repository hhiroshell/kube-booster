package warmup

import (
	"context"

	"golang.org/x/time/rate"
)

// RequestRateLimiter wraps x/time/rate.Limiter with a nil-safe design.
// A nil receiver is valid and means no rate limiting is applied.
type RequestRateLimiter struct {
	limiter *rate.Limiter
}

// NewRequestRateLimiter returns a new RequestRateLimiter capped at rps requests per second,
// or nil if rps <= 0 (unlimited).
func NewRequestRateLimiter(rps float64) *RequestRateLimiter {
	if rps <= 0 {
		return nil
	}
	return &RequestRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
	}
}

// Wait blocks until a token is available or ctx is done.
// A nil receiver is a no-op and returns nil immediately.
func (r *RequestRateLimiter) Wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.limiter.Wait(ctx)
}
