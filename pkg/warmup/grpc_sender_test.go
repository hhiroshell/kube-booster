package warmup

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	ctrl "sigs.k8s.io/controller-runtime"
)

// startTestGRPCServer starts a gRPC server on a random local port and returns its address.
// If withReflection is true, the gRPC reflection service is registered.
// The returned stop function must be called to release resources.
func startTestGRPCServer(t *testing.T, withReflection bool) (addr string, stop func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthSrv.SetServingStatus("grpc.health.v1.Health", healthpb.HealthCheckResponse_SERVING)

	if withReflection {
		reflection.Register(srv)
	}

	go srv.Serve(lis) //nolint:errcheck

	return lis.Addr().String(), srv.GracefulStop
}

func TestGRPCSender_Send_Success(t *testing.T) {
	addr, stop := startTestGRPCServer(t, true)
	defer stop()

	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)
	t.Cleanup(func() { sender.Close() }) //nolint:errcheck

	target := Target{
		Address: addr,
		Method:  "grpc.health.v1.Health/Check",
		Payload: []byte(`{}`),
	}

	resp := sender.Send(context.Background(), target)

	if resp.Error != nil {
		t.Fatalf("Send() unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Send() StatusCode = %d, want 200", resp.StatusCode)
	}
	if resp.Duration == 0 {
		t.Error("Send() Duration = 0, want > 0")
	}
}

func TestGRPCSender_Send_ReflectionUnavailable(t *testing.T) {
	addr, stop := startTestGRPCServer(t, false) // no reflection
	defer stop()

	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)
	t.Cleanup(func() { sender.Close() }) //nolint:errcheck

	target := Target{
		Address: addr,
		Method:  "grpc.health.v1.Health/Check",
		Payload: []byte(`{}`),
	}

	resp := sender.Send(context.Background(), target)

	if resp.Error == nil {
		t.Fatal("Send() expected error when reflection is unavailable, got nil")
	}
	if !strings.Contains(resp.Error.Error(), "server reflection unavailable") {
		t.Errorf("Send() error = %v, want containing \"server reflection unavailable\"", resp.Error)
	}
}

func TestGRPCSender_Send_InvalidMethodFormat(t *testing.T) {
	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)

	resp := sender.Send(context.Background(), Target{
		Address: "127.0.0.1:50051",
		Method:  "BadFormatNoSlash",
	})

	if resp.Error == nil {
		t.Fatal("Send() expected error for invalid method format, got nil")
	}
	if !strings.Contains(resp.Error.Error(), "invalid gRPC method") {
		t.Errorf("Send() error = %v, want containing \"invalid gRPC method\"", resp.Error)
	}
}

func TestGRPCSender_Send_MethodNotFound(t *testing.T) {
	addr, stop := startTestGRPCServer(t, true)
	defer stop()

	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)
	t.Cleanup(func() { sender.Close() }) //nolint:errcheck

	target := Target{
		Address: addr,
		Method:  "grpc.health.v1.Health/NonExistentMethod",
		Payload: []byte(`{}`),
	}

	resp := sender.Send(context.Background(), target)

	if resp.Error == nil {
		t.Fatal("Send() expected error for non-existent method, got nil")
	}
	if !strings.Contains(resp.Error.Error(), "not found") {
		t.Errorf("Send() error = %v, want containing \"not found\"", resp.Error)
	}
}

func TestGRPCSender_Send_ConnectionReuse(t *testing.T) {
	addr, stop := startTestGRPCServer(t, true)
	defer stop()

	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)
	t.Cleanup(func() { sender.Close() }) //nolint:errcheck

	target := Target{
		Address: addr,
		Method:  "grpc.health.v1.Health/Check",
		Payload: []byte(`{}`),
	}

	// First Send establishes the connection; subsequent Sends must reuse it.
	resp := sender.Send(context.Background(), target)
	if resp.Error != nil {
		t.Fatalf("Send() request 1 unexpected error: %v", resp.Error)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Send() request 1 StatusCode = %d, want 200", resp.StatusCode)
	}
	if sender.conn == nil {
		t.Fatal("expected sender.conn to be non-nil after first Send")
	}
	firstConn := sender.conn

	for i := 1; i < 3; i++ {
		resp := sender.Send(context.Background(), target)
		if resp.Error != nil {
			t.Fatalf("Send() request %d unexpected error: %v", i+1, resp.Error)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Send() request %d StatusCode = %d, want 200", i+1, resp.StatusCode)
		}
	}
	if sender.conn != firstConn {
		t.Error("expected same gRPC connection to be reused across Send calls")
	}
}

func TestGRPCSender_Send_ContextCancellation(t *testing.T) {
	addr, stop := startTestGRPCServer(t, true)
	defer stop()

	logger := ctrl.Log.WithName("test")
	sender := NewGRPCSender(logger)
	t.Cleanup(func() { sender.Close() }) //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	// Give the timeout a moment to fire.
	time.Sleep(5 * time.Millisecond)

	resp := sender.Send(ctx, Target{
		Address: addr,
		Method:  "grpc.health.v1.Health/Check",
		Payload: []byte(`{}`),
	})

	// Context expired: either an error (transport) or a cancelled/deadline status.
	if resp.Error == nil && resp.StatusCode == 200 {
		t.Error("Send() expected failure with expired context, got StatusCode 200 and no error")
	}
}

func TestParseGRPCMethod(t *testing.T) {
	tests := []struct {
		input       string
		wantService string
		wantMethod  string
		wantErr     bool
	}{
		{"grpc.health.v1.Health/Check", "grpc.health.v1.Health", "Check", false},
		{"pkg.Svc/Method", "pkg.Svc", "Method", false},
		{"NoSlash", "", "", true},
		{"Service/", "", "", true},
		{"/Method", "", "", true}, // leading slash: idx==0, rejected early
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			svc, meth, err := parseGRPCMethod(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseGRPCMethod(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseGRPCMethod(%q) unexpected error: %v", tt.input, err)
				return
			}
			if svc != tt.wantService {
				t.Errorf("parseGRPCMethod(%q) service = %q, want %q", tt.input, svc, tt.wantService)
			}
			if meth != tt.wantMethod {
				t.Errorf("parseGRPCMethod(%q) method = %q, want %q", tt.input, meth, tt.wantMethod)
			}
		})
	}
}

func TestGRPCSender_Close_NoConn(t *testing.T) {
	sender := NewGRPCSender(ctrl.Log.WithName("test"))
	if err := sender.Close(); err != nil {
		t.Errorf("Close() on unconnected sender returned error: %v", err)
	}
}
