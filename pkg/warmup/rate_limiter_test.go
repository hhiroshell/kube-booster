package warmup

import (
	"context"
	"testing"
	"time"
)

func TestNewRequestRateLimiter_ZeroRPS(t *testing.T) {
	rl := NewRequestRateLimiter(0)
	if rl != nil {
		t.Error("NewRequestRateLimiter(0) should return nil")
	}
}

func TestNewRequestRateLimiter_NegativeRPS(t *testing.T) {
	rl := NewRequestRateLimiter(-1)
	if rl != nil {
		t.Error("NewRequestRateLimiter(-1) should return nil")
	}
}

func TestRequestRateLimiter_NilReceiver_Wait(t *testing.T) {
	var rl *RequestRateLimiter
	err := rl.Wait(context.Background())
	if err != nil {
		t.Errorf("nil receiver Wait() should return nil, got %v", err)
	}
}

func TestRequestRateLimiter_Throttles(t *testing.T) {
	// 2 RPS limiter: tokens arrive every 500ms
	rl := NewRequestRateLimiter(2)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(2) returned nil")
	}

	// First Wait should succeed immediately (burst=1 grants first token instantly)
	ctx := context.Background()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	// Second Wait should take ~500ms
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("second Wait() error = %v", err)
	}
	elapsed := time.Since(start)

	// Allow a generous lower bound to avoid flakiness on slow CI
	if elapsed < 200*time.Millisecond {
		t.Errorf("second Wait() returned in %v, expected throttling (~500ms)", elapsed)
	}
}

func TestRequestRateLimiter_ContextCancellation(t *testing.T) {
	// Very low RPS so the Wait would block a long time
	rl := NewRequestRateLimiter(0.001)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(0.001) returned nil")
	}

	// Consume the burst token
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("initial Wait() error = %v", err)
	}

	// Now cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("Wait() with cancelled context should return error")
	}
}
