package warmup

import (
	"context"
	"math"

	"golang.org/x/time/rate"
)

// maxBurst caps the token bucket burst size to prevent integer overflow when
// converting a very large float64 rps value to int.
const maxBurst = 10_000

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
	burst := max(1, min(int(math.Round(rps)), maxBurst))
	return &RequestRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
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
