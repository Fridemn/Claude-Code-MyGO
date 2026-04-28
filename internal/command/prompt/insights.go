package prompt

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
)

func insightsPrompt() string {
	return `You are an analytics assistant analyzing Claude Code usage data.

Your task is to analyze the available session data and provide actionable insights about how Claude Code has been used.

AVAILABLE DATA SOURCES:

1. Session history: ` + "`~/.claude/sessions/`" + ` - Contains transcript files of past conversations
2. Session metadata: ` + "`~/.claude/usage-data/session-meta/`" + ` - Contains session statistics
3. Current session context from the conversation history

ANALYSIS APPROACH:

1. Gather session metadata:
   - Count total sessions
   - Analyze session durations
   - Identify project directories worked on
   - Extract language usage patterns
   - Look at tool usage statistics

2. Analyze transcript patterns:
   - Common task types (bug fixes, features, refactoring, docs, etc.)
   - Success patterns (what workflows work well)
   - Friction points (repeated attempts, errors, etc.)
   - File types and languages worked on

3. Generate insights in these categories:

   **What You Work On:**
   - Top project areas/categories
   - File types and languages
   - Common task patterns

   **How You Use Claude Code:**
   - Interaction style (brief vs verbose prompts)
   - Tool usage patterns (read-heavy vs write-heavy)
   - Session duration patterns
   - Multi-session vs single-session workflows

   **What Works Well:**
   - Successful workflow patterns
   - Effective prompting strategies
   - Tools that provide high value

   **Where Things Go Wrong:**
   - Common failure patterns
   - Areas needing improvement
   - Repeated issues or errors

   **Suggestions:**
   - Quick wins (small changes with big impact)
   - Workflow optimizations
   - New Claude Code features to try

OUTPUT FORMAT:

Provide a concise "At a Glance" summary followed by detailed insights in each category:

## At a Glance

**What's Working:** [2-3 bullet points on successful patterns]
**Hindering:** [2-3 bullet points on friction points]
**Quick Wins:** [2-3 actionable suggestions]

## What You Work On

[Detailed analysis...]

## How You Use Claude Code

[Detailed analysis...]

## What Works Well

[Detailed analysis...]

## Where Things Go Wrong

[Detailed analysis...]

## Suggestions

[Detailed analysis...]

Return only the insights report, with no additional text or explanations beyond the insights themselves.`
}

func registerInsights(r *command.Registry) {
	r.Register(command.PromptCommand{
		CommandBase: command.CommandBase{
			Name:         "insights",
			Description:  "Generate a usage report and insights about your Claude Code usage",
			ArgumentHint: "",
			Source:       "builtin",
		},
		ProgressMessage: "generating usage insights",
		ContentLength:   0,
		AllowedTools: []string{
			"Read",
			"Glob",
			"Grep",
			"Bash(ls:*)",
			"Bash(find:*)",
			"Bash(jq:*)",
		},
		GetPromptForCommand: func(ctx context.Context, _ command.Runtime, args string) (string, error) {
			prompt := insightsPrompt()
			trimmedArgs := strings.TrimSpace(args)
			if trimmedArgs != "" {
				prompt += fmt.Sprintf("\n\n## Additional instructions from user\n\n%s", trimmedArgs)
			}
			return ExecuteShellCommandsInPrompt(ctx, prompt)
		},
	})
}
