package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TeamMember represents a member of a team
type TeamMember struct {
	AgentID       string   `json:"agentId"`
	Name          string   `json:"name"`
	AgentType     string   `json:"agentType,omitempty"`
	Model         string   `json:"model,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	Color         string   `json:"color,omitempty"`
	JoinedAt      int64    `json:"joinedAt"`
	TmuxPaneID    string   `json:"tmuxPaneId"`
	CWD           string   `json:"cwd"`
	WorktreePath  string   `json:"worktreePath,omitempty"`
	SessionID     string   `json:"sessionId,omitempty"`
	Subscriptions []string `json:"subscriptions"`
	BackendType   string   `json:"backendType,omitempty"`
	IsActive      *bool    `json:"isActive,omitempty"` // nil/true = active, false = idle
	Mode          string   `json:"mode,omitempty"`
}

// TeamFile represents the team configuration file
type TeamFile struct {
	Name             string        `json:"name"`
	Description      string        `json:"description,omitempty"`
	CreatedAt        int64         `json:"createdAt"`
	LeadAgentID      string        `json:"leadAgentId"`
	LeadSessionID    string        `json:"leadSessionId,omitempty"`
	HiddenPaneIDs    []string      `json:"hiddenPaneIds,omitempty"`
	TeamAllowedPaths []interface{} `json:"teamAllowedPaths,omitempty"`
	Members          []TeamMember  `json:"members"`
}

// SanitizeName sanitizes a name for use in tmux window names, worktree paths, and file paths.
// Replaces all non-alphanumeric characters with hyphens and lowercases.
func SanitizeName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return strings.ToLower(re.ReplaceAllString(name, "-"))
}

// SanitizeAgentName sanitizes an agent name for use in deterministic agent IDs.
// Replaces @ with - to prevent ambiguity in the agentName@teamName format.
func SanitizeAgentName(name string) string {
	return strings.ReplaceAll(name, "@", "-")
}

// GetClaudeConfigHomeDir returns the Claude config home directory
func GetClaudeConfigHomeDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".claude")
}

// GetTeamsDir returns the path to the teams directory
func GetTeamsDir() string {
	return filepath.Join(GetClaudeConfigHomeDir(), "teams")
}

// GetTasksDir returns the path to the tasks directory for a given task list ID
func GetTasksDir(taskListID string) string {
	return filepath.Join(GetClaudeConfigHomeDir(), "tasks", taskListID)
}

// GetTeamDir returns the path to a team's directory
func GetTeamDir(teamName string) string {
	return filepath.Join(GetTeamsDir(), SanitizeName(teamName))
}

// GetTeamFilePath returns the path to a team's config.json file
func GetTeamFilePath(teamName string) string {
	return filepath.Join(GetTeamDir(teamName), "config.json")
}

// ReadTeamFile reads a team file by name
func ReadTeamFile(teamName string) (*TeamFile, error) {
	path := GetTeamFilePath(teamName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read team file: %w", err)
	}
	var teamFile TeamFile
	if err := json.Unmarshal(data, &teamFile); err != nil {
		return nil, fmt.Errorf("parse team file: %w", err)
	}
	return &teamFile, nil
}

// WriteTeamFile writes a team file
func WriteTeamFile(teamName string, teamFile *TeamFile) error {
	dir := GetTeamDir(teamName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create team directory: %w", err)
	}
	data, err := json.MarshalIndent(teamFile, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal team file: %w", err)
	}
	path := GetTeamFilePath(teamName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write team file: %w", err)
	}
	return nil
}

// CleanupTeamDirectories cleans up team and task directories for a given team name
func CleanupTeamDirectories(teamName string) error {
	sanitizedName := SanitizeName(teamName)

	// Clean up team directory
	teamDir := GetTeamDir(teamName)
	if err := os.RemoveAll(teamDir); err != nil {
		return fmt.Errorf("remove team directory: %w", err)
	}

	// Clean up tasks directory
	tasksDir := GetTasksDir(sanitizedName)
	if err := os.RemoveAll(tasksDir); err != nil {
		return fmt.Errorf("remove tasks directory: %w", err)
	}

	return nil
}

// EnsureTasksDir ensures the tasks directory exists for a task list ID
func EnsureTasksDir(taskListID string) error {
	dir := GetTasksDir(taskListID)
	return os.MkdirAll(dir, 0755)
}

// FormatAgentID formats an agent ID in the format `agentName@teamName`
func FormatAgentID(agentName, teamName string) string {
	return fmt.Sprintf("%s@%s", agentName, teamName)
}

// ParseAgentID parses an agent ID into its components
func ParseAgentID(agentID string) (agentName, teamName string, ok bool) {
	idx := strings.Index(agentID, "@")
	if idx == -1 {
		return "", "", false
	}
	return agentID[:idx], agentID[idx+1:], true
}
