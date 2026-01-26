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

func TestVegetaExecutor_Execute(t *testing.T) {
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

			executor := NewVegetaExecutor(logger)
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

func TestVegetaExecutor_Execute_ContextCancellation(t *testing.T) {
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

	executor := NewVegetaExecutor(logger)

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

func TestVegetaExecutor_Execute_MetricsCollection(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
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

	executor := NewVegetaExecutor(logger)
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

func parsePort(s string) int {
	port := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		}
	}
	return port
}

func TestMockExecutor(t *testing.T) {
	mock := &MockExecutor{
		Result: &Result{
			Success:           true,
			RequestsCompleted: 5,
			Message:           "test warmup",
		},
	}

	result := mock.Execute(context.Background(), &Config{})

	if !result.Success {
		t.Error("MockExecutor.Execute() Success = false, want true")
	}
	if result.RequestsCompleted != 5 {
		t.Errorf("MockExecutor.Execute() RequestsCompleted = %d, want 5", result.RequestsCompleted)
	}
}
