package collapse

import (
	"fmt"
	"strings"

	"claude-go/internal/tool"
	"claude-go/internal/ui"
)

// GroupToolUses matches TS applyGrouping behavior:
// - group by assistant message id + tool name
// - only emit grouped entries for groups with 2+ tool uses
// - emit each group once at first occurrence order
// - suppress grouped tool_result rows
func GroupToolUses(entries []ui.TranscriptEntry, tools *tool.Registry, verbose bool) []ui.TranscriptEntry {
	if verbose {
		// Evidence: src/utils/groupToolUses.ts:59-64
		return entries
	}

	// First pass: collect tool_use groups keyed by messageID:toolName.
	groups := make(map[string][]ui.TranscriptEntry)
	for _, entry := range entries {
		key, ok := groupKeyForToolUse(entry, tools)
		if !ok {
			continue
		}
		groups[key] = append(groups[key], entry)
	}

	// Keep only valid groups (2+) and collect grouped tool IDs.
	validGroups := make(map[string][]ui.TranscriptEntry)
	groupedToolUseIDs := make(map[string]bool)
	for key, group := range groups {
		if len(group) < 2 {
			continue
		}
		validGroups[key] = group
		for _, e := range group {
			if e.ToolUseID != "" {
				groupedToolUseIDs[e.ToolUseID] = true
			}
		}
	}

	// Second pass: preserve original order; emit each group once.
	result := make([]ui.TranscriptEntry, 0, len(entries))
	emittedGroups := make(map[string]bool)
	for _, entry := range entries {
		if entry.Kind == "tool_use" {
			key, ok := groupKeyForToolUse(entry, tools)
			if ok {
				group := validGroups[key]
				if len(group) >= 2 {
					if !emittedGroups[key] {
						emittedGroups[key] = true
						first := group[0]
						isActive := false
						for _, g := range group {
							if g.IsActive {
								isActive = true
								break
							}
						}
						result = append(result, ui.TranscriptEntry{
							Kind:      "grouped_tool_use",
							Title:     first.ToolName,
							Content:   fmt.Sprintf("%d operations", len(group)),
							UUID:      "grouped-" + first.UUID,
							ToolName:  first.ToolName,
							IsActive:  isActive,
							Timestamp: first.Timestamp,
							Meta: ui.EntryMeta{
								GroupMessages: group,
							},
						})
					}
					continue
				}
			}
			result = append(result, entry)
			continue
		}

		// Suppress grouped tool_result entries.
		if entry.Kind == "tool_result" && entry.ToolUseID != "" && groupedToolUseIDs[entry.ToolUseID] {
			continue
		}
		result = append(result, entry)
	}

	return result
}

func groupKeyForToolUse(entry ui.TranscriptEntry, tools *tool.Registry) (string, bool) {
	if entry.Kind != "tool_use" || entry.ToolName == "" || !supportsGrouping(entry.ToolName, tools) {
		return "", false
	}
	messageID := extractMessageID(entry.UUID)
	return messageID + ":" + entry.ToolName, true
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
	// TS checks dynamic tool capability (renderGroupedToolUse).
	// Go migration uses runtime capability first, with a minimal explicit fallback.
	if tools != nil {
		if t, ok := tools.Get(toolName); ok {
			if classifier, ok := t.(SearchOrReadClassifier); ok {
				if classifier.IsSearchOrReadCommand(tool.Input{}).IsCollapsible {
					return true
				}
			}
		}
	}

	switch toolName {
	case "Agent":
		return true
	default:
		return false
	}
}
