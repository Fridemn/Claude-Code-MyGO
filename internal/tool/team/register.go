package team

import "claude-code-go/internal/tool"

// RegisterTeamTools registers all team tools with the registry
func RegisterTeamTools(r *tool.Registry) {
	r.Register(TeamCreateTool{})
	r.Register(TeamDeleteTool{})
}