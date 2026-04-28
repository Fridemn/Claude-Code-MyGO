package collapse

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"claude-go/internal/tool"
	"claude-go/internal/tool/repl"
	"claude-go/internal/ui"
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
	relevantMemoryCount int
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
		} else if shouldAbsorbStopHookSummary(entry) && len(currentGroup.messages) > 0 {
			currentGroup.hookCount++
			currentGroup.hookTotalMs += extractHookTotalMs(entry)
			currentGroup.messages = append(currentGroup.messages, entry)
		} else if shouldAbsorbRelevantMemoriesAttachment(entry) && len(currentGroup.messages) > 0 {
			currentGroup.relevantMemoryCount += extractRelevantMemoryCount(entry)
			currentGroup.messages = append(currentGroup.messages, entry)
		} else if shouldAbsorbProgressMessage(entry) && len(currentGroup.messages) > 0 {
			absorbProgressHint(currentGroup, entry)
			currentGroup.messages = append(currentGroup.messages, entry)
		} else if shouldSkipMessage(entry) {
			// Don't flush the group for skippable messages
			if len(currentGroup.messages) > 0 && !isNestedMemoryAttachment(entry) {
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
	if entry.ToolName == "" {
		return tool.SearchOrReadResult{}
	}

	// Build input from raw tool input JSON first (TS parity), then fallback.
	input := decodeToolInput(entry.ToolInput)
	if len(input) == 0 && entry.Content != "" {
		input["command"] = entry.Content
	}

	if tools != nil {
		// Find the tool in active registry.
		t, ok := tools.Get(entry.ToolName)
		if ok {
			// Check if it implements SearchOrReadClassifier
			classifier, ok := t.(SearchOrReadClassifier)
			if ok {
				return classifier.IsSearchOrReadCommand(input)
			}
		}
	}

	// TS parity: in REPL mode primitives are hidden from registry. Keep collapse
	// behavior by falling back to REPL primitive tool classifiers.
	classification, ok := repl.ClassifyPrimitiveTool(entry.ToolName, input)
	if !ok {
		return tool.SearchOrReadResult{}
	}
	return classification
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
		if isMemorySearchInput(entry.ToolInput) {
			group.memorySearchCount++
			group.searchCount--
		} else if entry.Content != "" {
			group.nonMemSearchArgs = append(group.nonMemSearchArgs, entry.Content)
			group.latestDisplayHint = entry.Content
		}
	} else if classification.IsRead {
		// Track file path
		filePath := extractFilePathFromToolInput(entry.ToolInput)
		if filePath != "" {
			group.readFilePaths[filePath] = true
			if isAutoManagedMemoryFilePath(filePath) {
				group.memoryReadFilePaths[filePath] = true
			} else {
				group.latestDisplayHint = filePath
			}
		} else if entry.Content != "" {
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
	readCount := 0
	for p := range group.readFilePaths {
		if !group.memoryReadFilePaths[p] {
			readCount++
		}
	}
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
			MemoryReadCount:   len(group.memoryReadFilePaths) + group.relevantMemoryCount,
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

func decodeToolInput(raw string) tool.Input {
	input := tool.Input{}
	if strings.TrimSpace(raw) == "" {
		return input
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return input
	}
	for k, v := range parsed {
		input[k] = v
	}
	return input
}

func extractFilePathFromToolInput(raw string) string {
	input := decodeToolInput(raw)
	if v, ok := input["file_path"].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := input["path"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func isMemorySearchInput(raw string) bool {
	input := decodeToolInput(raw)
	if v, ok := input["path"].(string); ok && isAutoManagedMemoryFilePath(v) {
		return true
	}
	if v, ok := input["glob"].(string); ok && strings.Contains(strings.ToLower(v), "memory") {
		return true
	}
	if v, ok := input["command"].(string); ok && strings.Contains(strings.ToLower(v), "memory") {
		return true
	}
	return false
}

func isAutoManagedMemoryFilePath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return false
	}
	return strings.Contains(p, "/session-memory/") ||
		strings.Contains(p, "/agent-memory/") ||
		strings.Contains(p, "/agent-memory-local/") ||
		strings.Contains(p, "/.claude/memory/") ||
		strings.Contains(p, "/.claude/memory")
}

func shouldAbsorbStopHookSummary(entry ui.TranscriptEntry) bool {
	return entry.Kind == "system" && entry.Subtype == "stop_hook_summary"
}

func shouldAbsorbRelevantMemoriesAttachment(entry ui.TranscriptEntry) bool {
	return entry.Kind == "attachment" && strings.EqualFold(strings.TrimSpace(entry.Subtype), "relevant_memories")
}

func isNestedMemoryAttachment(entry ui.TranscriptEntry) bool {
	return entry.Kind == "attachment" && strings.Contains(strings.ToLower(entry.Subtype), "nested_memory")
}

func shouldAbsorbProgressMessage(entry ui.TranscriptEntry) bool {
	return entry.Kind == "progress"
}

func absorbProgressHint(group *GroupAccumulator, entry ui.TranscriptEntry) {
	if strings.TrimSpace(entry.Data) == "" {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(entry.Data), &payload); err != nil {
		return
	}

	progressType, _ := payload["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(progressType), "repl_tool_call") {
		return
	}
	phase, _ := payload["phase"].(string)
	if !strings.EqualFold(strings.TrimSpace(phase), "start") {
		return
	}

	rawInput, _ := payload["toolInput"].(map[string]any)
	if hint := extractReplProgressHint(rawInput); hint != "" {
		group.latestDisplayHint = hint
		return
	}
	if toolName, _ := payload["toolName"].(string); strings.TrimSpace(toolName) != "" {
		group.latestDisplayHint = strings.TrimSpace(toolName)
	}
}

func extractReplProgressHint(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	if filePath, _ := input["file_path"].(string); strings.TrimSpace(filePath) != "" {
		return strings.TrimSpace(filePath)
	}
	if pattern, _ := input["pattern"].(string); strings.TrimSpace(pattern) != "" {
		return strings.TrimSpace(pattern)
	}
	if command, _ := input["command"].(string); strings.TrimSpace(command) != "" {
		return strings.TrimSpace(command)
	}
	return ""
}

func extractHookTotalMs(entry ui.TranscriptEntry) int {
	raw := strings.TrimSpace(entry.Data)
	if raw == "" {
		raw = strings.TrimSpace(entry.Content)
	}
	if raw == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0
	}
	if v := getIntFromAny(payload["totalDurationMs"]); v > 0 {
		return v
	}
	if infos, ok := payload["hookInfos"].([]any); ok {
		total := 0
		for _, info := range infos {
			obj, ok := info.(map[string]any)
			if !ok {
				continue
			}
			total += getIntFromAny(obj["durationMs"])
		}
		return total
	}
	return 0
}

func extractRelevantMemoryCount(entry ui.TranscriptEntry) int {
	raw := strings.TrimSpace(entry.Data)
	if raw == "" {
		raw = strings.TrimSpace(entry.Content)
	}
	if raw == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0
	}
	if memories, ok := payload["memories"].([]any); ok {
		return len(memories)
	}
	if attachments, ok := payload["attachments"].([]any); ok {
		total := 0
		for _, attachment := range attachments {
			obj, ok := attachment.(map[string]any)
			if !ok {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(getStringFromAny(obj["type"])), "relevant_memories") {
				continue
			}
			if memories, ok := obj["memories"].([]any); ok {
				total += len(memories)
			}
		}
		return total
	}
	return 0
}

func getIntFromAny(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func getStringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
