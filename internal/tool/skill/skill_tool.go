package skill

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// SkillTool implements the Skill tool for invoking slash command skills
type SkillTool struct {
	commands []Command
}

// Command represents a skill/command that can be invoked
type Command interface {
	GetName() string
	GetDescription() string
	IsPrompt() bool
}

// BaseCommand provides a basic command implementation
type BaseCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"` // "prompt" or other
}

// GetName returns the command name
func (c BaseCommand) GetName() string { return c.Name }

// GetDescription returns the command description
func (c BaseCommand) GetDescription() string { return c.Description }

// IsPrompt returns true if this is a prompt-based command
func (c BaseCommand) IsPrompt() bool { return c.Type == "prompt" }

// CreateSkillTool creates a new Skill tool
func CreateSkillTool(commands []Command) *SkillTool {
	if commands == nil {
		commands = GetDefaultCommands()
	}
	return &SkillTool{commands: commands}
}

// Name returns the tool name
func (SkillTool) Name() string {
	return SkillToolName
}

// Description returns the tool description
func (SkillTool) Description() string {
	return SkillDescription
}

// IsReadOnly returns true for simple skills, false for skills that modify state
func (SkillTool) IsReadOnly(in tool.Input) bool {
	// By default, skills are read-only
	// In a full implementation, we would check the skill type
	return true
}

// ParametersSchema returns the JSON schema for the tool parameters
func (SkillTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"skill": tool.SchemaString("The skill name. E.g., \"commit\", \"review-pr\", or \"pdf\""),
		"args":  tool.SchemaString("Optional arguments for the skill"),
	}, "skill")
}

// SkillInput represents the parsed input
type SkillInput struct {
	Skill string
	Args  string
}

// SkillOutput represents the output
type SkillOutput struct {
	Success      bool     `json:"success"`
	CommandName  string   `json:"command_name"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Model        string   `json:"model,omitempty"`
	Status       string   `json:"status,omitempty"`
	Result       string   `json:"result,omitempty"`
	AgentID      string   `json:"agent_id,omitempty"`
}

// parseSkillInput extracts the input parameters
func parseSkillInput(in tool.Input) SkillInput {
	return SkillInput{
		Skill: tool.GetString(in, "skill"),
		Args:  tool.GetString(in, "args"),
	}
}

// Call executes the Skill tool
func (t *SkillTool) Call(ctx context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	input := parseSkillInput(in)

	// Validate required fields
	trimmed := strings.TrimSpace(input.Skill)
	if trimmed == "" {
		return tool.Result{}, fmt.Errorf("skill name is required")
	}

	// Remove leading slash if present
	commandName := strings.TrimPrefix(trimmed, "/")

	// Find the command
	var foundCommand Command
	for _, cmd := range t.commands {
		if cmd.GetName() == commandName {
			foundCommand = cmd
			break
		}
	}

	if foundCommand == nil {
		available := make([]string, len(t.commands))
		for i, c := range t.commands {
			available[i] = c.GetName()
		}
		return tool.Result{}, fmt.Errorf("unknown skill: %s. Available skills: %s", commandName, strings.Join(available, ", "))
	}

	// Check if this is a prompt-based skill
	if !foundCommand.IsPrompt() {
		return tool.Result{}, fmt.Errorf("skill %s is not a prompt-based skill", commandName)
	}

	// Return success - in a full implementation, we would expand the skill
	return tool.Result{
		Content: SkillOutput{
			Success:     true,
			CommandName: commandName,
			Status:      "inline",
		},
	}, nil
}

// SetCommands updates the available commands
func (t *SkillTool) SetCommands(commands []Command) {
	t.commands = commands
}

// GetCommands returns the current commands
func (t *SkillTool) GetCommands() []Command {
	return t.commands
}

// GetDefaultCommands returns the default built-in commands
func GetDefaultCommands() []Command {
	return []Command{
		BaseCommand{Name: "commit", Description: "Create a git commit", Type: "prompt"},
		BaseCommand{Name: "review-pr", Description: "Review a pull request", Type: "prompt"},
		BaseCommand{Name: "pdf", Description: "Work with PDF files", Type: "prompt"},
		BaseCommand{Name: "simplify", Description: "Review and simplify code", Type: "prompt"},
		BaseCommand{Name: "update-config", Description: "Update configuration settings", Type: "prompt"},
	}
}

// FindCommand finds a command by name
func FindCommand(name string, commands []Command) Command {
	for _, cmd := range commands {
		if cmd.GetName() == name {
			return cmd
		}
	}
	return nil
}

// BuiltInCommandNames returns the set of built-in command names
func BuiltInCommandNames() map[string]bool {
	names := make(map[string]bool)
	for _, cmd := range GetDefaultCommands() {
		names[cmd.GetName()] = true
	}
	return names
}

// RegisterSkillTools registers the Skill tool
func RegisterSkillTools(r *tool.Registry) {
	r.Register(CreateSkillTool(nil))
}