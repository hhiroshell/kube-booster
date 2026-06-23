package warmup

import (
	"context"
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
