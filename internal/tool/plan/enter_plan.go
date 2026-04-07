package plan

import (
	"context"
	"fmt"

	"claude-code-go/internal/tool"
)

// EnterPlanModeTool requests permission to enter plan mode for complex tasks
type EnterPlanModeTool struct{}

// EnterPlanModeTool creates a new EnterPlanModeTool
func CreateEnterPlanModeTool() *EnterPlanModeTool {
	return &EnterPlanModeTool{}
}

// Name returns the tool name
func (t *EnterPlanModeTool) Name() string {
	return EnterPlanModeToolName
}

// Description returns the tool description
func (t *EnterPlanModeTool) Description() string {
	return "Requests permission to enter plan mode for complex tasks requiring exploration and design"
}

// IsReadOnly returns true as this tool doesn't modify files
func (t *EnterPlanModeTool) IsReadOnly(_ tool.Input) bool {
	return true
}

// Call executes the EnterPlanMode tool
func (t *EnterPlanModeTool) Call(ctx context.Context, input tool.Input, rt tool.Runtime) (tool.Result, error) {
	// Check if this is being called in an agent context
	// Agent contexts cannot use EnterPlanMode
	// This would need to be determined by checking session flags or other state
	// For now, we allow it and check in the validation phase

	// Transition to plan mode
	// In the TS version, this calls handlePlanModeTransition and prepareContextForPlanMode
	// For Go, we need to set the permission mode to 'plan'

	result := tool.Result{
		Content: EnterPlanModeOutput{
			Message: "Entered plan mode. You should now focus on exploring the codebase and designing an implementation approach.",
		},
		Meta: map[string]any{
			"mode":          "plan",
			"needsApproval": true,
		},
	}

	return result, nil
}

// ParametersSchema returns the JSON schema for tool parameters
func (t *EnterPlanModeTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"strict":     true,
		"properties": map[string]any{},
	}
}

// EnterPlanModeInput represents the input for EnterPlanModeTool
// No parameters needed - just an empty input
type EnterPlanModeInput struct{}

// EnterPlanModeOutput represents the output of EnterPlanModeTool
type EnterPlanModeOutput struct {
	Message string `json:"message"`
}

// GetToolResultContent formats the tool result content for the model
func GetEnterPlanModeResultContent(message string) string {
	return fmt.Sprintf(`%s

In plan mode, you should:
1. Thoroughly explore the codebase using Glob, Grep, and Read tools
2. Understand existing patterns and architecture
3. Design an implementation approach
4. Present your plan to the user for approval
5. Use AskUserQuestion if you need to clarify the approach
6. Exit plan mode with ExitPlanMode when ready to implement

Remember: DO NOT write or edit any files yet. This is a read-only exploration and planning phase.`, message)
}

// Prompt returns the detailed prompt for the EnterPlanMode tool
func (t *EnterPlanModeTool) Prompt() string {
	return `Use this tool when a task has genuine ambiguity about the right approach and getting user input before coding would prevent significant rework. This tool transitions you into plan mode where you can explore the codebase and design an implementation approach for user approval.

## When to Use This Tool

Plan mode is valuable when the implementation approach is genuinely unclear. Use it when:

1. **Significant Architectural Ambiguity**: Multiple reasonable approaches exist and the choice meaningfully affects the codebase
   - Example: "Add caching to the API" - Redis vs in-memory vs file-based
   - Example: "Add real-time updates" - WebSockets vs SSE vs polling

2. **Unclear Requirements**: You need to explore and clarify before you can make progress
   - Example: "Make the app faster" - need to profile and identify bottlenecks
   - Example: "Refactor this module" - need to understand what the target architecture should be

3. **High-Impact Restructuring**: The task will significantly restructure existing code and getting buy-in first reduces risk
   - Example: "Redesign the authentication system"
   - Example: "Migrate from one state management approach to another"

## When NOT to Use This Tool

Skip plan mode when you can reasonably infer the right approach:
- The task is straightforward even if it touches multiple files
- The user's request is specific enough that the implementation path is clear
- You're adding a feature with an obvious implementation pattern (e.g., adding a button, a new endpoint following existing conventions)
- Bug fixes where the fix is clear once you understand the bug
- Research/exploration tasks (use the Agent tool instead)
- The user says something like "can we work on X" or "let's do X" - just get started

## Important Notes

- This tool REQUIRES user approval - they must consent to entering plan mode
`
}

// SearchHint returns the search hint for tool discovery
func (t *EnterPlanModeTool) SearchHint() string {
	return "switch to plan mode to design an approach before coding"
}