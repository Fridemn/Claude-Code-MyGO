package integration

import (
	"claude-code-go/internal/command"
)

// Register registers all integration commands with the registry
func Register(r *command.Registry) {
	registerIntegrationCommands(r)
}