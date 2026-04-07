package session

import (
	"claude-code-go/internal/command"
)

// Register registers all session commands with the registry
func Register(r *command.Registry) {
	registerSessionCommands(r)
}