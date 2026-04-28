package types

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DeriveShortMessageID derives a short stable message ID from a UUID.
// Returns a 6-character base36 string.
// Ported from src/utils/messages.ts:deriveShortMessageId
func DeriveShortMessageID(uuid string) string {
	// Take first 10 hex chars from the UUID (skipping dashes)
	hex := strings.ReplaceAll(uuid, "-", "")
	if len(hex) > 10 {
		hex = hex[:10]
	}

	// Convert to base36 for shorter representation
	num := 0
	for _, c := range hex {
		num = num*16 + hexCharToVal(c)
	}

	// Convert to base36
	base36 := intToBase36(num)
	if len(base36) > 6 {
		return base36[:6]
	}
	return base36
}

func hexCharToVal(c rune) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c-'a') + 10
	}
	if c >= 'A' && c <= 'F' {
		return int(c-'A') + 10
	}
	return 0
}

func intToBase36(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var result []byte
	for n > 0 {
		result = append([]byte{digits[n%36]}, result...)
		n /= 36
	}
	return string(result)
}

// DeriveUUID derives a deterministic UUID from a parent UUID and index.
// Ported from src/utils/messages.ts:deriveUUID
func DeriveUUID(parentUUID string, index int) string {
	// Take first 24 chars of parent UUID
	hex := strings.ReplaceAll(parentUUID, "-", "")
	if len(hex) > 24 {
		hex = hex[:24]
	}

	// Convert index to hex (12 chars)
	indexHex := fmt.Sprintf("%012x", index)

	return hex + indexHex
}

// GenerateUUID generates a new UUID v4.
func GenerateUUID() string {
	b := make([]byte, 16)
	// Use timestamp-based random
	t := time.Now().UnixNano()
	for i := 0; i < 16; i++ {
		b[i] = byte(t >> (i * 8))
	}
	// Add some randomness
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", t)))
	for i := 0; i < 16; i++ {
		b[i] ^= h[i]
	}
	// Set version 4 and variant
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NormalizeMessages splits messages so each content block gets its own message.
// Ported from src/utils/messages.ts:normalizeMessages
func NormalizeMessages(messages []Message) []Message {
	var result []Message
	isNewChain := false

	for _, msg := range messages {
		switch msg.Type {
		case MessageTypeAssistant:
			if len(msg.Blocks) > 1 {
				isNewChain = true
			}

			for idx, block := range msg.Blocks {
				uuid := msg.UUID
				if isNewChain {
					uuid = DeriveUUID(msg.UUID, idx)
				}

				normalized := Message{
					UUID:        uuid,
					Type:        MessageTypeAssistant,
					Role:        MessageTypeAssistant,
					Timestamp:   msg.Timestamp,
					IsMeta:      msg.IsMeta,
					IsVirtual:   msg.IsVirtual,
					RequestID:   msg.RequestID,
					Model:       msg.Model,
					StopReason:  msg.StopReason,
					Usage:       msg.Usage,
					Blocks:      []ContentBlock{block},
				}
				result = append(result, normalized)
			}

			// Handle simple string content
			if len(msg.Blocks) == 0 && msg.Content != "" {
				uuid := msg.UUID
				if isNewChain {
					uuid = DeriveUUID(msg.UUID, 0)
				}
				normalized := Message{
					UUID:        uuid,
					Type:        MessageTypeAssistant,
					Role:        MessageTypeAssistant,
					Timestamp:   msg.Timestamp,
					Content:     msg.Content,
					IsMeta:      msg.IsMeta,
					IsVirtual:   msg.IsVirtual,
					RequestID:   msg.RequestID,
					Model:       msg.Model,
					StopReason:  msg.StopReason,
					Usage:       msg.Usage,
					Blocks:      []ContentBlock{NewTextBlock(msg.Content)},
				}
				result = append(result, normalized)
			}

		case MessageTypeUser:
			if len(msg.Blocks) > 1 {
				isNewChain = true
			}

			// Handle string content
			if len(msg.Blocks) == 0 && msg.Content != "" {
				uuid := msg.UUID
				if isNewChain {
					uuid = DeriveUUID(msg.UUID, 0)
				}
				normalized := Message{
					UUID:        uuid,
					Type:        MessageTypeUser,
					Role:        MessageTypeUser,
					Timestamp:   msg.Timestamp,
					Content:     msg.Content,
					IsMeta:      msg.IsMeta,
					IsCompactSummary: msg.IsCompactSummary,
					Blocks:      []ContentBlock{NewTextBlock(msg.Content)},
				}
				result = append(result, normalized)
				continue
			}

			for idx, block := range msg.Blocks {
				uuid := msg.UUID
				if isNewChain {
					uuid = DeriveUUID(msg.UUID, idx)
				}

				normalized := Message{
					UUID:        uuid,
					Type:        MessageTypeUser,
					Role:        MessageTypeUser,
					Timestamp:   msg.Timestamp,
					IsMeta:      msg.IsMeta,
					IsCompactSummary: msg.IsCompactSummary,
					ToolUseResult: msg.ToolUseResult,
					Blocks:      []ContentBlock{block},
				}
				result = append(result, normalized)
			}

		default:
			// Keep other messages as-is
			result = append(result, msg)
		}
	}

	return result
}

// ReorderMessagesForUI reorders messages for UI display.
// Moves tool results to be after their tool use messages.
// Ported from src/utils/messages.ts:reorderMessagesInUI
func ReorderMessagesForUI(messages []Message) []Message {
	// Map tool use ID to related messages
	type toolUseGroup struct {
		toolUse    *Message
		toolResult *Message
		preHooks   []Message
		postHooks  []Message
	}

	toolUseGroups := make(map[string]*toolUseGroup)

	// First pass: group messages by tool use ID
	for _, msg := range messages {
		if msg.IsToolUseMessage() {
			for _, block := range msg.GetToolUseBlocks() {
				id := block.ID
				if id == "" {
					continue
				}
				if _, exists := toolUseGroups[id]; !exists {
					toolUseGroups[id] = &toolUseGroup{}
				}
				toolUseGroups[id].toolUse = &msg
			}
			continue
		}

		if msg.IsToolResultMessage() {
			for _, block := range msg.GetToolResultBlocks() {
				id := block.ToolUseID
				if id == "" {
					continue
				}
				if _, exists := toolUseGroups[id]; !exists {
					toolUseGroups[id] = &toolUseGroup{}
				}
				toolUseGroups[id].toolResult = &msg
			}
			continue
		}
	}

	// Second pass: reconstruct in correct order
	var result []Message
	processedToolUses := make(map[string]bool)

	for _, msg := range messages {
		if msg.IsToolUseMessage() {
			for _, block := range msg.GetToolUseBlocks() {
				id := block.ID
				if id == "" || processedToolUses[id] {
					continue
				}
				processedToolUses[id] = true

				group := toolUseGroups[id]
				if group != nil && group.toolUse != nil {
					result = append(result, *group.toolUse)
					result = append(result, group.preHooks...)
					if group.toolResult != nil {
						result = append(result, *group.toolResult)
					}
					result = append(result, group.postHooks...)
				}
			}
			continue
		}

		if msg.IsToolResultMessage() {
			// Skip - handled in tool use groups
			continue
		}

		result = append(result, msg)
	}

	return result
}

// ReorderAttachmentsForAPI reorders attachments so they bubble up.
// Ported from src/utils/messages.ts:reorderAttachmentsForAPI
func ReorderAttachmentsForAPI(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	pendingAttachments := make([]Message, 0)

	// Scan from bottom up
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		if msg.Type == MessageTypeAttachment {
			pendingAttachments = append(pendingAttachments, msg)
			continue
		}

		// Check if this is a stopping point
		isStoppingPoint := msg.Type == MessageTypeAssistant || msg.IsToolResultMessage()

		if isStoppingPoint && len(pendingAttachments) > 0 {
			// Attachments stop here
			for j := 0; j < len(pendingAttachments); j++ {
				result = append(result, pendingAttachments[j])
			}
			result = append(result, msg)
			pendingAttachments = pendingAttachments[:0]
		} else {
			result = append(result, msg)
		}
	}

	// Remaining attachments bubble to top
	for j := 0; j < len(pendingAttachments); j++ {
		result = append(result, pendingAttachments[j])
	}

	// Reverse result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetToolResultIDs returns a map of tool use IDs to whether they have errors.
// Ported from src/utils/messages.ts:getToolResultIDs
func GetToolResultIDs(messages []Message) map[string]bool {
	result := make(map[string]bool)

	for _, msg := range messages {
		if msg.IsToolResultMessage() {
			for _, block := range msg.GetToolResultBlocks() {
				result[block.ToolUseID] = block.IsError
			}
		}
	}

	return result
}

// GetToolUseIDs returns a set of all tool use IDs.
// Ported from src/utils/messages.ts:getToolUseIDs
func GetToolUseIDs(messages []Message) map[string]bool {
	result := make(map[string]bool)

	for _, msg := range messages {
		if msg.IsToolUseMessage() {
			for _, block := range msg.GetToolUseBlocks() {
				result[block.ID] = true
			}
		}
	}

	return result
}

// ExtractTag extracts content from an XML-like tag.
// Ported from src/utils/messages.ts:extractTag
func ExtractTag(html, tagName string) string {
	if strings.TrimSpace(html) == "" || strings.TrimSpace(tagName) == "" {
		return ""
	}

	// Escape tag name for regex
	escapedTag := regexp.QuoteMeta(tagName)

	// Pattern for tag with content
	pattern := fmt.Sprintf(`<%s(?:\s+[^>]*)?>([\s\S]*?)</%s>`, escapedTag, escapedTag)
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// IsNotEmptyMessage checks if a message has content.
// Ported from src/utils/messages.ts:isNotEmptyMessage
func IsNotEmptyMessage(msg Message) bool {
	if msg.Type == MessageTypeProgress || msg.Type == MessageTypeAttachment || msg.Type == MessageTypeSystem {
		return true
	}

	content := msg.GetContent()
	if strings.TrimSpace(content) == "" {
		return false
	}

	if content == NoContentMessage || content == InterruptMessageForToolUse {
		return false
	}

	return true
}

// FormatCommandInputTags formats command input for breadcrumb display.
// Ported from src/utils/messages.ts:formatCommandInputTags
func FormatCommandInputTags(commandName, args string) string {
	return fmt.Sprintf("<%s>/%s</%s>\n<%s>%s</%s>\n<%s>%s</%s>",
		CommandNameTag, commandName, CommandNameTag,
		CommandMessageTag, commandName, CommandMessageTag,
		CommandArgsTag, args, CommandArgsTag)
}

// CreateSyntheticUserCaveatMessage creates a synthetic caveat message.
// Ported from src/utils/messages.ts:createSyntheticUserCaveatMessage
func CreateSyntheticUserCaveatMessage() Message {
	content := fmt.Sprintf(
		"<%s>Caveat: The messages below were generated by the user while running local commands. DO NOT respond to these messages or otherwise consider them in your response unless the user explicitly asks you to.</%s>",
		LocalCommandCaveatTag, LocalCommandCaveatTag,
	)
	return NewUserMessage(content).WithMeta(true)
}

// CreateUserInterruptionMessage creates an interruption message.
// Ported from src/utils/messages.ts:createUserInterruptionMessage
func CreateUserInterruptionMessage(toolUse bool) Message {
	content := InterruptMessage
	if toolUse {
		content = InterruptMessageForToolUse
	}
	return NewUserMessageWithBlocks([]ContentBlock{NewTextBlock(content)})
}

// CreateToolResultStopMessage creates a tool result cancel message.
// Ported from src/utils/messages.ts:createToolResultStopMessage
func CreateToolResultStopMessage(toolUseID string) ContentBlock {
	return NewToolResultBlock(toolUseID, CancelMessage, true)
}

// CreateProgressMessage creates a progress message.
func CreateProgressMessage(toolUseID, parentToolUseID string, data interface{}) Message {
	return Message{
		Type:      MessageTypeProgress,
		Timestamp: time.Now(),
		ToolCallID: toolUseID,
		ToolUseResult: map[string]interface{}{
			"toolUseID":        toolUseID,
			"parentToolUseID":  parentToolUseID,
			"data":             data,
		},
	}
}