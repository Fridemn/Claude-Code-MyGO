package skills

import (
	"claude-go/internal/command"
)

// Register registers all skills commands with the registry
func Register(r *command.Registry) {
	registerSkills(r)
}