package stats

import (
	"claude-code-go/internal/command"
)

// Register registers all stats commands with the registry
func Register(r *command.Registry) {
	registerUsage(r)
	registerStats(r)
	registerCost(r)
	registerTools(r)
	registerTool(r)
	registerModel(r)
}