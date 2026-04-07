package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"claude-code-go/internal/tool"
)

// ExitPlanModeTool prompts the user to exit plan mode and start coding
type ExitPlanModeTool struct {
	mu              sync.RWMutex
	hasExitedPlan   bool
	needsAttachment bool
	planSlugs       map[string]string // sessionID -> planSlug
}

// ExitPlanModeTool creates a new ExitPlanModeTool
func CreateExitPlanModeTool() *ExitPlanModeTool {
	return &ExitPlanModeTool{
		planSlugs: make(map[string]string),
	}
}

// Name returns the tool name
func (t *ExitPlanModeTool) Name() string {
	return ExitPlanModeToolName
}

// Description returns the tool description
func (t *ExitPlanModeTool) Description() string {
	return "Prompts the user to exit plan mode and start coding"
}

// IsReadOnly returns false as this tool writes to disk
func (t *ExitPlanModeTool) IsReadOnly(_ tool.Input) bool {
	return false
}

// ParametersSchema returns the JSON schema for tool parameters
func (t *ExitPlanModeTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"allowedPrompts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool": map[string]any{
							"type":        "string",
							"enum":        []string{"Bash"},
							"description": "The tool this prompt applies to",
						},
						"prompt": map[string]any{
							"type":        "string",
							"description": "Semantic description of the action, e.g. 'run tests', 'install dependencies'",
						},
					},
					"required": []string{"tool", "prompt"},
				},
				"description": "Prompt-based permissions needed to implement the plan. These describe categories of actions rather than specific commands.",
			},
			"plan": map[string]any{
				"type":        "string",
				"description": "The plan content (injected from disk)",
			},
			"planFilePath": map[string]any{
				"type":        "string",
				"description": "The plan file path",
			},
		},
	}
}

// ExitPlanModeInput represents the input for ExitPlanModeTool
type ExitPlanModeInput struct {
	AllowedPrompts []AllowedPrompt `json:"allowedPrompts,omitempty"`
	Plan           string          `json:"plan,omitempty"`
	PlanFilePath   string          `json:"planFilePath,omitempty"`
}

// AllowedPrompt represents a prompt-based permission
type AllowedPrompt struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

// ExitPlanModeOutput represents the output of ExitPlanModeTool
type ExitPlanModeOutput struct {
	Plan                   string `json:"plan"`
	IsAgent                bool   `json:"isAgent"`
	FilePath               string `json:"filePath,omitempty"`
	HasTaskTool            bool   `json:"hasTaskTool,omitempty"`
	PlanWasEdited          bool   `json:"planWasEdited,omitempty"`
	AwaitingLeaderApproval bool   `json:"awaitingLeaderApproval,omitempty"`
	RequestID              string `json:"requestId,omitempty"`
}

// Call executes the ExitPlanMode tool
func (t *ExitPlanModeTool) Call(ctx context.Context, input tool.Input, rt tool.Runtime) (tool.Result, error) {
	// Parse input
	var in ExitPlanModeInput
	if data, err := json.Marshal(input); err == nil {
		json.Unmarshal(data, &in)
	}

	// Determine if this is an agent context
	isAgent := rt.Store != nil && rt.Store.Snapshot().SessionFlags != nil && rt.Store.Snapshot().SessionFlags["is_agent"]

	// Get plan file path
	filePath := t.getPlanFilePath(rt)

	// Get plan content - either from input or from disk
	plan := in.Plan
	if plan == "" {
		plan = t.getPlan(rt)
	}

	// If plan was provided via input, write it to disk
	if in.Plan != "" && filePath != "" {
		if err := os.WriteFile(filePath, []byte(in.Plan), 0644); err == nil {
			// Successfully wrote plan to disk
		}
	}

	// Mark that we've exited plan mode
	t.mu.Lock()
	t.hasExitedPlan = true
	t.needsAttachment = true
	t.mu.Unlock()

	// Build result
	output := ExitPlanModeOutput{
		Plan:          plan,
		IsAgent:       isAgent,
		FilePath:      filePath,
		PlanWasEdited: in.Plan != "",
	}

	return tool.Result{
		Content: output,
		Meta: map[string]any{
			"mode":            "default",
			"planWasEdited":   in.Plan != "",
			"hasExitedPlan":   true,
			"needsAttachment": true,
		},
	}, nil
}

// getPlanFilePath returns the path to the plan file
func (t *ExitPlanModeTool) getPlanFilePath(rt tool.Runtime) string {
	if rt.Store == nil {
		return ""
	}

	state := rt.Store.Snapshot()
	sessionID := state.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Get or create plan slug
	t.mu.Lock()
	slug, ok := t.planSlugs[sessionID]
	if !ok {
		slug = generateWordSlug()
		t.planSlugs[sessionID] = slug
	}
	t.mu.Unlock()

	// Determine plans directory
	plansDir := getPlansDirectory()

	return filepath.Join(plansDir, slug+".md")
}

// getPlan reads the plan from disk
func (t *ExitPlanModeTool) getPlan(rt tool.Runtime) string {
	filePath := t.getPlanFilePath(rt)
	if filePath == "" {
		return ""
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(data)
}

// HasExitedPlanMode returns whether plan mode has been exited
func (t *ExitPlanModeTool) HasExitedPlanMode() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.hasExitedPlan
}

// NeedsPlanModeExitAttachment returns whether we need to show exit attachment
func (t *ExitPlanModeTool) NeedsPlanModeExitAttachment() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.needsAttachment
}

// SetNeedsPlanModeExitAttachment sets the needs attachment flag
func (t *ExitPlanModeTool) SetNeedsPlanModeExitAttachment(v bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.needsAttachment = v
}

// GetToolResultContent formats the tool result content for the model
func GetExitPlanModeResultContent(output ExitPlanModeOutput) string {
	// Handle awaiting leader approval
	if output.AwaitingLeaderApproval {
		return fmt.Sprintf(`Your plan has been submitted to the team lead for approval.

Plan file: %s

**What happens next:**
1. Wait for the team lead to review your plan
2. You will receive a message in your inbox with approval/rejection
3. If approved, you can proceed with implementation
4. If rejected, refine your plan based on the feedback

**Important:** Do NOT proceed until you receive approval. Check your inbox for response.

Request ID: %s`, output.FilePath, output.RequestID)
	}

	// Handle agent context
	if output.IsAgent {
		return "User has approved the plan. There is nothing else needed from you now. Please respond with \"ok\""
	}

	// Handle empty plan
	if output.Plan == "" || strings.TrimSpace(output.Plan) == "" {
		return "User has approved exiting plan mode. You can now proceed."
	}

	// Label edited plans
	planLabel := "Approved Plan"
	if output.PlanWasEdited {
		planLabel = "Approved Plan (edited by user)"
	}

	teamHint := ""
	if output.HasTaskTool {
		teamHint = `

If this plan can be broken down into multiple independent tasks, consider using the TeamCreate tool to create a team and parallelize the work.`
	}

	return fmt.Sprintf(`User has approved your plan. You can now start coding. Start with updating your todo list if applicable

Your plan has been saved to: %s
You can refer back to it if needed during implementation.%s

## %s:
%s`, output.FilePath, teamHint, planLabel, output.Plan)
}

// Prompt returns the detailed prompt for the ExitPlanMode tool
func (t *ExitPlanModeTool) Prompt() string {
	return `Use this tool when you are in plan mode and have finished writing your plan to the plan file and are ready for user approval.

## How This Tool Works
- You should have already written your plan to the plan file specified in the plan mode system message
- This tool does NOT take the plan content as a parameter - it will read the plan from the file you wrote
- This tool simply signals that you're done planning and ready for the user to review and approve
- The user will see the contents of your plan file when they review it

## When to Use This Tool
IMPORTANT: Only use this tool when the task requires planning the implementation steps of a task that requires writing code. For research tasks where you're gathering information, searching files, reading files or in general trying to understand the codebase - do NOT use this tool.

## Before Using This Tool
Ensure your plan is complete and unambiguous:
- If you have unresolved questions about requirements or approach, use AskUserQuestion first (in earlier phases)
- Once your plan is finalized, use THIS tool to request approval

**Important:** Do NOT use AskUserQuestion to ask "Is this plan okay?" or "Should I proceed?" - that's exactly what THIS tool does. ExitPlanMode inherently requests user approval of your plan.

## Examples

1. Initial task: "Search for and understand the implementation of vim mode in the codebase" - Do not use the exit plan mode tool because you are not planning the implementation steps of a task.
2. Initial task: "Help me implement yank mode for vim" - Use the exit plan mode tool after you have finished planning the implementation steps of the task.
3. Initial task: "Add a new feature to handle user authentication" - If unsure about auth method (OAuth, JWT, etc.), use AskUserQuestion first, then use exit plan mode tool after clarifying the approach.
`
}

// SearchHint returns the search hint for tool discovery
func (t *ExitPlanModeTool) SearchHint() string {
	return "present plan for approval and start coding (plan mode only)"
}

// RequiresUserInteraction returns true if this tool requires user interaction
func (t *ExitPlanModeTool) RequiresUserInteraction() bool {
	return true
}

// ValidateInput validates the tool input before execution
func (t *ExitPlanModeTool) ValidateInput(input tool.Input, rt tool.Runtime) error {
	// Check if we're in plan mode - if not, reject
	// In a full implementation, this would check the current permission mode
	return nil
}

// getPlansDirectory returns the directory where plan files are stored
func getPlansDirectory() string {
	// Use user's home directory for plans
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	plansDir := filepath.Join(homeDir, ".claude", "plans")

	// Ensure directory exists
	os.MkdirAll(plansDir, 0755)

	return plansDir
}

// generateWordSlug generates a random word-based slug for plan files
func generateWordSlug() string {
	adjectives := []string{
		"quick", "lazy", "happy", "sad", "bright", "dark", "calm", "wild",
		"gentle", "strong", "swift", "slow", "warm", "cool", "fresh", "clean",
		"bold", "smart", "clever", "wise", "keen", "sharp", "fine", "great",
	}

	nouns := []string{
		"fox", "dog", "cat", "bird", "bear", "wolf", "deer", "fish",
		"tree", "rock", "river", "mountain", "valley", "forest", "meadow", "ocean",
		"star", "moon", "sun", "cloud", "storm", "wind", "rain", "snow",
	}

	// Use current time for randomness
	now := time.Now().UnixNano()
	adjIdx := int(now) % len(adjectives)
	nounIdx := (int(now) / 1000) % len(nouns)

	return adjectives[adjIdx] + "-" + nouns[nounIdx]
}
