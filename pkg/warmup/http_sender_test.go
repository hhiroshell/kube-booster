package warmup

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

func TestHTTPSender_Send(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	tests := []struct {
		name           string
		serverHandler  http.HandlerFunc
		wantStatusCode int
		wantErr        bool
	}{
		{
			name: "success 200",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Warmup-Request") != "true" {
					t.Errorf("expected X-Warmup-Request: true header")
				}
				if r.Header.Get("User-Agent") != "kube-booster/1.0" {
					t.Errorf("expected User-Agent: kube-booster/1.0 header")
				}
				w.WriteHeader(http.StatusOK)
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "server error 500",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "not found 404",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			sender := &HTTPSender{
				client: &http.Client{Timeout: 5 * time.Second},
				logger: logger,
			}

			target := Target{
				Address: server.URL + "/warmup",
				Method:  http.MethodGet,
				Headers: map[string]string{
					"User-Agent":       "kube-booster/1.0",
					"X-Warmup-Request": "true",
				},
			}

			resp := sender.Send(context.Background(), target)

			if tt.wantErr {
				if resp.Error == nil {
					t.Error("Send() expected error, got nil")
				}
				return
			}

			if resp.Error != nil {
				t.Errorf("Send() unexpected error: %v", resp.Error)
				return
			}
			if resp.StatusCode != tt.wantStatusCode {
				t.Errorf("Send() StatusCode = %d, want %d", resp.StatusCode, tt.wantStatusCode)
			}
			if resp.Duration == 0 {
				t.Error("Send() Duration = 0, want > 0")
			}
		})
	}
}

func TestHTTPSender_Send_MethodAndBody(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	var gotMethod, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body) //nolint:errcheck // test handler
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck // test handler
	}))
	defer server.Close()

	sender := &HTTPSender{
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}

	target := Target{
		Address: server.URL + "/api/warmup",
		Method:  http.MethodPost,
		Headers: map[string]string{"Content-Type": "application/json"},
		Payload: []byte(`{"action":"preload"}`),
	}

	resp := sender.Send(context.Background(), target)
	if resp.Error != nil {
		t.Fatalf("Send() unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotBody != `{"action":"preload"}` {
		t.Errorf("body = %s, want {\"action\":\"preload\"}", gotBody)
	}
	if string(resp.Body) != `{"status":"ok"}` {
		t.Errorf("Response.Body = %s, want {\"status\":\"ok\"}", resp.Body)
	}
}

func TestHTTPSender_Send_DefaultMethod(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := &HTTPSender{
		client: &http.Client{Timeout: 5 * time.Second},
		logger: logger,
	}

	// Method omitted → should default to GET
	resp := sender.Send(context.Background(), Target{Address: server.URL + "/"})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", gotMethod)
	}
}

func TestHTTPSender_Send_ConnectionError(t *testing.T) {
	logger := ctrl.Log.WithName("test")

	sender := &HTTPSender{
		client: &http.Client{Timeout: 1 * time.Second},
		logger: logger,
	}

	// Dial an address that refuses connections
	resp := sender.Send(context.Background(), Target{Address: "http://127.0.0.1:1"})
	if resp.Error == nil {
		t.Error("Send() expected connection error, got nil")
	}
}
