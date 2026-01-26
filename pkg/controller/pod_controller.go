package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/hhiroshell/kube-booster/pkg/webhook"
)

// PodReconciler reconciles pods with warmup readiness gates
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles pod reconciliation
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the pod
	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		if errors.IsNotFound(err) {
			// Pod was deleted
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch Pod")
		return ctrl.Result{}, err
	}

	// Check if our readiness gate exists
	hasGate := false
	for _, gate := range pod.Spec.ReadinessGates {
		if gate.ConditionType == corev1.PodConditionType(webhook.ReadinessGateName) {
			hasGate = true
			break
		}
	}

	if !hasGate {
		// This shouldn't happen due to predicates, but safety check
		return ctrl.Result{}, nil
	}

	// Check if our condition is already True
	if r.isConditionTrue(pod, webhook.ConditionTypeWarmupReady) {
		logger.V(1).Info("warmup condition already True, skipping")
		return ctrl.Result{}, nil
	}

	// Check if pod is in Running phase
	if pod.Status.Phase != corev1.PodRunning {
		logger.V(1).Info("pod not in Running phase, requeuing", "phase", pod.Status.Phase)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Check if all containers are ready
	if !r.areContainersReady(pod) {
		logger.V(1).Info("containers not ready, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Check if ContainersReady condition is True
	if !r.isConditionTrue(pod, string(corev1.ContainersReady)) {
		logger.V(1).Info("ContainersReady condition not True, requeuing")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// All conditions met, set our condition to True
	if err := r.setConditionTrue(ctx, pod); err != nil {
		logger.Error(err, "failed to update pod condition")
		return ctrl.Result{}, err
	}

	logger.Info("warmup readiness gate condition set to True")
	return ctrl.Result{}, nil
}

// isConditionTrue checks if a pod condition is True
func (r *PodReconciler) isConditionTrue(pod *corev1.Pod, conditionType string) bool {
	for _, condition := range pod.Status.Conditions {
		if string(condition.Type) == conditionType {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// areContainersReady checks if all containers are ready
func (r *PodReconciler) areContainersReady(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}
	return len(pod.Status.ContainerStatuses) > 0
}

// setConditionTrue updates the pod condition to True
func (r *PodReconciler) setConditionTrue(ctx context.Context, pod *corev1.Pod) error {
	// Create a copy for update
	podCopy := pod.DeepCopy()

	// Find and update or add the condition
	conditionUpdated := false
	for i, condition := range podCopy.Status.Conditions {
		if string(condition.Type) == webhook.ConditionTypeWarmupReady {
			podCopy.Status.Conditions[i].Status = corev1.ConditionTrue
			podCopy.Status.Conditions[i].LastTransitionTime = metav1.Now()
			podCopy.Status.Conditions[i].Reason = "WarmupComplete"
			podCopy.Status.Conditions[i].Message = "Warmup readiness check passed"
			conditionUpdated = true
			break
		}
	}

	if !conditionUpdated {
		// Add new condition
		newCondition := corev1.PodCondition{
			Type:               corev1.PodConditionType(webhook.ConditionTypeWarmupReady),
			Status:             corev1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "WarmupComplete",
			Message:            "Warmup readiness check passed",
		}
		podCopy.Status.Conditions = append(podCopy.Status.Conditions, newCondition)
	}

	// Update pod status
	if err := r.Status().Update(ctx, podCopy); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(HasReadinessGatePredicate()).
		Complete(r)
}
