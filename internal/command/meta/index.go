package meta

import (
	"claude-go/internal/command"
)

// Register registers all meta commands with the registry
func Register(r *command.Registry) {
	registerMetaCommands(r)
}