package agent

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/command"
)

func registerAgentCommands(r *command.Registry) {
	// agents command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:        "agents",
			Description: "list available agent types",
			Source:      "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			agents := runtime.Agents.Registry().List()
			lines := make([]string, 0, len(agents))
			for _, agent := range agents {
				lines = append(lines, fmt.Sprintf("%s  [%s]  %s", agent.AgentType, agent.Source, agent.WhenToUse))
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: strings.Join(lines, "\n")}, nil
		},
	})

	// agent command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "agent",
			Description:  "run an agent",
			ArgumentHint: "[--bg] <type> <prompt>",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) < 2 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("agent", "[--bg] <type> <prompt>"))
			}
			background := false
			if args[0] == "--bg" {
				background = true
				args = args[1:]
			}
			if len(args) < 2 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("agent", "[--bg] <type> <prompt>"))
			}
			result, err := runtime.Agents.Spawn(ctx, agent.SpawnInput{
				Description:  "slash command agent run",
				SubagentType: args[0],
				Prompt:       strings.Join(args[1:], " "),
				Background:   background,
			})
			if err != nil {
				return command.CommandResult{}, err
			}
			taskState := result.Task
			if background {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("background agent launched\nid=%s\ntype=%s\nstatus=%s\ndescription=%s", taskState.ID, taskState.AgentType, taskState.Status, taskState.Description)}, nil
			}
			if taskState.Status == "failed" {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent failed\nid=%s\ntype=%s\nerror=%s\nsummary=%s", taskState.ID, taskState.AgentType, taskState.Error, command.EmptyDash(taskState.Summary))}, nil
			}
			if taskState.Summary != "" {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent completed\nid=%s\ntype=%s\n\n%s", taskState.ID, taskState.AgentType, taskState.Summary)}, nil
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent completed\nid=%s\ntype=%s\n\n%s", taskState.ID, taskState.AgentType, taskState.Output)}, nil
		},
	})

	// tasks command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:        "tasks",
			Description: "list current agent tasks",
			Source:      "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: formatTaskList(runtime.Agents.Tasks().List())}, nil
		},
	})

	// task command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "task",
			Description:  "show one agent task",
			ArgumentHint: "<id>",
			Source:       "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) != 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("task", "<id>"))
			}
			taskState, ok := runtime.Agents.Tasks().Get(args[0])
			if !ok {
				return command.CommandResult{}, fmt.Errorf("task not found: %s", args[0])
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: formatTaskDetail(taskState)}, nil
		},
	})

	// tasklog command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "tasklog",
			Description:  "show one agent transcript",
			ArgumentHint: "<id>",
			Source:       "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) != 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("tasklog", "<id>"))
			}
			taskState, ok := runtime.Agents.Tasks().Get(args[0])
			if !ok {
				return command.CommandResult{}, fmt.Errorf("task not found: %s", args[0])
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: formatTaskTranscript(taskState)}, nil
		},
	})

	// send command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "send",
			Description:  "continue one agent task",
			ArgumentHint: "[--bg] <id> <prompt>",
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) < 2 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("send", "[--bg] <id> <prompt>"))
			}
			background := false
			if args[0] == "--bg" {
				background = true
				args = args[1:]
			}
			if len(args) < 2 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("send", "[--bg] <id> <prompt>"))
			}
			result, err := runtime.Agents.Continue(ctx, args[0], strings.Join(args[1:], " "), background)
			if err != nil {
				return command.CommandResult{}, err
			}
			if background {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("background continuation launched\nid=%s\ntype=%s\nstatus=%s\ndescription=%s", result.Task.ID, result.Task.AgentType, result.Task.Status, result.Task.Description)}, nil
			}
			if result.Task.Summary != "" {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent continued\nid=%s\ntype=%s\nstatus=%s\n\n%s", result.Task.ID, result.Task.AgentType, result.Task.Status, result.Task.Summary)}, nil
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent continued\nid=%s\ntype=%s\nstatus=%s\n\n%s", result.Task.ID, result.Task.AgentType, result.Task.Status, result.Task.Output)}, nil
		},
	})

	// resume command (alias for send)
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "resume",
			Description:  "alias of /send for continuing one agent task",
			ArgumentHint: "[--bg] <id> <prompt>",
			Aliases:      []string{"continue"},
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			// Find send command and execute it
			sendCmd, ok := r.Lookup("send")
			if !ok {
				return command.CommandResult{}, fmt.Errorf("send command is not registered")
			}
			if localCmd, ok := sendCmd.(command.LocalCommand); ok {
				return localCmd.Handler(ctx, runtime, args)
			}
			return command.CommandResult{}, fmt.Errorf("send command is not a local command")
		},
	})

	// wait command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "wait",
			Description:  "wait for one agent task to finish",
			ArgumentHint: "<id>",
			Source:       "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) != 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("wait", "<id>"))
			}
			taskState, err := runtime.Agents.WaitForTask(args[0])
			if err != nil {
				return command.CommandResult{}, err
			}
			if taskState.Status == "failed" {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent failed\nid=%s\ntype=%s\nerror=%s\nsummary=%s", taskState.ID, taskState.AgentType, taskState.Error, command.EmptyDash(taskState.Summary))}, nil
			}
			if taskState.Summary != "" {
				return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent finished\nid=%s\ntype=%s\nstatus=%s\n\n%s", taskState.ID, taskState.AgentType, taskState.Status, taskState.Summary)}, nil
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent finished\nid=%s\ntype=%s\nstatus=%s\n\n%s", taskState.ID, taskState.AgentType, taskState.Status, taskState.Output)}, nil
		},
	})

	// stop command
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:         "stop",
			Description:  "stop one agent task",
			ArgumentHint: "<id>",
			Source:       "builtin",
		},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.Agents == nil {
				return command.CommandResult{}, fmt.Errorf("agent manager is not configured")
			}
			if len(args) != 1 {
				return command.CommandResult{}, fmt.Errorf("%s", command.FormatCommandUsage("stop", "<id>"))
			}
			if err := runtime.Agents.Stop(args[0]); err != nil {
				return command.CommandResult{}, err
			}
			return command.CommandResult{Type: command.ResultTypeText, Value: fmt.Sprintf("agent stop requested\nid=%s\nstatus=killed", args[0])}, nil
		},
	})
}