package services

import (
	"strings"
)

// Micro-compact implementation for tool result clearing.
// Ported from src/services/compact/microCompact.ts

const (
	// TimeBasedMCClearedMessage is the marker for cleared tool results
	TimeBasedMCClearedMessage = "[Old tool result content cleared]"
	// DefaultGapThresholdMinutes is the default time gap threshold for time-based MC
	DefaultGapThresholdMinutes = 30
	// DefaultKeepRecent is the default number of recent tool results to keep
	DefaultKeepRecent = 3
)

// CompactableToolNames is the set of tool names that can be compacted.
// Ported from src/services/compact/microCompact.ts:COMPACTABLE_TOOLS
var CompactableToolNames = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"PowerShell": true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
	"Edit":      true,
	"Write":     true,
}

// TimeBasedMCConfig holds configuration for time-based microcompact.
type TimeBasedMCConfig struct {
	Enabled             bool
	GapThresholdMinutes int
	KeepRecent          int
}

// DefaultTimeBasedMCConfig returns default time-based MC configuration.
func DefaultTimeBasedMCConfig() TimeBasedMCConfig {
	return TimeBasedMCConfig{
		Enabled:             true,
		GapThresholdMinutes: DefaultGapThresholdMinutes,
		KeepRecent:          DefaultKeepRecent,
	}
}

// MicrocompactResult contains the result of microcompaction.
// Ported from src/services/compact/microCompact.ts:MicrocompactResult
type MicrocompactResult struct {
	Messages        []CompactMessage
	TokensSaved     int
	ToolsCleared    int
	ToolsKept       int
	DidCompact      bool
}

// MicrocompactMessages performs microcompaction on messages.
// Ported from src/services/compact/microCompact.ts:microcompactMessages
func MicrocompactMessages(messages []CompactMessage, config TimeBasedMCConfig) *MicrocompactResult {
	if !config.Enabled {
		return &MicrocompactResult{Messages: messages, DidCompact: false}
	}

	// Collect compactable tool IDs
	compactableIDs := collectCompactableToolIDs(messages)
	if len(compactableIDs) == 0 {
		return &MicrocompactResult{Messages: messages, DidCompact: false}
	}

	// Determine which to keep vs clear
	keepRecent := config.KeepRecent
	if keepRecent < 1 {
		keepRecent = 1
	}

	// Keep the last N tool IDs
	keepSet := make(map[string]bool)
	clearSet := make(map[string]bool)
	for i, id := range compactableIDs {
		if i >= len(compactableIDs)-keepRecent {
			keepSet[id] = true
		} else {
			clearSet[id] = true
		}
	}

	if len(clearSet) == 0 {
		return &MicrocompactResult{Messages: messages, DidCompact: false}
	}

	// Clear tool results in clearSet
	result := make([]CompactMessage, len(messages))
	tokensSaved := 0
	toolsCleared := 0

	for i, msg := range messages {
		if msg.Type != MessageTypeUser {
			result[i] = msg
			continue
		}

		// Check for tool results to clear
		copied := msg
		var newResults []ToolResultContent
		for _, tr := range msg.ToolResults {
			if clearSet[tr.ToolUseID] && tr.Content != TimeBasedMCClearedMessage {
				tokensSaved += EstimateTokenCount(tr.Content)
				toolsCleared++
				newResults = append(newResults, ToolResultContent{
					ToolUseID: tr.ToolUseID,
					Content:   TimeBasedMCClearedMessage,
				})
			} else {
				newResults = append(newResults, tr)
			}
		}

		if len(newResults) > 0 {
			copied.ToolResults = newResults
		}
		result[i] = copied
	}

	return &MicrocompactResult{
		Messages:     result,
		TokensSaved:  tokensSaved,
		ToolsCleared: toolsCleared,
		ToolsKept:    len(keepSet),
		DidCompact:   toolsCleared > 0,
	}
}

// collectCompactableToolIDs collects tool_use IDs for compactable tools.
// Ported from src/services/compact/microCompact.ts:collectCompactableToolIds
func collectCompactableToolIDs(messages []CompactMessage) []string {
	var ids []string
	seen := make(map[string]bool)

	for _, msg := range messages {
		if msg.Type != MessageTypeAssistant {
			continue
		}

		for _, tc := range msg.ToolCalls {
			if CompactableToolNames[tc.Name] && !seen[tc.ID] {
				ids = append(ids, tc.ID)
				seen[tc.ID] = true
			}
		}
	}

	return ids
}

// EstimateMessageTokens estimates tokens for messages with all block types.
// Ported from src/services/compact/microCompact.ts:estimateMessageTokens
func EstimateMessageTokensDetailed(messages []CompactMessage) int {
	totalTokens := 0

	for _, msg := range messages {
		if msg.Type != MessageTypeUser && msg.Type != MessageTypeAssistant {
			continue
		}

		// Count text content
		totalTokens += EstimateTokenCount(msg.Content)

		// Count tool calls
		for _, tc := range msg.ToolCalls {
			totalTokens += EstimateTokenCount(tc.Name)
			totalTokens += EstimateTokenCount(tc.Arguments)
		}

		// Count tool results
		for _, tr := range msg.ToolResults {
			totalTokens += EstimateToolResultTokens(tr)
		}

		// Count images
		totalTokens += len(msg.Images) * ImageDocumentMaxTokens

		// Count thinking blocks (if present in content)
		totalTokens += estimateThinkingTokens(msg.Content)
	}

	// Pad by 4/3 to be conservative
	return int(float64(totalTokens) * 1.33)
}

// estimateThinkingTokens estimates tokens for thinking blocks in content.
func estimateThinkingTokens(content string) int {
	// Look for <thinking> tags
	start := strings.Index(content, "<thinking>")
	if start == -1 {
		return 0
	}
	end := strings.Index(content, "</thinking>")
	if end == -1 || end <= start {
		return 0
	}
	return EstimateTokenCount(content[start+len("<thinking>") : end])
}

// IsMainThreadSource checks if the query source is from the main thread.
// Ported from src/services/compact/microCompact.ts:isMainThreadSource
func IsMainThreadSource(querySource string) bool {
	return querySource == "" || strings.HasPrefix(querySource, "repl_main_thread")
}