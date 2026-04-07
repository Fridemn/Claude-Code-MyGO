package services

// Message grouping for compaction.
// Ported from src/services/compact/grouping.ts

// GroupMessagesByApiRound groups messages at API-round boundaries.
// Ported from src/services/compact/grouping.ts:groupMessagesByApiRound
//
// A boundary fires when a NEW assistant response begins (different message.id
// from the prior assistant). For well-formed conversations this is an API-safe
// split point — the API contract requires every tool_use to be resolved before
// the next assistant turn.
func GroupMessagesByApiRound(messages []CompactMessage) [][]CompactMessage {
	groups := make([][]CompactMessage, 0)
	current := make([]CompactMessage, 0)

	// Message ID of the most recently seen assistant
	// Streaming chunks from the same API response share an id
	var lastAssistantId string

	for _, msg := range messages {
		if msg.Type == MessageTypeAssistant && msg.MessageID != lastAssistantId && len(current) > 0 {
			groups = append(groups, current)
			current = []CompactMessage{msg}
		} else {
			current = append(current, msg)
		}
		if msg.Type == MessageTypeAssistant {
			lastAssistantId = msg.MessageID
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

// FindLastCompactBoundaryIndex finds the index of the last compact boundary.
// Used for reactive compact to find where to start summarizing from.
func FindLastCompactBoundaryIndex(messages []CompactMessage) int {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Type == MessageTypeSystem && stringsContains(msg.Content, "compact boundary") {
			return i
		}
		if msg.Type == MessageTypeUser && msg.IsCompactSummary {
			return i
		}
	}
	return -1
}

// GetMessagesAfterCompactBoundary returns messages after the last compact boundary.
// Ported from src/utils/messages.ts:getMessagesAfterCompactBoundary
func GetMessagesAfterCompactBoundary(messages []CompactMessage) []CompactMessage {
	idx := FindLastCompactBoundaryIndex(messages)
	if idx < 0 {
		return messages
	}
	return messages[idx+1:]
}

// IsCompactBoundaryMessage checks if a message is a compact boundary.
func IsCompactBoundaryMessage(msg CompactMessage) bool {
	if msg.Type == MessageTypeSystem && stringsContains(msg.Content, "compact boundary") {
		return true
	}
	return msg.Type == MessageTypeUser && msg.IsCompactSummary
}

// stringsContains is a helper to avoid importing strings for simple check
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}