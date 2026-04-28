package help

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"claude-go/internal/command"
)

// Register registers the help command with the registry.
func Register(r *command.Registry) {
	r.Register(Command())
}

// Command returns the help command definition
func Command() command.LocalJSXCommand {
	return command.LocalJSXCommand{
		CommandBase: command.CommandBase{
			Name:          "help",
			DisplayName:   "Help",
			Description:   "Show help and available commands",
			Aliases:       []string{"?"},
			Source:        "builtin",
			UserInvocable: true,
		},
		Load: LoadModel,
	}
}

// LoadModel creates a Bubble Tea model for the help TUI
func LoadModel(ctx context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	commands := rt.Commands()

	// Filter builtin commands (non-hidden)
	builtinCommands := make([]command.Command, 0)
	customCommands := make([]command.Command, 0)

	for _, cmd := range commands {
		if cmd.GetBase().Hidden {
			continue
		}
		// Check if it's a builtin by source
		if cmd.GetBase().Source == "builtin" {
			builtinCommands = append(builtinCommands, cmd)
		} else {
			customCommands = append(customCommands, cmd)
		}
	}

	return CreateModel(builtinCommands, customCommands, rt.OnExit), nil
}
