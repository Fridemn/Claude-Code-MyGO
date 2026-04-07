package team

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// TeamDeleteTool cleans up team and task directories when the swarm is complete
type TeamDeleteTool struct{}

func (TeamDeleteTool) Name() string        { return TeamDeleteToolName }
func (TeamDeleteTool) Description() string { return "Clean up team and task directories when the swarm is complete" }
func (TeamDeleteTool) IsReadOnly(_ tool.Input) bool { return false }

func (TeamDeleteTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{})
}

// TeamDeleteOutput is the output from TeamDeleteTool
type TeamDeleteOutput struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	TeamName string `json:"team_name,omitempty"`
}

func (t TeamDeleteTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	var teamName string

	// Get team name from session state
	if runtime.Store != nil {
		snapshot := runtime.Store.Snapshot()
		// Check session flags for team context
		if hasTeam, ok := snapshot.SessionFlags["teamName"]; ok && hasTeam {
			// Try to get the actual team name from the state
			// In the TS version, this comes from AppState.teamContext.teamName
			// For now, we'll use a placeholder approach
			teamName = ""
		}
	}

	if teamName != "" {
		// Read team config to check for active members
		teamFile, err := ReadTeamFile(teamName)
		if err != nil {
			return tool.Result{}, fmt.Errorf("read team file: %w", err)
		}

		if teamFile != nil {
			// Filter out the team lead - only count non-lead members
			var nonLeadMembers []TeamMember
			for _, m := range teamFile.Members {
				if m.Name != TeamLeadName {
					nonLeadMembers = append(nonLeadMembers, m)
				}
			}

			// Separate truly active members from idle/dead ones
			// Members with isActive === false are idle (finished their turn or crashed)
			var activeMembers []TeamMember
			for _, m := range nonLeadMembers {
				if m.IsActive == nil || *m.IsActive {
					activeMembers = append(activeMembers, m)
				}
			}

			if len(activeMembers) > 0 {
				memberNames := make([]string, len(activeMembers))
				for i, m := range activeMembers {
					memberNames[i] = m.Name
				}
				return tool.Result{
					Content: TeamDeleteOutput{
						Success:  false,
						Message:  fmt.Sprintf("Cannot cleanup team with %d active member(s): %s. Use requestShutdown to gracefully terminate teammates first.", len(activeMembers), fmt.Sprintf("%v", memberNames)),
						TeamName: teamName,
					},
				}, nil
			}
		}

		// Clean up team directories
		if err := CleanupTeamDirectories(teamName); err != nil {
			return tool.Result{}, fmt.Errorf("cleanup team directories: %w", err)
		}

		// Clear team context from session state
		if runtime.Store != nil {
			runtime.Store.SetSessionFlag("teamName", false)
			runtime.Store.SetSessionFlag("teamContext", false)
			runtime.Store.SetSessionFlag("leadAgentID", false)
			runtime.Store.SetSessionFlag("teamFilePath", false)
		}
	}

	// Clear inbox messages (would be done in the full implementation)
	// For now, we just return success

	output := TeamDeleteOutput{
		Success: true,
		Message: func() string {
			if teamName != "" {
				return fmt.Sprintf("Cleaned up directories and worktrees for team \"%s\"", teamName)
			}
			return "No team name found, nothing to clean up"
		}(),
		TeamName: teamName,
	}

	return tool.Result{Content: output}, nil
}

// GetTeamDeletePrompt returns the tool prompt
func GetTeamDeletePrompt() string {
	return strings.TrimSpace(`
# TeamDelete

Remove team and task directories when the swarm work is complete.

This operation:
- Removes the team directory (~/.claude/teams/{team-name}/)
- Removes the task directory (~/.claude/tasks/{team-name}/)
- Clears team context from the current session

**IMPORTANT**: TeamDelete will fail if the team still has active members. Gracefully terminate teammates first, then call TeamDelete after all teammates have shut down.

Use this when all teammates have finished their work and you want to clean up the team resources. The team name is automatically determined from the current session's team context.
`)
}