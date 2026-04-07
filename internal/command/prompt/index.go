package prompt

import (
	"claude-code-go/internal/command"
)

// Register registers all prompt commands with the registry
func Register(r *command.Registry) {
	registerReview(r)
	registerInit(r)
}