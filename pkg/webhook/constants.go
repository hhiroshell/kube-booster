package webhook

const (
	// AnnotationWarmupEnabled is the annotation key to enable warmup
	AnnotationWarmupEnabled = "kube-booster.io/warmup"

	// AnnotationWarmupEndpoint is the annotation key to specify the warmup endpoint
	AnnotationWarmupEndpoint = "kube-booster.io/warmup-endpoint"

	// AnnotationWarmupRequests is the annotation key to specify the number of warmup requests
	AnnotationWarmupRequests = "kube-booster.io/warmup-requests"

	// AnnotationWarmupDuration is the annotation key to specify the warmup duration
	AnnotationWarmupDuration = "kube-booster.io/warmup-duration"

	// AnnotationWarmupPort is the annotation key to specify the warmup port
	AnnotationWarmupPort = "kube-booster.io/warmup-port"

	// ReadinessGateName is the name of the readiness gate injected into pods
	ReadinessGateName = "kube-booster.io/warmup-ready"

	// ConditionTypeWarmupReady is the condition type for warmup readiness
	ConditionTypeWarmupReady = "kube-booster.io/warmup-ready"

	// WarmupEnabledValue is the value that enables warmup
	WarmupEnabledValue = "enabled"
)
