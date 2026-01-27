package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/hhiroshell/kube-booster/pkg/warmup"
	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

func TestPodReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name             string
		pod              *corev1.Pod
		wantRequeue      bool
		wantCondition    bool
		wantConditionVal corev1.ConditionStatus
	}{
		{
			name: "set condition to True when all containers ready",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled: "enabled",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.ContainersReady,
							Status: corev1.ConditionTrue,
						},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "test", Ready: true},
					},
				},
			},
			wantRequeue:      false,
			wantCondition:    true,
			wantConditionVal: corev1.ConditionTrue,
		},
		{
			name: "requeue when pod not running",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled: "enabled",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			wantRequeue:   true,
			wantCondition: false,
		},
		{
			name: "requeue when containers not ready",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled: "enabled",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "test", Ready: false},
					},
				},
			},
			wantRequeue:   true,
			wantCondition: false,
		},
		{
			name: "skip when condition already True",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled: "enabled",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.ContainersReady,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodConditionType(webhook.ConditionTypeWarmupReady),
							Status: corev1.ConditionTrue,
						},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "test", Ready: true},
					},
				},
			},
			wantRequeue:      false,
			wantCondition:    true,
			wantConditionVal: corev1.ConditionTrue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pod).
				WithStatusSubresource(tt.pod).
				Build()

			// Create mock executor that always succeeds
			mockExecutor := &warmup.MockExecutor{
				Result: &warmup.Result{
					Success:           true,
					RequestsCompleted: 3,
					Message:           "mock warmup completed",
				},
			}

			reconciler := &PodReconciler{
				Client:         client,
				Scheme:         scheme,
				WarmupExecutor: mockExecutor,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.pod.Name,
					Namespace: tt.pod.Namespace,
				},
			}

			result, err := reconciler.Reconcile(context.Background(), req)
			if err != nil {
				t.Errorf("Reconcile() error = %v", err)
				return
			}

			if tt.wantRequeue && result.RequeueAfter == 0 {
				t.Error("Expected requeue but got none")
			}

			if !tt.wantRequeue && result.RequeueAfter > 0 {
				t.Errorf("Expected no requeue but got RequeueAfter = %v", result.RequeueAfter)
			}

			// Verify pod condition
			pod := &corev1.Pod{}
			if err := client.Get(context.Background(), req.NamespacedName, pod); err != nil {
				t.Errorf("Failed to get pod: %v", err)
				return
			}

			foundCondition := false
			for _, condition := range pod.Status.Conditions {
				if string(condition.Type) == webhook.ConditionTypeWarmupReady {
					foundCondition = true
					if tt.wantCondition && condition.Status != tt.wantConditionVal {
						t.Errorf("Condition status = %v, want %v", condition.Status, tt.wantConditionVal)
					}
					break
				}
			}

			if tt.wantCondition && !foundCondition {
				t.Error("Expected warmup condition to be set but it was not found")
			}
		})
	}
}

func TestPodReconciler_areContainersReady(t *testing.T) {
	reconciler := &PodReconciler{}

	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "all containers ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true},
						{Ready: true},
					},
				},
			},
			want: true,
		},
		{
			name: "some containers not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{Ready: true},
						{Ready: false},
					},
				},
			},
			want: false,
		},
		{
			name: "no containers",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reconciler.areContainersReady(tt.pod); got != tt.want {
				t.Errorf("areContainersReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodReconciler_isConditionTrue(t *testing.T) {
	reconciler := &PodReconciler{}

	tests := []struct {
		name          string
		pod           *corev1.Pod
		conditionType string
		want          bool
	}{
		{
			name: "condition is True",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.ContainersReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			conditionType: string(corev1.ContainersReady),
			want:          true,
		},
		{
			name: "condition is False",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.ContainersReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			conditionType: string(corev1.ContainersReady),
			want:          false,
		},
		{
			name: "condition not found",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{},
				},
			},
			conditionType: string(corev1.ContainersReady),
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reconciler.isConditionTrue(tt.pod, tt.conditionType); got != tt.want {
				t.Errorf("isConditionTrue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodReconciler_WarmupIntegration(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		pod           *corev1.Pod
		executor      warmup.Executor
		wantReason    string
		wantMsgPrefix string
	}{
		{
			name: "warmup success",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled:  "enabled",
						webhook.AnnotationWarmupEndpoint: "/health",
						webhook.AnnotationWarmupRequests: "5",
						webhook.AnnotationWarmupPort:     "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "app", Ready: true},
					},
				},
			},
			executor: &warmup.MockExecutor{
				Result: &warmup.Result{
					Success:           true,
					RequestsCompleted: 5,
					Message:           "warmup completed: 5/5 requests succeeded",
				},
			},
			wantReason:    "WarmupComplete",
			wantMsgPrefix: "warmup completed",
		},
		{
			name: "warmup failure (fail-open)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						webhook.AnnotationWarmupEnabled: "enabled",
						webhook.AnnotationWarmupPort:    "8080",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "app", Ready: true},
					},
				},
			},
			executor: &warmup.MockExecutor{
				Result: &warmup.Result{
					Success:           false,
					RequestsCompleted: 0,
					RequestsFailed:    3,
					Message:           "warmup failed: connection refused",
				},
			},
			wantReason:    "WarmupFailedOpen",
			wantMsgPrefix: "Warmup failed but pod marked ready",
		},
		{
			name: "no executor configured with single container port",
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
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "app", Ready: true},
					},
				},
			},
			executor:      nil, // No executor
			wantReason:    "WarmupComplete",
			wantMsgPrefix: "warmup skipped",
		},
		{
			name: "config error with multiple containers (fail-open)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app1", Image: "nginx"},
						{Name: "app2", Image: "redis"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "app1", Ready: true},
						{Name: "app2", Ready: true},
					},
				},
			},
			executor:      nil,
			wantReason:    "WarmupFailedOpen",
			wantMsgPrefix: "Warmup failed but pod marked ready",
		},
		{
			name: "config error with single container multiple ports (fail-open)",
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
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(webhook.ReadinessGateName)},
					},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.1",
					Conditions: []corev1.PodCondition{
						{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{Name: "app", Ready: true},
					},
				},
			},
			executor:      nil,
			wantReason:    "WarmupFailedOpen",
			wantMsgPrefix: "Warmup failed but pod marked ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pod).
				WithStatusSubresource(tt.pod).
				Build()

			reconciler := &PodReconciler{
				Client:         client,
				Scheme:         scheme,
				WarmupExecutor: tt.executor,
			}

			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.pod.Name,
					Namespace: tt.pod.Namespace,
				},
			}

			_, err := reconciler.Reconcile(context.Background(), req)
			if err != nil {
				t.Errorf("Reconcile() error = %v", err)
				return
			}

			// Verify pod condition
			pod := &corev1.Pod{}
			if err := client.Get(context.Background(), req.NamespacedName, pod); err != nil {
				t.Errorf("Failed to get pod: %v", err)
				return
			}

			for _, condition := range pod.Status.Conditions {
				if string(condition.Type) == webhook.ConditionTypeWarmupReady {
					if condition.Status != corev1.ConditionTrue {
						t.Errorf("Condition status = %v, want True", condition.Status)
					}
					if condition.Reason != tt.wantReason {
						t.Errorf("Condition reason = %v, want %v", condition.Reason, tt.wantReason)
					}
					if !hasPrefix(condition.Message, tt.wantMsgPrefix) {
						t.Errorf("Condition message = %v, want prefix %v", condition.Message, tt.wantMsgPrefix)
					}
					return
				}
			}
			t.Error("Expected warmup condition to be set but it was not found")
		})
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
