package warmup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	v1alpha1 "github.com/hhiroshell/kube-booster/pkg/api/v1alpha1"
)

func newTestConfig(podIP string, port int) *Config {
	return &Config{
		PodIP:        podIP,
		PodName:      "test-pod",
		PodNamespace: "default",
		Port:         port,
		Protocol:     ProtocolHTTP,
		RequestCount: 1,
		Timeout:      30 * time.Second,
	}
}

func TestScenarioExecutor_EmptySteps(t *testing.T) {
	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	config := newTestConfig("127.0.0.1", 8080)
	spec := &v1alpha1.WarmupConfigSpec{Steps: []v1alpha1.WarmupStep{}}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if !result.Success {
		t.Errorf("expected success for empty steps, got message: %s", result.Message)
	}
	if result.RequestsCompleted != 0 {
		t.Errorf("expected 0 completed, got %d", result.RequestsCompleted)
	}
}

func TestScenarioExecutor_NoPodIP(t *testing.T) {
	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	config := newTestConfig("", 8080)
	spec := &v1alpha1.WarmupConfigSpec{Steps: []v1alpha1.WarmupStep{{
		Requests: []v1alpha1.WarmupRequest{{Endpoint: "/"}},
	}}}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if result.Success {
		t.Error("expected failure for empty pod IP")
	}
	if result.Error == nil {
		t.Error("expected non-nil error for empty pod IP")
	}
}

func TestScenarioExecutor_SingleStep_Success(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{
			{
				Requests: []v1alpha1.WarmupRequest{
					{Endpoint: "/warmup", Method: "GET", Count: 1},
				},
			},
		},
	}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls)
	}
}

func TestScenarioExecutor_Count(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{
			{
				Requests: []v1alpha1.WarmupRequest{
					{Endpoint: "/", Count: 3},
				},
			},
		},
	}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if result.RequestsCompleted != 3 {
		t.Errorf("expected 3 completed, got %d", result.RequestsCompleted)
	}
	if calls != 3 {
		t.Errorf("expected 3 HTTP calls, got %d", calls)
	}
}

func TestScenarioExecutor_ResponseChaining(t *testing.T) {
	// Step 1: returns {"token":"secret"}; step 2 sends it as a header.
	var receivedAuth string
	step1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"secret"}`)) //nolint:errcheck // test handler
	}))
	defer step1Server.Close()

	step2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer step2Server.Close()

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))

	host1, port1 := parseTestServerAddr(t, step1Server.URL)
	host2, port2 := parseTestServerAddr(t, step2Server.URL)

	// Use step1 for extracting, step2 for verifying; we need two configs.
	// Since ScenarioExecutor uses a single config, we proxy both through one server
	// and use path routing.
	mux := http.NewServeMux()
	mux.HandleFunc("/step1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"secret"}`)) //nolint:errcheck // test handler
	})
	mux.HandleFunc("/step2", func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	combined := httptest.NewServer(mux)
	defer combined.Close()
	_ = host1
	_ = port1
	_ = host2
	_ = port2

	host, port := parseTestServerAddr(t, combined.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{
			{
				Requests: []v1alpha1.WarmupRequest{
					{
						Endpoint: "/step1",
						Extract:  map[string]string{"token": "$.token"},
					},
				},
			},
			{
				Requests: []v1alpha1.WarmupRequest{
					{
						Endpoint: "/step2",
						Headers:  map[string]string{"Authorization": "Bearer {{token}}"},
					},
				},
			},
		},
	}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}
	if receivedAuth != "Bearer secret" {
		t.Errorf("expected Authorization: Bearer secret, got %q", receivedAuth)
	}
}

func TestScenarioExecutor_PostWithBody(t *testing.T) {
	var gotMethod, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err == nil {
			b, _ := json.Marshal(m) //nolint:errcheck // test handler
			gotBody = string(b)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{
			{
				Requests: []v1alpha1.WarmupRequest{
					{
						Endpoint: "/api",
						Method:   "POST",
						Headers:  map[string]string{"Content-Type": "application/json"},
						Body:     `{"action":"preload"}`,
					},
				},
			},
		},
	}

	result := e.ExecuteScenario(context.Background(), config, spec)
	if !result.Success {
		t.Errorf("expected success, got: %s", result.Message)
	}
	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotBody != `{"action":"preload"}` {
		t.Errorf("unexpected body: %s", gotBody)
	}
}

func TestScenarioExecutor_ExpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
	}))
	defer server.Close()

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	// expectedStatus matches → success
	spec := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{{
			Requests: []v1alpha1.WarmupRequest{{
				Endpoint:       "/",
				ExpectedStatus: 201,
			}},
		}},
	}
	result := e.ExecuteScenario(context.Background(), config, spec)
	if result.RequestsCompleted != 1 || result.RequestsFailed != 0 {
		t.Errorf("expected 1 completed 0 failed, got %d/%d", result.RequestsCompleted, result.RequestsFailed)
	}

	// expectedStatus mismatch → fail-open (counted as failed, execution continues)
	spec2 := &v1alpha1.WarmupConfigSpec{
		Steps: []v1alpha1.WarmupStep{{
			Requests: []v1alpha1.WarmupRequest{{
				Endpoint:       "/",
				ExpectedStatus: 200, // server returns 201
			}},
		}},
	}
	result2 := e.ExecuteScenario(context.Background(), config, spec2)
	if result2.RequestsFailed != 1 {
		t.Errorf("expected 1 failed for mismatch, got %d", result2.RequestsFailed)
	}
}

func TestScenarioExecutor_StepTimeout(t *testing.T) {
	// Step 1 blocks indefinitely; step 2 should still execute after step timeout.
	blocked := make(chan struct{})
	step1Done := false
	step2Done := false

	mux := http.NewServeMux()
	mux.HandleFunc("/block", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-blocked:
		case <-r.Context().Done():
		}
		step1Done = true
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/quick", func(w http.ResponseWriter, r *http.Request) {
		step2Done = true
		w.WriteHeader(http.StatusOK)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	defer close(blocked)

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Timeout: "5s",
		Steps: []v1alpha1.WarmupStep{
			{
				Timeout: "100ms",
				Requests: []v1alpha1.WarmupRequest{
					{Endpoint: "/block"},
				},
			},
			{
				Requests: []v1alpha1.WarmupRequest{
					{Endpoint: "/quick"},
				},
			},
		},
	}

	result := e.ExecuteScenario(context.Background(), config, spec)
	// Step 1 timed out (counted as failed), step 2 should succeed
	_ = step1Done
	if !step2Done {
		t.Error("step 2 should have executed after step 1 timed out")
	}
	_ = result // fail-open: scenario continues even when a step times out
}

func TestScenarioExecutor_OverallTimeout(t *testing.T) {
	// Both steps block; overall timeout should cancel everything.
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-block:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	defer close(block)

	e := NewScenarioExecutor(ctrl.Log.WithName("test"))
	host, port := parseTestServerAddr(t, server.URL)
	config := newTestConfig(host, port)

	spec := &v1alpha1.WarmupConfigSpec{
		Timeout: "200ms",
		Steps: []v1alpha1.WarmupStep{
			{Requests: []v1alpha1.WarmupRequest{{Endpoint: "/"}}},
			{Requests: []v1alpha1.WarmupRequest{{Endpoint: "/"}}},
		},
	}

	start := time.Now()
	result := e.ExecuteScenario(context.Background(), config, spec)
	if time.Since(start) > 2*time.Second {
		t.Error("ExecuteScenario did not respect overall timeout")
	}
	if result.Error == nil {
		t.Error("expected context error on overall timeout")
	}
}

// parseTestServerAddr extracts host and port as int from a test server URL.
func parseTestServerAddr(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	// rawURL is like "http://127.0.0.1:PORT"
	trimmed := rawURL[len("http://"):]
	var port int
	var host string
	lastColon := len(trimmed) - 1
	for lastColon >= 0 && trimmed[lastColon] != ':' {
		lastColon--
	}
	host = trimmed[:lastColon]
	_, err := fmt.Sscanf(trimmed[lastColon+1:], "%d", &port)
	if err != nil {
		t.Fatalf("parseTestServerAddr: %v", err)
	}
	return host, port
}
