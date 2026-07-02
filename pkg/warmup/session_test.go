package warmup

import (
	"fmt"
	"sync"
	"testing"
)

func TestSessionContext_SetGet(t *testing.T) {
	sc := NewSessionContext()

	sc.Set("token", "abc123")
	v, ok := sc.Get("token")
	if !ok {
		t.Fatal("Get: key not found")
	}
	if v != "abc123" {
		t.Errorf("Get: got %v, want abc123", v)
	}

	_, ok = sc.Get("missing")
	if ok {
		t.Error("Get: expected false for missing key")
	}
}

func TestSessionContext_Interpolate(t *testing.T) {
	sc := NewSessionContext()
	sc.Set("token", "mytoken")
	sc.Set("user", "alice")
	sc.Set("count", 42)

	tests := []struct {
		input string
		want  string
	}{
		{"Bearer {{token}}", "Bearer mytoken"},
		{"Hello {{user}}!", "Hello alice!"},
		{"count={{count}}", "count=42"},
		{"no vars here", "no vars here"},
		{"{{missing}} stays", "{{missing}} stays"},
		{"{{token}} and {{user}}", "mytoken and alice"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sc.Interpolate(tt.input)
			if got != tt.want {
				t.Errorf("Interpolate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSessionContext_ConcurrentAccess(t *testing.T) {
	sc := NewSessionContext()
	const workers = 50

	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			sc.Set(fmt.Sprintf("key%d", i), i)
		}(i)
		go func(i int) {
			defer wg.Done()
			sc.Interpolate(fmt.Sprintf("{{key%d}}", i))
		}(i)
	}

	wg.Wait()
}
