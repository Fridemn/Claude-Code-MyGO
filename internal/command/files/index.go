package files

import (
	"claude-code-go/internal/command"
)

// Register registers all file commands with the registry
func Register(r *command.Registry) {
	registerFiles(r)
	registerRead(r)
	registerGrep(r)
}