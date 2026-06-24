package warmup

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
)

// HTTPSender executes a single HTTP warmup request.
type HTTPSender struct {
	client *http.Client
	logger logr.Logger
}

// Send issues one HTTP request to target.Address using target.Method (default GET)
// with optional target.Payload as the request body. Response body is captured and
// returned in Response.Body so callers can extract values for session chaining.
func (s *HTTPSender) Send(ctx context.Context, target Target) *Response {
	start := time.Now()

	method := target.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if len(target.Payload) > 0 {
		bodyReader = bytes.NewReader(target.Payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, target.Address, bodyReader)
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

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		s.logger.V(2).Info("failed to read response body", "error", readErr)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		s.logger.V(2).Info("failed to close response body", "error", closeErr)
	}

	return &Response{StatusCode: resp.StatusCode, Duration: duration, Body: body}
}

// Close is a no-op for HTTPSender (http.Client manages its own connection pool).
func (s *HTTPSender) Close() error { return nil }

// Ensure HTTPSender implements Sender.
var _ Sender = (*HTTPSender)(nil)
