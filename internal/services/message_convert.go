package services

import (
	"claude-code-go/internal/types"
)

// ConvertToCompactMessage converts types.Message to CompactMessage.
func ConvertToCompactMessage(msg types.Message) CompactMessage {
	result := CompactMessage{
		UUID:      msg.UUID,
		Type:      msg.Type,
		Role:      msg.Role,
		Content:   msg.Content,
		Images:    msg.Images,
		ToolUseID: msg.ToolCallID,
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		result.ToolCalls = make([]ToolCallContent, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			result.ToolCalls[i] = ToolCallContent{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
	}

	// Convert timestamp
	if !msg.Timestamp.IsZero() {
		result.Timestamp = msg.Timestamp.Format("2006-01-02T15:04:05.000Z")
	}

	// Set type based on role if not set
	if result.Type == "" {
		result.Type = msg.Role
	}

	return result
}

// ConvertToCompactMessages converts a slice of types.Message to CompactMessage.
func ConvertToCompactMessages(messages []types.Message) []CompactMessage {
	result := make([]CompactMessage, len(messages))
	for i, msg := range messages {
		result[i] = ConvertToCompactMessage(msg)
	}
	return result
}

// ConvertFromCompactMessage converts CompactMessage back to types.Message.
func ConvertFromCompactMessage(msg CompactMessage) types.Message {
	result := types.Message{
		UUID:      msg.UUID,
		Type:      msg.Type,
		Role:      msg.Role,
		Content:   msg.Content,
		Images:    msg.Images,
		ToolCallID: msg.ToolUseID,
	}

	// Convert tool calls
	if len(msg.ToolCalls) > 0 {
		result.ToolCalls = make([]types.ToolCall, len(msg.ToolCalls))
		for i, tc := range msg.ToolCalls {
			result.ToolCalls[i] = types.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
	}

	// Set role from type if not set
	if result.Role == "" {
		result.Role = msg.Type
	}

	return result
}

// ConvertFromCompactMessages converts a slice of CompactMessage back to types.Message.
func ConvertFromCompactMessages(messages []CompactMessage) []types.Message {
	result := make([]types.Message, len(messages))
	for i, msg := range messages {
		result[i] = ConvertFromCompactMessage(msg)
	}
	return result
}