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
				Duration:     DefaultDuration,
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
				Duration:     DefaultDuration,
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
				Duration:     DefaultDuration,
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
				Duration:     DefaultDuration,
				Port:         8080,
			},
		},
		{
			name: "custom duration",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupDuration: "60s",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     DefaultEndpointPath,
				RequestCount: DefaultRequestCount,
				Duration:     60 * time.Second,
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
						webhook.AnnotationWarmupDuration: "15s",
						webhook.AnnotationWarmupPort:     "3000",
					},
				},
			},
			wantConfig: &Config{
				Endpoint:     "/health",
				RequestCount: 5,
				Duration:     15 * time.Second,
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
			name: "invalid duration",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupDuration: "invalid",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "invalid warmup-duration value",
		},
		{
			name: "duration too short",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupDuration: "500ms",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
			},
			wantErr:     true,
			errContains: "warmup-duration must be at least 1s",
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
			if config.Duration != tt.wantConfig.Duration {
				t.Errorf("Duration = %v, want %v", config.Duration, tt.wantConfig.Duration)
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
