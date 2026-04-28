package files

import (
	"context"
	"fmt"

	"claude-go/internal/command"
	"claude-go/internal/tool"
)

func registerFiles(r *command.Registry) {
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "files",
			Description:  "list files under a directory",
			ArgumentHint: "[path]",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Tools == nil {
				return command.CommandResult{}, fmt.Errorf("tool registry is not configured")
			}
			path := "."
			if len(args) > 0 {
				path = args[0]
			}
			result, err := command.CallNamedTool(ctx, runtime.Tools, "list_files", tool.Input{
				"path":        path,
				"max_results": 200,
			})
			if err != nil {
				return command.CommandResult{}, err
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: command.StringifyToolContent(result.Content)}, nil
		},
	})
}