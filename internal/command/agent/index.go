package agent

import (
	"claude-code-go/internal/command"
)

// Register registers all agent commands with the registry
func Register(r *command.Registry) {
	registerAgentCommands(r)
}