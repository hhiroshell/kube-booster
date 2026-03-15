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
	// 2 RPS limiter: burst=2, so first two tokens are granted immediately
	rl := NewRequestRateLimiter(2)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(2) returned nil")
	}

	ctx := context.Background()

	// First two Waits should succeed immediately (burst=2 for 2 RPS)
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("second Wait() error = %v", err)
	}

	// Third Wait should take ~500ms (next token arrives after 1/rps seconds)
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatalf("third Wait() error = %v", err)
	}
	elapsed := time.Since(start)

	// Allow a generous lower bound to avoid flakiness on slow CI
	if elapsed < 200*time.Millisecond {
		t.Errorf("third Wait() returned in %v, expected throttling (~500ms)", elapsed)
	}
}

func TestNewRequestRateLimiter_BurstEqualsRPS(t *testing.T) {
	rl := NewRequestRateLimiter(10)
	if rl.limiter.Burst() != 10 {
		t.Errorf("burst = %d, want 10", rl.limiter.Burst())
	}
}

func TestNewRequestRateLimiter_FractionalRPS_BurstAtLeastOne(t *testing.T) {
	rl := NewRequestRateLimiter(0.5)
	if rl.limiter.Burst() != 1 {
		t.Errorf("burst = %d, want 1", rl.limiter.Burst())
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
