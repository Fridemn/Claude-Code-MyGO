package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// APIContentBlock represents a content block in API format.
type APIContentBlock struct {
	Type string `json:"type"`

	// For text
	Text string `json:"text,omitempty"`

	// For image
	Source *APIImageSource `json:"source,omitempty"`

	// For tool_use
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`

	// For tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// APIImageSource represents an image source for API.
type APIImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// APIMessage represents a message in API format.
type APIMessage struct {
	Role    string            `json:"role"`
	Content []APIContentBlock `json:"content"`
}

// ToAPIBlocks converts content blocks to API format.
func ToAPIBlocks(blocks []ContentBlock) []APIContentBlock {
	result := make([]APIContentBlock, 0, len(blocks))

	for _, block := range blocks {
		apiBlock := APIContentBlock{
			Type: block.Type,
		}

		switch block.Type {
		case ContentTypeText:
			apiBlock.Text = block.Text

		case ContentTypeImage:
			if block.Source != nil {
				apiBlock.Source = &APIImageSource{
					Type:      block.Source.Type,
					MediaType: block.Source.MediaType,
					Data:      block.Source.Data,
				}
			}

		case ContentTypeToolUse:
			apiBlock.ID = block.ID
			apiBlock.Name = block.Name
			apiBlock.Input = block.Input

		case ContentTypeToolResult:
			apiBlock.ToolUseID = block.ToolUseID
			apiBlock.IsError = block.IsError
			// Convert content
			switch c := block.Content.(type) {
			case string:
				apiBlock.Content = c
			case []ContentBlock:
				// Convert nested blocks to API format
				nested := make([]APIContentBlock, len(c))
				for i, nb := range c {
					nested[i] = APIContentBlock{
						Type: nb.Type,
						Text: nb.Text,
					}
				}
				apiBlock.Content = nested
			default:
				apiBlock.Content = fmt.Sprintf("%v", c)
			}

		case ContentTypeThinking:
			apiBlock.Text = block.Thinking

		case ContentTypeRedactedThinking:
			apiBlock.Text = block.Data
		}

		result = append(result, apiBlock)
	}

	return result
}

// ToAPIMessage converts a Message to API format.
func (m *Message) ToAPIMessage() APIMessage {
	content := m.Blocks
	if len(content) == 0 && m.Content != "" {
		content = []ContentBlock{NewTextBlock(m.Content)}
	}

	return APIMessage{
		Role:    m.Role,
		Content: ToAPIBlocks(content),
	}
}

// MessagesToAPI converts a slice of messages to API format.
func MessagesToAPI(messages []Message) []APIMessage {
	result := make([]APIMessage, 0, len(messages))

	for _, msg := range messages {
		// Skip meta and progress messages
		if msg.IsMeta || msg.Type == MessageTypeProgress || msg.Type == MessageTypeAttachment {
			continue
		}

		result = append(result, msg.ToAPIMessage())
	}

	return result
}

// FromAPIContentBlock converts an API content block to ContentBlock.
func FromAPIContentBlock(apiBlock APIContentBlock) ContentBlock {
	block := ContentBlock{
		Type: apiBlock.Type,
	}

	switch apiBlock.Type {
	case ContentTypeText:
		block.Text = apiBlock.Text

	case ContentTypeImage:
		if apiBlock.Source != nil {
			block.Source = &ImageSource{
				Type:      apiBlock.Source.Type,
				MediaType: apiBlock.Source.MediaType,
				Data:      apiBlock.Source.Data,
			}
		}

	case ContentTypeToolUse:
		block.ID = apiBlock.ID
		block.Name = apiBlock.Name
		block.Input = apiBlock.Input

	case ContentTypeToolResult:
		block.ToolUseID = apiBlock.ToolUseID
		block.IsError = apiBlock.IsError
		block.Content = apiBlock.Content
	}

	return block
}

// FromAPIMessage converts an API message to Message.
func FromAPIMessage(apiMsg APIMessage) Message {
	blocks := make([]ContentBlock, 0, len(apiMsg.Content))
	for _, apiBlock := range apiMsg.Content {
		blocks = append(blocks, FromAPIContentBlock(apiBlock))
	}

	return Message{
		Role:    apiMsg.Role,
		Type:    apiMsg.Role,
		Blocks:  blocks,
	}
}

// ReadImageFileToBlock reads an image file and creates an image content block.
func ReadImageFileToBlock(path string) (ContentBlock, error) {
	// This is a placeholder - actual implementation would read the file
	ext := strings.ToLower(filepath.Ext(path))
	mediaType := "image/png"

	switch ext {
	case ".jpg", ".jpeg":
		mediaType = "image/jpeg"
	case ".gif":
		mediaType = "image/gif"
	case ".webp":
		mediaType = "image/webp"
	}

	// Placeholder - in real implementation, read file and encode to base64
	return ContentBlock{
		Type: ContentTypeImage,
		Source: &ImageSource{
			Type:      "base64",
			MediaType: mediaType,
		},
	}, nil
}

// EncodeImageToBlock encodes image data to a content block.
func EncodeImageToBlock(data []byte, mediaType string) ContentBlock {
	return ContentBlock{
		Type: ContentTypeImage,
		Source: &ImageSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      base64.StdEncoding.EncodeToString(data),
		},
	}
}

// DecodeImageFromBlock decodes image data from a content block.
func DecodeImageFromBlock(block ContentBlock) ([]byte, error) {
	if block.Type != ContentTypeImage || block.Source == nil {
		return nil, fmt.Errorf("not an image block")
	}

	if block.Source.Type != "base64" {
		return nil, fmt.Errorf("unsupported image source type: %s", block.Source.Type)
	}

	return base64.StdEncoding.DecodeString(block.Source.Data)
}

// StripImagesFromMessages removes image blocks from messages.
func StripImagesFromMessages(messages []Message) []Message {
	result := make([]Message, len(messages))

	for i, msg := range messages {
		result[i] = msg.Clone()
		if len(result[i].Blocks) > 0 {
			filtered := make([]ContentBlock, 0, len(result[i].Blocks))
			for _, block := range result[i].Blocks {
				if block.Type != ContentTypeImage {
					filtered = append(filtered, block)
				}
			}
			result[i].Blocks = filtered
		}
	}

	return result
}

// StripDocumentsFromMessages removes document blocks from messages.
func StripDocumentsFromMessages(messages []Message) []Message {
	result := make([]Message, len(messages))

	for i, msg := range messages {
		result[i] = msg.Clone()
		if len(result[i].Blocks) > 0 {
			filtered := make([]ContentBlock, 0, len(result[i].Blocks))
			for _, block := range result[i].Blocks {
				if block.Type != ContentTypeDocument {
					filtered = append(filtered, block)
				}
			}
			result[i].Blocks = filtered
		}
	}

	return result
}

// MergeMessages combines consecutive messages of the same role.
func MergeMessages(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	result := make([]Message, 0, len(messages))
	current := messages[0].Clone()

	for i := 1; i < len(messages); i++ {
		msg := messages[i]

		if msg.Role == current.Role && msg.Type != MessageTypeTool && current.Type != MessageTypeTool {
			// Merge content
			if msg.Content != "" {
				if current.Content != "" {
					current.Content += "\n" + msg.Content
				} else {
					current.Content = msg.Content
				}
			}
			// Merge blocks
			current.Blocks = append(current.Blocks, msg.Blocks...)
		} else {
			result = append(result, current)
			current = msg.Clone()
		}
	}

	result = append(result, current)
	return result
}

// TruncateMessages truncates messages to fit within a token limit.
func TruncateMessages(messages []Message, maxTokens int, estimateTokens func(string) int) []Message {
	if maxTokens <= 0 || len(messages) == 0 {
		return messages
	}

	// Calculate total tokens
	totalTokens := 0
	for _, msg := range messages {
		for _, block := range msg.Blocks {
			if block.Type == ContentTypeText {
				totalTokens += estimateTokens(block.Text)
			}
		}
		if msg.Content != "" {
			totalTokens += estimateTokens(msg.Content)
		}
	}

	if totalTokens <= maxTokens {
		return messages
	}

	// Truncate from the beginning, keeping recent messages
	result := make([]Message, 0, len(messages))
	currentTokens := 0

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		msgTokens := 0
		for _, block := range msg.Blocks {
			if block.Type == ContentTypeText {
				msgTokens += estimateTokens(block.Text)
			}
		}
		if msg.Content != "" {
			msgTokens += estimateTokens(msg.Content)
		}

		if currentTokens+msgTokens > maxTokens {
			break
		}

		currentTokens += msgTokens
		result = append([]Message{msg}, result...)
	}

	return result
}

// GetLastAssistantMessage returns the last assistant message.
func GetLastAssistantMessage(messages []Message) *Message {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Type == MessageTypeAssistant {
			return &messages[i]
		}
	}
	return nil
}

// HasToolCallsInLastAssistantTurn checks if the last assistant message has tool calls.
func HasToolCallsInLastAssistantTurn(messages []Message) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Type == MessageTypeAssistant {
			return messages[i].HasToolUse()
		}
	}
	return false
}