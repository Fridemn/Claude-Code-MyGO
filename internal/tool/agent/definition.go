package agent

import (
	"strings"
)

// SettingSource indicates where an agent definition comes from
type SettingSource string

const (
	SourceBuiltIn       SettingSource = "built-in"
	SourceUserSettings  SettingSource = "userSettings"
	SourceProject       SettingSource = "projectSettings"
	SourcePolicy        SettingSource = "policySettings"
	SourceFlag          SettingSource = "flagSettings"
	SourcePlugin        SettingSource = "plugin"
)

// PermissionMode defines the permission mode for an agent
type PermissionMode string

const (
	PermissionDefault          PermissionMode = "default"
	PermissionAcceptEdits      PermissionMode = "acceptEdits"
	PermissionPlan             PermissionMode = "plan"
	PermissionBypass           PermissionMode = "bypassPermissions"
	PermissionAuto             PermissionMode = "auto"
)

// BaseAgentDefinition contains common fields for all agent types
type BaseAgentDefinition struct {
	AgentType        string                `json:"agent_type"`
	WhenToUse        string                `json:"when_to_use"`
	Tools            []string              `json:"tools,omitempty"`
	DisallowedTools  []string              `json:"disallowed_tools,omitempty"`
	Skills           []string              `json:"skills,omitempty"`
	McpServers       []AgentMcpServerSpec  `json:"mcp_servers,omitempty"`
	Hooks            map[string]any        `json:"hooks,omitempty"`
	Color            string                `json:"color,omitempty"`
	Model            string                `json:"model,omitempty"`
	Effort           any                   `json:"effort,omitempty"` // string level or int
	PermissionMode   PermissionMode        `json:"permission_mode,omitempty"`
	MaxTurns         int                   `json:"max_turns,omitempty"`
	Filename         string                `json:"filename,omitempty"`
	BaseDir          string                `json:"base_dir,omitempty"`
	RequiredMcpServers []string            `json:"required_mcp_servers,omitempty"`
	Background       bool                  `json:"background,omitempty"`
	InitialPrompt    string                `json:"initial_prompt,omitempty"`
	Memory           string                `json:"memory,omitempty"` // user, project, local
	Isolation        string                `json:"isolation,omitempty"` // worktree, remote
	OmitClaudeMd     bool                  `json:"omit_claude_md,omitempty"`
}

// BuiltInAgentDefinition represents a built-in agent
type BuiltInAgentDefinition struct {
	BaseAgentDefinition
	Source          SettingSource `json:"source"`
	systemPromptFunc func() string `json:"-"`
	Callback        func() `json:"-"`
}

// CustomAgentDefinition represents a custom agent from user/project/policy settings
type CustomAgentDefinition struct {
	BaseAgentDefinition
	systemPromptFunc func() string `json:"-"`
	Source          SettingSource `json:"source"`
	Filename        string        `json:"filename,omitempty"`
	BaseDir         string        `json:"base_dir,omitempty"`
}

// PluginAgentDefinition represents an agent from a plugin
type PluginAgentDefinition struct {
	BaseAgentDefinition
	systemPromptFunc func() string `json:"-"`
	Source          SettingSource `json:"source"`
	Filename        string        `json:"filename,omitempty"`
	Plugin          string        `json:"plugin"`
}

// AgentDefinition is the union type for all agent types
type AgentDefinition interface {
	GetAgentType() string
	GetWhenToUse() string
	GetTools() []string
	GetDisallowedTools() []string
	GetSource() SettingSource
	GetSystemPrompt() string
	IsBuiltIn() bool
}

// AgentMcpServerSpec represents an MCP server specification in agent definitions
type AgentMcpServerSpec struct {
	// Name is a reference to an existing server by name
	Name string `json:"name,omitempty"`
	// Inline is an inline definition as { name: config }
	Inline map[string]any `json:"inline,omitempty"`
}

// Implement AgentDefinition interface for BuiltInAgentDefinition
func (a BuiltInAgentDefinition) GetAgentType() string        { return a.AgentType }
func (a BuiltInAgentDefinition) GetWhenToUse() string       { return a.WhenToUse }
func (a BuiltInAgentDefinition) GetTools() []string         { return a.Tools }
func (a BuiltInAgentDefinition) GetDisallowedTools() []string { return a.DisallowedTools }
func (a BuiltInAgentDefinition) GetSource() SettingSource   { return a.Source }
func (a BuiltInAgentDefinition) GetSystemPrompt() string {
	if a.systemPromptFunc != nil {
		return a.systemPromptFunc()
	}
	return ""
}
func (a BuiltInAgentDefinition) IsBuiltIn() bool { return true }

// Implement AgentDefinition interface for CustomAgentDefinition
func (a CustomAgentDefinition) GetAgentType() string         { return a.AgentType }
func (a CustomAgentDefinition) GetWhenToUse() string        { return a.WhenToUse }
func (a CustomAgentDefinition) GetTools() []string          { return a.Tools }
func (a CustomAgentDefinition) GetDisallowedTools() []string { return a.DisallowedTools }
func (a CustomAgentDefinition) GetSource() SettingSource    { return a.Source }
func (a CustomAgentDefinition) GetSystemPrompt() string {
	if a.systemPromptFunc != nil {
		return a.systemPromptFunc()
	}
	return ""
}
func (a CustomAgentDefinition) IsBuiltIn() bool { return false }

// Implement AgentDefinition interface for PluginAgentDefinition
func (a PluginAgentDefinition) GetAgentType() string         { return a.AgentType }
func (a PluginAgentDefinition) GetWhenToUse() string        { return a.WhenToUse }
func (a PluginAgentDefinition) GetTools() []string          { return a.Tools }
func (a PluginAgentDefinition) GetDisallowedTools() []string { return a.DisallowedTools }
func (a PluginAgentDefinition) GetSource() SettingSource    { return a.Source }
func (a PluginAgentDefinition) GetSystemPrompt() string {
	if a.systemPromptFunc != nil {
		return a.systemPromptFunc()
	}
	return ""
}
func (a PluginAgentDefinition) IsBuiltIn() bool { return false }

// GeneralPurposeAgent is the default agent type
var GeneralPurposeAgent = BuiltInAgentDefinition{
	BaseAgentDefinition: BaseAgentDefinition{
		AgentType: "general-purpose",
		WhenToUse: "General purpose agent for any task",
	},
	Source: SourceBuiltIn,
	systemPromptFunc: func() string {
		return generalPurposeAgentPrompt
	},
}

const generalPurposeAgentPrompt = `You are a general-purpose AI assistant. You help users with a wide variety of tasks including:
- Writing and editing code
- Answering questions about codebases
- Performing research and analysis
- Executing commands and scripts
- File operations

Be thorough but concise in your responses. When writing code, follow best practices and include appropriate error handling.`

// HasRequiredMcpServers checks if an agent's required MCP servers are available
func HasRequiredMcpServers(agent AgentDefinition, availableServers []string) bool {
	required := getRequiredMcpServers(agent)
	if len(required) == 0 {
		return true
	}
	for _, pattern := range required {
		found := false
		for _, server := range availableServers {
			if strings.Contains(strings.ToLower(server), strings.ToLower(pattern)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func getRequiredMcpServers(agent AgentDefinition) []string {
	switch a := agent.(type) {
	case *BuiltInAgentDefinition:
		return a.RequiredMcpServers
	case *CustomAgentDefinition:
		return a.RequiredMcpServers
	case *PluginAgentDefinition:
		return a.RequiredMcpServers
	}
	return nil
}

// FilterAgentsByMcpRequirements filters agents based on MCP server requirements
func FilterAgentsByMcpRequirements(agents []AgentDefinition, availableServers []string) []AgentDefinition {
	var result []AgentDefinition
	for _, agent := range agents {
		if HasRequiredMcpServers(agent, availableServers) {
			result = append(result, agent)
		}
	}
	return result
}

// GetBuiltInAgents returns the list of built-in agents
func GetBuiltInAgents() []AgentDefinition {
	return []AgentDefinition{
		&GeneralPurposeAgent,
		&ExploreAgent,
		&PlanAgent,
	}
}

// ExploreAgent is a built-in agent for code exploration
var ExploreAgent = BuiltInAgentDefinition{
	BaseAgentDefinition: BaseAgentDefinition{
		AgentType:    "Explore",
		WhenToUse:    "Best for deep exploration of codebases to understand structure, patterns, and relationships",
		Tools:        []string{"Read", "Glob", "Grep", "Bash"},
		OmitClaudeMd: true,
	},
	Source: SourceBuiltIn,
	systemPromptFunc: func() string {
		return exploreAgentPrompt
	},
}

const exploreAgentPrompt = `You are an exploration agent specialized in understanding codebases. Your job is to thoroughly explore and document code structure, patterns, and relationships.

Guidelines:
- Start with broad searches and narrow down
- Document what you find clearly
- Identify key files and their purposes
- Note any interesting patterns or potential issues
- Provide a summary of your findings

Do NOT make any code changes. This is a read-only exploration task.`

// PlanAgent is a built-in agent for planning
var PlanAgent = BuiltInAgentDefinition{
	BaseAgentDefinition: BaseAgentDefinition{
		AgentType:    "Plan",
		WhenToUse:    "Best for creating detailed implementation plans before coding",
		Tools:        []string{"Read", "Glob", "Grep", "Bash"},
		OmitClaudeMd: true,
	},
	Source: SourceBuiltIn,
	systemPromptFunc: func() string {
		return planAgentPrompt
	},
}

const planAgentPrompt = `You are a planning agent specialized in creating detailed implementation plans. Your job is to analyze requirements and create step-by-step plans.

Guidelines:
- Understand the full scope of the task
- Break down work into manageable steps
- Identify dependencies and potential issues
- Consider edge cases and error handling
- Provide clear, actionable plans

Do NOT make any code changes. This is a planning task only.`
