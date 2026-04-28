package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Message types
const (
	MessageTypeUser       = "user"
	MessageTypeAssistant  = "assistant"
	MessageTypeSystem     = "system"
	MessageTypeTool       = "tool"
	MessageTypeProgress   = "progress"
	MessageTypeAttachment = "attachment"
)

// Role constants for convenience
const (
	RoleUser      = MessageTypeUser
	RoleAssistant = MessageTypeAssistant
	RoleSystem    = MessageTypeSystem
	RoleTool      = MessageTypeTool
)

// ContentBlock types
const (
	ContentTypeText             = "text"
	ContentTypeImage            = "image"
	ContentTypeToolUse          = "tool_use"
	ContentTypeToolResult       = "tool_result"
	ContentTypeThinking         = "thinking"
	ContentTypeRedactedThinking = "redacted_thinking"
	ContentTypeDocument         = "document"
)

// ContentBlock represents a content block in a message.
type ContentBlock struct {
	Type string `json:"type"`

	// For text blocks
	Text string `json:"text,omitempty"`

	// For image blocks
	Source *ImageSource `json:"source,omitempty"`

	// For tool_use blocks
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result blocks
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"` // string or []ContentBlock
	IsError   bool        `json:"is_error,omitempty"`

	// For thinking blocks
	Thinking string `json:"thinking,omitempty"`

	// For redacted_thinking blocks
	Data string `json:"data,omitempty"`
}

// ImageSource represents an image source.
type ImageSource struct {
	Type      string `json:"type"` // "base64" or "url"
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"` // base64 data
	URL       string `json:"url,omitempty"`  // URL for url type
}

// Message represents a chat message with full content block support.
type Message struct {
	UUID       string         `json:"uuid,omitempty"`
	Type       string         `json:"type,omitempty"`
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`        // Simple string content
	Blocks     []ContentBlock `json:"content_blocks,omitempty"` // Content blocks for API
	Images     []string       `json:"images,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`

	// Metadata
	IsMeta                    bool `json:"is_meta,omitempty"`
	IsVirtual                 bool `json:"is_virtual,omitempty"`
	IsCompactSummary          bool `json:"is_compact_summary,omitempty"`
	IsVisibleInTranscriptOnly bool `json:"is_visible_in_transcript_only,omitempty"`
	IsAPIErrorMessage         bool `json:"is_api_error_message,omitempty"`

	// For tool results
	ToolUseResult interface{} `json:"tool_use_result,omitempty"`

	// Request ID for API calls
	RequestID string `json:"request_id,omitempty"`

	// Model used (for assistant messages)
	Model string `json:"model,omitempty"`

	// Usage statistics
	Usage *Usage `json:"usage,omitempty"`

	// Stop reason
	StopReason string `json:"stop_reason,omitempty"`
}

// Usage represents token usage statistics.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ToolCall represents a tool call in a message.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	ToolCallID string      `json:"tool_call_id"`
	Content    interface{} `json:"content"` // string or []ContentBlock
	IsError    bool        `json:"is_error,omitempty"`
}

// ToolDefinition defines a tool that can be called.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Conversation represents a conversation history.
type Conversation struct {
	ID       string    `json:"id"`
	Messages []Message `json:"messages"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
}

// NewMessage creates a new message with the given role and content.
func NewMessage(role, content string) Message {
	return Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) Message {
	return NewMessage(MessageTypeUser, content)
}

// NewUserMessageWithBlocks creates a user message with content blocks.
func NewUserMessageWithBlocks(blocks []ContentBlock) Message {
	return Message{
		Type:      MessageTypeUser,
		Role:      MessageTypeUser,
		Blocks:    blocks,
		Timestamp: time.Now(),
	}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(content string) Message {
	return NewMessage(MessageTypeAssistant, content)
}

// NewAssistantMessageWithBlocks creates an assistant message with content blocks.
func NewAssistantMessageWithBlocks(blocks []ContentBlock) Message {
	return Message{
		Type:      MessageTypeAssistant,
		Role:      MessageTypeAssistant,
		Blocks:    blocks,
		Timestamp: time.Now(),
	}
}

// CreateSystemMessage creates a new system message.
func CreateSystemMessage(content string) Message {
	return NewMessage(MessageTypeSystem, content)
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(toolUseID string, content interface{}, isError bool) Message {
	return Message{
		Type:       MessageTypeUser,
		Role:       MessageTypeUser,
		ToolCallID: toolUseID,
		Content:    fmt.Sprintf("%v", content),
		Timestamp:  time.Now(),
		Blocks: []ContentBlock{
			{
				Type:      ContentTypeToolResult,
				ToolUseID: toolUseID,
				Content:   content,
				IsError:   isError,
			},
		},
	}
}

// NewToolUseBlock creates a tool_use content block.
func NewToolUseBlock(id, name string, input interface{}) ContentBlock {
	inputJSON, _ := json.Marshal(input)
	return ContentBlock{
		Type:  ContentTypeToolUse,
		ID:    id,
		Name:  name,
		Input: inputJSON,
	}
}

// NewTextBlock creates a text content block.
func NewTextBlock(text string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeText,
		Text: text,
	}
}

// NewImageBlock creates an image content block.
func NewImageBlock(mediaType, base64Data string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeImage,
		Source: &ImageSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      base64Data,
		},
	}
}

// NewToolResultBlock creates a tool_result content block.
func NewToolResultBlock(toolUseID string, content interface{}, isError bool) ContentBlock {
	return ContentBlock{
		Type:      ContentTypeToolResult,
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// GetContent returns the message content as a string.
func (m *Message) GetContent() string {
	if m.Content != "" {
		return m.Content
	}

	// Build content from blocks
	var result string
	for _, block := range m.Blocks {
		if block.Type == ContentTypeText {
			result += block.Text
		}
	}
	return result
}

// GetToolUseBlocks returns all tool_use blocks in the message.
func (m *Message) GetToolUseBlocks() []ContentBlock {
	var result []ContentBlock
	for _, block := range m.Blocks {
		if block.Type == ContentTypeToolUse {
			result = append(result, block)
		}
	}
	return result
}

// GetToolResultBlocks returns all tool_result blocks in the message.
func (m *Message) GetToolResultBlocks() []ContentBlock {
	var result []ContentBlock
	for _, block := range m.Blocks {
		if block.Type == ContentTypeToolResult {
			result = append(result, block)
		}
	}
	return result
}

// HasToolUse returns true if the message contains tool_use blocks.
func (m *Message) HasToolUse() bool {
	for _, block := range m.Blocks {
		if block.Type == ContentTypeToolUse {
			return true
		}
	}
	return false
}

// HasToolCalls returns true if the message has tool calls.
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// HasToolResult returns true if the message contains tool_result blocks.
func (m *Message) HasToolResult() bool {
	for _, block := range m.Blocks {
		if block.Type == ContentTypeToolResult {
			return true
		}
	}
	return false
}

// IsToolUseMessage returns true if this is a tool use request message.
func (m *Message) IsToolUseMessage() bool {
	return m.Type == MessageTypeAssistant && m.HasToolUse()
}

// IsToolResultMessage returns true if this is a tool result message.
func (m *Message) IsToolResultMessage() bool {
	return m.Type == MessageTypeUser && m.HasToolResult()
}

// Clone creates a deep copy of the message.
func (m *Message) Clone() Message {
	clone := *m
	if m.Blocks != nil {
		clone.Blocks = make([]ContentBlock, len(m.Blocks))
		copy(clone.Blocks, m.Blocks)
	}
	if m.Images != nil {
		clone.Images = make([]string, len(m.Images))
		copy(clone.Images, m.Images)
	}
	if m.ToolCalls != nil {
		clone.ToolCalls = make([]ToolCall, len(m.ToolCalls))
		copy(clone.ToolCalls, m.ToolCalls)
	}
	return clone
}

// WithUUID sets the UUID and returns the message.
func (m Message) WithUUID(uuid string) Message {
	m.UUID = uuid
	return m
}

// WithTimestamp sets the timestamp and returns the message.
func (m Message) WithTimestamp(t time.Time) Message {
	m.Timestamp = t
	return m
}

// WithMeta sets the IsMeta flag and returns the message.
func (m Message) WithMeta(isMeta bool) Message {
	m.IsMeta = isMeta
	return m
}

// WithBlocks sets the content blocks and returns the message.
func (m Message) WithBlocks(blocks []ContentBlock) Message {
	m.Blocks = blocks
	return m
}

// WithUsage sets the usage and returns the message.
func (m Message) WithUsage(usage *Usage) Message {
	m.Usage = usage
	return m
}

// NormalizeObjectRawMessage ensures raw JSON is non-empty, valid JSON.
// Invalid or empty payloads are normalized to "{}" so persistence cannot fail.
func NormalizeObjectRawMessage(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage(`{}`)
	}
	if !json.Valid(trimmed) {
		return json.RawMessage(`{}`)
	}
	normalized := make(json.RawMessage, len(trimmed))
	copy(normalized, trimmed)
	return normalized
}
