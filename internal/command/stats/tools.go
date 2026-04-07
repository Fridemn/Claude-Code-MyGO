package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"claude-code-go/internal/command"
)

func registerTools(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "tools",
		Description: "list available tools",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.Tools == nil {
				return "", fmt.Errorf("tool registry is not configured")
			}
			definitions := runtime.Tools.List()
			lines := make([]string, 0, len(definitions))
			for _, definition := range definitions {
				mode := "write"
				if definition.IsReadOnly(nil) {
					mode = "read"
				}
				lines = append(lines, fmt.Sprintf("%s  [%s]  %s", definition.Name(), mode, definition.Description()))
			}
			sort.Strings(lines)
			return strings.Join(lines, "\n"), nil
		},
	})
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

func registerModel(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "model",
		Description: "show current model settings",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return fmt.Sprintf("model=%s\nbase_url=%s", runtime.Config.Model, runtime.Config.BaseURL), nil
		},
	})
}