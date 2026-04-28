package agent

import (
	"claude-go/internal/command"
)

// Register registers all agent commands with the registry
func Register(r *command.Registry) {
	registerAgentCommands(r)
}