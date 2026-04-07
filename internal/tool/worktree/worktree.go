package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"claude-code-go/internal/bootstrap"
)

const (
	enterWorktreeToolName = "EnterWorktree"
	exitWorktreeToolName  = "ExitWorktree"
	maxWorktreeSlugLength = 64
)

// Valid segment pattern for worktree slugs
var validWorktreeSlugSegment = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// WorktreeSession represents an active worktree session
type WorktreeSession struct {
	OriginalCWD        string `json:"original_cwd"`
	WorktreePath       string `json:"worktree_path"`
	WorktreeName       string `json:"worktree_name"`
	WorktreeBranch     string `json:"worktree_branch,omitempty"`
	OriginalBranch     string `json:"original_branch,omitempty"`
	OriginalHeadCommit string `json:"original_head_commit,omitempty"`
	SessionID          string `json:"session_id"`
	TmuxSessionName    string `json:"tmux_session_name,omitempty"`
	HookBased          bool   `json:"hook_based,omitempty"`
	CreationDurationMs int    `json:"creation_duration_ms,omitempty"`
}

// Session state management
var (
	currentWorktreeSession *WorktreeSession
	worktreeSessionMu      sync.Mutex
)

// GetCurrentWorktreeSession returns the current active worktree session
func GetCurrentWorktreeSession() *WorktreeSession {
	worktreeSessionMu.Lock()
	defer worktreeSessionMu.Unlock()
	return currentWorktreeSession
}

// SetCurrentWorktreeSession sets the current worktree session
func SetCurrentWorktreeSession(session *WorktreeSession) {
	worktreeSessionMu.Lock()
	defer worktreeSessionMu.Unlock()
	currentWorktreeSession = session
}

// ClearCurrentWorktreeSession clears the current worktree session
func ClearCurrentWorktreeSession() {
	worktreeSessionMu.Lock()
	defer worktreeSessionMu.Unlock()
	currentWorktreeSession = nil
}

// ValidateWorktreeSlug validates a worktree slug to prevent path traversal
func ValidateWorktreeSlug(slug string) error {
	if len(slug) > maxWorktreeSlugLength {
		return fmt.Errorf("Invalid worktree name: must be %d characters or fewer (got %d)", maxWorktreeSlugLength, len(slug))
	}

	// Check each segment
	for _, segment := range strings.Split(slug, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("Invalid worktree name \"%s\": must not contain \".\" or \"..\" path segments", slug)
		}
		if !validWorktreeSlugSegment.MatchString(segment) {
			return fmt.Errorf("Invalid worktree name \"%s\": each \"/\"-separated segment must be non-empty and contain only letters, digits, dots, underscores, and dashes", slug)
		}
	}

	return nil
}

// FlattenSlug flattens nested slugs for branch names and directory paths
func FlattenSlug(slug string) string {
	return strings.ReplaceAll(slug, "/", "+")
}

// WorktreeBranchName generates the git branch name for a worktree
func WorktreeBranchName(slug string) string {
	return "worktree-" + FlattenSlug(slug)
}

// WorktreesDir returns the path to the worktrees directory
func WorktreesDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".claude", "worktrees")
}

// WorktreePathFor returns the path for a specific worktree
func WorktreePathFor(repoRoot, slug string) string {
	return filepath.Join(WorktreesDir(repoRoot), FlattenSlug(slug))
}

// FindGitRoot finds the git repository root directory
func FindGitRoot(path string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// FindCanonicalGitRoot finds the canonical git root (resolving through worktrees)
func FindCanonicalGitRoot(path string) string {
	// First try to find the common dir (for worktrees)
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		// Fall back to regular git root
		return FindGitRoot(path)
	}

	commonDir := strings.TrimSpace(string(output))
	if commonDir == ".git" {
		// Not in a worktree, return the toplevel
		return FindGitRoot(path)
	}

	// Resolve the common dir relative to the path
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(path, commonDir)
	}

	// The canonical root is the parent of .git
	gitDir := filepath.Dir(commonDir)
	// But if it ends in .git/worktrees, go up two levels
	if strings.HasSuffix(commonDir, "/.git/worktrees") || strings.HasSuffix(commonDir, "\\worktrees") {
		gitDir = filepath.Dir(filepath.Dir(commonDir))
	}

	return gitDir
}

// GetBranch returns the current git branch name
func GetBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		// Might be on detached HEAD
		cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
		branch = strings.TrimSpace(string(output))
	}
	return branch, nil
}

// GetDefaultBranch returns the default branch name (origin/main or origin/master)
func GetDefaultBranch() (string, error) {
	// Try main first
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/main")
	if cmd.Run() == nil {
		return "origin/main", nil
	}

	// Try master
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/master")
	if cmd.Run() == nil {
		return "origin/master", nil
	}

	// Fall back to HEAD
	return "HEAD", nil
}

// GetHeadCommit returns the current HEAD commit SHA
func GetHeadCommit() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// CreateWorktree creates a new git worktree
func CreateWorktree(repoRoot, worktreePath, branch, base string) error {
	// Create the worktrees directory if needed
	worktreesDir := WorktreesDir(repoRoot)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return fmt.Errorf("Failed to create worktrees directory: %w", err)
	}

	// Create worktree with -B to reset any orphan branch
	cmd := exec.Command("git", "worktree", "add", "-B", branch, worktreePath, base)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to create worktree: %s", string(output))
	}

	return nil
}

// RemoveWorktree removes a git worktree
func RemoveWorktree(repoRoot, worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to remove worktree: %s", string(output))
	}
	return nil
}

// DeleteBranch deletes a git branch
func DeleteBranch(repoRoot, branch string) error {
	// Wait a bit to ensure git has released all locks
	time.Sleep(100 * time.Millisecond)

	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Could not delete worktree branch: %s", string(output))
	}
	return nil
}

// CountWorktreeChanges counts uncommitted files and commits since creation
func CountWorktreeChanges(worktreePath, originalHeadCommit string) (int, int, error) {
	// Count changed files
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to get git status: %w", err)
	}

	changedFiles := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			changedFiles++
		}
	}

	// Count commits since original head
	if originalHeadCommit == "" {
		// Cannot count commits without baseline - return unknown
		return changedFiles, 0, nil
	}

	cmd = exec.Command("git", "rev-list", "--count", originalHeadCommit+"..HEAD")
	cmd.Dir = worktreePath
	output, err = cmd.Output()
	if err != nil {
		return changedFiles, 0, fmt.Errorf("Failed to count commits: %w", err)
	}

	commits := 0
	if s := strings.TrimSpace(string(output)); s != "" {
		commits = parseIntSafe(s)
	}

	return changedFiles, commits, nil
}

// parseIntSafe safely parses an integer string
func parseIntSafe(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// CreateWorktreeForSession creates a worktree and switches the session into it
func CreateWorktreeForSession(sessionID, slug string, store *bootstrap.Store) (*WorktreeSession, error) {
	// Validate slug
	if err := ValidateWorktreeSlug(slug); err != nil {
		return nil, err
	}

	// Get current directory
	cwd := store.GetCWD()
	originalCwd := cwd

	// Find git root
	gitRoot := FindCanonicalGitRoot(cwd)
	if gitRoot == "" {
		return nil, fmt.Errorf("Cannot create a worktree: not in a git repository and no WorktreeCreate hooks are configured. Configure WorktreeCreate/WorktreeRemove hooks in settings.json to use worktree isolation with other VCS systems.")
	}

	// If in a worktree, change to the main repo root
	if gitRoot != cwd {
		if err := os.Chdir(gitRoot); err != nil {
			return nil, fmt.Errorf("Failed to change to main repo root: %w", err)
		}
		store.SetCWD(gitRoot)
		cwd = gitRoot
	}

	// Get current branch and commit
	originalBranch, _ := GetBranch()
	headCommit, err := GetHeadCommit()
	if err != nil {
		headCommit = ""
	}

	// Determine paths
	worktreePath := WorktreePathFor(gitRoot, slug)
	worktreeBranch := WorktreeBranchName(slug)

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists - resume it
		// Change into the worktree
		if err := os.Chdir(worktreePath); err != nil {
			return nil, fmt.Errorf("Failed to change to worktree: %w", err)
		}
		store.SetCWD(worktreePath)

		session := &WorktreeSession{
			OriginalCWD:        originalCwd,
			WorktreePath:       worktreePath,
			WorktreeName:       slug,
			WorktreeBranch:     worktreeBranch,
			OriginalBranch:     originalBranch,
			OriginalHeadCommit: headCommit,
			SessionID:          sessionID,
		}
		SetCurrentWorktreeSession(session)
		return session, nil
	}

	// Get base branch
	baseBranch, _ := GetDefaultBranch()

	// Create worktree
	if err := CreateWorktree(gitRoot, worktreePath, worktreeBranch, baseBranch); err != nil {
		return nil, err
	}

	// Change into the worktree
	if err := os.Chdir(worktreePath); err != nil {
		return nil, fmt.Errorf("Failed to change to worktree: %w", err)
	}
	store.SetCWD(worktreePath)

	session := &WorktreeSession{
		OriginalCWD:        originalCwd,
		WorktreePath:       worktreePath,
		WorktreeName:       slug,
		WorktreeBranch:     worktreeBranch,
		OriginalBranch:     originalBranch,
		OriginalHeadCommit: headCommit,
		SessionID:          sessionID,
	}

	SetCurrentWorktreeSession(session)

	return session, nil
}

// KeepWorktree leaves the worktree intact but exits the session
func KeepWorktree(store *bootstrap.Store) error {
	session := GetCurrentWorktreeSession()
	if session == nil {
		return nil
	}

	// Change back to original directory
	if err := os.Chdir(session.OriginalCWD); err != nil {
		return fmt.Errorf("Failed to change back to original directory: %w", err)
	}
	store.SetCWD(session.OriginalCWD)

	ClearCurrentWorktreeSession()

	return nil
}

// CleanupWorktree removes the worktree and cleans up
func CleanupWorktree(store *bootstrap.Store) error {
	session := GetCurrentWorktreeSession()
	if session == nil {
		return nil
	}

	// Change back to original directory first
	if err := os.Chdir(session.OriginalCWD); err != nil {
		return fmt.Errorf("Failed to change back to original directory: %w", err)
	}
	store.SetCWD(session.OriginalCWD)

	// Get git root for cleanup operations
	gitRoot := FindCanonicalGitRoot(session.OriginalCWD)
	if gitRoot == "" {
		gitRoot = session.OriginalCWD
	}

	// Remove the worktree
	if session.HookBased {
		// Hook-based worktree cleanup would be handled by hooks
		// For now, just clear the session
	} else {
		if err := RemoveWorktree(gitRoot, session.WorktreePath); err != nil {
			// Log error but continue cleanup
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}

		// Delete the worktree branch
		if session.WorktreeBranch != "" {
			if err := DeleteBranch(gitRoot, session.WorktreeBranch); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			}
		}
	}

	ClearCurrentWorktreeSession()

	return nil
}

// KillTmuxSession kills a tmux session by name
func KillTmuxSession(sessionName string) bool {
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run() == nil
}