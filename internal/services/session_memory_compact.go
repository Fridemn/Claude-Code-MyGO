package services

import (
	"os"
	"strings"
)

// Session Memory Compact configuration
// Ported from src/services/compact/sessionMemoryCompact.ts

// SessionMemoryCompactConfig holds configuration for session memory compaction
type SessionMemoryCompactConfig struct {
	MinTokens            int // Minimum tokens to preserve after compaction
	MinTextBlockMessages int // Minimum number of messages with text blocks to keep
	MaxTokens            int // Maximum tokens to preserve after compaction (hard cap)
}

// DefaultSMCompactConfig returns default session memory compact configuration
func DefaultSMCompactConfig() SessionMemoryCompactConfig {
	return SessionMemoryCompactConfig{
		MinTokens:            10000,
		MinTextBlockMessages: 5,
		MaxTokens:            40000,
	}
}

// Default session memory template
// Ported from src/services/SessionMemory/prompts.ts:DEFAULT_SESSION_MEMORY_TEMPLATE
const DefaultSessionMemoryTemplate = `# Session Title
_A short and distinctive 5-10 word descriptive title for the session. Super info dense, no filler_

# Current State
_What is actively being worked on right now? Pending tasks not yet completed. Immediate next steps._

# Task specification
_What did the user ask to build? Any design decisions or other explanatory context_

# Files and Functions
_What are the important files? In short, what do they contain and why are they relevant?_

# Workflow
_What bash commands are usually run and in what order? How to interpret their output if not obvious?_

# Errors & Corrections
_Errors encountered and how they were fixed. What did the user correct? What approaches failed and should not be tried again?_

# Codebase and System Documentation
_What are the important system components? How do they work/fit together?_

# Learnings
_What has worked well? What has not? What to avoid? Do not duplicate items from other sections_

# Key results
_If the user asked a specific output such as an answer to a question, a table, or other document, repeat the exact result here_

# Worklog
_Step by step, what was attempted, done? Very terse summary for each step_
`

// Session Memory constants for truncation
const (
	MaxSectionLength             = 2000
	MaxTotalSessionMemoryTokens  = 12000
)

// SessionMemoryCompactService handles session memory compaction
// Ported from src/services/compact/sessionMemoryCompact.ts
type SessionMemoryCompactService struct {
	config        SessionMemoryCompactConfig
	lastSummarizedMessageID string
	sessionMemoryPath string
}

// NewSessionMemoryCompactService creates a new session memory compact service
func NewSessionMemoryCompactService(sessionMemoryPath string) *SessionMemoryCompactService {
	return &SessionMemoryCompactService{
		config: DefaultSMCompactConfig(),
		sessionMemoryPath: sessionMemoryPath,
	}
}

// SetConfig sets the session memory compact configuration
func (s *SessionMemoryCompactService) SetConfig(config SessionMemoryCompactConfig) {
	s.config = config
}

// GetConfig returns the current configuration
func (s *SessionMemoryCompactService) GetConfig() SessionMemoryCompactConfig {
	return s.config
}

// GetLastSummarizedMessageID returns the last summarized message ID
func (s *SessionMemoryCompactService) GetLastSummarizedMessageID() string {
	return s.lastSummarizedMessageID
}

// SetLastSummarizedMessageID sets the last summarized message ID
func (s *SessionMemoryCompactService) SetLastSummarizedMessageID(id string) {
	s.lastSummarizedMessageID = id
}

// HasTextBlocks checks if a message contains text blocks
// Ported from src/services/compact/sessionMemoryCompact.ts:hasTextBlocks
func HasTextBlocks(msg CompactMessage) bool {
	if msg.Type == MessageTypeAssistant {
		// Assistant messages have text content
		return len(msg.Content) > 0
	}
	if msg.Type == MessageTypeUser {
		// User messages have text content if Content is not empty
		return len(msg.Content) > 0
	}
	return false
}

// GetToolResultIDs extracts tool_use_ids from tool_result blocks
// Ported from src/services/compact/sessionMemoryCompact.ts:getToolResultIds
func GetToolResultIDs(msg CompactMessage) []string {
	if msg.Type != MessageTypeUser {
		return nil
	}

	var ids []string
	for _, tr := range msg.ToolResults {
		if tr.ToolUseID != "" {
			ids = append(ids, tr.ToolUseID)
		}
	}
	return ids
}

// HasToolUseWithIDs checks if an assistant message has tool_use blocks with any of the given IDs
// Ported from src/services/compact/sessionMemoryCompact.ts:hasToolUseWithIds
func HasToolUseWithIDs(msg CompactMessage, toolUseIDs map[string]bool) bool {
	if msg.Type != MessageTypeAssistant {
		return false
	}

	for _, tc := range msg.ToolCalls {
		if toolUseIDs[tc.ID] {
			return true
		}
	}
	return false
}

// AdjustIndexToPreserveAPIInvariants adjusts the start index to ensure we don't split
// tool_use/tool_result pairs or thinking blocks
// Ported from src/services/compact/sessionMemoryCompact.ts:adjustIndexToPreserveAPIInvariants
func AdjustIndexToPreserveAPIInvariants(messages []CompactMessage, startIndex int) int {
	if startIndex <= 0 || startIndex >= len(messages) {
		return startIndex
	}

	adjustedIndex := startIndex

	// Step 1: Handle tool_use/tool_result pairs
	// Collect tool_result IDs from ALL messages in the kept range
	var allToolResultIDs []string
	for i := startIndex; i < len(messages); i++ {
		allToolResultIDs = append(allToolResultIDs, GetToolResultIDs(messages[i])...)
	}

	if len(allToolResultIDs) > 0 {
		// Collect tool_use IDs already in the kept range
		toolUseIDsInKeptRange := make(map[string]bool)
		for i := adjustedIndex; i < len(messages); i++ {
			msg := messages[i]
			if msg.Type == MessageTypeAssistant {
				for _, tc := range msg.ToolCalls {
					toolUseIDsInKeptRange[tc.ID] = true
				}
			}
		}

		// Only look for tool_uses that are NOT already in the kept range
		neededToolUseIDs := make(map[string]bool)
		for _, id := range allToolResultIDs {
			if !toolUseIDsInKeptRange[id] {
				neededToolUseIDs[id] = true
			}
		}

		// Find the assistant message(s) with matching tool_use blocks
		for i := adjustedIndex - 1; i >= 0 && len(neededToolUseIDs) > 0; i-- {
			msg := messages[i]
			if HasToolUseWithIDs(msg, neededToolUseIDs) {
				adjustedIndex = i
				// Remove found tool_use_ids from the set
				for _, tc := range msg.ToolCalls {
					delete(neededToolUseIDs, tc.ID)
				}
			}
		}
	}

	// Step 2: Handle thinking blocks that share MessageID with kept assistant messages
	// Collect MessageIDs from assistant messages in the kept range
	messageIDsInKeptRange := make(map[string]bool)
	for i := adjustedIndex; i < len(messages); i++ {
		msg := messages[i]
		if msg.Type == MessageTypeAssistant && msg.MessageID != "" {
			messageIDsInKeptRange[msg.MessageID] = true
		}
	}

	// Look backwards for assistant messages with the same MessageID
	for i := adjustedIndex - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Type == MessageTypeAssistant && msg.MessageID != "" && messageIDsInKeptRange[msg.MessageID] {
			// This message has the same MessageID as one in the kept range
			adjustedIndex = i
		}
	}

	return adjustedIndex
}

// CalculateMessagesToKeepIndex calculates the starting index for messages to keep after compaction
// Ported from src/services/compact/sessionMemoryCompact.ts:calculateMessagesToKeepIndex
func CalculateMessagesToKeepIndex(messages []CompactMessage, lastSummarizedIndex int, config SessionMemoryCompactConfig) int {
	if len(messages) == 0 {
		return 0
	}

	// Start from the message after lastSummarizedIndex
	startIndex := 0
	if lastSummarizedIndex >= 0 {
		startIndex = lastSummarizedIndex + 1
	} else {
		startIndex = len(messages)
	}

	// Calculate current tokens and text-block message count from startIndex to end
	totalTokens := 0
	textBlockMessageCount := 0
	for i := startIndex; i < len(messages); i++ {
		msg := messages[i]
		totalTokens += EstimateMessageTokensDetailed([]CompactMessage{msg})
		if HasTextBlocks(msg) {
			textBlockMessageCount++
		}
	}

	// Check if we already hit the max cap
	if totalTokens >= config.MaxTokens {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Check if we already meet both minimums
	if totalTokens >= config.MinTokens && textBlockMessageCount >= config.MinTextBlockMessages {
		return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
	}

	// Expand backwards until we meet both minimums or hit max cap
	// Floor at the last boundary
	floor := 0
	for i := len(messages) - 1; i >= 0; i-- {
		if IsCompactBoundaryMessage(messages[i]) {
			floor = i + 1
			break
		}
	}

	for i := startIndex - 1; i >= floor; i-- {
		msg := messages[i]
		msgTokens := EstimateMessageTokensDetailed([]CompactMessage{msg})
		totalTokens += msgTokens
		if HasTextBlocks(msg) {
			textBlockMessageCount++
		}
		startIndex = i

		// Stop if we hit the max cap
		if totalTokens >= config.MaxTokens {
			break
		}

		// Stop if we meet both minimums
		if totalTokens >= config.MinTokens && textBlockMessageCount >= config.MinTextBlockMessages {
			break
		}
	}

	return AdjustIndexToPreserveAPIInvariants(messages, startIndex)
}

// LoadSessionMemoryTemplate loads the session memory template
// Ported from src/services/SessionMemory/prompts.ts:loadSessionMemoryTemplate
func LoadSessionMemoryTemplate() string {
	// In full implementation, this would load from ~/.claude/session-memory/config/template.md
	return DefaultSessionMemoryTemplate
}

// IsSessionMemoryEmpty checks if the session memory content is essentially empty (matches the template)
// Ported from src/services/SessionMemory/prompts.ts:isSessionMemoryEmpty
func IsSessionMemoryEmpty(content string) bool {
	template := LoadSessionMemoryTemplate()
	return strings.TrimSpace(content) == strings.TrimSpace(template)
}

// LoadSessionMemoryContent loads the session memory content from disk
func LoadSessionMemoryContent(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// TruncateSessionMemoryResult holds the result of truncating session memory
type TruncateSessionMemoryResult struct {
	TruncatedContent string
	WasTruncated     bool
}

// TruncateSessionMemoryForCompact truncates session memory sections that exceed per-section limits
// Ported from src/services/SessionMemory/prompts.ts:truncateSessionMemoryForCompact
func TruncateSessionMemoryForCompact(content string) TruncateSessionMemoryResult {
	lines := strings.Split(content, "\n")
	maxCharsPerSection := MaxSectionLength * 4 // roughTokenCountEstimation uses length/4

	var outputLines []string
	var currentSectionLines []string
	var currentSectionHeader string
	wasTruncated := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			result := flushSessionSection(currentSectionHeader, currentSectionLines, maxCharsPerSection)
			outputLines = append(outputLines, result.Lines...)
			if result.WasTruncated {
				wasTruncated = true
			}
			currentSectionHeader = line
			currentSectionLines = nil
		} else {
			currentSectionLines = append(currentSectionLines, line)
		}
	}

	// Flush the last section
	result := flushSessionSection(currentSectionHeader, currentSectionLines, maxCharsPerSection)
	outputLines = append(outputLines, result.Lines...)
	if result.WasTruncated {
		wasTruncated = true
	}

	return TruncateSessionMemoryResult{
		TruncatedContent: strings.Join(outputLines, "\n"),
		WasTruncated:     wasTruncated,
	}
}

// FlushSessionSectionResult holds the result of flushing a session section
type FlushSessionSectionResult struct {
	Lines        []string
	WasTruncated bool
}

// flushSessionSection flushes a session section with truncation if needed
// Ported from src/services/SessionMemory/prompts.ts:flushSessionSection
func flushSessionSection(sectionHeader string, sectionLines []string, maxCharsPerSection int) FlushSessionSectionResult {
	if sectionHeader == "" {
		return FlushSessionSectionResult{Lines: sectionLines, WasTruncated: false}
	}

	sectionContent := strings.Join(sectionLines, "\n")
	if len(sectionContent) <= maxCharsPerSection {
		lines := append([]string{sectionHeader}, sectionLines...)
		return FlushSessionSectionResult{Lines: lines, WasTruncated: false}
	}

	// Truncate at a line boundary near the limit
	charCount := 0
	var keptLines []string
	keptLines = append(keptLines, sectionHeader)
	for _, line := range sectionLines {
		if charCount+len(line)+1 > maxCharsPerSection {
			break
		}
		keptLines = append(keptLines, line)
		charCount += len(line) + 1
	}
	keptLines = append(keptLines, "\n[... section truncated for length ...]")
	return FlushSessionSectionResult{Lines: keptLines, WasTruncated: true}
}

// CreateSessionMemoryCompactResult creates a CompactionResult from session memory
// Ported from src/services/compact/sessionMemoryCompact.ts:createCompactionResultFromSessionMemory
func CreateSessionMemoryCompactResult(
	messages []CompactMessage,
	sessionMemory string,
	messagesToKeep []CompactMessage,
	transcriptPath string,
) *CompactionResult {
	preCompactTokenCount := EstimateMessagesTokenCount(messages)

	boundaryMarker := CreateCompactBoundaryMessage(true, preCompactTokenCount, "")

	// Truncate oversized sections
	truncateResult := TruncateSessionMemoryForCompact(sessionMemory)
	truncatedContent := truncateResult.TruncatedContent

	// Build summary content
	var summaryContent string
	if transcriptPath != "" {
		summaryContent = "This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n" + truncatedContent + "\n\nIf you need specific details from before compaction, read the full transcript at: " + transcriptPath
	} else {
		summaryContent = "This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n" + truncatedContent
	}

	if truncateResult.WasTruncated {
		summaryContent += "\n\nSome session memory sections were truncated for length."
	}

	summaryContent += "\n\nRecent messages are preserved verbatim."

	summaryMessages := []CompactMessage{
		{
			Type:             MessageTypeUser,
			Role:             MessageTypeUser,
			Content:          summaryContent,
			IsCompactSummary: true,
		},
	}

	// Annotate boundary with preserved segment
	annotatedBoundary := boundaryMarker
	if len(messagesToKeep) > 0 && len(summaryMessages) > 0 {
		annotatedBoundary = AnnotateBoundaryWithPreservedSegment(
			boundaryMarker,
			summaryMessages[len(summaryMessages)-1].UUID,
			messagesToKeep,
		)
	}

	// Calculate post-compact token count
	postCompactTokenCount := EstimateMessagesTokenCount(summaryMessages)

	return &CompactionResult{
		BoundaryMarker:            annotatedBoundary,
		SummaryMessages:           summaryMessages,
		MessagesToKeep:            messagesToKeep,
		PreCompactTokenCount:      preCompactTokenCount,
		PostCompactTokenCount:     postCompactTokenCount,
		TruePostCompactTokenCount: postCompactTokenCount,
	}
}

// TrySessionMemoryCompaction attempts to use session memory for compaction
// Returns nil if session memory compaction cannot be used
// Ported from src/services/compact/sessionMemoryCompact.ts:trySessionMemoryCompaction
func TrySessionMemoryCompaction(
	messages []CompactMessage,
	sessionMemoryPath string,
	autoCompactThreshold int,
) *CompactionResult {
	// Load session memory content
	sessionMemory, err := LoadSessionMemoryContent(sessionMemoryPath)
	if err != nil || sessionMemory == "" {
		return nil
	}

	// Check if session memory is empty (matches template)
	if IsSessionMemoryEmpty(sessionMemory) {
		return nil
	}

	config := DefaultSMCompactConfig()

	// Find the last summarized message index
	// In a real implementation, this would use the stored lastSummarizedMessageID
	lastSummarizedIndex := -1
	for i, msg := range messages {
		if msg.IsCompactSummary {
			lastSummarizedIndex = i
		}
	}

	// Calculate the starting index for messages to keep
	startIndex := CalculateMessagesToKeepIndex(messages, lastSummarizedIndex, config)

	// Filter out old compact boundary messages from messagesToKeep
	var messagesToKeep []CompactMessage
	for i := startIndex; i < len(messages); i++ {
		msg := messages[i]
		if !IsCompactBoundaryMessage(msg) {
			messagesToKeep = append(messagesToKeep, msg)
		}
	}

	// Get transcript path (empty for now, would be configured)
	transcriptPath := ""

	result := CreateSessionMemoryCompactResult(messages, sessionMemory, messagesToKeep, transcriptPath)

	// Check if threshold is exceeded
	if result.TruePostCompactTokenCount >= autoCompactThreshold {
		return nil
	}

	return result
}

// ShouldUseSessionMemoryCompaction checks if session memory compaction should be used
// This is controlled by feature flags (in full implementation)
func ShouldUseSessionMemoryCompaction() bool {
	// In full implementation, this would check feature flags:
	// - tengu_session_memory
	// - tengu_sm_compact
	// - Environment variables for testing
	// For now, return false to use traditional compact
	return false
}
