package collapse

import (
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
	"claude-code-go/internal/ui"
)

// GroupToolUses groups consecutive tool uses of the same type from the same assistant message
// Matches TS logic from src/utils/groupToolUses.ts
func GroupToolUses(entries []ui.TranscriptEntry, tools *tool.Registry, verbose bool) []ui.TranscriptEntry {
	if verbose {
		// Skip grouping in verbose mode
		// Evidence: src/utils/groupToolUses.ts:52-64
		return entries
	}

	result := make([]ui.TranscriptEntry, 0, len(entries))
	groups := make(map[string][]ui.TranscriptEntry)
	currentMessageID := ""

	flushGroups := func() {
		for _, group := range groups {
			if len(group) < 2 {
				// Single item - don't group (need 2+ for grouping)
				// Evidence: src/utils/groupToolUses.ts:82-85
				for _, entry := range group {
					result = append(result, entry)
				}
			} else {
				// Multiple items - create grouped entry
				// Evidence: src/utils/groupToolUses.ts:86-94
				result = append(result, ui.TranscriptEntry{
					Kind:     "grouped_tool_use",
					Title:    group[0].ToolName,
					Content:  fmt.Sprintf("%d operations", len(group)),
					UUID:     group[0].UUID + "-group",
					ToolName: group[0].ToolName,
					Meta: ui.EntryMeta{
						GroupMessages: group,
					},
				})
			}
		}
		groups = make(map[string][]ui.TranscriptEntry)
	}

	for _, entry := range entries {
		// Non-tool entries break groups
		if entry.Kind != "tool_use" {
			flushGroups()
			currentMessageID = ""
			result = append(result, entry)
			continue
		}

		// Extract message ID from UUID (format: "msg-X-tool-Y")
		messageID := extractMessageID(entry.UUID)

		// Different message breaks groups
		// Evidence: src/utils/groupToolUses.ts:67-80 (grouping by message.id + toolName)
		if messageID != currentMessageID {
			flushGroups()
			currentMessageID = messageID
		}

		// Check if tool supports grouping
		// Evidence: src/utils/groupToolUses.ts:19-31
		if !supportsGrouping(entry.ToolName, tools) {
			result = append(result, entry)
			continue
		}

		// Add to group (key = messageID:toolName)
		key := messageID + ":" + entry.ToolName
		groups[key] = append(groups[key], entry)
	}

	// Flush remaining groups
	flushGroups()

	return result
}

// extractMessageID extracts the base message ID from a tool UUID
// Example: "msg-5-tool-2" -> "msg-5"
func extractMessageID(uuid string) string {
	parts := strings.Split(uuid, "-tool-")
	if len(parts) > 0 {
		return parts[0]
	}
	return uuid
}

// supportsGrouping checks if a tool implements grouping
// In TS, this checks for renderGroupedToolUse implementation
// Evidence: src/utils/groupToolUses.ts:19-31
func supportsGrouping(toolName string, tools *tool.Registry) bool {
	// TODO: This should check if tool implements a GroupingSupport interface
	// For now, use an allowlist of known groupable tools
	switch toolName {
	case "FileRead", "FileSearch", "FileList",
		"MemoryRead", "MemorySearch",
		"MCPCall":
		return true
	default:
		return false
	}
}
