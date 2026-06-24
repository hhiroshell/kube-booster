package warmup

import (
	"fmt"
	"strings"
	"sync"
)

// SessionContext is a thread-safe key/value store used to pass values between
// warmup requests within a single scenario execution. Values are set via Extract
// rules on WarmupRequest and interpolated into subsequent request fields using
// {{varName}} syntax.
type SessionContext struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewSessionContext returns an empty SessionContext.
func NewSessionContext() *SessionContext {
	return &SessionContext{data: make(map[string]any)}
}

// Set stores a value under key. Safe for concurrent use.
func (sc *SessionContext) Set(key string, value any) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.data[key] = value
}

// Get retrieves a value by key. Returns (value, true) if found.
// Safe for concurrent use.
func (sc *SessionContext) Get(key string) (any, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	v, ok := sc.data[key]
	return v, ok
}

// Interpolate replaces every {{varName}} token in s with the string representation
// of the corresponding session value. Tokens referencing unknown keys are left as-is.
// Safe for concurrent use.
func (sc *SessionContext) Interpolate(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	for k, v := range sc.data {
		s = strings.ReplaceAll(s, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	return s
}
