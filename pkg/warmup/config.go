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

	// DefaultDuration is the default duration for warmup requests
	DefaultDuration = 30 * time.Second

	// DefaultEndpointPath is the default endpoint path for warmup requests
	DefaultEndpointPath = "/"
)

// Config holds the warmup configuration parsed from pod annotations
type Config struct {
	// Endpoint is the URL path for warmup requests (from kube-booster.io/warmup-endpoint)
	Endpoint string

	// RequestCount is the number of warmup requests to send (from kube-booster.io/warmup-requests)
	RequestCount int

	// Duration is the total duration for all warmup requests (from kube-booster.io/warmup-duration)
	Duration time.Duration

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
		Duration:     DefaultDuration,
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
			config.RequestCount = reqCount
		}

		// Parse duration
		if durationStr, ok := annotations[webhook.AnnotationWarmupDuration]; ok && durationStr != "" {
			duration, err := time.ParseDuration(durationStr)
			if err != nil {
				return config, fmt.Errorf("invalid warmup-duration value %q: %w", durationStr, err)
			}
			if duration < time.Second {
				return config, fmt.Errorf("warmup-duration must be at least 1s, got %v", duration)
			}
			config.Duration = duration
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

// BuildEndpointURL constructs the full URL for warmup requests
func (c *Config) BuildEndpointURL() string {
	endpoint := c.Endpoint
	// Ensure endpoint starts with /
	if len(endpoint) == 0 || endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}
	return fmt.Sprintf("http://%s:%d%s", c.PodIP, c.Port, endpoint)
}
