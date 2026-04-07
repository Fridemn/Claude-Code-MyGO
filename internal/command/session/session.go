package session

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/command"
	"claude-code-go/internal/task"
	"claude-code-go/internal/types"
)

func registerSessionCommands(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "history",
		Description: "dump current session history",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return formatTaskTranscript(&task.AgentTask{Messages: runtime.Engine.Messages()}), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "compact",
		Description: "compact current conversation state",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if runtime.CompactSession == nil {
				return "", fmt.Errorf("session compaction is not configured")
			}
			maxMessages := 12
			if len(args) > 0 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("compact", ""))
			}
			before, after := runtime.CompactSession(maxMessages)
			lines := []string{
				"session compacted",
				fmt.Sprintf("messages_before=%d", before),
				fmt.Sprintf("messages_after=%d", after),
				fmt.Sprintf("messages_removed=%d", before-after),
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "prompt",
		Description: "show current system prompt",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			msgs := runtime.Engine.Messages()
			if len(msgs) == 0 || msgs[0].Role != types.RoleSystem {
				return "", nil
			}
			return strings.TrimSpace(msgs[0].Content), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "clear",
		Description: "redraw the current session screen",
		Aliases:     []string{"reset", "new"},
		Hidden:      true,
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "screen cleared", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "exit",
		Description: "quit the session",
		Hidden:      true,
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "config",
		Description: "show active config summary",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.Engine == nil {
				return "", fmt.Errorf("engine is not configured")
			}
			msgs := runtime.Engine.Messages()
			lines := []string{
				fmt.Sprintf("app=%s", runtime.Config.AppName),
				fmt.Sprintf("session=%s", runtime.Engine.SessionID()),
				fmt.Sprintf("model=%s", runtime.Config.Model),
				fmt.Sprintf("base_url=%s", runtime.Config.BaseURL),
				fmt.Sprintf("max_turns=%d", runtime.Config.MaxTurns),
				fmt.Sprintf("session_dir=%s", runtime.Config.SessionDir),
				fmt.Sprintf("messages=%d", len(msgs)),
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}

func formatTaskTranscript(taskState *task.AgentTask) string {
	if len(taskState.Messages) == 0 {
		return "no transcript recorded"
	}
	lines := make([]string, 0, len(taskState.Messages)*3)
	for i, msg := range taskState.Messages {
		lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, strings.ToUpper(msg.Role)))
		lines = append(lines, msg.Content)
		if i < len(taskState.Messages)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}