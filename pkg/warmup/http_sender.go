package warmup

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

// HTTPSender executes a single HTTP GET warmup request.
type HTTPSender struct {
	client *http.Client
	logger logr.Logger
}

// Send issues one HTTP GET to target.Address and returns the response.
func (s *HTTPSender) Send(ctx context.Context, target Target) *Response {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.Address, nil)
	if err != nil {
		return &Response{Error: err, Duration: time.Since(start)}
	}

	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return &Response{Error: err, Duration: duration}
	}

	if _, drainErr := io.Copy(io.Discard, resp.Body); drainErr != nil {
		s.logger.V(2).Info("failed to drain response body", "error", drainErr)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		s.logger.V(2).Info("failed to close response body", "error", closeErr)
	}

	return &Response{StatusCode: resp.StatusCode, Duration: duration}
}

// Close is a no-op for HTTPSender (http.Client manages its own connection pool).
func (s *HTTPSender) Close() error { return nil }

// Ensure HTTPSender implements Sender.
var _ Sender = (*HTTPSender)(nil)
