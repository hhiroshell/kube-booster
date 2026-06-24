package warmup

import (
	"context"

	v1alpha1 "github.com/hhiroshell/kube-booster/pkg/api/v1alpha1"
)

// MockExecutor is a mock implementation of Executor for testing
type MockExecutor struct {
	Result *Result
}

// Execute returns the configured Result or a default success result
func (m *MockExecutor) Execute(ctx context.Context, config *Config) *Result {
	if m.Result != nil {
		return m.Result
	}
	return &Result{
		Success:           true,
		RequestsCompleted: config.RequestCount,
		Message:           "mock warmup completed",
	}
}

// Ensure MockExecutor implements Executor
var _ Executor = (*MockExecutor)(nil)

// MockScenarioExecutor is a mock implementation of ScenarioExecutorIface for testing.
type MockScenarioExecutor struct {
	Result *Result
	Called bool
}

// ExecuteScenario records the call and returns the configured Result.
func (m *MockScenarioExecutor) ExecuteScenario(_ context.Context, _ *Config, _ *v1alpha1.WarmupConfigSpec) *Result {
	m.Called = true
	if m.Result != nil {
		return m.Result
	}
	return &Result{
		Success:           true,
		RequestsCompleted: 1,
		Message:           "mock scenario completed",
	}
}

// Ensure MockScenarioExecutor implements ScenarioExecutorIface.
var _ ScenarioExecutorIface = (*MockScenarioExecutor)(nil)
