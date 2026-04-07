package dev

import (
	"claude-code-go/internal/command"
)

// Register registers all dev commands with the registry
func Register(r *command.Registry) {
	registerDoctor(r)
	registerDiff(r)
}
