package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"claude-go/internal/command"
)

func registerUsage(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "usage",
		Description: "show usage counters grouped by model",
		Load:        loadUsageModel,
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return strings.Join(usageLines(runtime), "\n"), nil
		},
	})
}

func usageLines(runtime command.Runtime) []string {
	if runtime.State == nil {
		return []string{"state store is not configured"}
	}
	state := runtime.State.Snapshot()
	lines := []string{
		"Usage",
		fmt.Sprintf("turns=%d", state.TurnCount),
		fmt.Sprintf("tool_calls=%d", state.ToolCallCount),
		fmt.Sprintf("api_duration=%s", state.TotalAPIDuration),
	}
	keys := make([]string, 0, len(state.ModelUsage))
	for model := range state.ModelUsage {
		keys = append(keys, model)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		lines = append(lines, "", "models:")
		for _, model := range keys {
			lines = append(lines, fmt.Sprintf("- %s: %d", model, state.ModelUsage[model]))
		}
	}
	return lines
}

func registerStats(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "stats",
		Description: "show session statistics",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.State == nil {
				return "", fmt.Errorf("state store is not configured")
			}
			state := runtime.State.Snapshot()
			lines := []string{
				fmt.Sprintf("session=%s", command.EmptyDash(state.SessionID)),
				fmt.Sprintf("model=%s", command.EmptyDash(state.CurrentModel)),
				fmt.Sprintf("turns=%d", state.TurnCount),
				fmt.Sprintf("tool_calls=%d", state.ToolCallCount),
				fmt.Sprintf("api_duration=%s", state.TotalAPIDuration),
				fmt.Sprintf("tool_duration=%s", state.TotalToolDuration),
				fmt.Sprintf("last_error=%s", command.EmptyDash(state.LastError)),
			}
			if len(state.ModelUsage) > 0 {
				keys := make([]string, 0, len(state.ModelUsage))
				for model := range state.ModelUsage {
					keys = append(keys, model)
				}
				sort.Strings(keys)
				lines = append(lines, "", "model_usage:")
				for _, model := range keys {
					lines = append(lines, fmt.Sprintf("- %s: %d", model, state.ModelUsage[model]))
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}

func registerCost(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "cost",
		Description: "show tracked cost counters",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.State == nil {
				return "", fmt.Errorf("state store is not configured")
			}
			state := runtime.State.Snapshot()
			return fmt.Sprintf("total_cost_usd=%.6f\napi_duration=%s\ntool_duration=%s", state.TotalCostUSD, state.TotalAPIDuration, state.TotalToolDuration), nil
		},
	})
}
