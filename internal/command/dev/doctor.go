package dev

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
	"claude-go/internal/tool"
)

func registerDoctor(r *command.Registry) {
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:        "doctor",
			Description: "run a local health check for the current CLI session",
			Source:      "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, _ []string) (command.CommandResult, error) {
			lines := []string{
				fmt.Sprintf("app=%s", runtime.Config.AppName),
				fmt.Sprintf("model=%s", command.EmptyDash(runtime.Config.Model)),
				fmt.Sprintf("base_url=%s", command.EmptyDash(runtime.Config.BaseURL)),
				fmt.Sprintf("session_dir=%s", command.EmptyDash(runtime.Config.SessionDir)),
			}
			if runtime.State != nil {
				state := runtime.State.Snapshot()
				lines = append(lines,
					fmt.Sprintf("cwd=%s", command.EmptyDash(state.CWD)),
					fmt.Sprintf("session=%s", command.EmptyDash(state.SessionID)),
				)
			}
			if runtime.Tools != nil {
				lines = append(lines, fmt.Sprintf("tool_count=%d", len(runtime.Tools.List())))
			}
			if runtime.Agents != nil {
				lines = append(lines, fmt.Sprintf("agent_count=%d", len(runtime.Agents.Registry().List())))
			}

			gitCheck, err := command.CallNamedTool(ctx, runtime.Tools, "exec_command", tool.Input{
				"command": "git rev-parse --is-inside-work-tree",
			})
			if err != nil {
				lines = append(lines, "git=unavailable")
			} else {
				lines = append(lines, "git="+strings.TrimSpace(command.StringifyToolContent(gitCheck.Content)))
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: strings.Join(lines, "\n")}, nil
		},
	})
}