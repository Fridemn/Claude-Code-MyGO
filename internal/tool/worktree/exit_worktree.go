package worktree

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/tool"
)

// ExitWorktreeTool exits a worktree session created by EnterWorktree.
type ExitWorktreeTool struct{}

func (ExitWorktreeTool) Name() string { return exitWorktreeToolName }

func (ExitWorktreeTool) Description() string {
	return "Exits a worktree session created by EnterWorktree and restores the original working directory"
}

func (ExitWorktreeTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"keep", "remove"},
				"description": "\"keep\" leaves the worktree and branch on disk; \"remove\" deletes both.",
			},
			"discard_changes": map[string]any{
				"type":        "boolean",
				"description": "Required true when action is \"remove\" and the worktree has uncommitted files or unmerged commits. The tool will refuse and list them otherwise.",
			},
		},
		"required": []string{"action"},
	}
}

func (ExitWorktreeTool) IsReadOnly(in tool.Input) bool {
	action, _ := in["action"].(string)
	return action == "keep"
}

// ExitWorktreeOutput represents the output of ExitWorktree tool
type ExitWorktreeOutput struct {
	Action           string `json:"action"`
	OriginalCWD      string `json:"originalCwd"`
	WorktreePath     string `json:"worktreePath"`
	WorktreeBranch   string `json:"worktreeBranch,omitempty"`
	TmuxSessionName  string `json:"tmuxSessionName,omitempty"`
	DiscardedFiles   int    `json:"discardedFiles,omitempty"`
	DiscardedCommits int    `json:"discardedCommits,omitempty"`
	Message          string `json:"message"`
}

// Prompt for ExitWorktreeTool
func getExitWorktreeToolPrompt() string {
	return "Exit a worktree session created by EnterWorktree and return the session to the original working directory.\n\n" +
		"## Scope\n\n" +
		"This tool ONLY operates on worktrees created by EnterWorktree in this session. It will NOT touch:\n" +
		"- Worktrees you created manually with `git worktree add`\n" +
		"- Worktrees from a previous session (even if created by EnterWorktree then)\n" +
		"- The directory you're in if EnterWorktree was never called\n\n" +
		"If called outside an EnterWorktree session, the tool is a **no-op**: it reports that no worktree session is active and takes no action. Filesystem state is unchanged.\n\n" +
		"## When to Use\n\n" +
		"- The user explicitly asks to \"exit the worktree\", \"leave the worktree\", \"go back\", or otherwise end the worktree session\n" +
		"- Do NOT call this proactively - only when the user asks\n\n" +
		"## Parameters\n\n" +
		"- `action` (required): `keep` or `remove`\n" +
		"  - `keep` - leave the worktree directory and branch intact on disk. Use this if the user wants to come back to the work later, or if there are changes to preserve.\n" +
		"  - `remove` - delete the worktree directory and its branch. Use this for a clean exit when the work is done or abandoned.\n" +
		"- `discard_changes` (optional, default false): only meaningful with action: \"remove\". If the worktree has uncommitted files or commits not on the original branch, the tool will REFUSE to remove it unless this is set to true. If the tool returns an error listing changes, confirm with the user before re-invoking with discard_changes: true.\n\n" +
		"## Behavior\n\n" +
		"- Restores the session's working directory to where it was before EnterWorktree\n" +
		"- Clears CWD-dependent caches (system prompt sections, memory files, plans directory) so the session state reflects the original directory\n" +
		"- If a tmux session was attached to the worktree: killed on remove, left running on keep (its name is returned so the user can reattach)\n" +
		"- Once exited, EnterWorktree can be called again to create a fresh worktree\n"
}

func (ExitWorktreeTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// Check if in a worktree session
	session := GetCurrentWorktreeSession()
	if session == nil {
		// No-op: not in a worktree session
		return tool.Result{
			Content: ExitWorktreeOutput{
				Message: "No-op: there is no active EnterWorktree session to exit. This tool only operates on worktrees created by EnterWorktree in the current session - it will not touch worktrees created manually or in a previous session. No filesystem changes were made.",
			},
		}, nil
	}

	// Get action
	action, _ := in["action"].(string)
	if action == "" {
		action = "keep"
	}

	// Get discard_changes
	discardChanges, _ := in["discard_changes"].(bool)

	// Count changes if removing
	changedFiles := 0
	commits := 0

	if action == "remove" && !discardChanges {
		// Check for changes before removing
		cf, cm, err := CountWorktreeChanges(session.WorktreePath, session.OriginalHeadCommit)
		if err != nil {
			return tool.Result{
				Content: ExitWorktreeOutput{
					Message: fmt.Sprintf("Could not verify worktree state at %s. Refusing to remove without explicit confirmation. Re-invoke with discard_changes: true to proceed - or use action: \"keep\" to preserve the worktree.", session.WorktreePath),
				},
			}, nil
		}
		changedFiles = cf
		commits = cm

		if changedFiles > 0 || commits > 0 {
			var parts []string
			if changedFiles > 0 {
				fileWord := "file"
				if changedFiles > 1 {
					fileWord = "files"
				}
				parts = append(parts, fmt.Sprintf("%d uncommitted %s", changedFiles, fileWord))
			}
			if commits > 0 {
				commitWord := "commit"
				if commits > 1 {
					commitWord = "commits"
				}
				branchInfo := session.WorktreeBranch
				if branchInfo == "" {
					branchInfo = "the worktree branch"
				}
				parts = append(parts, fmt.Sprintf("%d %s on %s", commits, commitWord, branchInfo))
			}

			return tool.Result{
				Content: ExitWorktreeOutput{
					Message: fmt.Sprintf("Worktree has %s. Removing will discard this work permanently. Confirm with the user, then re-invoke with discard_changes: true - or use action: \"keep\" to preserve the worktree.", strings.Join(parts, " and ")),
				},
			}, nil
		}
	}

	// Capture session data before clearing
	originalCwd := session.OriginalCWD
	worktreePath := session.WorktreePath
	worktreeBranch := session.WorktreeBranch
	tmuxSessionName := session.TmuxSessionName

	// Handle keep vs remove
	var message string

	if action == "keep" {
		// Keep the worktree intact
		if err := KeepWorktree(runtime.Store); err != nil {
			return tool.Result{}, err
		}

		tmuxNote := ""
		if tmuxSessionName != "" {
			tmuxNote = fmt.Sprintf(" Tmux session %s is still running; reattach with: tmux attach -t %s", tmuxSessionName, tmuxSessionName)
		}

		branchInfo := ""
		if worktreeBranch != "" {
			branchInfo = " on branch " + worktreeBranch
		}

		message = fmt.Sprintf("Exited worktree. Your work is preserved at %s%s. Session is now back in %s.%s",
			worktreePath, branchInfo, originalCwd, tmuxNote)

		return tool.Result{
			Content: ExitWorktreeOutput{
				Action:          "keep",
				OriginalCWD:     originalCwd,
				WorktreePath:    worktreePath,
				WorktreeBranch:  worktreeBranch,
				TmuxSessionName: tmuxSessionName,
				Message:         message,
			},
		}, nil
	}

	// action === "remove"
	// Kill tmux session if exists
	if tmuxSessionName != "" {
		KillTmuxSession(tmuxSessionName)
	}

	// Get accurate counts for output
	cf, cm, _ := CountWorktreeChanges(worktreePath, session.OriginalHeadCommit)
	if cf > changedFiles {
		changedFiles = cf
	}
	if cm > commits {
		commits = cm
	}

	// Remove the worktree
	if err := CleanupWorktree(runtime.Store); err != nil {
		return tool.Result{}, err
	}

	// Build discard note
	discardParts := []string{}
	if commits > 0 {
		commitWord := "commit"
		if commits > 1 {
			commitWord = "commits"
		}
		discardParts = append(discardParts, fmt.Sprintf("%d %s", commits, commitWord))
	}
	if changedFiles > 0 {
		fileWord := "file"
		if changedFiles > 1 {
			fileWord = "files"
		}
		discardParts = append(discardParts, fmt.Sprintf("%d uncommitted %s", changedFiles, fileWord))
	}

	discardNote := ""
	if len(discardParts) > 0 {
		discardNote = fmt.Sprintf(" Discarded %s.", strings.Join(discardParts, " and "))
	}

	message = fmt.Sprintf("Exited and removed worktree at %s.%s Session is now back in %s.",
		worktreePath, discardNote, originalCwd)

	return tool.Result{
		Content: ExitWorktreeOutput{
			Action:           "remove",
			OriginalCWD:      originalCwd,
			WorktreePath:     worktreePath,
			WorktreeBranch:   worktreeBranch,
			DiscardedFiles:   changedFiles,
			DiscardedCommits: commits,
			Message:          message,
		},
	}, nil
}

// Ensure the tool implements Definition interface
var _ tool.Definition = ExitWorktreeTool{}