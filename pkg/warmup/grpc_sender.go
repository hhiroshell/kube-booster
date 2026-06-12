package warmup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// GRPCSender executes a single gRPC unary warmup request using server reflection.
// It dials lazily on the first Send call and reuses the connection and cached method
// descriptor for subsequent calls.
type GRPCSender struct {
	logger     logr.Logger
	conn       *grpc.ClientConn
	methodDesc protoreflect.MethodDescriptor // cached after first successful reflection lookup
}

// NewGRPCSender creates a new GRPCSender.
func NewGRPCSender(logger logr.Logger) *GRPCSender {
	return &GRPCSender{logger: logger}
}

// Send invokes the gRPC method described by target.Method on target.Address.
// It uses server reflection to discover the method descriptor dynamically, without
// requiring compiled proto files.
//
// Return values:
//   - Success → StatusCode 200, Error nil
//   - Application-level gRPC error (non-OK status) → StatusCode 500, Error nil
//     (latency is recorded; the application processed the request)
//   - Transport/reflection failure → Error non-nil (latency is not recorded)
func (s *GRPCSender) Send(ctx context.Context, target Target) *Response {
	start := time.Now()

	// Lazy dial on first Send.
	if s.conn == nil {
		conn, err := grpc.NewClient(target.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return &Response{Error: fmt.Errorf("grpc dial %s: %w", target.Address, err), Duration: time.Since(start)}
		}
		s.conn = conn
	}

	// Lazy reflection lookup: discover and cache the method descriptor.
	if s.methodDesc == nil {
		serviceSymbol, methodName, err := parseGRPCMethod(target.Method)
		if err != nil {
			return &Response{Error: err, Duration: time.Since(start)}
		}
		md, err := resolveMethodDescriptor(ctx, s.conn, serviceSymbol, methodName)
		if err != nil {
			return &Response{Error: err, Duration: time.Since(start)}
		}
		s.methodDesc = md
	}

	// Build request message from JSON payload.
	reqMsg := dynamicpb.NewMessage(s.methodDesc.Input())
	if len(target.Payload) > 0 {
		if err := protojson.Unmarshal(target.Payload, reqMsg); err != nil {
			return &Response{Error: fmt.Errorf("invalid gRPC payload: %w", err), Duration: time.Since(start)}
		}
	}

	// Invoke unary RPC.
	respMsg := dynamicpb.NewMessage(s.methodDesc.Output())
	methodPath := "/" + string(s.methodDesc.Parent().FullName()) + "/" + string(s.methodDesc.Name())
	err := s.conn.Invoke(ctx, methodPath, reqMsg, respMsg)
	duration := time.Since(start)

	if err != nil {
		st, _ := status.FromError(err)
		// Context cancellation/deadline: signal the caller to stop the loop.
		if st.Code() == codes.Canceled || st.Code() == codes.DeadlineExceeded {
			return &Response{Error: err, Duration: duration}
		}
		// Other gRPC errors: application processed the request; record latency, count as fail.
		s.logger.V(2).Info("gRPC warmup request failed",
			"method", target.Method, "code", st.Code(), "message", st.Message())
		return &Response{StatusCode: 500, Duration: duration}
	}

	body, err := protojson.Marshal(respMsg)
	if err != nil {
		s.logger.V(2).Info("failed to marshal gRPC response body", "error", err)
	}
	return &Response{StatusCode: 200, Duration: duration, Body: body}
}

// Close releases the underlying gRPC connection.
func (s *GRPCSender) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// parseGRPCMethod splits "package.Service/Method" into ("package.Service", "Method").
func parseGRPCMethod(fullMethod string) (serviceSymbol, methodName string, err error) {
	idx := strings.LastIndex(fullMethod, "/")
	if idx < 0 || idx == len(fullMethod)-1 {
		return "", "", fmt.Errorf("invalid gRPC method %q: expected format \"package.Service/Method\"", fullMethod)
	}
	return fullMethod[:idx], fullMethod[idx+1:], nil
}

// resolveMethodDescriptor uses server reflection to find and validate the method descriptor.
func resolveMethodDescriptor(ctx context.Context, conn *grpc.ClientConn, serviceSymbol, methodName string) (protoreflect.MethodDescriptor, error) {
	fdBytes, err := fetchFileDescriptorBytes(ctx, conn, serviceSymbol)
	if err != nil {
		return nil, fmt.Errorf("server reflection unavailable or service %q not found: %w", serviceSymbol, err)
	}

	files, err := buildFileSet(fdBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build file descriptors for service %q: %w", serviceSymbol, err)
	}

	d, err := files.FindDescriptorByName(protoreflect.FullName(serviceSymbol))
	if err != nil {
		return nil, fmt.Errorf("service %q not found in reflection response: %w", serviceSymbol, err)
	}

	svcDesc, ok := d.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("symbol %q is not a service", serviceSymbol)
	}

	md := svcDesc.Methods().ByName(protoreflect.Name(methodName))
	if md == nil {
		return nil, fmt.Errorf("method %q not found in service %q", methodName, serviceSymbol)
	}

	if md.IsStreamingClient() || md.IsStreamingServer() {
		return nil, fmt.Errorf("only unary RPCs are supported; %q is a streaming method", serviceSymbol+"/"+methodName)
	}

	return md, nil
}

// fetchFileDescriptorBytes uses the gRPC server reflection service to retrieve the
// serialized FileDescriptorProto bytes for the file containing serviceSymbol.
func fetchFileDescriptorBytes(ctx context.Context, conn *grpc.ClientConn, serviceSymbol string) ([][]byte, error) {
	stream, err := reflectionpb.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open reflection stream: %w", err)
	}
	defer stream.CloseSend() //nolint:errcheck

	if err := stream.Send(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: serviceSymbol,
		},
	}); err != nil {
		return nil, fmt.Errorf("reflection request send error: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("reflection stream recv error: %w", err)
	}

	if errResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_ErrorResponse); ok {
		return nil, fmt.Errorf("reflection error (code %d): %s",
			errResp.ErrorResponse.GetErrorCode(), errResp.ErrorResponse.GetErrorMessage())
	}

	fileResp, ok := resp.MessageResponse.(*reflectionpb.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected reflection response type: %T", resp.MessageResponse)
	}

	return fileResp.FileDescriptorResponse.GetFileDescriptorProto(), nil
}

// buildFileSet constructs a protoregistry.Files from serialized FileDescriptorProto bytes.
// Dependencies are resolved from the received set first, then from the global registry
// (for well-known types like google/protobuf/timestamp.proto).
func buildFileSet(fdBytes [][]byte) (*protoregistry.Files, error) {
	protos := make([]*descriptorpb.FileDescriptorProto, 0, len(fdBytes))
	for _, b := range fdBytes {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fdp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FileDescriptorProto: %w", err)
		}
		protos = append(protos, fdp)
	}

	byName := make(map[string]*descriptorpb.FileDescriptorProto, len(protos))
	for _, fdp := range protos {
		byName[fdp.GetName()] = fdp
	}

	files := new(protoregistry.Files)
	registered := make(map[string]bool)

	var register func(name string) error
	register = func(name string) error {
		if registered[name] {
			return nil
		}
		fdp, ok := byName[name]
		if !ok {
			// Well-known types (e.g. google/protobuf/any.proto) live in the global registry.
			if _, err := protoregistry.GlobalFiles.FindFileByPath(name); err == nil {
				registered[name] = true
				return nil
			}
			return fmt.Errorf("missing file descriptor for dependency %q", name)
		}
		for _, dep := range fdp.GetDependency() {
			if err := register(dep); err != nil {
				return err
			}
		}
		resolver := &fileSetResolver{local: files}
		fd, err := protodesc.NewFile(fdp, resolver)
		if err != nil {
			return fmt.Errorf("failed to build descriptor for %q: %w", name, err)
		}
		if err := files.RegisterFile(fd); err != nil {
			// Duplicate registration is harmless (can occur with shared transitive deps).
			if !strings.Contains(err.Error(), "already registered") {
				return fmt.Errorf("failed to register file descriptor %q: %w", name, err)
			}
		}
		registered[name] = true
		return nil
	}

	for _, fdp := range protos {
		if err := register(fdp.GetName()); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// fileSetResolver resolves proto file dependencies by checking the local (in-progress)
// file set first, then falling back to the global proto registry for well-known types.
type fileSetResolver struct {
	local *protoregistry.Files
}

func (r *fileSetResolver) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if fd, err := r.local.FindFileByPath(path); err == nil {
		return fd, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(path)
}

func (r *fileSetResolver) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	if d, err := r.local.FindDescriptorByName(name); err == nil {
		return d, nil
	}
	// Fall back to global registry for well-known descriptors.
	if mt, err := protoregistry.GlobalTypes.FindMessageByName(name); err == nil {
		return mt.Descriptor(), nil
	}
	return nil, fmt.Errorf("descriptor %q not found", name)
}

// Ensure GRPCSender implements Sender.
var _ Sender = (*GRPCSender)(nil)
