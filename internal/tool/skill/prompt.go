package skill

import "strings"

// SkillToolPrompt is the prompt for the Skill tool
const SkillToolPrompt = `Execute a skill within the main conversation

When users ask you to perform tasks, check if any of the available skills match. Skills provide specialized capabilities and domain knowledge.

When users reference a "slash command" or "/<something>" (e.g., "/commit", "/review-pr"), they are referring to a skill. Use this tool to invoke it.

How to invoke:
- Use this tool with the skill name and optional arguments
- Examples:
  - ` + "`skill: \"pdf\"`" + ` - invoke the pdf skill
  - ` + "`skill: \"commit\", args: \"-m 'Fix bug'\"`" + ` - invoke with arguments
  - ` + "`skill: \"review-pr\", args: \"123\"`" + ` - invoke with arguments
  - ` + "`skill: \"ms-office-suite:pdf\"`" + ` - invoke using fully qualified name

Important:
- Available skills are listed in system-reminder messages in the conversation
- When a skill matches the user's request, this is a BLOCKING REQUIREMENT: invoke the relevant Skill tool BEFORE generating any other response about the task
- NEVER mention a skill without actually calling this tool
- Do not invoke a skill that is already running
- Do not use this tool for built-in CLI commands (like /help, /clear, etc.)
- If you see a <command-name> tag in the current conversation turn, the skill has ALREADY been loaded - follow the instructions directly instead of calling this tool again
`

// GetPrompt returns the prompt for the Skill tool
func GetPrompt() string {
	return strings.TrimSpace(SkillToolPrompt)
}

// SkillBudgetContextPercent is the percentage of context window for skill listing
const SkillBudgetContextPercent = 0.01

// CharsPerToken is the approximate characters per token
const CharsPerToken = 4

// DefaultCharBudget is the fallback character budget
const DefaultCharBudget = 8000

// MaxListingDescChars is the max description length in listings
const MaxListingDescChars = 250

// GetCharBudget calculates the character budget for skill listing
func GetCharBudget(contextWindowTokens int) int {
	if contextWindowTokens > 0 {
		return int(float64(contextWindowTokens) * CharsPerToken * SkillBudgetContextPercent)
	}
	return DefaultCharBudget
}