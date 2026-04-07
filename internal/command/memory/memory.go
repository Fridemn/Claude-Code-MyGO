package memory

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/command"
	"claude-code-go/internal/types"
)

func registerMemory(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "memory",
		Description: "show a compact session memory snapshot",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.Engine == nil {
				return "", fmt.Errorf("engine is not configured")
			}
			messages := runtime.Engine.Messages()
			if len(messages) == 0 {
				return "no session memory", nil
			}

			lines := []string{
				fmt.Sprintf("session=%s", runtime.Engine.SessionID()),
				fmt.Sprintf("messages=%d", len(messages)),
				"",
				"recent_memory:",
			}
			start := len(messages) - 8
			if start < 0 {
				start = 0
			}
			for _, msg := range messages[start:] {
				lines = append(lines, fmt.Sprintf("- %s: %s", msg.Role, summarizeMessage(msg)))
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}

func summarizeMessage(msg types.Message) string {
	text := strings.TrimSpace(msg.Content)
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) <= 120 {
		return text
	}
	return text[:117] + "..."
}