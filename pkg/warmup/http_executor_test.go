package warmup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestHTTPExecutor_Execute(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	tests := []struct {
		name           string
		config         *Config
		serverHandler  http.HandlerFunc
		wantSuccess    bool
		wantErrContain string
	}{
		{
			name: "successful warmup",
			config: &Config{
				Endpoint:     "/warmup",
				RequestCount: 3,
				Timeout:      5 * time.Second,
				PodName:      "test-pod",
				PodNamespace: "default",
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Warmup-Request") != "true" {
					t.Error("expected X-Warmup-Request header")
				}
				if r.Header.Get("User-Agent") != "kube-booster/1.0" {
					t.Error("expected User-Agent header")
				}
				w.WriteHeader(http.StatusOK)
			},
			wantSuccess: true,
		},
		{
			name: "warmup with server errors",
			config: &Config{
				Endpoint:     "/warmup",
				RequestCount: 3,
				Timeout:      5 * time.Second,
				PodName:      "test-pod",
				PodNamespace: "default",
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantSuccess: false,
		},
		{
			name: "no pod IP",
			config: &Config{
				Endpoint:     "/warmup",
				RequestCount: 3,
				Timeout:      5 * time.Second,
				PodIP:        "", // No IP set
				PodName:      "test-pod",
				PodNamespace: "default",
			},
			wantSuccess:    false,
			wantErrContain: "pod IP not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.serverHandler != nil {
				server = httptest.NewServer(tt.serverHandler)
				defer server.Close()

				// Extract host and port from test server
				addr := server.Listener.Addr().String()
				parts := strings.Split(addr, ":")
				if len(parts) == 2 {
					tt.config.PodIP = parts[0]
					tt.config.Port = parsePort(parts[1])
				}
			}

			executor := NewHTTPExecutor(logger)
			result := executor.Execute(context.Background(), tt.config)

			if tt.wantErrContain != "" {
				if result.Error == nil {
					t.Errorf("Execute() expected error containing %q, got nil", tt.wantErrContain)
					return
				}
				if !strings.Contains(result.Error.Error(), tt.wantErrContain) &&
					!strings.Contains(result.Message, tt.wantErrContain) {
					t.Errorf("Execute() error = %v, message = %v, want containing %q",
						result.Error, result.Message, tt.wantErrContain)
				}
				return
			}

			if tt.wantSuccess && !result.Success {
				t.Errorf("Execute() Success = %v, want true. Message: %s", result.Success, result.Message)
			}
			if !tt.wantSuccess && result.Success && result.Error == nil {
				t.Errorf("Execute() Success = %v, want false", result.Success)
			}
		})
	}
}

func TestHTTPExecutor_Execute_ContextCancellation(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := server.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	config := &Config{
		Endpoint:     "/",
		RequestCount: 10,
		Timeout:      30 * time.Second,
		PodIP:        parts[0],
		Port:         parsePort(parts[1]),
		PodName:      "test-pod",
		PodNamespace: "default",
	}

	executor := NewHTTPExecutor(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result := executor.Execute(ctx, config)

	if result.Error == nil {
		t.Error("Execute() expected context error, got nil")
	}
	if result.Error != context.DeadlineExceeded {
		t.Errorf("Execute() error = %v, want context.DeadlineExceeded", result.Error)
	}
}

func TestHTTPExecutor_Execute_MetricsCollection(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond) // Small delay for latency measurement
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := server.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	config := &Config{
		Endpoint:     "/",
		RequestCount: 5,
		Timeout:      10 * time.Second,
		PodIP:        parts[0],
		Port:         parsePort(parts[1]),
		PodName:      "test-pod",
		PodNamespace: "default",
	}

	executor := NewHTTPExecutor(logger)
	result := executor.Execute(context.Background(), config)

	if !result.Success {
		t.Errorf("Execute() Success = false, want true. Message: %s", result.Message)
	}

	if result.RequestsCompleted < 1 {
		t.Errorf("Execute() RequestsCompleted = %d, want > 0", result.RequestsCompleted)
	}

	if result.TotalDuration == 0 {
		t.Error("Execute() TotalDuration = 0, want > 0")
	}

	// P50 and P99 should be set
	if result.LatencyP50 == 0 {
		t.Error("Execute() LatencyP50 = 0, want > 0")
	}

	// Message should be populated
	if result.Message == "" {
		t.Error("Execute() Message is empty")
	}
}

func TestHTTPExecutor_Execute_BackToBack(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := server.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	config := &Config{
		Endpoint:     "/",
		RequestCount: 5,
		Timeout:      30 * time.Second,
		PodIP:        parts[0],
		Port:         parsePort(parts[1]),
		PodName:      "test-pod",
		PodNamespace: "default",
	}

	executor := NewHTTPExecutor(logger)
	result := executor.Execute(context.Background(), config)

	if !result.Success {
		t.Errorf("Execute() Success = false, want true. Message: %s", result.Message)
	}

	// With back-to-back requests against a fast server and 30s timeout,
	// total duration should be well under 30s
	if result.TotalDuration > 5*time.Second {
		t.Errorf("Execute() TotalDuration = %v, expected much less than timeout (ASAP model)", result.TotalDuration)
	}

	if result.RequestsCompleted != 5 {
		t.Errorf("Execute() RequestsCompleted = %d, want 5", result.RequestsCompleted)
	}
}

func TestCalculatePercentiles(t *testing.T) {
	tests := []struct {
		name      string
		latencies []time.Duration
		wantP50   time.Duration
		wantP99   time.Duration
	}{
		{
			name:      "empty slice",
			latencies: nil,
			wantP50:   0,
			wantP99:   0,
		},
		{
			name:      "single element",
			latencies: []time.Duration{100 * time.Millisecond},
			wantP50:   100 * time.Millisecond,
			wantP99:   100 * time.Millisecond,
		},
		{
			name: "known values",
			latencies: []time.Duration{
				10 * time.Millisecond,
				20 * time.Millisecond,
				30 * time.Millisecond,
				40 * time.Millisecond,
				50 * time.Millisecond,
				60 * time.Millisecond,
				70 * time.Millisecond,
				80 * time.Millisecond,
				90 * time.Millisecond,
				100 * time.Millisecond,
			},
			wantP50: 60 * time.Millisecond,  // index 5 (10*50/100=5) → 60ms
			wantP99: 100 * time.Millisecond, // index 9 (10*99/100=9)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p50, p99 := calculatePercentiles(tt.latencies)
			if p50 != tt.wantP50 {
				t.Errorf("calculatePercentiles() p50 = %v, want %v", p50, tt.wantP50)
			}
			if p99 != tt.wantP99 {
				t.Errorf("calculatePercentiles() p99 = %v, want %v", p99, tt.wantP99)
			}
		})
	}
}

func parsePort(s string) int {
	port := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		}
	}
	return port
}
