package warmup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"

	v1alpha1 "github.com/hhiroshell/kube-booster/pkg/api/v1alpha1"
)

const (
	defaultScenarioTimeout = 120 * time.Second
	defaultStepTimeout     = 30 * time.Second
	maxScenarioTimeout     = 5 * time.Minute
)

// ScenarioExecutor is implemented by defaultScenarioExecutor and any test doubles.
type ScenarioExecutor interface {
	ExecuteScenario(ctx context.Context, config *Config, spec *v1alpha1.WarmupConfigSpec) *Result
}

// Ensure defaultScenarioExecutor implements ScenarioExecutor.
var _ ScenarioExecutor = (*defaultScenarioExecutor)(nil)

// ScenarioExecutorOption is a functional option for defaultScenarioExecutor.
type ScenarioExecutorOption func(*defaultScenarioExecutor)

// WithScenarioRateLimiter sets the shared rate limiter on the defaultScenarioExecutor.
// Passing nil disables rate limiting.
func WithScenarioRateLimiter(rl *RequestRateLimiter) ScenarioExecutorOption {
	return func(e *defaultScenarioExecutor) {
		e.rateLimiter = rl
	}
}

// defaultScenarioExecutor orchestrates multi-step, scenario-based warmup defined in a
// WarmupConfig CRD. Steps are executed sequentially; within a step, requests are
// executed sequentially with optional {{varName}} interpolation from prior responses.
type defaultScenarioExecutor struct {
	logger      logr.Logger
	rateLimiter *RequestRateLimiter
	httpClient  *http.Client
}

// NewScenarioExecutor creates a new ScenarioExecutor.
func NewScenarioExecutor(logger logr.Logger, opts ...ScenarioExecutorOption) *defaultScenarioExecutor {
	e := &defaultScenarioExecutor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ExecuteScenario runs all steps in spec sequentially and returns a combined Result.
func (e *defaultScenarioExecutor) ExecuteScenario(
	ctx context.Context,
	config *Config,
	spec *v1alpha1.WarmupConfigSpec,
) *Result {
	result := &Result{}

	if config.PodIP == "" {
		result.Error = ErrNoPodIP
		result.Message = "cannot execute scenario: pod IP not set"
		return result
	}

	overallTimeout := defaultScenarioTimeout
	if spec.Timeout != "" {
		if d, err := time.ParseDuration(spec.Timeout); err == nil && d > 0 {
			overallTimeout = d
		}
	}
	if overallTimeout > maxScenarioTimeout {
		overallTimeout = maxScenarioTimeout
	}

	scenarioCtx, cancel := context.WithTimeout(ctx, overallTimeout)
	defer cancel()

	if len(spec.Steps) == 0 {
		return &Result{Success: true, Message: "scenario warmup skipped: no steps defined"}
	}

	session := NewSessionContext()
	start := time.Now()
	totalCompleted, totalFailed := 0, 0

	for stepIdx, step := range spec.Steps {
		if scenarioCtx.Err() != nil {
			break
		}

		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("step-%d", stepIdx+1)
		}

		stepTimeout := defaultStepTimeout
		if step.Timeout != "" {
			if d, err := time.ParseDuration(step.Timeout); err == nil && d > 0 {
				stepTimeout = d
			}
		}

		stepCtx, stepCancel := context.WithTimeout(scenarioCtx, stepTimeout)
		completed, failed := e.executeStep(stepCtx, config, step, session, stepName)
		stepCancel()

		totalCompleted += completed
		totalFailed += failed
	}

	result.RequestsCompleted = totalCompleted
	result.RequestsFailed = totalFailed
	result.TotalDuration = time.Since(start)
	result.Success = totalCompleted > 0

	if scenarioCtx.Err() != nil {
		result.Error = scenarioCtx.Err()
	}

	result.Message = result.BuildMessage()

	e.logger.V(1).Info("scenario completed",
		"pod", config.PodName,
		"namespace", config.PodNamespace,
		"requestsCompleted", totalCompleted,
		"requestsFailed", totalFailed,
		"duration", result.TotalDuration)

	return result
}

// executeStep runs all requests in a step sequentially and returns (completed, failed).
func (e *defaultScenarioExecutor) executeStep(
	ctx context.Context,
	config *Config,
	step v1alpha1.WarmupStep,
	session *SessionContext,
	stepName string,
) (completed, failed int) {
	// GRPCSender is created per-step (different steps may target different gRPC methods).
	var grpcSender *GRPCSender
	defer func() {
		if grpcSender != nil {
			_ = grpcSender.Close() //nolint:errcheck // gRPC connection close errors are non-actionable
		}
	}()

	for reqIdx, req := range step.Requests {
		if ctx.Err() != nil {
			break
		}

		reqName := req.Name
		if reqName == "" {
			reqName = fmt.Sprintf("%s/req-%d", stepName, reqIdx+1)
		}

		protocol := req.Protocol
		if protocol == "" {
			protocol = config.Protocol
		}
		if protocol == "" {
			protocol = ProtocolHTTP
		}

		count := req.Count
		if count < 1 {
			count = 1
		}

		var lastBody []byte
		httpSender := &HTTPSender{client: e.httpClient, logger: e.logger}
		for i := 0; i < count; i++ {
			if ctx.Err() != nil {
				break
			}
			if err := e.rateLimiter.Wait(ctx); err != nil {
				failed += count - i
				break
			}

			var resp *Response
			switch protocol {
			case ProtocolGRPC:
				if grpcSender == nil {
					grpcSender = NewGRPCSender(e.logger)
				}
				resp = grpcSender.Send(ctx, Target{
					Address: config.BuildGRPCAddress(),
					Method:  req.GRPCMethod,
					Payload: []byte(session.Interpolate(req.GRPCPayload)),
				})
			default:
				endpoint := session.Interpolate(req.Endpoint)
				if endpoint == "" {
					endpoint = DefaultEndpointPath
				}
				body := []byte(session.Interpolate(req.Body))

				interpolatedHeaders := make(map[string]string, len(req.Headers)+2)
				interpolatedHeaders["User-Agent"] = "kube-booster/1.0"
				interpolatedHeaders["X-Warmup-Request"] = "true"
				for k, v := range req.Headers {
					interpolatedHeaders[k] = session.Interpolate(v)
				}

				method := req.Method
				if method == "" {
					method = http.MethodGet
				}

				resp = httpSender.Send(ctx, Target{
					Address: config.BuildEndpointURLFor(endpoint),
					Method:  method,
					Headers: interpolatedHeaders,
					Payload: body,
				})
			}

			if resp.Error != nil {
				failed++
				e.logger.V(2).Info("request failed", "request", reqName, "error", resp.Error)
				continue
			}

			lastBody = resp.Body

			ok := isSuccess(resp.StatusCode, req.ExpectedStatus, protocol)
			if ok {
				completed++
			} else {
				failed++
				e.logger.V(2).Info("request returned unexpected status",
					"request", reqName, "status", resp.StatusCode, "expected", req.ExpectedStatus)
			}
		}

		// Extract session variables from the last response body.
		if len(req.Extract) > 0 && len(lastBody) > 0 {
			extractVariables(lastBody, req.Extract, session, e.logger, reqName)
		}
	}
	return completed, failed
}

// isSuccess returns true when the response should be counted as completed.
func isSuccess(statusCode, expectedStatus int, protocol string) bool {
	if protocol == ProtocolGRPC {
		return statusCode == 200
	}
	if expectedStatus != 0 {
		return statusCode == expectedStatus
	}
	return statusCode >= 200 && statusCode < 400
}

// extractVariables parses body as JSON and stores matched JSONPath values in session.
// Only supports simple dot-paths of the form $.key or $.a.b (no arrays, no filters).
func extractVariables(body []byte, extract map[string]string, session *SessionContext, logger logr.Logger, reqName string) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		logger.V(1).Info("extract: response body is not valid JSON", "request", reqName)
		return
	}
	for varName, path := range extract {
		val, err := jsonPathLookup(doc, path)
		if err != nil {
			logger.V(1).Info("extract: JSONPath lookup failed",
				"request", reqName, "var", varName, "path", path, "error", err)
			continue
		}
		session.Set(varName, val)
	}
}

// jsonPathLookup resolves a simple dot-path expression ($.key or $.a.b) in a JSON map.
// Only map traversal is supported; arrays and filter expressions are not.
func jsonPathLookup(doc map[string]any, path string) (any, error) {
	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("unsupported JSONPath expression %q: must start with '$.'", path)
	}
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	var cur any = doc
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path segment %q: value is not an object", part)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("key %q not found", part)
		}
	}
	return cur, nil
}
