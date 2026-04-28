package prompt

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
)

// commitPrompt generates the prompt for the /commit command.
// This matches the TS implementation in src/commands/commit.ts
func commitPrompt() string {
	return `## Context

- Current git status: !` + "`git status`" + `
- Current git diff (staged and unstaged changes): !` + "`git diff HEAD`" + `
- Current branch: !` + "`git branch --show-current`" + `
- Recent commits: !` + "`git log --oneline -10`" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- CRITICAL: ALWAYS create NEW commits. NEVER use git commit --amend, unless the user explicitly requests it
- Do not commit files that likely contain secrets (.env, credentials.json, etc). Warn the user if they specifically request to commit those files
- If there are no changes to commit (i.e., no untracked files and no modifications), do not create an empty commit
- Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported

## Your task

Based on the above changes, create a single git commit:

1. Analyze all staged changes and draft a commit message:
   - Look at the recent commits above to follow this repository's commit message style
   - Summarize the nature of the changes (new feature, enhancement, bug fix, refactoring, test, docs, etc.)
   - Ensure the message accurately reflects the changes and their purpose (i.e. "add" means a wholly new feature, "update" means an enhancement to an existing feature, "fix" means a bug fix, etc.)
   - Draft a concise (1-2 sentences) commit message that focuses on the "why" rather than the "what"

2. Stage relevant files and create the commit using HEREDOC syntax:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `

You have the capability to call multiple tools in a single response. Stage and create the commit using a single message. Do not use any other tools or do anything else. Do not send any other text or messages besides these tool calls.`
}

func registerCommit(r *command.Registry) {
	r.Register(command.PromptCommand{
		CommandBase: command.CommandBase{
			Name:         "commit",
			Description:  "Create a git commit",
			ArgumentHint: "",
			Source:       "builtin",
		},
		ProgressMessage: "creating commit",
		ContentLength:   0,
		AllowedTools: []string{
			"Bash(git add:*)",
			"Bash(git status:*)",
			"Bash(git commit:*)",
		},
		GetPromptForCommand: func(ctx context.Context, _ command.Runtime, _ string) (string, error) {
			prompt := commitPrompt()
			return ExecuteShellCommandsInPrompt(ctx, prompt)
		},
	})
}

// commitPushPRPrompt generates the prompt for the /commit-push-pr command.
// This matches the TS implementation in src/commands/commit-push-pr.ts
func commitPushPRPrompt() string {
	return `## Context

- ` + "`git status`" + `: !` + "`git status`" + `
- ` + "`git diff HEAD`" + `: !` + "`git diff HEAD`" + `
- ` + "`git branch --show-current`" + `: !` + "`git branch --show-current`" + `
- ` + "`gh pr view --json number 2>/dev/null || true`" + `: !` + "`gh pr view --json number 2>/dev/null || true`" + `

## Git Safety Protocol

- NEVER update the git config
- NEVER run destructive/irreversible git commands (like push --force, hard reset, etc) unless the user explicitly requests them
- NEVER skip hooks (--no-verify, --no-gpg-sign, etc) unless the user explicitly requests it
- NEVER run force push to main/master, warn the user if they request it
- Do not commit files that likely contain secrets (.env, credentials.json, etc)
- Never use git commands with the -i flag (like git rebase -i or git add -i) since they require interactive input which is not supported

## Your task

Based on the above changes:
1. Create a single commit with an appropriate message using heredoc syntax:
` + "```" + `
git commit -m "$(cat <<'EOF'
Commit message here.
EOF
)"
` + "```" + `
2. Push the branch to origin
3. If a PR already exists for this branch (check the gh pr view output above), update the PR title and body using ` + "`gh pr edit`" + ` to reflect the current diff. Otherwise, create a pull request using ` + "`gh pr create`" + ` with heredoc syntax for the body.
   - IMPORTANT: Keep PR titles short (under 70 characters). Use the body for details.
` + "```" + `
gh pr create --title "Short, descriptive title" --body "$(cat <<'EOF'
## Summary
<1-3 bullet points>

## Test plan
[Bulleted markdown checklist of TODOs for testing the pull request...]
EOF
)"
` + "```" + `

You have the capability to call multiple tools in a single response. You MUST do all of the above in a single message.

Return the PR URL when you're done, so the user can see it.`
}

func registerCommitPushPR(r *command.Registry) {
	r.Register(command.PromptCommand{
		CommandBase: command.CommandBase{
			Name:         "commit-push-pr",
			Description:  "Commit, push, and open a PR",
			ArgumentHint: "",
			Source:       "builtin",
		},
		ProgressMessage: "creating commit and PR",
		ContentLength:   0,
		AllowedTools: []string{
			"Bash(git checkout --branch:*)",
			"Bash(git checkout -b:*)",
			"Bash(git add:*)",
			"Bash(git status:*)",
			"Bash(git push:*)",
			"Bash(git commit:*)",
			"Bash(gh pr create:*)",
			"Bash(gh pr edit:*)",
			"Bash(gh pr view:*)",
			"Bash(gh pr merge:*)",
		},
		GetPromptForCommand: func(ctx context.Context, _ command.Runtime, args string) (string, error) {
			prompt := commitPushPRPrompt()

			// Append user instructions if args provided
			trimmedArgs := strings.TrimSpace(args)
			if trimmedArgs != "" {
				prompt += fmt.Sprintf("\n\n## Additional instructions from user\n\n%s", trimmedArgs)
			}

			return ExecuteShellCommandsInPrompt(ctx, prompt)
		},
	})
}
