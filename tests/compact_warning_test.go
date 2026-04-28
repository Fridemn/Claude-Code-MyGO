package tests

import (
	"testing"

	"claude-go/internal/services"
)

func TestCompactWarningState(t *testing.T) {
	t.Parallel()

	// Reset state before test
	services.ResetCompactWarningState()

	// Initially not suppressed
	if services.IsCompactWarningSuppressed() {
		t.Error("expected warning not suppressed initially")
	}

	// Suppress warning
	services.SuppressCompactWarning()
	if !services.IsCompactWarningSuppressed() {
		t.Error("expected warning suppressed after SuppressCompactWarning")
	}

	// Clear suppression
	services.ClearCompactWarningSuppression()
	if services.IsCompactWarningSuppressed() {
		t.Error("expected warning not suppressed after ClearCompactWarningSuppression")
	}
}

func TestCompactWarningStateMultiple(t *testing.T) {
	t.Parallel()

	services.ResetCompactWarningState()

	// Multiple suppress calls should be idempotent
	services.SuppressCompactWarning()
	services.SuppressCompactWarning()
	services.SuppressCompactWarning()

	if !services.IsCompactWarningSuppressed() {
		t.Error("expected warning suppressed after multiple SuppressCompactWarning calls")
	}

	// Multiple clear calls should be idempotent
	services.ClearCompactWarningSuppression()
	services.ClearCompactWarningSuppression()

	if services.IsCompactWarningSuppressed() {
		t.Error("expected warning not suppressed after multiple ClearCompactWarningSuppression calls")
	}
}

func TestFileUnchangedStub(t *testing.T) {
	t.Parallel()

	// Check the stub constant
	expectedStub := "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."
	if services.FileUnchangedStub != expectedStub {
		t.Errorf("FileUnchangedStub mismatch:\ngot:  %s\nwant: %s", services.FileUnchangedStub, expectedStub)
	}
}

func TestIsFileUnchangedStub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		content  string
		expected bool
	}{
		{"File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading.", true},
		{"File unchanged since last read.", false}, // Partial match - not a valid stub
		{"This is some file content", false},
		{"", false},
		{"The content from the earlier Read tool_result in this conversation is still current", false},
	}

	for _, tc := range tests {
		result := services.IsFileUnchangedStub(tc.content)
		if result != tc.expected {
			t.Errorf("IsFileUnchangedStub(%q) = %v, want %v", tc.content, result, tc.expected)
		}
	}
}

func TestCollectReadToolFilePaths(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{
			Type: "assistant",
			Role: "assistant",
			ToolCalls: []services.ToolCallContent{
				{ID: "tc1", Name: "Read", Arguments: `{"file_path": "/path/to/file1.go"}`},
				{ID: "tc2", Name: "Write", Arguments: `{"file_path": "/path/to/file2.go"}`},
			},
		},
		{
			Type: "user",
			Role: "user",
			ToolResults: []services.ToolResultContent{
				{ToolUseID: "tc1", Content: "content of file1"},
			},
		},
	}

	paths := services.CollectReadToolFilePaths(messages)

	// Should contain /path/to/file1.go
	if !paths["/path/to/file1.go"] {
		t.Error("expected /path/to/file1.go in paths")
	}

	// Should not contain Write tool path
	if paths["/path/to/file2.go"] {
		t.Error("Write tool path should not be included")
	}
}

func TestCollectReadToolFilePathsWithStub(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{
			Type: "assistant",
			Role: "assistant",
			ToolCalls: []services.ToolCallContent{
				{ID: "tc1", Name: "Read", Arguments: `{"file_path": "/path/to/file1.go"}`},
				{ID: "tc2", Name: "Read", Arguments: `{"file_path": "/path/to/file2.go"}`},
			},
		},
		{
			Type: "user",
			Role: "user",
			ToolResults: []services.ToolResultContent{
				{ToolUseID: "tc1", Content: "content of file1"},
				{ToolUseID: "tc2", Content: "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."},
			},
		},
	}

	paths := services.CollectReadToolFilePaths(messages)

	// tc1 should be included (normal tool result)
	if !paths["/path/to/file1.go"] {
		t.Error("expected /path/to/file1.go in paths (normal tool result)")
	}

	// tc2 should NOT be included (stub result - will be re-injected)
	if paths["/path/to/file2.go"] {
		t.Error("/path/to/file2.go should not be included (stub result)")
	}
}

func TestExtractFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arguments string
		expected  string
	}{
		{`{"file_path": "/path/to/file.go"}`, "/path/to/file.go"},
		{`{"file_path":"/path/to/file.go"}`, "/path/to/file.go"},
		{`{"other_key": "value"}`, ""},
		{`{"file_path": 123}`, ""},
		{`{}`, ""},
	}

	for _, tc := range tests {
		// Call the internal function through CollectReadToolFilePaths
		// This is a simplified test - in real usage, CollectReadToolFilePaths handles it
		msg := services.CompactMessage{
			Type: "assistant",
			ToolCalls: []services.ToolCallContent{
				{ID: "tc1", Name: "Read", Arguments: tc.arguments},
			},
		}
		paths := services.CollectReadToolFilePaths([]services.CompactMessage{msg})

		if tc.expected == "" {
			if len(paths) != 0 {
				t.Errorf("expected no paths for %q, got %v", tc.arguments, paths)
			}
		} else {
			if !paths[tc.expected] {
				t.Errorf("expected %q in paths for %q, got %v", tc.expected, tc.arguments, paths)
			}
		}
	}
}

func TestGetUnchangedFilePaths(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{
			Type: "assistant",
			Role: "assistant",
			ToolCalls: []services.ToolCallContent{
				{ID: "tc1", Name: "Read", Arguments: `{"file_path": "/path/to/file1.go"}`},
				{ID: "tc2", Name: "Read", Arguments: `{"file_path": "/path/to/file2.go"}`},
			},
		},
		{
			Type: "user",
			Role: "user",
			ToolResults: []services.ToolResultContent{
				{ToolUseID: "tc1", Content: "content of file1"},
				{ToolUseID: "tc2", Content: "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."},
			},
		},
	}

	paths := services.GetUnchangedFilePaths(messages)

	// Only tc2 has stub, so only file2 should be in unchanged paths
	if paths["/path/to/file2.go"] != true {
		t.Error("expected /path/to/file2.go in unchanged paths (has stub)")
	}

	// file1 should not be in unchanged paths (has content)
	if paths["/path/to/file1.go"] {
		t.Error("/path/to/file1.go should not be in unchanged paths (has content)")
	}
}

func TestFileReadToolConstants(t *testing.T) {
	t.Parallel()

	if services.FileReadToolName != "Read" {
		t.Errorf("FileReadToolName should be 'Read', got %q", services.FileReadToolName)
	}

	if services.MaxLinesToRead != 2000 {
		t.Errorf("MaxLinesToRead should be 2000, got %d", services.MaxLinesToRead)
	}
}