package memory

import (
	"claude-code-go/internal/command"
)

// Register registers all memory commands with the registry
func Register(r *command.Registry) {
	registerMemory(r)
}