package services

import (
	"strings"
)

// File Read Tool constants and helpers for compact
// Ported from src/tools/FileReadTool/prompt.ts and src/services/compact/compact.ts

// FileReadToolName is the name of the Read tool
const FileReadToolName = "Read"

// FileUnchangedStub is a marker for unchanged files.
// When a file hasn't changed since the last read, we use this stub instead of re-reading.
const FileUnchangedStub = "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."

// MaxLinesToRead is the maximum number of lines to read by default
const MaxLinesToRead = 2000

// PostCompactMaxTokensPerFile is the max tokens for post-compact file attachments
const PostCompactMaxTokensPerFileDefault = 5000

// IsFileUnchangedStub checks if content is a file unchanged stub
func IsFileUnchangedStub(content string) bool {
	return strings.HasPrefix(content, FileUnchangedStub)
}

// CollectReadToolFilePaths collects file paths from Read tool calls in messages.
// Skips Reads whose tool_result is a dedup stub — the stub points at an
// earlier full Read that may have been compacted away.
// Ported from src/services/compact/compact.ts:collectReadToolFilePaths
func CollectReadToolFilePaths(messages []CompactMessage) map[string]bool {
	// First pass: collect stub tool_use_ids
	stubIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Type != MessageTypeUser {
			continue
		}
		for _, tr := range msg.ToolResults {
			if IsFileUnchangedStub(tr.Content) {
				stubIDs[tr.ToolUseID] = true
			}
		}
	}

	// Second pass: collect file paths from non-stub tool_uses
	paths := make(map[string]bool)
	for _, msg := range messages {
		if msg.Type != MessageTypeAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			// Skip stub tool_uses
			if stubIDs[tc.ID] {
				continue
			}
			// Only process Read tool calls
			if tc.Name != FileReadToolName {
				continue
			}
			// Extract file_path from arguments
			path := extractFilePathFromArguments(tc.Arguments)
			if path != "" {
				paths[path] = true
			}
		}
	}

	return paths
}

// extractFilePathFromArguments extracts the file_path from tool call arguments JSON
func extractFilePathFromArguments(arguments string) string {
	// Simple extraction for {"file_path": "path"} or {"file_path":"path"}
	// This is a simplified version - full implementation would use JSON parsing
	prefix := `"file_path"`
	start := strings.Index(arguments, prefix)
	if start == -1 {
		return ""
	}

	// Find the opening quote after "file_path"
	quoteStart := start + len(prefix)
	for quoteStart < len(arguments) && arguments[quoteStart] != '"' {
		quoteStart++
	}
	if quoteStart >= len(arguments) {
		return ""
	}
	quoteStart++ // Move past the opening quote

	// Find the closing quote
	quoteEnd := quoteStart
	for quoteEnd < len(arguments) && arguments[quoteEnd] != '"' {
		if arguments[quoteEnd] == '\\' {
			quoteEnd++ // Skip escaped characters
		}
		quoteEnd++
	}

	if quoteEnd <= quoteStart {
		return ""
	}

	return arguments[quoteStart:quoteEnd]
}

// FileReadResult represents a file read result for compact purposes
type FileReadResult struct {
	ToolUseID string
	Path      string
	Content   string
	Timestamp int64
}

// CollectFileReadResults collects file read results from messages
// Ported from src/services/compact/compact.ts (simplified)
func CollectFileReadResults(messages []CompactMessage) []FileReadResult {
	var results []FileReadResult

	for _, msg := range messages {
		if msg.Type != MessageTypeUser {
			continue
		}
		for _, tr := range msg.ToolResults {
			// Try to find the matching tool_use to get the path
			path := findToolUsePath(messages, tr.ToolUseID)
			if path != "" {
				results = append(results, FileReadResult{
					ToolUseID: tr.ToolUseID,
					Path:      path,
					Content:   tr.Content,
				})
			}
		}
	}

	return results
}

// findToolUsePath finds the file path for a tool_use by ID
func findToolUsePath(messages []CompactMessage, toolUseID string) string {
	for _, msg := range messages {
		if msg.Type != MessageTypeAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if tc.ID == toolUseID && tc.Name == FileReadToolName {
				return extractFilePathFromArguments(tc.Arguments)
			}
		}
	}
	return ""
}

// GetUnchangedFilePaths returns paths that are referenced by unchanged stubs
// These files should be re-injected in post-compact attachments
func GetUnchangedFilePaths(messages []CompactMessage) map[string]bool {
	stubIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Type != MessageTypeUser {
			continue
		}
		for _, tr := range msg.ToolResults {
			if IsFileUnchangedStub(tr.Content) {
				stubIDs[tr.ToolUseID] = true
			}
		}
	}

	paths := make(map[string]bool)
	for _, msg := range messages {
		if msg.Type != MessageTypeAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if stubIDs[tc.ID] && tc.Name == FileReadToolName {
				path := extractFilePathFromArguments(tc.Arguments)
				if path != "" {
					paths[path] = true
				}
			}
		}
	}

	return paths
}
