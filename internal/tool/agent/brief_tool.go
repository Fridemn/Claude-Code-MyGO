package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"claude-code-go/internal/tool"
)

// Brief tool constants
const (
	BriefToolName       = "SendUserMessage"
	LegacyBriefToolName = "Brief"
)

// BriefDescription is the tool description
const BriefDescription = "Send a message to the user"

// BriefToolPrompt is the prompt for the Brief tool
const BriefToolPrompt = `Send a message the user will read. Text outside this tool is visible in the detail view, but most won't open it — the answer lives here.

` + "`message`" + ` supports markdown. ` + "`attachments`" + ` takes file paths (absolute or cwd-relative) for images, diffs, logs.

` + "`status`" + ` labels intent: 'normal' when replying to what they just asked; 'proactive' when you're initiating — a scheduled task finished, a blocker surfaced during background work, you need input on something they haven't asked about. Set it honestly; downstream routing uses it.`

// BriefTool implements the SendUserMessage/Brief tool
type BriefTool struct{}

// BriefTool creates a new Brief tool
func CreateBriefTool() *BriefTool {
	return &BriefTool{}
}

// Name returns the tool name
func (BriefTool) Name() string {
	return BriefToolName
}

// Description returns the tool description
func (BriefTool) Description() string {
	return BriefDescription
}

// IsReadOnly returns true since sending messages doesn't modify files
func (BriefTool) IsReadOnly(tool.Input) bool {
	return true
}

// ParametersSchema returns the JSON schema for the tool parameters
func (BriefTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"message": tool.SchemaString("The message for the user. Supports markdown formatting."),
		"attachments": tool.SchemaArray(
			"Optional file paths (absolute or relative to cwd) to attach. Use for photos, screenshots, diffs, logs, or any file the user should see alongside your message.",
			tool.SchemaString("File path"),
		),
		"status": tool.SchemaEnumString(
			"Use 'proactive' when you're surfacing something the user hasn't asked for and needs to see now — task completion while they're away, a blocker you hit, an unsolicited status update. Use 'normal' when replying to something the user just said.",
			"normal", "proactive",
		),
	}, "message", "status")
}

// BriefInput represents the parsed input
type BriefInput struct {
	Message     string
	Attachments []string
	Status      string
}

// BriefOutput represents the output
type BriefOutput struct {
	Message     string             `json:"message"`
	Attachments []BriefAttachment  `json:"attachments,omitempty"`
	SentAt      string             `json:"sent_at,omitempty"`
}

// BriefAttachment represents an attachment
type BriefAttachment struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsImage bool   `json:"is_image"`
}

// parseBriefInput extracts the input parameters
func parseBriefInput(in tool.Input) BriefInput {
	return BriefInput{
		Message:     tool.GetString(in, "message"),
		Attachments: tool.GetStringSlice(in, "attachments"),
		Status:      tool.GetString(in, "status"),
	}
}

// Call executes the Brief tool
func (BriefTool) Call(_ context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	input := parseBriefInput(in)

	// Validate required fields
	if strings.TrimSpace(input.Message) == "" {
		return tool.Result{}, fmt.Errorf("message is required")
	}

	// Default status to normal
	if input.Status == "" {
		input.Status = "normal"
	}

	// Validate status
	if input.Status != "normal" && input.Status != "proactive" {
		return tool.Result{}, fmt.Errorf("status must be 'normal' or 'proactive'")
	}

	output := BriefOutput{
		Message: input.Message,
		SentAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Handle attachments
	if len(input.Attachments) > 0 {
		output.Attachments = make([]BriefAttachment, 0, len(input.Attachments))
		for _, path := range input.Attachments {
			// In a real implementation, we would resolve the path and get file info
			output.Attachments = append(output.Attachments, BriefAttachment{
				Path:    path,
				IsImage: isImageFile(path),
			})
		}
	}

	return tool.Result{Content: output}, nil
}

// isImageFile checks if a file is an image based on extension
func isImageFile(path string) bool {
	ext := strings.ToLower(path)
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg"}
	for _, imgExt := range imageExts {
		if strings.HasSuffix(ext, imgExt) {
			return true
		}
	}
	return false
}

// GetBriefPrompt returns the prompt for the Brief tool
func GetBriefPrompt() string {
	return BriefToolPrompt
}

// GetBriefProactiveSection returns the proactive section for the system prompt
func GetBriefProactiveSection() string {
	return `## Talking to the user

` + BriefToolName + ` is where your replies go. Text outside it is visible if the user expands the detail view, but most won't — assume unread. Anything you want them to actually see goes through ` + BriefToolName + `. The failure mode: the real answer lives in plain text while ` + BriefToolName + ` just says "done!" — they see "done!" and miss everything.

So: every time the user says something, the reply they actually read comes through ` + BriefToolName + `. Even for "hi". Even for "thanks".

If you can answer right away, send the answer. If you need to go look — run a command, read files, check something — ack first in one line ("On it — checking the test output"), then work, then send the result. Without the ack they're staring at a spinner.

For longer work: ack → work → result. Between those, send a checkpoint when something useful happened — a decision you made, a surprise you hit, a phase boundary. Skip the filler ("running tests...") — a checkpoint earns its place by carrying information.

Keep messages tight — the decision, the file:line, the PR number. Second person always ("your config"), never third.`
}