package warmup

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

const (
	// DefaultRequestCount is the default number of warmup requests to send
	DefaultRequestCount = 3

	// MaxRequestCount is the maximum allowed warmup requests (aligned with JVM C2 JIT threshold)
	MaxRequestCount = 12000

	// DefaultTimeout is the default maximum timeout for warmup requests
	DefaultTimeout = 30 * time.Second

	// MaxTimeout is the maximum allowed warmup timeout
	MaxTimeout = 5 * time.Minute

	// DefaultEndpointPath is the default endpoint path for warmup requests
	DefaultEndpointPath = "/"

	// ProtocolHTTP selects HTTP warmup (default)
	ProtocolHTTP = "http"

	// ProtocolGRPC selects gRPC warmup
	ProtocolGRPC = "grpc"

	// DefaultGRPCPayload is the default JSON payload for gRPC warmup requests
	DefaultGRPCPayload = "{}"
)

// Config holds the warmup configuration parsed from pod annotations
type Config struct {
	// Endpoint is the URL path for warmup requests (from kube-booster.io/warmup-endpoint)
	Endpoint string

	// RequestCount is the number of warmup requests to send (from kube-booster.io/warmup-requests)
	RequestCount int

	// Timeout is the maximum timeout for the warmup phase (from kube-booster.io/warmup-timeout)
	Timeout time.Duration

	// PodIP is the IP address of the pod (set by controller)
	PodIP string

	// PodName is the name of the pod (set by controller)
	PodName string

	// PodNamespace is the namespace of the pod (set by controller)
	PodNamespace string

	// Port is the port to use for warmup requests
	Port int

	// Protocol is the warmup protocol: "http" (default) or "grpc"
	Protocol string

	// GRPCMethod is the fully-qualified gRPC method ("package.Service/Method"), required when Protocol == "grpc"
	GRPCMethod string

	// GRPCPayload is the JSON-encoded request payload for gRPC warmup, defaults to "{}"
	GRPCPayload string

	// WarmupConfigName is the name of a WarmupConfig CR in the pod's namespace.
	// When non-empty, the controller uses scenario-based warmup instead of the
	// single-endpoint annotation-based warmup.
	WarmupConfigName string
}

// ParseConfig parses warmup configuration from pod annotations
func ParseConfig(pod *corev1.Pod) (*Config, error) {
	config := &Config{
		Endpoint:     DefaultEndpointPath,
		RequestCount: DefaultRequestCount,
		Timeout:      DefaultTimeout,
		Protocol:     ProtocolHTTP,
		GRPCPayload:  DefaultGRPCPayload,
	}

	if pod == nil {
		return config, fmt.Errorf("pod is nil, cannot determine warmup port")
	}

	// Parse annotations if present
	annotations := pod.Annotations
	if annotations != nil {
		// Parse endpoint
		if endpoint, ok := annotations[webhook.AnnotationWarmupEndpoint]; ok && endpoint != "" {
			config.Endpoint = endpoint
		}

		// Parse request count
		if reqCountStr, ok := annotations[webhook.AnnotationWarmupRequests]; ok && reqCountStr != "" {
			reqCount, err := strconv.Atoi(reqCountStr)
			if err != nil {
				return config, fmt.Errorf("invalid warmup-requests value %q: %w", reqCountStr, err)
			}
			if reqCount < 1 {
				return config, fmt.Errorf("warmup-requests must be at least 1, got %d", reqCount)
			}
			if reqCount > MaxRequestCount {
				return config, fmt.Errorf("warmup-requests must not exceed %d, got %d", MaxRequestCount, reqCount)
			}
			config.RequestCount = reqCount
		}

		// Parse timeout
		if timeoutStr, ok := annotations[webhook.AnnotationWarmupTimeout]; ok && timeoutStr != "" {
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return config, fmt.Errorf("invalid warmup-timeout value %q: %w", timeoutStr, err)
			}
			if timeout < time.Second {
				return config, fmt.Errorf("warmup-timeout must be at least 1s, got %v", timeout)
			}
			if timeout > MaxTimeout {
				return config, fmt.Errorf("warmup-timeout must not exceed %v, got %v", MaxTimeout, timeout)
			}
			config.Timeout = timeout
		}

		// Parse protocol
		if protocol, ok := annotations[webhook.AnnotationWarmupProtocol]; ok && protocol != "" {
			switch protocol {
			case ProtocolHTTP, ProtocolGRPC:
				config.Protocol = protocol
			default:
				return config, fmt.Errorf("invalid warmup-protocol value %q: must be %q or %q", protocol, ProtocolHTTP, ProtocolGRPC)
			}
		}

		// Parse gRPC method
		if method, ok := annotations[webhook.AnnotationWarmupGRPCMethod]; ok && method != "" {
			if _, _, err := parseGRPCMethod(method); err != nil {
				return config, fmt.Errorf("invalid %s value: %w", webhook.AnnotationWarmupGRPCMethod, err)
			}
			config.GRPCMethod = method
		}

		// Parse gRPC payload
		if payload, ok := annotations[webhook.AnnotationWarmupGRPCPayload]; ok && payload != "" {
			if !json.Valid([]byte(payload)) {
				return config, fmt.Errorf("invalid %s value: not valid JSON", webhook.AnnotationWarmupGRPCPayload)
			}
			config.GRPCPayload = payload
		}

		// Validate gRPC config
		if config.Protocol == ProtocolGRPC && config.GRPCMethod == "" {
			return config, fmt.Errorf("annotation %s is required when %s is %q",
				webhook.AnnotationWarmupGRPCMethod, webhook.AnnotationWarmupProtocol, ProtocolGRPC)
		}

		// Parse WarmupConfig reference
		if name, ok := annotations[webhook.AnnotationWarmupConfig]; ok && name != "" {
			config.WarmupConfigName = name
		}

		// Parse port from annotation
		if portStr, ok := annotations[webhook.AnnotationWarmupPort]; ok && portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return config, fmt.Errorf("invalid warmup-port value %q: %w", portStr, err)
			}
			if port < 1 || port > 65535 {
				return config, fmt.Errorf("warmup-port must be between 1 and 65535, got %d", port)
			}
			config.Port = port
			return config, nil
		}
	}

	// No port annotation, try to auto-detect from container spec
	// Only auto-detect when there's exactly 1 container with exactly 1 port
	if len(pod.Spec.Containers) == 1 {
		container := pod.Spec.Containers[0]
		if len(container.Ports) == 1 {
			config.Port = int(container.Ports[0].ContainerPort)
			return config, nil
		} else if len(container.Ports) > 1 {
			return config, fmt.Errorf("container %q has multiple ports, please specify warmup port using annotation %s",
				container.Name, webhook.AnnotationWarmupPort)
		}
	} else if len(pod.Spec.Containers) > 1 {
		return config, fmt.Errorf("pod has multiple containers, please specify warmup port using annotation %s",
			webhook.AnnotationWarmupPort)
	}

	// No containers or no ports found
	return config, fmt.Errorf("cannot determine warmup port: no container ports found, please specify using annotation %s",
		webhook.AnnotationWarmupPort)
}

// BuildGRPCAddress returns the "host:port" address for gRPC dial
func (c *Config) BuildGRPCAddress() string {
	return fmt.Sprintf("%s:%d", c.PodIP, c.Port)
}

// BuildEndpointURL constructs the full URL for warmup requests
func (c *Config) BuildEndpointURL() string {
	endpoint := c.Endpoint
	// Ensure endpoint starts with /
	if len(endpoint) == 0 || endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}
	return fmt.Sprintf("http://%s:%d%s", c.PodIP, c.Port, endpoint)
}
