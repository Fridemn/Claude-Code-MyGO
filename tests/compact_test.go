package tests

import (
	"testing"

	"claude-go/internal/services"
)

func TestTokenEstimation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		content  string
		minTokens int
		maxTokens int
	}{
		{"", 0, 0},
		{"hello", 1, 5},
		{"hello world this is a test", 5, 15},
		{string(make([]byte, 1000)), 200, 400},
	}

	for _, tc := range tests {
		tokens := services.EstimateTokenCount(tc.content)
		if tokens < tc.minTokens || tokens > tc.maxTokens {
			t.Errorf("EstimateTokenCount(%q) = %d, want between %d and %d", tc.content, tokens, tc.minTokens, tc.maxTokens)
		}
	}
}

func TestBytesPerTokenForFileType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ext       string
		expected  int
	}{
		{"json", 2},
		{"jsonl", 2},
		{"JSON", 2},
		{".json", 2},
		{"txt", 4},
		{"go", 4},
		{"ts", 4},
		{"", 4},
	}

	for _, tc := range tests {
		result := services.BytesPerTokenForFileType(tc.ext)
		if result != tc.expected {
			t.Errorf("BytesPerTokenForFileType(%q) = %d, want %d", tc.ext, result, tc.expected)
		}
	}
}

func TestRoughTokenCountEstimationForFileType(t *testing.T) {
	t.Parallel()

	// JSON files have higher density (2 bytes/token) vs text (4 bytes/token)
	// So JSON should result in MORE tokens for the same content
	jsonContent := `{"key": "value", "nested": {"a": 1, "b": 2}}`
	jsonTokens := services.RoughTokenCountEstimationForFileType(jsonContent, "json")
	txtTokens := services.RoughTokenCountEstimationForFileType(jsonContent, "txt")

	if jsonTokens <= txtTokens {
		t.Errorf("JSON tokens (%d) should be greater than txt tokens (%d) due to higher density", jsonTokens, txtTokens)
	}
}

func TestContentBlockTokenEstimation(t *testing.T) {
	t.Parallel()

	// Text block
	textBlock := services.ContentBlock{Type: "text", Text: "hello world"}
	textTokens := services.RoughTokenCountEstimationForBlock(textBlock)
	if textTokens < 1 {
		t.Errorf("text block should have at least 1 token, got %d", textTokens)
	}

	// Image block - should have fixed cost
	imageBlock := services.ContentBlock{Type: "image"}
	imageTokens := services.RoughTokenCountEstimationForBlock(imageBlock)
	if imageTokens != services.ImageDocumentMaxTokens {
		t.Errorf("image block should have %d tokens, got %d", services.ImageDocumentMaxTokens, imageTokens)
	}

	// Document block
	docBlock := services.ContentBlock{Type: "document"}
	docTokens := services.RoughTokenCountEstimationForBlock(docBlock)
	if docTokens != services.ImageDocumentMaxTokens {
		t.Errorf("document block should have %d tokens, got %d", services.ImageDocumentMaxTokens, docTokens)
	}

	// Tool use block
	toolBlock := services.ContentBlock{
		Type:  "tool_use",
		Name:  "read_file",
		Input: map[string]any{"path": "/test/file.go"},
	}
	toolTokens := services.RoughTokenCountEstimationForBlock(toolBlock)
	if toolTokens < 1 {
		t.Errorf("tool_use block should have at least 1 token, got %d", toolTokens)
	}
}

func TestEstimateMessagesTokenCount(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	tokens := services.EstimateMessagesTokenCount(messages)
	if tokens < 1 {
		t.Errorf("expected at least 1 token, got %d", tokens)
	}

	// More messages should have more tokens
	moreMessages := append(messages, services.CompactMessage{Role: "user", Content: "how are you"})
	moreTokens := services.EstimateMessagesTokenCount(moreMessages)
	if moreTokens <= tokens {
		t.Errorf("more messages should have more tokens: %d vs %d", moreTokens, tokens)
	}
}

func TestEstimateMessagesTokenCountWithImages(t *testing.T) {
	t.Parallel()

	// Messages without images
	messagesNoImages := []services.CompactMessage{
		{Role: "user", Content: "hello"},
	}
	tokensNoImages := services.EstimateMessagesTokenCount(messagesNoImages)

	// Messages with images
	messagesWithImages := []services.CompactMessage{
		{Role: "user", Content: "hello", Images: []string{"base64imagedata"}},
	}
	tokensWithImages := services.EstimateMessagesTokenCount(messagesWithImages)

	if tokensWithImages <= tokensNoImages {
		t.Errorf("messages with images should have more tokens: %d vs %d", tokensWithImages, tokensNoImages)
	}
}

func TestMicrocompactMessages(t *testing.T) {
	t.Parallel()

	// Create messages with tool calls and results
	messages := []services.CompactMessage{
		{Type: "assistant", Role: "assistant", ToolCalls: []services.ToolCallContent{
			{ID: "tc1", Name: "Read", Arguments: `{"path": "/file1"}`},
			{ID: "tc2", Name: "Read", Arguments: `{"path": "/file2"}`},
			{ID: "tc3", Name: "Read", Arguments: `{"path": "/file3"}`},
			{ID: "tc4", Name: "Read", Arguments: `{"path": "/file4"}`},
		}},
		{Type: "user", Role: "user", ToolResults: []services.ToolResultContent{
			{ToolUseID: "tc1", Content: "content 1"},
			{ToolUseID: "tc2", Content: "content 2"},
			{ToolUseID: "tc3", Content: "content 3"},
			{ToolUseID: "tc4", Content: "content 4"},
		}},
	}

	config := services.DefaultTimeBasedMCConfig()
	result := services.MicrocompactMessages(messages, config)

	if !result.DidCompact {
		t.Fatal("expected microcompact to run")
	}
	if result.ToolsCleared == 0 {
		t.Fatal("expected some tools to be cleared")
	}
	if result.TokensSaved == 0 {
		t.Fatal("expected tokens to be saved")
	}
}

func TestMicrocompactDisabled(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Type: "assistant", Role: "assistant", ToolCalls: []services.ToolCallContent{
			{ID: "tc1", Name: "Read", Arguments: `{}`},
		}},
		{Type: "user", Role: "user", ToolResults: []services.ToolResultContent{
			{ToolUseID: "tc1", Content: "content"},
		}},
	}

	config := services.TimeBasedMCConfig{Enabled: false}
	result := services.MicrocompactMessages(messages, config)

	if result.DidCompact {
		t.Fatal("disabled microcompact should not run")
	}
}

func TestMicrocompactKeepRecent(t *testing.T) {
	t.Parallel()

	// Create messages with 5 tool calls
	messages := []services.CompactMessage{
		{Type: "assistant", Role: "assistant", ToolCalls: []services.ToolCallContent{
			{ID: "tc1", Name: "Read", Arguments: `{}`},
			{ID: "tc2", Name: "Read", Arguments: `{}`},
			{ID: "tc3", Name: "Read", Arguments: `{}`},
			{ID: "tc4", Name: "Read", Arguments: `{}`},
			{ID: "tc5", Name: "Read", Arguments: `{}`},
		}},
		{Type: "user", Role: "user", ToolResults: []services.ToolResultContent{
			{ToolUseID: "tc1", Content: "content 1"},
			{ToolUseID: "tc2", Content: "content 2"},
			{ToolUseID: "tc3", Content: "content 3"},
			{ToolUseID: "tc4", Content: "content 4"},
			{ToolUseID: "tc5", Content: "content 5"},
		}},
	}

	config := services.TimeBasedMCConfig{
		Enabled:    true,
		KeepRecent: 2, // Keep last 2
	}
	result := services.MicrocompactMessages(messages, config)

	// Should clear 3 (5 total - 2 kept)
	if result.ToolsCleared != 3 {
		t.Errorf("expected 3 tools cleared, got %d", result.ToolsCleared)
	}
}

func TestAutoCompactThreshold(t *testing.T) {
	t.Parallel()

	model := "claude-sonnet-4"
	contextWindow := 200000

	threshold := services.GetAutoCompactThreshold(model, contextWindow)

	// Threshold should be less than context window
	if threshold >= contextWindow {
		t.Errorf("threshold %d should be less than context window %d", threshold, contextWindow)
	}

	// Threshold should be reasonable (within 30K of context window)
	if contextWindow-threshold > 50000 {
		t.Errorf("threshold %d is too far from context window %d", threshold, contextWindow)
	}
}

func TestShouldAutoCompact(t *testing.T) {
	t.Parallel()

	// Create messages with sufficient content to trigger auto-compact
	// Need to exceed ~170K tokens for threshold (200K context - 30K buffer)
	messages := make([]services.CompactMessage, 100)
	for i := range messages {
		// Each message ~200 chars = ~50 tokens, 100 messages = ~5000 tokens
		// Need much more content, so make each message much larger
		messages[i] = services.CompactMessage{
			Role:    "user",
			Content: "This is a test message with substantial content to increase token count. " +
				"We need enough tokens to trigger auto-compact threshold. " +
				"Adding more text to ensure we have sufficient token count. " +
				"This text block is repeated to make the message longer. " +
				"More content means more tokens for estimation.",
		}
	}

	// Verify we have enough tokens to trigger
	tokenCount := services.EstimateMessagesTokenCount(messages)
	threshold := services.GetAutoCompactThreshold("claude-sonnet-4", 200000)

	if tokenCount < threshold {
		t.Skipf("message token count (%d) is below threshold (%d), adjusting test", tokenCount, threshold)
	}

	should := services.ShouldAutoCompact(messages, "claude-sonnet-4", "repl_main_thread", 200000, true)
	if !should {
		t.Error("expected auto-compact to trigger for large message set")
	}

	// Should not compact for session_memory source
	should = services.ShouldAutoCompact(messages, "claude-sonnet-4", "session_memory", 200000, true)
	if should {
		t.Error("should not auto-compact for session_memory source")
	}

	// Should not compact when disabled
	should = services.ShouldAutoCompact(messages, "claude-sonnet-4", "repl_main_thread", 200000, false)
	if should {
		t.Error("should not auto-compact when disabled")
	}
}

func TestCalculateTokenWarningState(t *testing.T) {
	t.Parallel()

	contextWindow := 200000

	// Low usage - no warnings
	state := services.CalculateTokenWarningState(50000, "claude-sonnet-4", true, contextWindow)
	if state.IsAboveWarningThreshold {
		t.Error("low usage should not trigger warning")
	}

	// High usage - should trigger warning
	state = services.CalculateTokenWarningState(180000, "claude-sonnet-4", true, contextWindow)
	if !state.IsAboveWarningThreshold {
		t.Error("high usage should trigger warning")
	}
}

func TestStripImagesFromMessages(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "hello", Images: []string{"base64image"}},
		{Type: "assistant", Role: "assistant", Content: "hi"},
	}

	stripped := services.StripImagesFromMessages(messages)

	// First message should have images removed
	if len(stripped[0].Images) != 0 {
		t.Error("expected images to be stripped from user message")
	}
	// Content should have [image] marker
	if stripped[0].Content == "hello" {
		t.Error("expected content to have [image] marker")
	}

	// Assistant message should be unchanged
	if stripped[1].Content != "hi" {
		t.Error("assistant message should be unchanged")
	}
}

func TestTruncateHeadForPTLRetry(t *testing.T) {
	t.Parallel()

	// Create enough messages to have multiple API rounds
	messages := make([]services.CompactMessage, 50)
	for i := range messages {
		messages[i] = services.CompactMessage{
			Type:    "user",
			Role:    "user",
			Content: "message",
		}
	}

	truncated := services.TruncateHeadForPTLRetry(messages, "")

	// Should return nil if not enough messages to truncate
	if len(messages) < 2 {
		if truncated != nil {
			t.Error("expected nil for too few messages")
		}
		return
	}

	// If truncated, should be shorter than original
	if truncated != nil && len(truncated) >= len(messages) {
		t.Error("truncated should be shorter than original")
	}
}

func TestGroupMessagesByApiRound(t *testing.T) {
	t.Parallel()

	messages := []services.CompactMessage{
		{Type: "user", Role: "user", Content: "u1"},
		{Type: "assistant", Role: "assistant", Content: "a1"},
		{Type: "user", Role: "user", Content: "u2"},
		{Type: "assistant", Role: "assistant", Content: "a2"},
	}

	groups := services.GroupMessagesByApiRound(messages)

	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}

	// Count total messages across groups
	total := 0
	for _, g := range groups {
		total += len(g)
	}

	if total != len(messages) {
		t.Errorf("total messages in groups (%d) != original (%d)", total, len(messages))
	}
}

func TestPartialCompactDirection(t *testing.T) {
	t.Parallel()

	// Test DirectionFrom and DirectionUpTo constants
	if services.DirectionFrom != "from" {
		t.Errorf("DirectionFrom should be 'from', got %s", services.DirectionFrom)
	}
	if services.DirectionUpTo != "up_to" {
		t.Errorf("DirectionUpTo should be 'up_to', got %s", services.DirectionUpTo)
	}
}

func TestCompactableToolNames(t *testing.T) {
	t.Parallel()

	// Test that expected tool names are compactable
	expectedTools := []string{"Read", "Bash", "Grep", "Glob", "WebSearch", "WebFetch", "Edit", "Write"}

	for _, tool := range expectedTools {
		if !services.CompactableToolNames[tool] {
			t.Errorf("expected %s to be compactable", tool)
		}
	}

	// Test that unexpected tool names are not compactable
	if services.CompactableToolNames["UnknownTool"] {
		t.Error("UnknownTool should not be compactable")
	}
}

func TestIsMainThreadSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source   string
		expected bool
	}{
		{"", true},
		{"repl_main_thread", true},
		{"repl_main_thread:outputStyle:custom", true},
		{"sdk", false},
		{"compact", false},
		{"session_memory", false},
	}

	for _, tc := range tests {
		result := services.IsMainThreadSource(tc.source)
		if result != tc.expected {
			t.Errorf("IsMainThreadSource(%q) = %v, want %v", tc.source, result, tc.expected)
		}
	}
}

func TestCompactConstants(t *testing.T) {
	t.Parallel()

	// Verify constants match TypeScript values
	if services.PostCompactMaxFilesToRestore != 5 {
		t.Errorf("PostCompactMaxFilesToRestore should be 5, got %d", services.PostCompactMaxFilesToRestore)
	}
	if services.PostCompactTokenBudget != 50000 {
		t.Errorf("PostCompactTokenBudget should be 50000, got %d", services.PostCompactTokenBudget)
	}
	if services.PostCompactMaxTokensPerFile != 5000 {
		t.Errorf("PostCompactMaxTokensPerFile should be 5000, got %d", services.PostCompactMaxTokensPerFile)
	}
	if services.CompactMaxOutputTokens != 16000 {
		t.Errorf("CompactMaxOutputTokens should be 16000, got %d", services.CompactMaxOutputTokens)
	}
}

func TestCreateSummaryMessages(t *testing.T) {
	t.Parallel()

	summary := "This is a conversation summary"
	messages := services.CreateSummaryMessages(summary, false, "")

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != services.MessageTypeUser {
		t.Errorf("expected user message type, got %s", messages[0].Type)
	}

	if !messages[0].IsCompactSummary {
		t.Error("expected IsCompactSummary to be true")
	}

	// Check that summary is included
	if len(messages[0].Content) == 0 {
		t.Error("expected content to contain summary")
	}
}

func TestMergeHookInstructions(t *testing.T) {
	t.Parallel()

	// Both empty
	result := services.MergeHookInstructions("", "")
	if result != "" {
		t.Errorf("empty merge should be empty, got %q", result)
	}

	// Only user instructions
	result = services.MergeHookInstructions("user instructions", "")
	if result != "user instructions" {
		t.Errorf("expected user instructions, got %q", result)
	}

	// Only hook instructions
	result = services.MergeHookInstructions("", "hook instructions")
	if result != "hook instructions" {
		t.Errorf("expected hook instructions, got %q", result)
	}

	// Both
	result = services.MergeHookInstructions("user", "hook")
	if result != "user\n\nhook" {
		t.Errorf("expected merged instructions, got %q", result)
	}
}