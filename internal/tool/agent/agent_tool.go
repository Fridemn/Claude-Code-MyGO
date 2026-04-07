package agent

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
)

// AgentTool implements the Agent tool for spawning subagents
type AgentTool struct {
	agents []AgentDefinition
}

// CreateAgentTool creates a new Agent tool with the given agent definitions
func CreateAgentTool(agents []AgentDefinition) *AgentTool {
	if agents == nil {
		agents = GetBuiltInAgents()
	}
	return &AgentTool{agents: agents}
}

// Name returns the tool name
func (AgentTool) Name() string {
	return AgentToolName
}

// Description returns the tool description
func (AgentTool) Description() string {
	return AgentToolDescription
}

// IsReadOnly returns false since spawning agents can have side effects
func (AgentTool) IsReadOnly(tool.Input) bool {
	return false
}

// ParametersSchema returns the JSON schema for the tool parameters
func (AgentTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"description": tool.SchemaString("A short (3-5 word) description of the task"),
		"prompt":      tool.SchemaString("The task for the agent to perform"),
		"subagent_type": tool.SchemaEnumString(
			"The type of specialized agent to use for this task",
			"general-purpose", "Explore", "Plan",
		),
		"model": tool.SchemaEnumString(
			"Optional model override for this agent",
			"sonnet", "opus", "haiku",
		),
		"run_in_background": tool.SchemaBoolean("Set to true to run this agent in the background. You will be notified when it completes."),
		"name": tool.SchemaString("Name for the spawned agent. Makes it addressable via SendMessage({to: name}) while running."),
		"team_name": tool.SchemaString("Team name for spawning. Uses current team context if omitted."),
		"mode": tool.SchemaEnumString(
			"Permission mode for spawned teammate",
			"default", "acceptEdits", "plan", "bypassPermissions", "auto",
		),
		"isolation": tool.SchemaEnumString(
			"Isolation mode. \"worktree\" creates a temporary git worktree so the agent works on an isolated copy of the repo.",
			"worktree",
		),
		"cwd": tool.SchemaString("Absolute path to run the agent in. Overrides the working directory for all filesystem and shell operations within this agent."),
	}, "description", "prompt")
}

// AgentToolInput represents the parsed input for the Agent tool
type AgentToolInput struct {
	Description     string
	Prompt          string
	SubagentType    string
	Model           string
	RunInBackground bool
	Name            string
	TeamName        string
	Mode            string
	Isolation       string
	Cwd             string
}

// parseInput extracts the input parameters
func parseInput(in tool.Input) AgentToolInput {
	return AgentToolInput{
		Description:     tool.GetString(in, "description"),
		Prompt:          tool.GetString(in, "prompt"),
		SubagentType:    tool.GetString(in, "subagent_type"),
		Model:           tool.GetString(in, "model"),
		RunInBackground: tool.GetBool(in, "run_in_background"),
		Name:            tool.GetString(in, "name"),
		TeamName:        tool.GetString(in, "team_name"),
		Mode:            tool.GetString(in, "mode"),
		Isolation:       tool.GetString(in, "isolation"),
		Cwd:             tool.GetString(in, "cwd"),
	}
}

// Call executes the Agent tool
func (t *AgentTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	input := parseInput(in)

	// Validate required fields
	if strings.TrimSpace(input.Prompt) == "" {
		return tool.Result{}, fmt.Errorf("prompt is required")
	}
	if strings.TrimSpace(input.Description) == "" {
		return tool.Result{}, fmt.Errorf("description is required")
	}

	// Determine the agent type to use
	agentType := input.SubagentType
	if agentType == "" {
		agentType = "general-purpose"
	}

	// Find the agent definition
	var selectedAgent AgentDefinition
	for _, agent := range t.agents {
		if agent.GetAgentType() == agentType {
			selectedAgent = agent
			break
		}
	}

	if selectedAgent == nil {
		availableTypes := make([]string, len(t.agents))
		for i, a := range t.agents {
			availableTypes[i] = a.GetAgentType()
		}
		return tool.Result{}, fmt.Errorf("agent type '%s' not found. Available agents: %s", agentType, strings.Join(availableTypes, ", "))
	}

	// Check if we should spawn an async agent
	shouldRunAsync := input.RunInBackground || input.Name != "" || input.TeamName != ""

	// Create the spawn request
	req := tool.AgentSpawnRequest{
		Type:        agentType,
		Prompt:      input.Prompt,
		Description: input.Description,
		Background:  shouldRunAsync,
	}

	// Use the runtime to spawn the agent
	if runtime.SpawnAgent != nil {
		task, err := runtime.SpawnAgent(ctx, req)
		if err != nil {
			return tool.Result{}, fmt.Errorf("failed to spawn agent: %w", err)
		}

		if shouldRunAsync {
			return tool.Result{
				Content: map[string]any{
					"status":           "async_launched",
					"agent_id":         task.ID,
					"description":      input.Description,
					"prompt":           input.Prompt,
					"agent_type":       agentType,
					"output_file":      getTaskOutputPath(task.ID),
					"can_read_output":  true,
				},
			}, nil
		}

		return tool.Result{
			Content: map[string]any{
				"status":     "completed",
				"agent_id":   task.ID,
				"agent_type": agentType,
				"prompt":     input.Prompt,
				"output":     task.Output,
				"summary":    task.Summary,
			},
		}, nil
	}

	// Fallback: just return what we would have done
	return tool.Result{
		Content: map[string]any{
			"status":     "simulated",
			"agent_type": agentType,
			"prompt":     input.Prompt,
			"message":    "Agent spawned (runtime.SpawnAgent not configured)",
		},
	}, nil
}

// SetAgents updates the available agent definitions
func (t *AgentTool) SetAgents(agents []AgentDefinition) {
	t.agents = agents
}

// GetAgents returns the current agent definitions
func (t *AgentTool) GetAgents() []AgentDefinition {
	return t.agents
}

// getTaskOutputPath returns the output file path for a task
func getTaskOutputPath(taskID string) string {
	return fmt.Sprintf("/tmp/claude-agent-%s-output.txt", taskID)
}

// GetPrompt returns the prompt for the Agent tool
func (t *AgentTool) GetPrompt(isCoordinator bool, allowedAgentTypes []string) string {
	return GetPrompt(t.agents, isCoordinator, allowedAgentTypes)
}

// TaskResult represents the result from an agent task
type TaskResult struct {
	Status    string `json:"status"`
	AgentID   string `json:"agent_id,omitempty"`
	Output    string `json:"output,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Error     string `json:"error,omitempty"`
}

// AgentTask wraps task.AgentTask for use in results
type AgentTask = task.AgentTask

// RegisterAgentTools registers all agent-related tools
func RegisterAgentTools(r *tool.Registry) {
	r.Register(CreateAgentTool(nil))
	r.Register(CreateSendMessageTool())
	r.Register(CreateBriefTool())
}