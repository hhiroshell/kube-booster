package warmup

import (
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

const (
	// DefaultRequestCount is the default number of warmup requests to send
	DefaultRequestCount = 3

	// DefaultTimeout is the default timeout for warmup requests
	DefaultTimeout = 30 * time.Second

	// DefaultEndpointPath is the default endpoint path for warmup requests
	DefaultEndpointPath = "/"

	// DefaultPort is the default port for warmup requests (first container port or 8080)
	DefaultPort = 8080
)

// Config holds the warmup configuration parsed from pod annotations
type Config struct {
	// Endpoint is the URL path for warmup requests (from kube-booster.io/warmup-endpoint)
	Endpoint string

	// RequestCount is the number of warmup requests to send (from kube-booster.io/warmup-requests)
	RequestCount int

	// Timeout is the total timeout for all warmup requests (from kube-booster.io/warmup-timeout)
	Timeout time.Duration

	// PodIP is the IP address of the pod (set by controller)
	PodIP string

	// PodName is the name of the pod (set by controller)
	PodName string

	// PodNamespace is the namespace of the pod (set by controller)
	PodNamespace string

	// Port is the port to use for warmup requests
	Port int
}

// ParseConfig parses warmup configuration from pod annotations
func ParseConfig(pod *corev1.Pod) (*Config, error) {
	config := &Config{
		Endpoint:     DefaultEndpointPath,
		RequestCount: DefaultRequestCount,
		Timeout:      DefaultTimeout,
		Port:         DefaultPort,
	}

	if pod == nil {
		return config, nil
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
			config.Timeout = timeout
		}
	}

	// Try to determine port from container spec
	if len(pod.Spec.Containers) > 0 {
		container := pod.Spec.Containers[0]
		if len(container.Ports) > 0 {
			config.Port = int(container.Ports[0].ContainerPort)
		}
	}

	return config, nil
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
