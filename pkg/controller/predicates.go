package controller

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

// HasReadinessGatePredicate filters events to only process pods with our readiness gate
func HasReadinessGatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return hasReadinessGate(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return hasReadinessGate(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // We don't care about deletions
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasReadinessGate(e.Object)
		},
	}
}

// hasReadinessGate checks if a pod has our readiness gate
func hasReadinessGate(obj interface{}) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}

	for _, gate := range pod.Spec.ReadinessGates {
		if gate.ConditionType == corev1.PodConditionType(webhook.ReadinessGateName) {
			return true
		}
	}
	return false
}
