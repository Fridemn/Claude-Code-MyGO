package sleep

import (
	"context"
	"fmt"
	"time"

	"claude-code-go/internal/tool"
)

// SleepTool waits for a specified duration.
// It can be interrupted via context cancellation.
type SleepTool struct{}

func (SleepTool) Name() string { return "Sleep" }

func (SleepTool) Description() string {
	return `Wait for a specified duration. The user can interrupt the sleep at any time.

Use this when the user tells you to sleep or rest, when you have nothing to do, or when you're waiting for something.

You can call this concurrently with other tools — it won't interfere with them.

Prefer this over ` + "`Bash(sleep ...)`" + ` — it doesn't hold a shell process.`
}

func (SleepTool) IsReadOnly(tool.Input) bool { return true }

func (SleepTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"duration_ms": tool.SchemaInteger("Duration to wait in milliseconds. Must be a positive integer."),
	}, "duration_ms")
}

func (SleepTool) Call(ctx context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	durationMs, ok := in["duration_ms"]
	if !ok {
		return tool.Result{}, fmt.Errorf("duration_ms is required")
	}

	var ms int
	switch v := durationMs.(type) {
	case int:
		ms = v
	case int64:
		ms = int(v)
	case float64:
		ms = int(v)
	default:
		return tool.Result{}, fmt.Errorf("duration_ms must be an integer")
	}

	if ms <= 0 {
		return tool.Result{}, fmt.Errorf("duration_ms must be a positive integer")
	}

	duration := time.Duration(ms) * time.Millisecond

	select {
	case <-time.After(duration):
		return tool.Result{Content: fmt.Sprintf("Waited for %d milliseconds", ms)}, nil
	case <-ctx.Done():
		return tool.Result{Content: "Sleep interrupted"}, ctx.Err()
	}
}

func RegisterSleepTools(r *tool.Registry) {
	r.Register(SleepTool{})
}