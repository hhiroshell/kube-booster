package warmup

import (
	"context"
	"time"
)

// Sender executes a single warmup request.
type Sender interface {
	Send(ctx context.Context, target Target) *Response
	Close() error
}

// Target describes a single warmup request.
type Target struct {
	// Address is "host:port" for gRPC or a full URL for HTTP.
	Address string

	// Method is the HTTP verb (e.g. "GET") or gRPC method ("package.Service/Method").
	Method string

	// Headers contains additional request headers. Ignored by GRPCSender.
	Headers map[string]string

	// Payload is the request body; JSON-encoded for gRPC warmup, unused for HTTP GET.
	// Ignored by HTTPSender.
	Payload []byte
}

// Response is the outcome of a single Send call.
type Response struct {
	// StatusCode is the HTTP status code, or 200 (gRPC OK) / 500 (gRPC error).
	StatusCode int

	// Duration is the round-trip time from the start of Send to receiving the response.
	Duration time.Duration

	// Body is the response body (may be nil).
	Body []byte

	// Error is non-nil when the request could not be completed at the transport level.
	// Application-level failures (e.g. HTTP 5xx, non-OK gRPC status) are represented
	// via StatusCode rather than Error so that latency is still recorded.
	Error error
}
