package files

import (
	"context"
	"fmt"
	"strconv"

	"claude-code-go/internal/command"
	"claude-code-go/internal/tool"
)

func registerRead(r *command.Registry) {
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "read",
			Description:  "read a file with optional offset and limit",
			ArgumentHint: "<path> [offset] [limit]",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Tools == nil {
				return command.CommandResult{}, fmt.Errorf("tool registry is not configured")
			}
			if len(args) < 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("read", "<path> [offset] [limit]"))
			}
			input := tool.Input{"file_path": args[0]}
			if len(args) > 1 {
				offset, err := strconv.Atoi(args[1])
				if err != nil {
					return command.CommandResult{}, fmt.Errorf("invalid offset: %s", args[1])
				}
				input["offset"] = offset
			}
			if len(args) > 2 {
				limit, err := strconv.Atoi(args[2])
				if err != nil {
					return command.CommandResult{}, fmt.Errorf("invalid limit: %s", args[2])
				}
				input["limit"] = limit
			}
			result, err := command.CallNamedTool(ctx, runtime.Tools, "Read", input)
			if err != nil {
				return command.CommandResult{}, err
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: command.StringifyToolContent(result.Content)}, nil
		},
	})
}