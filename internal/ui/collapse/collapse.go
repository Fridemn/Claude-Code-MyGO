package collapse

import (
	"strings"

	"claude-code-go/internal/tool"
	"claude-code-go/internal/ui"
)

// GroupAccumulator accumulates data for a collapsed group
type GroupAccumulator struct {
	messages            []ui.TranscriptEntry
	searchCount         int
	readFilePaths       map[string]bool
	readOperationCount  int
	listCount           int
	toolUseIds          map[string]bool
	memorySearchCount   int
	memoryReadFilePaths map[string]bool
	memoryWriteCount    int
	nonMemSearchArgs    []string
	latestDisplayHint   string
	mcpCallCount        int
	mcpServerNames      map[string]bool
	bashCount           int
	hookCount           int
	hookTotalMs         int
	commits             []ui.GitCommit
	prs                 []ui.GitPR
	branches            []ui.GitBranch
	pushes              []ui.GitPush
}

func newGroupAccumulator() *GroupAccumulator {
	return &GroupAccumulator{
		messages:            make([]ui.TranscriptEntry, 0),
		readFilePaths:       make(map[string]bool),
		toolUseIds:          make(map[string]bool),
		memoryReadFilePaths: make(map[string]bool),
		mcpServerNames:      make(map[string]bool),
	}
}

// SearchOrReadClassifier is an interface for tools that can classify themselves
type SearchOrReadClassifier interface {
	IsSearchOrReadCommand(input tool.Input) tool.SearchOrReadResult
}

// ReadSearchGroups collapses consecutive read/search operations into summary groups
func ReadSearchGroups(entries []ui.TranscriptEntry, tools *tool.Registry, verbose bool) []ui.TranscriptEntry {
	if verbose {
		return entries // Don't collapse in verbose mode
	}

	result := make([]ui.TranscriptEntry, 0, len(entries))
	currentGroup := newGroupAccumulator()
	deferredSkippable := make([]ui.TranscriptEntry, 0)

	flushGroup := func() {
		if len(currentGroup.messages) == 0 {
			return
		}

		// Create collapsed entry
		collapsed := createCollapsedEntry(currentGroup)
		result = append(result, collapsed)

		// Add deferred messages
		for _, deferred := range deferredSkippable {
			result = append(result, deferred)
		}
		deferredSkippable = deferredSkippable[:0]

		// Reset group
		currentGroup = newGroupAccumulator()
	}

	for _, entry := range entries {
		// Check if this is a collapsible tool use
		if isCollapsibleToolUse(entry, tools) {
			// Get tool classification
			classification := getToolClassification(entry, tools)

			// Update group accumulator based on classification
			updateGroupFromClassification(currentGroup, entry, classification)

			currentGroup.messages = append(currentGroup.messages, entry)
		} else if isCollapsibleToolResult(entry, currentGroup.toolUseIds) {
			// This is a tool result for a tool use in the current group
			currentGroup.messages = append(currentGroup.messages, entry)
		} else if shouldSkipMessage(entry) {
			// Don't flush the group for skippable messages
			if len(currentGroup.messages) > 0 {
				deferredSkippable = append(deferredSkippable, entry)
			} else {
				result = append(result, entry)
			}
		} else if isTextBreaker(entry) {
			// Assistant text breaks the group
			flushGroup()
			result = append(result, entry)
		} else if isNonCollapsibleToolUse(entry, tools) {
			// Non-collapsible tool use breaks the group
			flushGroup()
			result = append(result, entry)
		} else {
			// Other messages break the group
			flushGroup()
			result = append(result, entry)
		}
	}

	flushGroup()
	return result
}

// isCollapsibleToolUse checks if an entry is a collapsible tool use
func isCollapsibleToolUse(entry ui.TranscriptEntry, tools *tool.Registry) bool {
	if entry.Kind != "tool_use" {
		return false
	}

	classification := getToolClassification(entry, tools)
	return classification.IsCollapsible
}

// isCollapsibleToolResult checks if an entry is a tool result for tools in the current group
func isCollapsibleToolResult(entry ui.TranscriptEntry, toolUseIds map[string]bool) bool {
	if entry.Kind != "tool_result" {
		return false
	}

	// Check if this tool result's ID is in our tracked set
	if entry.ToolUseID != "" && toolUseIds[entry.ToolUseID] {
		return true
	}

	return false
}

// shouldSkipMessage returns true for messages that should not break a group
func shouldSkipMessage(entry ui.TranscriptEntry) bool {
	// Skip thinking blocks
	if entry.Kind == "thinking" || entry.Kind == "redacted_thinking" {
		return true
	}

	// Skip attachments
	if entry.Kind == "attachment" {
		return true
	}

	// Skip system messages
	if entry.Kind == "system" {
		return true
	}

	return false
}

// isTextBreaker returns true if the entry is assistant text that should break a group
func isTextBreaker(entry ui.TranscriptEntry) bool {
	if entry.Kind == "assistant" || entry.Kind == "assistant_streaming" {
		if strings.TrimSpace(entry.Content) != "" {
			return true
		}
	}
	return false
}

// isNonCollapsibleToolUse returns true if the entry is a non-collapsible tool use
func isNonCollapsibleToolUse(entry ui.TranscriptEntry, tools *tool.Registry) bool {
	if entry.Kind != "tool_use" {
		return false
	}

	classification := getToolClassification(entry, tools)
	return !classification.IsCollapsible
}

// getToolClassification gets the SearchOrReadResult for a tool use entry
func getToolClassification(entry ui.TranscriptEntry, tools *tool.Registry) tool.SearchOrReadResult {
	if entry.ToolName == "" || tools == nil {
		return tool.SearchOrReadResult{}
	}

	// Find the tool
	t, ok := tools.Get(entry.ToolName)
	if !ok {
		return tool.SearchOrReadResult{}
	}

	// Check if it implements SearchOrReadClassifier
	classifier, ok := t.(SearchOrReadClassifier)
	if !ok {
		return tool.SearchOrReadResult{}
	}

	// Build input from entry content (simplified)
	input := tool.Input{}
	if entry.Content != "" {
		// Parse as simple key=value or JSON if possible
		input["command"] = entry.Content
		input["file_path"] = entry.Content
		input["pattern"] = entry.Content
	}

	return classifier.IsSearchOrReadCommand(input)
}

// updateGroupFromClassification updates the group accumulator based on tool classification
func updateGroupFromClassification(group *GroupAccumulator, entry ui.TranscriptEntry, classification tool.SearchOrReadResult) {
	// Track tool use ID
	if entry.ToolUseID != "" {
		group.toolUseIds[entry.ToolUseID] = true
	}

	if classification.IsMemoryWrite {
		group.memoryWriteCount++
	} else if classification.IsAbsorbedSilently {
		// Don't count - absorbed silently
	} else if classification.MCPServerName != "" {
		group.mcpCallCount++
		group.mcpServerNames[classification.MCPServerName] = true
		group.latestDisplayHint = classification.MCPServerName
	} else if classification.IsBash {
		group.bashCount++
		group.latestDisplayHint = entry.Content
	} else if classification.IsList {
		group.listCount++
		group.latestDisplayHint = entry.Content
	} else if classification.IsSearch {
		group.searchCount++
		if entry.Content != "" {
			group.nonMemSearchArgs = append(group.nonMemSearchArgs, entry.Content)
			group.latestDisplayHint = entry.Content
		}
	} else if classification.IsRead {
		// Track file path
		if entry.Content != "" {
			group.readFilePaths[entry.Content] = true
			group.latestDisplayHint = entry.Content
		} else {
			group.readOperationCount++
		}
	}
}

// createCollapsedEntry creates a collapsed transcript entry from a group accumulator
func createCollapsedEntry(group *GroupAccumulator) ui.TranscriptEntry {
	// Calculate read count
	readCount := len(group.readFilePaths)
	if readCount == 0 {
		readCount = group.readOperationCount
	}

	// Determine if group is active (has in-progress tools)
	isActive := false
	for _, msg := range group.messages {
		if msg.IsActive {
			isActive = true
			break
		}
	}

	// Get first message for timestamp/UUID
	var firstMsg ui.TranscriptEntry
	if len(group.messages) > 0 {
		firstMsg = group.messages[0]
	}

	// Build MCP server names list
	mcpServerNames := make([]string, 0, len(group.mcpServerNames))
	for name := range group.mcpServerNames {
		mcpServerNames = append(mcpServerNames, name)
	}

	return ui.TranscriptEntry{
		Kind:     "collapsed",
		IsActive: isActive,
		UUID:     "collapsed-" + firstMsg.UUID,
		Meta: ui.EntryMeta{
			SearchCount:       group.searchCount,
			ReadCount:         readCount,
			ListCount:         group.listCount,
			MemorySearchCount: group.memorySearchCount,
			MemoryReadCount:   len(group.memoryReadFilePaths),
			MemoryWriteCount:  group.memoryWriteCount,
			MCPCallCount:      group.mcpCallCount,
			MCPServerNames:    mcpServerNames,
			BashCount:         group.bashCount,
			HookCount:         group.hookCount,
			HookTotalMs:       group.hookTotalMs,
			Commits:           group.commits,
			PRs:               group.prs,
			Branches:          group.branches,
			Pushes:            group.pushes,
			DisplayHint:       group.latestDisplayHint,
			GroupMessages:     group.messages,
		},
		Timestamp: firstMsg.Timestamp,
	}
}