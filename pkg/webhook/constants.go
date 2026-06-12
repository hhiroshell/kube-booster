package webhook

const (
	// AnnotationWarmupEnabled is the annotation key to enable warmup
	AnnotationWarmupEnabled = "kube-booster.io/warmup"

	// AnnotationWarmupEndpoint is the annotation key to specify the warmup endpoint
	AnnotationWarmupEndpoint = "kube-booster.io/warmup-endpoint"

	// AnnotationWarmupRequests is the annotation key to specify the number of warmup requests
	AnnotationWarmupRequests = "kube-booster.io/warmup-requests"

	// AnnotationWarmupTimeout is the annotation key to specify the maximum warmup timeout
	AnnotationWarmupTimeout = "kube-booster.io/warmup-timeout"

	// AnnotationWarmupPort is the annotation key to specify the warmup port
	AnnotationWarmupPort = "kube-booster.io/warmup-port"

	// AnnotationWarmupProtocol is the annotation key to specify the warmup protocol ("http" or "grpc")
	AnnotationWarmupProtocol = "kube-booster.io/warmup-protocol"

	// AnnotationWarmupGRPCMethod is the annotation key to specify the gRPC method ("package.Service/Method")
	AnnotationWarmupGRPCMethod = "kube-booster.io/warmup-grpc-method"

	// AnnotationWarmupGRPCPayload is the annotation key to specify the gRPC request payload (JSON)
	AnnotationWarmupGRPCPayload = "kube-booster.io/warmup-grpc-payload"

	// ReadinessGateName is the name of the readiness gate injected into pods
	ReadinessGateName = "kube-booster.io/warmup-ready"

	// ConditionTypeWarmupReady is the condition type for warmup readiness
	ConditionTypeWarmupReady = "kube-booster.io/warmup-ready"

	// WarmupEnabledValue is the value that enables warmup
	WarmupEnabledValue = "enabled"
)
