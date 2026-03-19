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
	// 10 RPS → burst=10: all 10 tokens should be available immediately
	rl := NewRequestRateLimiter(10)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(10) returned nil")
	}
	for i := range 10 {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		err := rl.Wait(ctx)
		cancel()
		if err != nil {
			t.Fatalf("Wait() #%d timed out, expected burst of 10", i+1)
		}
	}
	// 11th token requires refill (~100ms at 10 RPS); must block within 50ms timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Error("Wait() #11 returned immediately, expected throttling beyond burst")
	}
}

func TestNewRequestRateLimiter_FractionalRPS_BurstAtLeastOne(t *testing.T) {
	// 0.5 RPS → burst=1: first token available immediately, second must wait ~2s
	rl := NewRequestRateLimiter(0.5)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(0.5) returned nil")
	}
	ctx1, cancel1 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel1()
	if err := rl.Wait(ctx1); err != nil {
		t.Fatal("first Wait() timed out, expected burst of at least 1")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if err := rl.Wait(ctx2); err == nil {
		t.Error("second Wait() returned immediately, expected throttling beyond burst=1")
	}
}

func TestNewRequestRateLimiter_LargeRPS_NoPanic(t *testing.T) {
	// Very large float64 must not cause integer overflow or panic in rate.NewLimiter
	rl := NewRequestRateLimiter(1e19)
	if rl == nil {
		t.Fatal("NewRequestRateLimiter(1e19) returned nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err != nil && ctx.Err() == nil {
		t.Fatalf("Wait() returned unexpected error: %v", err)
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
