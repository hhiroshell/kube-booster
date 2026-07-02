package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// WarmupConfig defines multi-step, scenario-based warmup requests for a pod.
// A pod selects a WarmupConfig by setting the kube-booster.io/warmup-config
// annotation to the name of the WarmupConfig in the same namespace.
type WarmupConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WarmupConfigSpec `json:"spec"`
}

// WarmupConfigList contains a list of WarmupConfig.
type WarmupConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []WarmupConfig `json:"items"`
}

// WarmupConfigSpec is the desired state of a WarmupConfig.
type WarmupConfigSpec struct {
	// Steps is the ordered list of warmup steps. Steps are executed sequentially.
	// +kubebuilder:validation:MinItems=1
	Steps []WarmupStep `json:"steps"`

	// Timeout is the overall time limit for all steps combined.
	// Parsed as a Go duration string (e.g. "120s", "2m"). Default: "120s".
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

// WarmupStep groups one or more requests that are executed as a unit.
type WarmupStep struct {
	// Name is an optional human-readable label for the step (used in logs and events).
	// +optional
	Name string `json:"name,omitempty"`

	// Requests is the list of warmup requests executed within this step. Requests
	// are executed sequentially in the order they appear.
	// +kubebuilder:validation:MinItems=1
	Requests []WarmupRequest `json:"requests"`

	// Timeout is the time limit for this step. Parsed as a Go duration string.
	// Default: "30s".
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

// WarmupRequest describes a single warmup call within a step.
type WarmupRequest struct {
	// Name is an optional label used in log output.
	// +optional
	Name string `json:"name,omitempty"`

	// Protocol selects the warmup transport: "http" (default) or "grpc".
	// If omitted, the protocol defaults to the pod annotation value or "http".
	// +kubebuilder:validation:Enum=http;grpc
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Endpoint is the URL path for HTTP requests (e.g. "/api/cache/load").
	// Ignored for gRPC requests.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Method is the HTTP verb (e.g. "GET", "POST"). Default: "GET".
	// Ignored for gRPC requests.
	// +optional
	Method string `json:"method,omitempty"`

	// Headers contains additional HTTP request headers.
	// Values may reference session variables with {{varName}} syntax.
	// Ignored for gRPC requests.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the HTTP request body.
	// Values may reference session variables with {{varName}} syntax.
	// Ignored for gRPC requests.
	// +optional
	Body string `json:"body,omitempty"`

	// GRPCMethod is the fully-qualified gRPC method ("package.Service/Method").
	// Required when Protocol is "grpc". Requires server reflection to be enabled on
	// the target pod.
	// +optional
	GRPCMethod string `json:"grpcMethod,omitempty"`

	// GRPCPayload is the JSON-encoded request message for gRPC warmup.
	// Values may reference session variables with {{varName}} syntax.
	// Default: "{}".
	// +optional
	GRPCPayload string `json:"grpcPayload,omitempty"`

	// Count is the number of times to repeat this request. Default: 1.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Count int `json:"count,omitempty"`

	// Extract maps session variable names to simple JSONPath expressions
	// (e.g. "$.token" or "$.nested.key"). The value is extracted from the last
	// response body and stored in the session for use by subsequent requests via
	// {{varName}} interpolation. Supported syntax: $.key and $.a.b (no arrays,
	// no filters).
	// +optional
	Extract map[string]string `json:"extract,omitempty"`

	// ExpectedStatus is the HTTP status code that counts as success. When set,
	// any other status code increments RequestsFailed (warmup still continues
	// fail-open). When 0 (default), status codes 200–399 are treated as success.
	// Ignored for gRPC requests.
	// +optional
	ExpectedStatus int `json:"expectedStatus,omitempty"`
}
