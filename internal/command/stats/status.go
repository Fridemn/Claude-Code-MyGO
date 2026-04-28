package stats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"claude-go/internal/command"
)

func registerStatus(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "status",
		Description: "show current session status",
		Load:        loadStatusModel,
		Handler: func(ctx context.Context, runtime command.Runtime, _ []string) (string, error) {
			return buildStatusText(runtime), nil
		},
	})
}

// buildStatusText creates status output similar to TS Settings Status tab
func buildStatusText(runtime command.Runtime) string {
	var lines []string
	lines = append(lines, "Status")

	if runtime.State == nil {
		lines = append(lines, "", "state store is not configured")
		return strings.Join(lines, "\n")
	}

	state := runtime.State.Snapshot()

	// Session info
	lines = append(lines, "", "Session:")
	lines = append(lines, fmt.Sprintf("  id: %s", emptyDash(state.SessionID)))
	lines = append(lines, fmt.Sprintf("  model: %s", emptyDash(state.CurrentModel)))
	lines = append(lines, fmt.Sprintf("  cwd: %s", state.CWD))
	lines = append(lines, fmt.Sprintf("  interactive: %v", state.IsInteractive))

	// Timing
	lines = append(lines, "", "Timing:")
	lines = append(lines, fmt.Sprintf("  started: %s", state.StartTime.Format(time.RFC3339)))
	duration := time.Since(state.StartTime)
	lines = append(lines, fmt.Sprintf("  elapsed: %s", duration.Round(time.Second)))

	// Usage stats
	lines = append(lines, "", "Usage:")
	lines = append(lines, fmt.Sprintf("  turns: %d", state.TurnCount))
	lines = append(lines, fmt.Sprintf("  tool_calls: %d", state.ToolCallCount))
	lines = append(lines, fmt.Sprintf("  api_duration: %s", state.TotalAPIDuration.Round(time.Millisecond)))
	lines = append(lines, fmt.Sprintf("  tool_duration: %s", state.TotalToolDuration.Round(time.Millisecond)))
	if state.TotalCostUSD > 0 {
		lines = append(lines, fmt.Sprintf("  cost: $%.6f", state.TotalCostUSD))
	}

	// Model usage breakdown
	if len(state.ModelUsage) > 0 {
		lines = append(lines, "", "Models used:")
		for model, count := range state.ModelUsage {
			lines = append(lines, fmt.Sprintf("  %s: %d calls", model, count))
		}
	}

	// Effort level
	effort := getEffortFromState(runtime)
	if effort != "" {
		lines = append(lines, "", "Settings:")
		lines = append(lines, fmt.Sprintf("  effort: %s", effort))
	}

	// Editor mode
	if state.EditorMode != "" {
		lines = append(lines, fmt.Sprintf("  editor_mode: %s", state.EditorMode))
	}

	// Last error if present
	if state.LastError != "" {
		lines = append(lines, "", "Last error:")
		lines = append(lines, fmt.Sprintf("  %s", state.LastError))
	}

	return strings.Join(lines, "\n")
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}