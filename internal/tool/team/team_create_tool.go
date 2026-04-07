package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	"claude-code-go/internal/tool"
)

// TeamCreateTool creates a new team for coordinating multiple agents
type TeamCreateTool struct{}

func (TeamCreateTool) Name() string        { return TeamCreateToolName }
func (TeamCreateTool) Description() string { return "Create a new team for coordinating multiple agents" }
func (TeamCreateTool) IsReadOnly(_ tool.Input) bool { return false }

func (TeamCreateTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"team_name": tool.SchemaString("Name for the new team to create."),
		"description": tool.SchemaString("Team description/purpose."),
		"agent_type": tool.SchemaString("Type/role of the team lead (e.g., \"researcher\", \"test-runner\"). Used for team file and inter-agent coordination."),
	}, "team_name")
}

// TeamCreateOutput is the output from TeamCreateTool
type TeamCreateOutput struct {
	TeamName     string `json:"team_name"`
	TeamFilePath string `json:"team_file_path"`
	LeadAgentID  string `json:"lead_agent_id"`
}

func (t TeamCreateTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	teamName := tool.GetString(in, "team_name")
	description := tool.GetString(in, "description")
	agentType := tool.GetString(in, "agent_type")

	// Validate input
	if strings.TrimSpace(teamName) == "" {
		return tool.Result{}, fmt.Errorf("team_name is required for TeamCreate")
	}

	// Check if team context already exists - restrict to one team per leader
	if runtime.Store != nil {
		snapshot := runtime.Store.Snapshot()
		// Check if there's existing team context in the session flags
		if existingTeam, ok := snapshot.SessionFlags["teamName"]; ok && existingTeam {
			if teamCtx, ok := snapshot.SessionFlags["teamContext"]; ok && teamCtx {
				return tool.Result{}, fmt.Errorf("Already leading a team. A leader can only manage one team at a time. Use TeamDelete to end the current team before creating a new one.")
			}
		}
	}

	// Generate a unique team name if the provided one already exists
	finalTeamName := generateUniqueTeamName(teamName)

	// Generate deterministic agent ID for the team lead
	leadAgentID := FormatAgentID(TeamLeadName, finalTeamName)
	leadAgentType := agentType
	if leadAgentType == "" {
		leadAgentType = TeamLeadName
	}

	// Get the team lead's current model from config or default
	leadModel := "claude-sonnet-4-20250514"
	if runtime.Store != nil {
		snapshot := runtime.Store.Snapshot()
		if snapshot.MainLoopModel != "" {
			leadModel = snapshot.MainLoopModel
		} else if snapshot.CurrentModel != "" {
			leadModel = snapshot.CurrentModel
		}
	}

	// Get current working directory
	cwd := "."
	if runtime.Store != nil {
		cwd = runtime.Store.GetCWD()
	}

	// Get session ID
	sessionID := ""
	if runtime.Store != nil {
		snapshot := runtime.Store.Snapshot()
		sessionID = snapshot.SessionID
	}

	teamFilePath := GetTeamFilePath(finalTeamName)

	// Create team file
	teamFile := &TeamFile{
		Name:          finalTeamName,
		Description:   description,
		CreatedAt:     time.Now().UnixMilli(),
		LeadAgentID:   leadAgentID,
		LeadSessionID: sessionID,
		Members: []TeamMember{
			{
				AgentID:       leadAgentID,
				Name:          TeamLeadName,
				AgentType:     leadAgentType,
				Model:         leadModel,
				JoinedAt:      time.Now().UnixMilli(),
				TmuxPaneID:    "",
				CWD:           cwd,
				Subscriptions: []string{},
			},
		},
	}

	if err := WriteTeamFile(finalTeamName, teamFile); err != nil {
		return tool.Result{}, fmt.Errorf("write team file: %w", err)
	}

	// Create the corresponding task list directory
	taskListID := SanitizeName(finalTeamName)
	if err := EnsureTasksDir(taskListID); err != nil {
		return tool.Result{}, fmt.Errorf("create tasks directory: %w", err)
	}

	// Update session state with team context
	if runtime.Store != nil {
		runtime.Store.SetSessionFlag("teamName", true)
		runtime.Store.SetSessionFlag("teamContext", true)
		runtime.Store.SetSessionFlag("leadAgentID", true)
		runtime.Store.SetSessionFlag("teamFilePath", true)
	}

	output := TeamCreateOutput{
		TeamName:     finalTeamName,
		TeamFilePath: teamFilePath,
		LeadAgentID:  leadAgentID,
	}

	return tool.Result{Content: output}, nil
}

// generateUniqueTeamName generates a unique team name by checking if the provided name already exists
func generateUniqueTeamName(providedName string) string {
	// If the team doesn't exist, use the provided name
	teamFile, err := ReadTeamFile(providedName)
	if err != nil || teamFile == nil {
		return providedName
	}

	// Team exists, generate a new unique name using timestamp
	return fmt.Sprintf("%s-%d", SanitizeName(providedName), time.Now().UnixMilli())
}

// GetTeamCreatePrompt returns the tool prompt
func GetTeamCreatePrompt() string {
	return strings.TrimSpace(`
# TeamCreate

## When to Use

Use this tool proactively whenever:
- The user explicitly asks to use a team, swarm, or group of agents
- The user mentions wanting agents to work together, coordinate, or collaborate
- A task is complex enough that it would benefit from parallel work by multiple agents

When in doubt about whether a task warrants a team, prefer spawning a team.

## Choosing Agent Types for Teammates

When spawning teammates via the Agent tool, choose the subagent_type based on what tools the agent needs:
- **Read-only agents** (e.g., Explore, Plan) cannot edit or write files
- **Full-capability agents** (e.g., general-purpose) have access to all tools
- **Custom agents** may have their own tool restrictions

Create a new team to coordinate multiple agents working on a project. Teams have a 1:1 correspondence with task lists (Team = TaskList).

{
  "team_name": "my-project",
  "description": "Working on feature X"
}

This creates:
- A team file at ~/.claude/teams/{team-name}/config.json
- A corresponding task list directory at ~/.claude/tasks/{team-name}/

## Team Workflow

1. Create a team with TeamCreate
2. Create tasks using Task tools
3. Spawn teammates using Agent tool
4. Assign tasks using TaskUpdate
5. Teammates work on assigned tasks
6. Shutdown team when complete
`)
}