package worktree

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"claude-go/internal/tool"
)

// EnterWorktreeTool creates an isolated git worktree and switches the session into it.
type EnterWorktreeTool struct{}

func (EnterWorktreeTool) Name() string { return enterWorktreeToolName }

func (EnterWorktreeTool) Description() string {
	return "Creates an isolated worktree (via git or configured hooks) and switches the session into it"
}

func (EnterWorktreeTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Optional name for the worktree. Each \"/\"-separated segment may contain only letters, digits, dots, underscores, and dashes; max 64 chars total. A random name is generated if not provided.",
			},
		},
	}
}

func (EnterWorktreeTool) IsReadOnly(_ tool.Input) bool { return false }

// EnterWorktreeOutput represents the output of EnterWorktree tool
type EnterWorktreeOutput struct {
	WorktreePath   string `json:"worktreePath"`
	WorktreeBranch string `json:"worktreeBranch,omitempty"`
	Message        string `json:"message"`
}

// Prompt for EnterWorktreeTool
func getEnterWorktreeToolPrompt() string {
	return "Use this tool ONLY when the user explicitly asks to work in a worktree. This tool creates an isolated git worktree and switches the current session into it.\n\n" +
		"## When to Use\n\n" +
		"- The user explicitly says \"worktree\" (e.g., \"start a worktree\", \"work in a worktree\", \"create a worktree\", \"use a worktree\")\n\n" +
		"## When NOT to Use\n\n" +
		"- The user asks to create a branch, switch branches, or work on a different branch — use git commands instead\n" +
		"- The user asks to fix a bug or work on a feature — use normal git workflow unless they specifically mention worktrees\n" +
		"- Never use this tool unless the user explicitly mentions \"worktree\"\n\n" +
		"## Requirements\n\n" +
		"- Must be in a git repository, OR have WorktreeCreate/WorktreeRemove hooks configured in settings.json\n" +
		"- Must not already be in a worktree\n\n" +
		"## Behavior\n\n" +
		"- In a git repository: creates a new git worktree inside `.claude/worktrees/` with a new branch based on HEAD\n" +
		"- Outside a git repository: delegates to WorktreeCreate/WorktreeRemove hooks for VCS-agnostic isolation\n" +
		"- Switches the session's working directory to the new worktree\n" +
		"- Use ExitWorktree to leave the worktree mid-session (keep or remove). On session exit, if still in the worktree, the user will be prompted to keep or remove it\n\n" +
		"## Parameters\n\n" +
		"- `name` (optional): A name for the worktree. If not provided, a random name is generated.\n"
}

func (EnterWorktreeTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// Check if already in a worktree session
	if GetCurrentWorktreeSession() != nil {
		return tool.Result{}, fmt.Errorf("Already in a worktree session")
	}

	// Get session ID
	sessionID := ""
	if runtime.Store != nil {
		sessionID = runtime.Store.Snapshot().SessionID
	}
	if sessionID == "" {
		// Generate a random session ID if not set
		sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}

	// Get or generate slug
	name, _ := in["name"].(string)
	if name == "" {
		name = generateWorktreeSlug()
	}

	// Validate slug
	if err := ValidateWorktreeSlug(name); err != nil {
		return tool.Result{}, err
	}

	// Create worktree
	session, err := CreateWorktreeForSession(sessionID, name, runtime.Store)
	if err != nil {
		return tool.Result{}, err
	}

	// Build branch info
	branchInfo := ""
	if session.WorktreeBranch != "" {
		branchInfo = " on branch " + session.WorktreeBranch
	}

	message := fmt.Sprintf("Created worktree at %s%s. The session is now working in the worktree. Use ExitWorktree to leave mid-session, or exit the session to be prompted.",
		session.WorktreePath, branchInfo)

	return tool.Result{
		Content: EnterWorktreeOutput{
			WorktreePath:   session.WorktreePath,
			WorktreeBranch: session.WorktreeBranch,
			Message:        message,
		},
	}, nil
}

// generateWorktreeSlug generates a random worktree slug
func generateWorktreeSlug() string {
	adjectives := []string{"swift", "bright", "calm", "keen", "bold"}
	nouns := []string{"fox", "owl", "elm", "oak", "ray"}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	adj := adjectives[r.Intn(len(adjectives))]
	noun := nouns[r.Intn(len(nouns))]
	suffix := fmt.Sprintf("%04d", r.Intn(10000))

	return adj + "-" + noun + "-" + suffix
}

// Ensure the tool implements Definition interface
var _ tool.Definition = EnterWorktreeTool{}