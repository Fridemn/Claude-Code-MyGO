package agent

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/tool"
)

// SendMessage tool constants
const (
	SendMessageToolName = "SendMessage"
)

// SendMessageDescription is the tool description
const SendMessageDescription = "Send a message to another agent"

// SendMessageTool implements the SendMessage tool for agent communication
type SendMessageTool struct{}

// SendMessageTool creates a new SendMessage tool
func CreateSendMessageTool() *SendMessageTool {
	return &SendMessageTool{}
}

// Name returns the tool name
func (SendMessageTool) Name() string {
	return SendMessageToolName
}

// Description returns the tool description
func (SendMessageTool) Description() string {
	return SendMessageDescription
}

// IsReadOnly returns true since sending messages doesn't modify files
func (SendMessageTool) IsReadOnly(tool.Input) bool {
	return true
}

// ParametersSchema returns the JSON schema for the tool parameters
func (SendMessageTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"to": tool.SchemaString("Recipient: teammate name, or \"*\" for broadcast to all teammates"),
		"summary": tool.SchemaString("A 5-10 word summary shown as a preview in the UI (required when message is a string)"),
		"message": tool.SchemaUnion(
			"Plain text message content or structured message",
			tool.SchemaString("Plain text message content"),
			tool.SchemaObject(map[string]any{
				"type": tool.SchemaEnumString("Message type", "shutdown_request", "shutdown_response", "plan_approval_response"),
				"request_id": tool.SchemaString("Request ID for response messages"),
				"approve":    tool.SchemaBoolean("Whether to approve the request"),
				"reason":     tool.SchemaString("Reason for the decision"),
				"feedback":   tool.SchemaString("Feedback for plan rejection"),
			}),
		),
	}, "to", "message")
}

// SendMessageInput represents the parsed input
type SendMessageInput struct {
	To      string
	Summary string
	Message interface{} // string or StructuredMessage
}

// StructuredMessage represents a structured message
type StructuredMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Approve   bool   `json:"approve,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Feedback  string `json:"feedback,omitempty"`
}

// MessageRouting contains routing information
type MessageRouting struct {
	Sender      string `json:"sender,omitempty"`
	SenderColor string `json:"sender_color,omitempty"`
	Target      string `json:"target,omitempty"`
	TargetColor string `json:"target_color,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Content     string `json:"content,omitempty"`
}

// MessageOutput represents the output for a simple message
type MessageOutput struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Routing *MessageRouting `json:"routing,omitempty"`
}

// BroadcastOutput represents the output for a broadcast message
type BroadcastOutput struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	Recipients []string        `json:"recipients,omitempty"`
	Routing    *MessageRouting `json:"routing,omitempty"`
}

// RequestOutput represents the output for a request message
type RequestOutput struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Target    string `json:"target,omitempty"`
}

// parseSendMessageInput extracts the input parameters
func parseSendMessageInput(in tool.Input) SendMessageInput {
	var message interface{}
	if m, ok := in["message"]; ok {
		message = m
	}
	return SendMessageInput{
		To:      tool.GetString(in, "to"),
		Summary: tool.GetString(in, "summary"),
		Message: message,
	}
}

// Call executes the SendMessage tool
func (SendMessageTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	input := parseSendMessageInput(in)

	// Validate required fields
	if strings.TrimSpace(input.To) == "" {
		return tool.Result{}, fmt.Errorf("'to' must not be empty")
	}

	// Handle different message types
	switch msg := input.Message.(type) {
	case string:
		// Plain text message
		if input.To == "*" {
			// Broadcast
			return handleBroadcast(msg, input.Summary)
		}
		return handleMessage(input.To, msg, input.Summary, runtime)
	case map[string]interface{}:
		// Structured message
		return handleStructuredMessage(input.To, msg, runtime)
	default:
		return tool.Result{}, fmt.Errorf("invalid message type: expected string or object")
	}
}

// handleMessage handles a simple message to a teammate
func handleMessage(to, content, summary string, runtime tool.Runtime) (tool.Result, error) {
	// If we have a ContinueAgent function, try to continue an existing agent
	if runtime.ContinueAgent != nil {
		_, err := runtime.ContinueAgent(context.Background(), to, content, false)
		if err != nil {
			return tool.Result{
				Content: MessageOutput{
					Success: false,
					Message: fmt.Sprintf("Failed to deliver message to %s: %v", to, err),
				},
			}, nil
		}
	}

	return tool.Result{
		Content: MessageOutput{
			Success: true,
			Message: fmt.Sprintf("Message sent to %s", to),
			Routing: &MessageRouting{
				Target:  "@" + to,
				Summary: summary,
				Content: content,
			},
		},
	}, nil
}

// handleBroadcast handles a broadcast message to all teammates
func handleBroadcast(content, summary string) (tool.Result, error) {
	return tool.Result{
		Content: BroadcastOutput{
			Success: true,
			Message: "Message broadcast to all teammates",
			Routing: &MessageRouting{
				Target:  "@team",
				Summary: summary,
				Content: content,
			},
		},
	}, nil
}

// handleStructuredMessage handles structured messages
func handleStructuredMessage(to string, msg map[string]interface{}, runtime tool.Runtime) (tool.Result, error) {
	msgType, _ := msg["type"].(string)

	switch msgType {
	case "shutdown_request":
		return tool.Result{
			Content: RequestOutput{
				Success: true,
				Message: fmt.Sprintf("Shutdown request sent to %s", to),
				Target:  to,
			},
		}, nil

	case "shutdown_response":
		requestID, _ := msg["request_id"].(string)
		approve, _ := msg["approve"].(bool)
		reason, _ := msg["reason"].(string)

		// If not approved, require a reason
		if !approve && reason == "" {
			return tool.Result{}, fmt.Errorf("reason is required when rejecting a shutdown request")
		}

		status := "approved"
		if !approve {
			status = "rejected"
		}

		return tool.Result{
			Content: RequestOutput{
				Success:   true,
				Message:   fmt.Sprintf("Shutdown %s for request %s", status, requestID),
				RequestID: requestID,
			},
		}, nil

	case "plan_approval_response":
		requestID, _ := msg["request_id"].(string)
		approve, _ := msg["approve"].(bool)
		feedback, _ := msg["feedback"].(string)

		status := "approved"
		if !approve {
			status = "rejected"
		}

		return tool.Result{
			Content: RequestOutput{
				Success:   true,
				Message:   fmt.Sprintf("Plan %s for %s. %s", status, to, feedback),
				RequestID: requestID,
				Target:    to,
			},
		}, nil

	default:
		return tool.Result{}, fmt.Errorf("unknown message type: %s", msgType)
	}
}

// GetPrompt returns the prompt for the SendMessage tool
func GetSendMessagePrompt() string {
	return strings.TrimSpace(`
# SendMessage

Send a message to another agent.

` + "```json" + `
{"to": "researcher", "summary": "assign task 1", "message": "start on task #1"}
` + "```" + `

| ` + "`to`" + ` | |
|---|---|
| ` + "`\"researcher\"`" + ` | Teammate by name |
| ` + "`\"*\"`" + ` | Broadcast to all teammates — expensive (linear in team size), use only when everyone genuinely needs it |

Your plain text output is NOT visible to other agents — to communicate, you MUST call this tool. Messages from teammates are delivered automatically; you don't check an inbox. Refer to teammates by name, never by UUID. When relaying, don't quote the original — it's already rendered to the user.

## Protocol responses

If you receive a JSON message with ` + "`type: \"shutdown_request\"`" + ` or ` + "`type: \"plan_approval_request\"`" + `, respond with the matching ` + "`_response`" + ` type — echo the ` + "`request_id`" + `, set ` + "`approve`" + ` true/false:

` + "```json" + `
{"to": "team-lead", "message": {"type": "shutdown_response", "request_id": "...", "approve": true}}
{"to": "researcher", "message": {"type": "plan_approval_response", "request_id": "...", "approve": false, "feedback": "add error handling"}}
` + "```" + `

Approving shutdown terminates your process. Rejecting plan sends the teammate back to revise. Don't originate ` + "`shutdown_request`" + ` unless asked. Don't send structured JSON status messages — use TaskUpdate.
`)
}