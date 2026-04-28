package dev

import (
	"context"
	"fmt"

	"claude-go/internal/command"
	"claude-go/internal/tool"
)

func registerDiff(r *command.Registry) {
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "diff",
			Description:  "show git diff summary or diff for a path",
			ArgumentHint: "[path]",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Tools == nil {
				return command.CommandResult{}, fmt.Errorf("tool registry is not configured")
			}
			target := "."
			if len(args) > 0 {
				target = args[0]
			}
			cmd := fmt.Sprintf("git diff --stat -- %s && printf '\\n' && git diff -- %s", command.ShellQuote(target), command.ShellQuote(target))
			result, err := command.CallNamedTool(ctx, runtime.Tools, "exec_command", tool.Input{
				"command": cmd,
			})
			if err != nil {
				return command.CommandResult{}, err
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: command.StringifyToolContent(result.Content)}, nil
		},
	})
}