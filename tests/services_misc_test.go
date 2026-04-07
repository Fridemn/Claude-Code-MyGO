package tests

import (
	"context"
	"testing"

	"claude-code-go/internal/services"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

func TestCompactServiceKeepsTailWindow(t *testing.T) {
	t.Parallel()

	svc := services.EmptyCompactService()
	messages := []types.Message{
		{Role: types.RoleUser, Content: "1"},
		{Role: types.RoleAssistant, Content: "2"},
		{Role: types.RoleUser, Content: "3"},
	}

	all, err := svc.Compact(context.Background(), messages, 10, "", false)
	if err != nil {
		t.Fatalf("compact failed: %v", err)
	}
	if len(all.SummaryMessages) != 3 {
		t.Fatalf("expected all messages when below limit, got %d", len(all.SummaryMessages))
	}

	compacted, err := svc.Compact(context.Background(), messages, 2, "", false)
	if err != nil {
		t.Fatalf("compact failed: %v", err)
	}
	// After compacting to keep 2 messages, we should have 1 summary + 2 kept = 3 messages total
	// But the last 2 messages should be in MessagesToKeep
	if len(compacted.MessagesToKeep) != 2 {
		t.Fatalf("expected 2 messages kept, got %d", len(compacted.MessagesToKeep))
	}
}

func TestAgentSummaryServicePrefersLastAssistantMessage(t *testing.T) {
	t.Parallel()

	svc := services.EmptyAgentSummaryService()
	messages := []types.Message{
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Content: "first"},
		{Role: types.RoleUser, Content: "follow-up"},
		{Role: types.RoleAssistant, Content: "final"},
	}
	summary := svc.Summarize(messages, "fallback")
	if summary != "final" {
		t.Fatalf("unexpected summary: %q", summary)
	}

	summary = svc.Summarize([]types.Message{{Role: types.RoleUser, Content: "hello"}}, "fallback")
	if summary != "fallback" {
		t.Fatalf("expected fallback summary, got %q", summary)
	}
}

func TestPromptSuggestionService(t *testing.T) {
	t.Parallel()

	svc := services.EmptyPromptSuggestionService()
	if out := svc.Suggest("   "); out != nil {
		t.Fatalf("expected nil suggestions for empty input, got %#v", out)
	}

	out := svc.Suggest("refactor this module")
	if len(out) != 3 {
		t.Fatalf("expected 3 prompt suggestions, got %#v", out)
	}
}

func TestSessionMemoryServiceSnapshotCopiesMessages(t *testing.T) {
	t.Parallel()

	svc := services.CreateSessionMemoryService()
	original := []types.Message{
		{Role: types.RoleUser, Content: "hello"},
	}

	snapshot := svc.Snapshot(original)
	if len(snapshot) != 1 || snapshot[0].Content != "hello" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}

	snapshot[0].Content = "changed"
	if original[0].Content != "hello" {
		t.Fatalf("expected snapshot to be independent of original slice")
	}
}

func TestToolUseSummaryService(t *testing.T) {
	t.Parallel()

	svc := services.EmptyToolUseSummaryService()
	if out := svc.Summarize("grep", tool.Result{Error: "boom"}); out != "grep failed: boom" {
		t.Fatalf("unexpected error summary: %q", out)
	}
	if out := svc.Summarize("grep", tool.Result{Content: ""}); out != "grep completed" {
		t.Fatalf("unexpected empty summary: %q", out)
	}
	if out := svc.Summarize("grep", tool.Result{Content: "2 matches"}); out != "grep completed: 2 matches" {
		t.Fatalf("unexpected content summary: %q", out)
	}
}

