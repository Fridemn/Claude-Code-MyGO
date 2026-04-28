package files

import (
	"context"
	"fmt"

	"claude-go/internal/command"
	"claude-go/internal/tool"
)

func registerGrep(r *command.Registry) {
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "grep",
			Description:  "search file contents under a directory",
			ArgumentHint: "<pattern> [path]",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Tools == nil {
				return command.CommandResult{}, fmt.Errorf("tool registry is not configured")
			}
			if len(args) < 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("grep", "<pattern> [path]"))
			}
			path := "."
			if len(args) > 1 {
				path = args[1]
			}
			result, err := command.CallNamedTool(ctx, runtime.Tools, "Grep", tool.Input{
				"pattern":    args[0],
				"path":       path,
				"head_limit": 200,
			})
			if err != nil {
				return command.CommandResult{}, err
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: command.StringifyToolContent(result.Content)}, nil
		},
	})
}