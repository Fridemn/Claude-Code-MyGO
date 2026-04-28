package tests

import (
	"testing"

	"claude-go/internal/services"
)

func TestSessionMemoryCompactConfig(t *testing.T) {
	t.Parallel()

	config := services.DefaultSMCompactConfig()

	if config.MinTokens != 10000 {
		t.Errorf("MinTokens should be 10000, got %d", config.MinTokens)
	}
	if config.MinTextBlockMessages != 5 {
		t.Errorf("MinTextBlockMessages should be 5, got %d", config.MinTextBlockMessages)
	}
	if config.MaxTokens != 40000 {
		t.Errorf("MaxTokens should be 40000, got %d", config.MaxTokens)
	}
}

func TestHasTextBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg      services.CompactMessage
		expected bool
	}{
		{services.CompactMessage{Type: "user", Role: "user", Content: "hello"}, true},
		{services.CompactMessage{Type: "user", Role: "user", Content: ""}, false},
		{services.CompactMessage{Type: "assistant", Role: "assistant", Content: "response"}, true},
		{services.CompactMessage{Type: "assistant", Role: "assistant", Content: ""}, false},
		{services.CompactMessage{Type: "system", Role: "system", Content: "system"}, false},
	}

	for _, tc := range tests {
		result := services.HasTextBlocks(tc.msg)
		if result != tc.expected {
			t.Errorf("HasTextBlocks(%+v) = %v, want %v", tc.msg, result, tc.expected)
		}
	}
}

func TestGetToolResultIDs(t *testing.T) {
	t.Parallel()

	// User message with tool results
	msg := services.CompactMessage{
		Type: "user",
		Role: "user",
		ToolResults: []services.ToolResultContent{
			{ToolUseID: "tc1", Content: "result1"},
			{ToolUseID: "tc2", Content: "result2"},
		},
	}

	ids := services.GetToolResultIDs(msg)
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}

	// Assistant message - should return nil
	assistantMsg := services.CompactMessage{Type: "assistant", Role: "assistant"}
	ids = services.GetToolResultIDs(assistantMsg)
	if ids != nil {
		t.Error("assistant message should return nil")
	}
}

func TestHasToolUseWithIDs(t *testing.T) {
	t.Parallel()

	toolUseIDs := map[string]bool{"tc1": true, "tc2": true}

	// Assistant message with matching tool_use
	msg := services.CompactMessage{
		Type: "assistant",
		Role: "assistant",
		ToolCalls: []services.ToolCallContent{
			{ID: "tc1", Name: "Read"},
		},
	}

	if !services.HasToolUseWithIDs(msg, toolUseIDs) {
		t.Error("expected to find matching tool_use ID")
	}

	// Assistant message without matching tool_use
	msg2 := services.CompactMessage{
		Type: "assistant",
		Role: "assistant",
		ToolCalls: []services.ToolCallContent{
			{ID: "tc3", Name: "Write"},
		},
	}

	if services.HasToolUseWithIDs(msg2, toolUseIDs) {
		t.Error("should not find matching tool_use ID")
	}

	// User message - should return false
	userMsg := services.CompactMessage{Type: "user", Role: "user"}
	if services.HasToolUseWithIDs(userMsg, toolUseIDs) {
		t.Error("user message should return false")
	}
}

func TestAdjustIndexToPreserveAPIInvariants(t *testing.T) {
	t.Parallel()

	// Test basic case - no adjustment needed
	messages := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "u1"},
		{Type: "assistant", Role: "assistant", Content: "a1", MessageID: "m1"},
		{Type: "user", Role: "user", Content: "u2"},
		{Type: "assistant", Role: "assistant", Content: "a2", MessageID: "m2"},
	}

	result := services.AdjustIndexToPreserveAPIInvariants(messages, 2)
	if result != 2 {
		t.Errorf("expected index 2, got %d", result)
	}
}

func TestAdjustIndexToPreserveAPIInvariantsWithToolResults(t *testing.T) {
	t.Parallel()

	// Test tool_use/tool_result pair preservation
	// When we start at index 2, the kept range is [2, 3, 4]
	// Message at index 3 has tool_result for tc1, but tc1's tool_use is at index 0 (outside kept range)
	// So we should adjust to include index 0
	messages := []services.CompactMessage{
		{Type: "assistant", Role: "assistant", Content: "a1", MessageID: "m1",
			ToolCalls: []services.ToolCallContent{
				{ID: "tc1", Name: "Read"},
			},
		},
		{Type: "user", Role: "user", Content: "u1"},
		{Type: "assistant", Role: "assistant", Content: "a2", MessageID: "m2"},
		{Type: "user", Role: "user", Content: "u2",
			ToolResults: []services.ToolResultContent{
				{ToolUseID: "tc1", Content: "result"},
			},
		},
		{Type: "user", Role: "user", Content: "u3"},
	}

	// Start at index 2, tool_result at index 3 references tool_use at index 0
	result := services.AdjustIndexToPreserveAPIInvariants(messages, 2)
	// Should adjust to include the tool_use at index 0
	if result > 0 {
		t.Errorf("expected index <= 0 to preserve tool pair, got %d", result)
	}
}

func TestCalculateMessagesToKeepIndex(t *testing.T) {
	t.Parallel()

	config := services.DefaultSMCompactConfig()

	// Create messages
	messages := make([]services.CompactMessage, 20)
	for i := range messages {
		messages[i] = services.CompactMessage{
			Type:    "user",
			Role:    "user",
			Content: "message content that adds some tokens",
		}
	}

	// Calculate index
	result := services.CalculateMessagesToKeepIndex(messages, -1, config)

	if result < 0 || result > len(messages) {
		t.Errorf("index %d out of range [0, %d]", result, len(messages))
	}
}

func TestIsSessionMemoryEmpty(t *testing.T) {
	t.Parallel()

	// Empty template should match
	template := services.LoadSessionMemoryTemplate()
	if !services.IsSessionMemoryEmpty(template) {
		t.Error("template should be considered empty")
	}

	// Modified content should not match
	modified := template + "\nSome additional content"
	if services.IsSessionMemoryEmpty(modified) {
		t.Error("modified content should not be considered empty")
	}
}

func TestTruncateSessionMemoryForCompact(t *testing.T) {
	t.Parallel()

	// Content within limits
	shortContent := `# Section 1
Some content here.

# Section 2
More content.
`
	result := services.TruncateSessionMemoryForCompact(shortContent)
	if result.WasTruncated {
		t.Error("short content should not be truncated")
	}

	// Content exceeding limits
	var longSection string
	for i := 0; i < 10000; i++ {
		longSection += "x"
	}
	longContent := `# Section 1
` + longSection + `

# Section 2
Short content.
`
	result = services.TruncateSessionMemoryForCompact(longContent)
	if !result.WasTruncated {
		t.Error("long content should be truncated")
	}
	if !contains(result.TruncatedContent, "truncated for length") {
		t.Error("truncated content should have truncation marker")
	}
}

func TestCreateSessionMemoryCompactResult(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "hello"},
		{Type: "assistant", Role: "assistant", Content: "hi"},
	}

	sessionMemory := `# Session Title
Test session

# Current State
Working on tests
`

	messagesToKeep := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "recent message"},
	}

	result := services.CreateSessionMemoryCompactResult(messages, sessionMemory, messagesToKeep, "")

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if len(result.SummaryMessages) == 0 {
		t.Error("expected summary messages")
	}

	if result.PreCompactTokenCount <= 0 {
		t.Errorf("expected positive preCompactTokenCount, got %d", result.PreCompactTokenCount)
	}
}

func TestTrySessionMemoryCompaction(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "hello"},
		{Type: "assistant", Role: "assistant", Content: "hi"},
	}

	// With empty session memory path, should return nil
	result := services.TrySessionMemoryCompaction(messages, "", 100000)
	if result != nil {
		t.Error("expected nil for empty path")
	}
}

func TestGetSessionMemoryPath(t *testing.T) {
	t.Parallel()

	path := services.GetSessionMemoryPath("/home/user/.claude")
	expected := "/home/user/.claude/session-notes.md"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

// Helper function

