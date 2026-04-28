package dev

import (
	"claude-go/internal/command"
)

// Register registers all dev commands with the registry
func Register(r *command.Registry) {
	registerDoctor(r)
	registerDiff(r)
}
