package webhook

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestPodMutator_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme) //nolint:errcheck // scheme registration never fails

	tests := []struct {
		name        string
		pod         *corev1.Pod
		wantPatches bool
		wantAllowed bool
		wantMessage string
	}{
		{
			name: "inject readiness gate when warmup is enabled",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationWarmupEnabled: WarmupEnabledValue,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
				},
			},
			wantPatches: true,
			wantAllowed: true,
		},
		{
			name: "skip injection when warmup is not enabled",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						"other-annotation": "value",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
				},
			},
			wantPatches: false,
			wantAllowed: true,
			wantMessage: "warmup not enabled",
		},
		{
			name: "skip injection when readiness gate already exists",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationWarmupEnabled: WarmupEnabledValue,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
					ReadinessGates: []corev1.PodReadinessGate{
						{ConditionType: corev1.PodConditionType(ReadinessGateName)},
					},
				},
			},
			wantPatches: false,
			wantAllowed: true,
			wantMessage: "readiness gate already present",
		},
		{
			name: "skip injection when warmup annotation has wrong value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationWarmupEnabled: "disabled",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "test", Image: "nginx"},
					},
				},
			},
			wantPatches: false,
			wantAllowed: true,
			wantMessage: "warmup not enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := admission.NewDecoder(scheme)
			mutator := &PodMutator{
				decoder: decoder,
			}

			// Encode pod to JSON
			podBytes, err := json.Marshal(tt.pod)
			if err != nil {
				t.Fatalf("failed to marshal pod: %v", err)
			}

			// Create admission request
			req := admission.Request{}
			req.Object = runtime.RawExtension{
				Raw: podBytes,
			}

			// Handle the request
			resp := mutator.Handle(context.Background(), req)

			// Verify response
			if resp.Allowed != tt.wantAllowed {
				t.Errorf("Handle() allowed = %v, want %v", resp.Allowed, tt.wantAllowed)
			}

			if tt.wantMessage != "" && resp.Result != nil && resp.Result.Message != tt.wantMessage {
				t.Errorf("Handle() message = %v, want %v", resp.Result.Message, tt.wantMessage)
			}

			// Verify patches match expectation
			gotPatches := len(resp.Patches) > 0
			if gotPatches != tt.wantPatches {
				t.Errorf("Handle() gotPatches = %v, wantPatches = %v", gotPatches, tt.wantPatches)
			}
		})
	}
}

func TestPodMutator_InjectDecoder(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme) //nolint:errcheck // scheme registration never fails

	mutator := &PodMutator{}
	decoder := admission.NewDecoder(scheme)

	err := mutator.InjectDecoder(decoder)
	if err != nil {
		t.Errorf("InjectDecoder() error = %v", err)
	}

	if mutator.decoder == nil {
		t.Error("InjectDecoder() did not set decoder")
	}
}
