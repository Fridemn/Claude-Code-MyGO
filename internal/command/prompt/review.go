package prompt

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
)

func registerReview(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindPrompt,
		Name:         "review",
		Description:  "ask the model for a code review-style pass",
		ArgumentHint: "<topic>",
		Handler: func(_ context.Context, _ command.Runtime, args []string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("review", "<topic>"))
			}
			return "Review the following with a code review mindset. Prioritize bugs, regressions, risks, and missing tests.\n\nTarget:\n" + strings.Join(args, " "), nil
		},
	})
}

func registerInit(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindPrompt,
		Name:         "init",
		Description:  "ask the model to initialize project understanding",
		ArgumentHint: "[goal]",
		Handler: func(_ context.Context, _ command.Runtime, args []string) (string, error) {
			goal := strings.TrimSpace(strings.Join(args, " "))
			if goal == "" {
				goal = "Understand this repository, identify the main modules, and propose the first implementation steps."
			}
			return goal, nil
		},
	})
}