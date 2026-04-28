package prompt

import (
	"claude-go/internal/command"
)

// Register registers all prompt commands with the registry
func Register(r *command.Registry) {
	registerReview(r)
	registerInit(r)
	registerCommit(r)
	registerCommitPushPR(r)
	registerSecurityReview(r)
	registerPRComments(r)
	registerInsights(r)
}