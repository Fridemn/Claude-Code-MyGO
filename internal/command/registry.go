package command

import (
	"context"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// Registry manages all commands
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
	order    []string // Maintains registration order
}

// EmptyRegistry creates a new empty command registry
func EmptyRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
		order:    make([]string, 0),
	}
}

// Register adds a command to the registry
func (r *Registry) Register(cmd Command) {
	r.mu.Lock()
	defer r.mu.Unlock()

	base := cmd.GetBase()
	r.commands[base.Name] = cmd
	r.order = append(r.order, base.Name)

	// Register aliases
	for _, alias := range base.Aliases {
		r.commands[alias] = cmd
	}
}

// Lookup finds a command by name or alias
func (r *Registry) Lookup(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	name = strings.TrimPrefix(name, "/")
	cmd, ok := r.commands[name]
	return cmd, ok
}

// List returns all registered commands (without duplicates)
func (r *Registry) List() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	result := make([]Command, 0, len(r.order))

	for _, name := range r.order {
		if seen[name] {
			continue
		}
		cmd, ok := r.commands[name]
		if !ok {
			continue
		}
		seen[name] = true
		result = append(result, cmd)
	}

	return result
}

// ListVisible returns commands that are not hidden
func (r *Registry) ListVisible() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	result := make([]Command, 0, len(r.order))

	for _, name := range r.order {
		if seen[name] {
			continue
		}
		cmd, ok := r.commands[name]
		if !ok {
			continue
		}
		if cmd.GetBase().Hidden {
			continue
		}
		seen[name] = true
		result = append(result, cmd)
	}

	return result
}

// Execute runs a local command
func (r *Registry) Execute(ctx context.Context, input string, rt Runtime) (CommandResult, bool, error) {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return CommandResult{}, false, nil
	}

	name := strings.TrimPrefix(parts[0], "/")
	cmd, ok := r.commands[name]
	if !ok {
		return CommandResult{}, false, nil
	}

	// Check command type
	switch c := cmd.(type) {
	case LegacyCommand:
		// LegacyCommand - execute based on its Type field
		switch c.Type {
		case KindLocal, KindLocalJSX:
			if c.Handler == nil {
				return CommandResult{Type: ResultTypeText, Value: "command not implemented"}, true, nil
			}
			result, err := c.Handler(ctx, rt, parts[1:])
			if err != nil {
				return CommandResult{}, true, err
			}
			return CommandResult{Type: ResultTypeText, Value: result}, true, nil
		case KindPrompt:
			if c.Handler == nil {
				return CommandResult{Type: ResultTypeText, Value: "command not implemented"}, true, nil
			}
			result, err := c.Handler(ctx, rt, parts[1:])
			if err != nil {
				return CommandResult{}, true, err
			}
			return CommandResult{Type: ResultTypeText, Value: result}, true, nil
		}
		return CommandResult{}, true, nil
	case LocalCommand:
		result, err := c.Execute(ctx, rt, parts[1:])
		return result, true, err
	case LocalJSXCommand:
		// JSX commands should be handled via LoadModel
		return CommandResult{
			Type:  ResultTypeText,
			Value: "command requires TUI rendering",
		}, true, nil
	case PromptCommand:
		// Prompt commands should be handled via GetPrompt
		return CommandResult{
			Type:  ResultTypeText,
			Value: "command requires prompt expansion",
		}, true, nil
	}

	return CommandResult{}, false, nil
}

// LoadModel loads a TUI command model for Bubble Tea rendering
func (r *Registry) LoadModel(ctx context.Context, input string, rt Runtime) (tea.Model, Command, bool, error) {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return nil, nil, false, nil
	}

	name := strings.TrimPrefix(parts[0], "/")
	cmd, ok := r.commands[name]
	if !ok {
		return nil, nil, false, nil
	}

	jsxCmd, ok := cmd.(LocalJSXCommand)
	if !ok {
		// Check if it's a LegacyCommand with JSX type
		legacyCmd, isLegacy := cmd.(LegacyCommand)
		if !isLegacy || legacyCmd.Type != KindLocalJSX {
			return nil, nil, false, nil
		}
		// LegacyCommand JSX doesn't have a LoadModel implementation yet
		return nil, nil, false, nil
	}

	model, err := jsxCmd.LoadModel(ctx, rt, parts[1:])
	if err != nil {
		return nil, nil, true, err
	}

	return model, cmd, true, nil
}

// GetPrompt returns the prompt for a prompt command
func (r *Registry) GetPrompt(ctx context.Context, input string, rt Runtime) (string, Command, bool, error) {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return "", nil, false, nil
	}

	name := strings.TrimPrefix(parts[0], "/")
	cmd, ok := r.commands[name]
	if !ok {
		return "", nil, false, nil
	}

	promptCmd, ok := cmd.(PromptCommand)
	if !ok {
		// Check if it's a LegacyCommand with Prompt type
		legacyCmd, isLegacy := cmd.(LegacyCommand)
		if !isLegacy || legacyCmd.Type != KindPrompt || legacyCmd.Handler == nil {
			return "", nil, false, nil
		}
		// Use LegacyCommand's handler
		// For LegacyCommand with KindPrompt, we need to parse the args
		promptArgs := parts[1:]
		prompt, err := legacyCmd.Handler(ctx, rt, promptArgs)
		if err != nil {
			return "", nil, true, err
		}
		return prompt, cmd, true, nil
	}

	args := ""
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}

	prompt, err := promptCmd.GetPrompt(ctx, rt, args)
	if err != nil {
		return "", nil, true, err
	}

	return prompt, cmd, true, nil
}

// Names returns all command names
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return append([]string(nil), r.order...)
}

// BuiltInCommandNames returns a set of built-in command names
func (r *Registry) BuiltInCommandNames() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make(map[string]bool)
	for _, name := range r.order {
		cmd, ok := r.commands[name]
		if ok && cmd.GetBase().Source == "builtin" {
			names[name] = true
		}
	}
	return names
}

// LegacyCommand is the old command struct format for backward compatibility
type LegacyCommand struct {
	Type                   Kind
	Name                   string
	DisplayName            string
	Description            string
	Aliases                []string
	ArgumentHint           string
	WhenToUse              string
	AllowedTools           []string
	Model                  string
	DisableModelInvocation bool
	BaseDir                string
	SupportsNonInteractive bool
	Hidden                 bool
	Handler                func(ctx context.Context, rt Runtime, args []string) (string, error)
}

// GetBase returns the CommandBase for LegacyCommand
func (c LegacyCommand) GetBase() CommandBase {
	return CommandBase{
		Name:                   c.Name,
		DisplayName:            c.DisplayName,
		Description:            c.Description,
		Aliases:                c.Aliases,
		ArgumentHint:           c.ArgumentHint,
		WhenToUse:              c.WhenToUse,
		Hidden:                 c.Hidden,
		Source:                 "legacy",
		DisableModelInvocation: c.DisableModelInvocation,
	}
}

// GetKind returns the Kind for LegacyCommand
func (c LegacyCommand) GetKind() Kind {
	return c.Type
}

// RegisterLegacy registers a command using the old struct format for backward compatibility
func (r *Registry) RegisterLegacy(cmd LegacyCommand) {
	base := CommandBase{
		Name:                   cmd.Name,
		DisplayName:            cmd.DisplayName,
		Description:            cmd.Description,
		Aliases:                cmd.Aliases,
		ArgumentHint:           cmd.ArgumentHint,
		WhenToUse:              cmd.WhenToUse,
		Hidden:                 cmd.Hidden,
		Source:                 "builtin",
		DisableModelInvocation: cmd.DisableModelInvocation,
	}

	switch cmd.Type {
	case KindLocal, KindLocalJSX:
		// Treat both as LocalCommand for now (LocalJSX needs TUI implementation)
		localCmd := LocalCommand{
			CommandBase:            base,
			SupportsNonInteractive: cmd.SupportsNonInteractive,
		}
		if cmd.Handler != nil {
			localCmd.Handler = func(ctx context.Context, rt Runtime, args []string) (CommandResult, error) {
				result, err := cmd.Handler(ctx, rt, args)
				if err != nil {
					return CommandResult{}, err
				}
				return CommandResult{Type: ResultTypeText, Value: result}, nil
			}
		}
		r.Register(localCmd)
	default:
		// Default to LocalCommand
		localCmd := LocalCommand{
			CommandBase: base,
		}
		if cmd.Handler != nil {
			localCmd.Handler = func(ctx context.Context, rt Runtime, args []string) (CommandResult, error) {
				result, err := cmd.Handler(ctx, rt, args)
				if err != nil {
					return CommandResult{}, err
				}
				return CommandResult{Type: ResultTypeText, Value: result}, nil
			}
		}
		r.Register(localCmd)
	}
}