package stats

import (
	"context"
	"fmt"

	"claude-go/internal/command"
)

func registerTools(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "tools",
		Description: "list available tools",
		Load:        loadToolsModel,
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return joinLines(renderToolsLines(runtime)), nil
		},
	})
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	if len(lines) == 1 {
		return lines[0]
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n" + line
	}
	return out
}

func registerTool(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "tool",
		Description:  "show one tool definition",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if runtime.Tools == nil {
				return "", fmt.Errorf("tool registry is not configured")
			}
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("tool", "<name>"))
			}
			definition, ok := runtime.Tools.Get(args[0])
			if !ok {
				return "", fmt.Errorf("tool not found: %s", args[0])
			}
			mode := "write"
			if definition.IsReadOnly(nil) {
				mode = "read"
			}
			return fmt.Sprintf("name=%s\nmode=%s\ndescription=%s", definition.Name(), mode, definition.Description()), nil
		},
	})
}
