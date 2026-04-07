package command

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
)

// Kind represents the command type
type Kind string

const (
	KindLocal    Kind = "local"     // Simple text output
	KindLocalJSX Kind = "local-jsx" // TUI component (Bubble Tea)
	KindPrompt   Kind = "prompt"    // Expands to prompt sent to model
)

// ResultType represents the type of command result
type ResultType string

const (
	ResultTypeText    ResultType = "text"
	ResultTypeCompact ResultType = "compact"
	ResultTypeSkip    ResultType = "skip"
)

// CommandResult represents the result of a command execution
type CommandResult struct {
	Type    ResultType `json:"type"`
	Value   string     `json:"value,omitempty"`
	Display string     `json:"display,omitempty"` // "user", "system", "skip"
}

// CommandBase contains the common fields for all commands
type CommandBase struct {
	Name                   string   `json:"name"`
	DisplayName            string   `json:"display_name,omitempty"`
	Description            string   `json:"description"`
	Aliases                []string `json:"aliases,omitempty"`
	ArgumentHint           string   `json:"argument_hint,omitempty"`
	WhenToUse              string   `json:"when_to_use,omitempty"`
	Version                string   `json:"version,omitempty"`
	Hidden                 bool     `json:"hidden,omitempty"`
	UserInvocable          bool     `json:"user_invocable"`
	DisableModelInvocation bool     `json:"disable_model_invocation,omitempty"`
	Source                 string   `json:"source"` // "builtin", "plugin", "skills", "mcp", "bundled"
	LoadedFrom             string   `json:"loaded_from"`
}

// LocalCommand is a simple command that returns text
type LocalCommand struct {
	CommandBase
	SupportsNonInteractive bool `json:"supports_non_interactive"`
	Handler                func(ctx context.Context, rt Runtime, args []string) (CommandResult, error)
}

// LocalJSXCommand is a TUI command that returns a Bubble Tea model
type LocalJSXCommand struct {
	CommandBase
	// Load returns a Bubble Tea model for interactive rendering
	Load func(ctx context.Context, rt Runtime, args []string) (tea.Model, error)
}

// PromptCommand expands to a prompt sent to the model
type PromptCommand struct {
	CommandBase
	ProgressMessage string   `json:"progress_message"`
	ContentLength   int      `json:"content_length"`
	ArgNames        []string `json:"arg_names,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	Model           string   `json:"model,omitempty"`
	// GetPromptForCommand returns the prompt content
	GetPromptForCommand func(ctx context.Context, rt Runtime, args string) (string, error)
}

// Command is the interface for all command types
type Command interface {
	GetBase() CommandBase
	GetKind() Kind
}

// GetBase returns the command base
func (c LocalCommand) GetBase() CommandBase    { return c.CommandBase }
func (c LocalJSXCommand) GetBase() CommandBase { return c.CommandBase }
func (c PromptCommand) GetBase() CommandBase   { return c.CommandBase }

// GetKind returns the command kind
func (c LocalCommand) GetKind() Kind    { return KindLocal }
func (c LocalJSXCommand) GetKind() Kind { return KindLocalJSX }
func (c PromptCommand) GetKind() Kind   { return KindPrompt }

// Execute executes a local command
func (c LocalCommand) Execute(ctx context.Context, rt Runtime, args []string) (CommandResult, error) {
	if c.Handler == nil {
		return CommandResult{Type: ResultTypeText, Value: "command not implemented"}, nil
	}
	return c.Handler(ctx, rt, args)
}

// LoadModel loads a TUI command model
func (c LocalJSXCommand) LoadModel(ctx context.Context, rt Runtime, args []string) (tea.Model, error) {
	if c.Load == nil {
		return nil, nil
	}
	return c.Load(ctx, rt, args)
}

// GetPrompt returns the prompt for a prompt command
func (c PromptCommand) GetPrompt(ctx context.Context, rt Runtime, args string) (string, error) {
	if c.GetPromptForCommand == nil {
		return "", nil
	}
	return c.GetPromptForCommand(ctx, rt, args)
}

// LegacyHandler is the old-style handler function for backward compatibility
type LegacyHandler func(ctx context.Context, rt Runtime, args []string) (string, error)

// Handler is an alias for LegacyHandler for backward compatibility
type Handler = LegacyHandler

// CreateLocalCommand creates a LocalCommand from a legacy handler for backward compatibility
func CreateLocalCommand(name, description string, handler LegacyHandler) LocalCommand {
	return LocalCommand{
		CommandBase: CommandBase{
			Name:        name,
			Description: description,
			Source:      "builtin",
		},
		Handler: func(ctx context.Context, rt Runtime, args []string) (CommandResult, error) {
			result, err := handler(ctx, rt, args)
			if err != nil {
				return CommandResult{}, err
			}
			return CommandResult{Type: ResultTypeText, Value: result}, nil
		},
	}
}

// CreateLocalCommandWithHint creates a LocalCommand with argument hint
func CreateLocalCommandWithHint(name, description, hint string, handler LegacyHandler) LocalCommand {
	return LocalCommand{
		CommandBase: CommandBase{
			Name:         name,
			Description:  description,
			ArgumentHint: hint,
			Source:       "builtin",
		},
		Handler: func(ctx context.Context, rt Runtime, args []string) (CommandResult, error) {
			result, err := handler(ctx, rt, args)
			if err != nil {
				return CommandResult{}, err
			}
			return CommandResult{Type: ResultTypeText, Value: result}, nil
		},
	}
}