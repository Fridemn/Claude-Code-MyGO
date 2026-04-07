package worktree

import "claude-code-go/internal/tool"

// RegisterWorktreeTools registers the EnterWorktree and ExitWorktree tools
func RegisterWorktreeTools(r *tool.Registry) {
	r.Register(EnterWorktreeTool{})
	r.Register(ExitWorktreeTool{})
}