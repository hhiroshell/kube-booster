package warmup

import "context"

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
