// grpc-demo-server is a minimal gRPC server used to demonstrate kube-booster's
// gRPC warmup feature. It registers the standard gRPC health checking service
// and enables server reflection so kube-booster can discover the method
// descriptor without compiled proto files.
//
// Usage:
//
//	grpc-demo-server [--port <port>]
//
// The server listens on port 50051 by default. Override with --port or the
// GRPC_PORT environment variable.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := flag.Int("port", defaultPort(), "port to listen on")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", addr, err)
	}

	srv := grpc.NewServer()

	// Register the standard gRPC health checking service and mark the server SERVING.
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Enable server reflection so kube-booster (and tools like grpcurl) can
	// discover method descriptors at runtime without compiled proto files.
	reflection.Register(srv)

	log.Printf("grpc-demo-server listening on %s", addr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

// defaultPort returns the value of GRPC_PORT if set, otherwise 50051.
func defaultPort() int {
	if v := os.Getenv("GRPC_PORT"); v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err == nil && p > 0 {
			return p
		}
	}
	return 50051
}
