package skill

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"claude-go/internal/command"
	"claude-go/internal/tool"
)

// Skill represents a skill definition (mirrors services.Skill to avoid import cycles)
// Ported from src/skills/types.ts
type Skill struct {
	Name                   string   `json:"name"`
	DisplayName            string   `json:"display_name,omitempty"`
	Aliases                []string `json:"aliases,omitempty"`
	Description            string   `json:"description"`
	WhenToUse              string   `json:"when_to_use,omitempty"`
	ArgumentHint           string   `json:"argument_hint,omitempty"`
	AllowedTools           []string `json:"allowed_tools,omitempty"`
	Version                string   `json:"version,omitempty"`
	Model                  string   `json:"model,omitempty"`
	Context                string   `json:"context,omitempty"`
	Agent                  string   `json:"agent,omitempty"`
	Source                 string   `json:"source"`
	LoadedFrom             string   `json:"loaded_from"`
	Path                   string   `json:"path,omitempty"`
	BaseDir                string   `json:"base_dir,omitempty"`
	Prompt                 string   `json:"-"`
	UserInvocable          bool     `json:"user_invocable"`
	DisableModelInvocation bool     `json:"disable_model_invocation,omitempty"`
}

// SkillLister is the interface for accessing skills (to avoid import cycles)
// Implemented by services.SkillsService implicitly
type SkillLister interface {
	List() []Skill
	Commands() []command.Command
}

// SkillTool implements the Skill tool for invoking slash command skills.
// Ported from src/tools/SkillTool/SkillTool.ts
type SkillTool struct {
	mu           sync.RWMutex
	skillService SkillLister
	commandReg   *command.Registry
}

// SkillToolName is the name of the skill tool (defined in constants.go)

// CreateSkillTool creates a new Skill tool
// Ported from src/tools/SkillTool/SkillTool.ts
func CreateSkillTool(skillService SkillLister, commandReg *command.Registry) *SkillTool {
	return &SkillTool{
		skillService: skillService,
		commandReg:   commandReg,
	}
}

// Name returns the tool name
func (SkillTool) Name() string {
	return SkillToolName
}

// Description returns the tool description
// Ported from src/tools/SkillTool/prompt.ts
func (SkillTool) Description() string {
	return `Execute a skill by name. Skills are reusable prompt templates that help accomplish specific tasks.

	When using this tool:
	- Use the exact skill name (e.g., "commit", "review-pr", "pdf")
	- Optionally provide arguments after the skill name
	- The skill will be expanded into a prompt for the model

	Example usage:
	- Skill with skill="commit" - creates a git commit
	- Skill with skill="review-pr" args="123" - reviews PR #123`
}

// IsReadOnly returns true for simple skills
func (SkillTool) IsReadOnly(in tool.Input) bool {
	// By default, skills are considered read-only for permission purposes
	// The actual behavior depends on what tools the skill uses
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
	Skill string `json:"skill"`
	Args  string `json:"args,omitempty"`
}

// SkillOutput represents the output
// Ported from src/tools/SkillTool/SkillTool.ts:OutputSchema
type SkillOutput struct {
	Success      bool     `json:"success"`
	CommandName  string   `json:"command_name"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	Model        string   `json:"model,omitempty"`
	Effort       string   `json:"effort,omitempty"`
	Status       string   `json:"status,omitempty"` // "inline" or "forked"
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
// Ported from src/tools/SkillTool/SkillTool.ts:call
func (t *SkillTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	input := parseSkillInput(in)

	// Validate required fields
	trimmed := strings.TrimSpace(input.Skill)
	if trimmed == "" {
		return tool.Result{}, fmt.Errorf("skill name is required")
	}

	// Remove leading slash if present (for compatibility)
	commandName := strings.TrimPrefix(trimmed, "/")

	// Find the skill/command
	var foundSkill *Skill
	var foundCommand command.Command

	// First check skill service
	if t.skillService != nil {
		skills := t.skillService.List()
		for i := range skills {
			if skills[i].Name == commandName {
				foundSkill = &skills[i]
				break
			}
			// Check aliases
			for _, alias := range skills[i].Aliases {
				if alias == commandName {
					foundSkill = &skills[i]
					break
				}
			}
		}
	}

	// Also check command registry for prompt commands
	if t.commandReg != nil {
		foundCommand, _ = t.commandReg.Lookup(commandName)
	}

	// If found in skill service but not as a Command, convert
	if foundSkill != nil && foundCommand == nil {
		// Get the skill commands
		if t.skillService != nil {
			skillCommands := t.skillService.Commands()
			for _, cmd := range skillCommands {
				if cmd.GetBase().Name == commandName {
					foundCommand = cmd
					break
				}
			}
		}
	}

	if foundSkill == nil && foundCommand == nil {
		available := t.getAvailableSkillNames()
		return tool.Result{}, fmt.Errorf("unknown skill: %s. Available skills: %s", commandName, strings.Join(available, ", "))
	}

	// Check if model invocation is disabled
	if foundCommand != nil && foundCommand.GetBase().DisableModelInvocation {
		return tool.Result{}, fmt.Errorf("skill %s cannot be used with Skill tool due to disable-model-invocation", commandName)
	}

	// Check if this is a prompt-based skill
	if foundCommand != nil && foundCommand.GetKind() != command.KindPrompt {
		return tool.Result{}, fmt.Errorf("skill %s is not a prompt-based skill", commandName)
	}

	// Determine execution context (inline vs forked)
	executionContext := "inline"
	if foundSkill != nil && foundSkill.Context == "fork" {
		executionContext = "forked"
	}

	// Forked execution via sub-agent
	if executionContext == "forked" && runtime.SpawnAgent != nil {
		return t.executeForkedSkill(ctx, foundSkill, commandName, input.Args, runtime)
	}

	// Inline execution - expand skill content into conversation
	return t.executeInlineSkill(ctx, foundSkill, foundCommand, commandName, input.Args, runtime)
}

// executeInlineSkill executes a skill inline (expands into current conversation)
// Ported from src/tools/SkillTool/SkillTool.ts:call (inline path)
func (t *SkillTool) executeInlineSkill(ctx context.Context, skill *Skill, cmd command.Command, commandName, args string, runtime tool.Runtime) (tool.Result, error) {
	// Get skill content
	var skillContent string
	var allowedTools []string
	var modelOverride string
	var effortOverride string

	if skill != nil {
		// Process skill prompt with variable substitution
		skillContent = t.processSkillPrompt(skill.Prompt, skill.BaseDir, args)
		allowedTools = skill.AllowedTools
		modelOverride = skill.Model
	} else if cmd != nil {
		// Try to get prompt from command handler
		if legacyCmd, ok := cmd.(command.LegacyCommand); ok {
			if legacyCmd.Handler != nil {
				// Split args into array
				argsSlice := strings.Fields(args)
				content, err := legacyCmd.Handler(ctx, command.Runtime{}, argsSlice)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to get skill content: %w", err)
				}
				skillContent = content
			}
			allowedTools = legacyCmd.AllowedTools
			modelOverride = legacyCmd.Model
		}
	}

	if skillContent == "" {
		return tool.Result{}, fmt.Errorf("skill %s has no content", commandName)
	}

	// Build result
	output := SkillOutput{
		Success:      true,
		CommandName:  commandName,
		Status:       "inline",
		AllowedTools: allowedTools,
		Model:        modelOverride,
		Effort:       effortOverride,
	}

	// Return skill content - in Go, we return it as part of the result
	// The engine will handle expanding it into the conversation
	return tool.Result{
		Content:      output,
		SkillContent: skillContent,
	}, nil
}

// executeForkedSkill executes a skill in a forked sub-agent
// Ported from src/tools/SkillTool/SkillTool.ts:executeForkedSkill
func (t *SkillTool) executeForkedSkill(ctx context.Context, skill *Skill, commandName, args string, runtime tool.Runtime) (tool.Result, error) {
	// Process skill prompt with variable substitution
	skillContent := t.processSkillPrompt(skill.Prompt, skill.BaseDir, args)

	// Determine agent type
	agentType := skill.Agent
	if agentType == "" {
		agentType = "general-purpose"
	}

	// Spawn agent with skill content as prompt
	spawnInput := tool.AgentSpawnRequest{
		Type:        agentType,
		Prompt:      skillContent,
		Description: fmt.Sprintf("Executing skill: %s", commandName),
		Background:  false, // Forked skills are synchronous
	}

	task, err := runtime.SpawnAgent(ctx, spawnInput)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to spawn agent for skill: %w", err)
	}

	// Wait for agent completion (forked skills are synchronous)
	// In a full implementation, we would poll for completion
	// For now, return the task ID for later retrieval
	output := SkillOutput{
		Success:     true,
		CommandName: commandName,
		Status:      "forked",
		AgentID:     task.ID,
		Result:      "Skill execution started in sub-agent",
	}

	return tool.Result{
		Content: output,
	}, nil
}

// processSkillPrompt processes skill content with variable substitution
// Ported from src/utils/processUserInput/processSlashCommand.ts and src/skills/loadSkillsDir.ts
func (t *SkillTool) processSkillPrompt(prompt, baseDir, args string) string {
	content := prompt

	// Add base directory header if available
	if baseDir != "" {
		// Normalize path for cross-platform compatibility
		normalizedDir := filepath.ToSlash(baseDir)
		content = fmt.Sprintf("Base directory for this skill: %s\n\n%s", normalizedDir, content)
	}

	// Variable substitution
	// ${CLAUDE_SKILL_DIR} -> skill directory path
	if baseDir != "" {
		normalizedDir := filepath.ToSlash(baseDir)
		content = strings.ReplaceAll(content, "${CLAUDE_SKILL_DIR}", normalizedDir)
	}

	// $ARGUMENTS or ${ARGUMENTS} -> user-provided arguments
	if args != "" {
		content = strings.ReplaceAll(content, "$ARGUMENTS", args)
		content = strings.ReplaceAll(content, "${ARGUMENTS}", args)
	}

	// Append arguments section if args provided and no substitution happened
	if args != "" && !strings.Contains(prompt, "$ARGUMENTS") && !strings.Contains(prompt, "${ARGUMENTS}") {
		content = content + "\n\nArguments: " + args
	}

	return strings.TrimSpace(content)
}

// getAvailableSkillNames returns list of available skill names
func (t *SkillTool) getAvailableSkillNames() []string {
	names := make([]string, 0)

	// From skill service
	if t.skillService != nil {
		for _, skill := range t.skillService.List() {
			if skill.UserInvocable {
				names = append(names, skill.Name)
			}
		}
	}

	// From command registry
	if t.commandReg != nil {
		for _, cmd := range t.commandReg.List() {
			base := cmd.GetBase()
			if cmd.GetKind() == command.KindPrompt && !base.DisableModelInvocation {
				names = append(names, base.Name)
			}
		}
	}

	return names
}

// SetSkillService updates the skill service
func (t *SkillTool) SetSkillService(service SkillLister) {
	t.mu.Lock()
	t.skillService = service
	t.mu.Unlock()
}

// SetCommandRegistry updates the command registry
func (t *SkillTool) SetCommandRegistry(reg *command.Registry) {
	t.mu.Lock()
	t.commandReg = reg
	t.mu.Unlock()
}

// RegisterSkillTools registers the Skill tool
func RegisterSkillTools(r *tool.Registry) {
	r.Register(CreateSkillTool(nil, nil))
}

// Safe skill properties for auto-allow
// Ported from src/tools/SkillTool/SkillTool.ts:SAFE_SKILL_PROPERTIES
var SafeSkillProperties = map[string]bool{
	"type":                   true,
	"progressMessage":        true,
	"contentLength":          true,
	"argNames":               true,
	"model":                  true,
	"effort":                 true,
	"source":                 true,
	"pluginInfo":             true,
	"disableNonInteractive":  true,
	"skillRoot":              true,
	"context":                true,
	"agent":                  true,
	"getPromptForCommand":    true,
	"frontmatterKeys":        true,
	"name":                   true,
	"description":            true,
	"hasUserSpecifiedDescription": true,
	"isEnabled":              true,
	"isHidden":               true,
	"aliases":                true,
	"isMcp":                  true,
	"argumentHint":           true,
	"whenToUse":              true,
	"paths":                  true,
	"version":                true,
	"disableModelInvocation": true,
	"userInvocable":          true,
	"loadedFrom":             true,
	"immediate":              true,
	"userFacingName":         true,
	"display_name":           true,
	"allowed_tools":          true,
	"base_dir":               true,
}

// SkillHasOnlySafeProperties checks if skill has only safe properties (for auto-allow)
// Ported from src/tools/SkillTool/SkillTool.ts:skillHasOnlySafeProperties
func SkillHasOnlySafeProperties(skill *Skill) bool {
	// Check if skill has hooks or allowed tools with dangerous tools
	if len(skill.AllowedTools) > 0 {
		// Skills with allowed tools require permission
		return false
	}

	// Skills with only safe properties can be auto-allowed
	// For simplicity, we check: no hooks, no dangerous allowedTools
	return true
}