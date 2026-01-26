package warmup

import (
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
			name: "nil pod returns defaults",
			pod:  nil,
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Port:         DefaultPort,
			},
		},
		{
			name: "pod without annotations returns defaults",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Port:         DefaultPort,
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
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     "/api/warmup",
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Port:         DefaultPort,
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
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: 10,
				Timeout:      DefaultTimeout,
				Port:         DefaultPort,
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
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      60 * time.Second,
				Port:         DefaultPort,
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
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     "/health",
				RequestCount: 5,
				Timeout:      15 * time.Second,
				Port:         DefaultPort,
			},
		},
		{
			name: "port from container spec",
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
								{ContainerPort: 3000},
							},
						},
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Timeout:      DefaultTimeout,
				Port:         3000,
			},
		},
		{
			name: "invalid request count",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupRequests: "invalid",
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
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-requests must be at least 1",
		},
		{
			name: "invalid timeout",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupTimeout: "invalid",
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
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-timeout must be at least 1s",
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
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
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

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
