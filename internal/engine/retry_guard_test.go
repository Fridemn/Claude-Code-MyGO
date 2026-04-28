package engine

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"claude-go/internal/tool"
)

type fakeDeniedTool struct {
	callCount int
}

func (f *fakeDeniedTool) Name() string {
	return "Bash"
}

func (f *fakeDeniedTool) Description() string {
	return "fake bash tool"
}

func (f *fakeDeniedTool) IsReadOnly(tool.Input) bool {
	return false
}

func (f *fakeDeniedTool) Call(context.Context, tool.Input, tool.Runtime) (tool.Result, error) {
	f.callCount++
	return tool.Result{}, fmt.Errorf("permission required: Command 'rm' is potentially destructive")
}

func TestCallToolWithRetryGuardCachesDeniedCommandAcrossCalls(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	fake := &fakeDeniedTool{}
	reg.Register(fake)

	eng := &Engine{
		tools:                reg,
		nonRetryableFailures: map[string]string{},
	}

	call := tool.CallSpec{
		Name: "Bash",
		Input: tool.Input{
			"command": "rm /tmp/demo.txt",
		},
	}

	first := eng.callToolWithRetryGuard(context.Background(), call)
	if !strings.Contains(strings.ToLower(first.result.Error), "permission required:") {
		t.Fatalf("expected first call to return permission-required error, got %q", first.result.Error)
	}
	if fake.callCount != 1 {
		t.Fatalf("expected tool to be executed once, got %d", fake.callCount)
	}

	second := eng.callToolWithRetryGuard(context.Background(), call)
	if !strings.Contains(second.result.Error, "this call was already denied") {
		t.Fatalf("expected second call to be short-circuited, got %q", second.result.Error)
	}
	if fake.callCount != 1 {
		t.Fatalf("expected second call to skip tool execution, got %d total executions", fake.callCount)
	}
}
