package integration

import (
	"claude-go/internal/command"
)

// Register registers all integration commands with the registry
func Register(r *command.Registry) {
	registerIntegrationCommands(r)
}