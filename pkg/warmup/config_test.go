package warmup

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		pod         *corev1.Pod
		wantConfig  *Config
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil pod returns error",
			pod:         nil,
			wantErr:     true,
			errContains: "pod is nil",
		},
		{
			name: "pod with single container and single port auto-detects port",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
							},
						},
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         8080,
			},
		},
		{
			name: "pod with explicit port annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupPort: "3000",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
								{ContainerPort: 9090},
							},
						},
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         3000,
			},
		},
		{
			name: "custom endpoint",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEndpoint: "/api/warmup",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     "/api/warmup",
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         8080,
			},
		},
		{
			name: "custom request count",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupRequests: "10",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: 10,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         8080,
			},
		},
		{
			name: "custom timeout",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupTimeout: "60s",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      60 * time.Second,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         8080,
			},
		},
		{
			name: "all custom values",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEndpoint: "/health",
						webhook.AnnotationWarmupRequests: "5",
						webhook.AnnotationWarmupTimeout:  "15s",
						webhook.AnnotationWarmupPort:     "3000",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     "/health",
				RequestCount: 5,
				Timeout:      15 * time.Second,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         3000,
			},
		},
		{
			name: "multiple containers without port annotation returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app1", Image: "nginx", Ports: []corev1.ContainerPort{{ContainerPort: 8080}}},
						{Name: "app2", Image: "redis", Ports: []corev1.ContainerPort{{ContainerPort: 6379}}},
					},
				},
			},
			wantErr:     true,
			errContains: "pod has multiple containers",
		},
		{
			name: "single container with multiple ports without annotation returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080},
								{ContainerPort: 9090},
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "container \"app\" has multiple ports",
		},
		{
			name: "no container ports without annotation returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
				},
			},
			wantErr:     true,
			errContains: "cannot determine warmup port",
		},
		{
			name: "invalid request count",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupRequests: "invalid",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid warmup-requests value",
		},
		{
			name: "request count less than 1",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupRequests: "0",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-requests must be at least 1",
		},
		{
			name: "request count exceeds maximum",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupRequests: "15000",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-requests must not exceed 12000",
		},
		{
			name: "invalid timeout",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupTimeout: "invalid",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid warmup-timeout value",
		},
		{
			name: "timeout too short",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupTimeout: "500ms",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-timeout must be at least 1s",
		},
		{
			name: "timeout exceeds maximum",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupTimeout: "10m",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-timeout must not exceed 5m0s",
		},
		{
			name: "invalid port annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupPort: "invalid",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid warmup-port value",
		},
		{
			name: "port out of range",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupPort: "70000",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-port must be between 1 and 65535",
		},
		{
			name: "grpc protocol with method",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol:   "grpc",
						webhook.AnnotationWarmupGRPCMethod: "my.service.v1.MyService/Warmup",
						webhook.AnnotationWarmupPort:       "9090",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolGRPC,
				GRPCMethod:   "my.service.v1.MyService/Warmup",
				GRPCPayload:  DefaultGRPCPayload,
				Port:         9090,
			},
		},
		{
			name: "grpc protocol with method and custom payload",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol:    "grpc",
						webhook.AnnotationWarmupGRPCMethod:  "my.service.v1.MyService/Warmup",
						webhook.AnnotationWarmupGRPCPayload: `{"key":"value"}`,
						webhook.AnnotationWarmupPort:        "9090",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolGRPC,
				GRPCMethod:   "my.service.v1.MyService/Warmup",
				GRPCPayload:  `{"key":"value"}`,
				Port:         9090,
			},
		},
		{
			name: "http protocol explicit",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol: "http",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Protocol:     ProtocolHTTP,
				GRPCPayload:  DefaultGRPCPayload,
				Port:         8080,
			},
		},
		{
			name: "grpc without method returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol: "grpc",
						webhook.AnnotationWarmupPort:     "9090",
					},
				},
			},
			wantErr:     true,
			errContains: "is required when",
		},
		{
			name: "grpc with invalid method format returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol:   "grpc",
						webhook.AnnotationWarmupGRPCMethod: "/Check",
						webhook.AnnotationWarmupPort:       "9090",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid gRPC method",
		},
		{
			name: "grpc with invalid json payload returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol:    "grpc",
						webhook.AnnotationWarmupGRPCMethod:  "my.service.v1.MyService/Warmup",
						webhook.AnnotationWarmupGRPCPayload: "not-json",
						webhook.AnnotationWarmupPort:        "9090",
					},
				},
			},
			wantErr:     true,
			errContains: "not valid JSON",
		},
		{
			name: "unknown protocol returns error",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupProtocol: "ftp",
						webhook.AnnotationWarmupPort:     "21",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid warmup-protocol value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseConfig(tt.pod)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseConfig() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseConfig() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseConfig() unexpected error = %v", err)
				return
			}

			if config.Endpoint != tt.wantConfig.Endpoint {
				t.Errorf("Endpoint = %v, want %v", config.Endpoint, tt.wantConfig.Endpoint)
			}
			if config.RequestCount != tt.wantConfig.RequestCount {
				t.Errorf("RequestCount = %v, want %v", config.RequestCount, tt.wantConfig.RequestCount)
			}
			if config.Timeout != tt.wantConfig.Timeout {
				t.Errorf("Timeout = %v, want %v", config.Timeout, tt.wantConfig.Timeout)
			}
			if config.Port != tt.wantConfig.Port {
				t.Errorf("Port = %v, want %v", config.Port, tt.wantConfig.Port)
			}
			if config.Protocol != tt.wantConfig.Protocol {
				t.Errorf("Protocol = %v, want %v", config.Protocol, tt.wantConfig.Protocol)
			}
			if config.GRPCMethod != tt.wantConfig.GRPCMethod {
				t.Errorf("GRPCMethod = %v, want %v", config.GRPCMethod, tt.wantConfig.GRPCMethod)
			}
			if config.GRPCPayload != tt.wantConfig.GRPCPayload {
				t.Errorf("GRPCPayload = %v, want %v", config.GRPCPayload, tt.wantConfig.GRPCPayload)
			}
		})
	}
}

func TestConfig_BuildEndpointURL(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name: "basic endpoint",
			config: &Config{
				PodIP:    "10.0.0.1",
				Port:     8080,
				Endpoint: "/",
			},
			want: "http://10.0.0.1:8080/",
		},
		{
			name: "custom path",
			config: &Config{
				PodIP:    "10.0.0.1",
				Port:     8080,
				Endpoint: "/api/warmup",
			},
			want: "http://10.0.0.1:8080/api/warmup",
		},
		{
			name: "path without leading slash",
			config: &Config{
				PodIP:    "10.0.0.1",
				Port:     3000,
				Endpoint: "health",
			},
			want: "http://10.0.0.1:3000/health",
		},
		{
			name: "empty path",
			config: &Config{
				PodIP:    "10.0.0.1",
				Port:     8080,
				Endpoint: "",
			},
			want: "http://10.0.0.1:8080/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.BuildEndpointURL(); got != tt.want {
				t.Errorf("BuildEndpointURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
