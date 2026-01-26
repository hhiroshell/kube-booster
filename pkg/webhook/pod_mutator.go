package webhook

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// PodMutator handles pod mutation for injecting readiness gates
type PodMutator struct {
	Client  client.Client
	decoder admission.Decoder
}

// NewPodMutator creates a new PodMutator with the given client and scheme
func NewPodMutator(c client.Client, scheme *runtime.Scheme) *PodMutator {
	return &PodMutator{
		Client:  c,
		decoder: admission.NewDecoder(scheme),
	}
}

// Handle processes the admission request
func (pm *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := pm.decoder.DecodeRaw(req.Object, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if warmup is enabled via annotation
	if pod.Annotations[AnnotationWarmupEnabled] != WarmupEnabledValue {
		return admission.Allowed("warmup not enabled")
	}

	// Check if readiness gate already exists (idempotency)
	for _, gate := range pod.Spec.ReadinessGates {
		if gate.ConditionType == corev1.PodConditionType(ReadinessGateName) {
			return admission.Allowed("readiness gate already present")
		}
	}

	// Inject readiness gate
	if pod.Spec.ReadinessGates == nil {
		pod.Spec.ReadinessGates = []corev1.PodReadinessGate{}
	}
	pod.Spec.ReadinessGates = append(pod.Spec.ReadinessGates, corev1.PodReadinessGate{
		ConditionType: corev1.PodConditionType(ReadinessGateName),
	})

	// Marshal the modified pod
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Return patch response
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder
func (pm *PodMutator) InjectDecoder(d admission.Decoder) error {
	pm.decoder = d
	return nil
}
