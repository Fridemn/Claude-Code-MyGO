package types

import (
	"time"

	"claude-code-go/internal/constants"
)

const (
	RoleSystem    = constants.RoleSystem
	RoleUser      = constants.RoleUser
	RoleAssistant = constants.RoleAssistant
	RoleTool      = "tool"
)

// Message represents a chat message
type Message struct {
	// UUID is a unique identifier for the message
	UUID string `json:"uuid,omitempty"`
	// Type is the message type (user, assistant, system, attachment, etc.)
	Type       string     `json:"type,omitempty"`
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Images     []string   `json:"images,omitempty"`       // Image URLs or base64 data
	Name       string     `json:"name,omitempty"`         // For tool/function messages
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // Tool calls in assistant messages
	ToolCallID string     `json:"tool_call_id,omitempty"` // Tool call ID for tool result messages
	Timestamp  time.Time  `json:"timestamp"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ToolDefinition defines a tool that can be called
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Conversation represents a conversation history
type Conversation struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

// Message creates a new message
func CreateMessage(role, content string) Message {
	return Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// UserMessage creates a new user message
func CreateUserMessage(content string) Message {
	return CreateMessage(RoleUser, content)
}

// AssistantMessage creates a new assistant message
func CreateAssistantMessage(content string) Message {
	return CreateMessage(RoleAssistant, content)
}

// SystemMessage creates a new system message
func CreateSystemMessage(content string) Message {
	return CreateMessage(RoleSystem, content)
}

// WithImages adds images to a message
func (m Message) WithImages(images ...string) Message {
	m.Images = images
	return m
}

// WithToolCalls adds tool calls to a message
func (m Message) WithToolCalls(calls ...ToolCall) Message {
	m.ToolCalls = calls
	return m
}

// IsToolResult returns true if this message is a tool result
func (m Message) IsToolResult() bool {
	return m.Role == RoleTool && m.ToolCallID != ""
}

// HasToolCalls returns true if this message contains tool calls
func (m Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// HasImages returns true if this message contains images
func (m Message) HasImages() bool {
	return len(m.Images) > 0
}
