package memory

import (
	"context"
	"strings"

	"claude-go/internal/command"
	"claude-go/internal/types"
)

func registerMemory(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "memory",
		Description: "show a compact session memory snapshot",
		Load:        loadMemoryModel,
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return renderMemorySnapshot(runtime), nil
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
