package meta

import (
	"claude-code-go/internal/command"
)

// Register registers all meta commands with the registry
func Register(r *command.Registry) {
	registerMetaCommands(r)
}