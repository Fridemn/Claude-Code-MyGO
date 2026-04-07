package skills

import (
	"claude-code-go/internal/command"
)

// Register registers all skills commands with the registry
func Register(r *command.Registry) {
	registerSkills(r)
}