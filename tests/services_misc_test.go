package tests

import (
	"context"
	"testing"

	"claude-go/internal/services"
	"claude-go/internal/types"
)

func TestCompactServiceKeepsTailWindow(t *testing.T) {
	t.Parallel()

	svc := services.EmptyCompactService()
	messages := []services.CompactMessage{
		{Role: types.RoleUser, Content: "1"},
		{Role: types.RoleAssistant, Content: "2"},
		{Role: types.RoleUser, Content: "3"},
	}

	all, err := svc.Compact(context.Background(), messages, "", false)
	if err != nil {
		t.Fatalf("compact failed: %v", err)
	}
	if len(all.SummaryMessages) < 1 {
		t.Fatalf("expected summary messages, got %d", len(all.SummaryMessages))
	}
}

func TestAgentSummaryServicePrefersLastAssistantMessage(t *testing.T) {
	t.Parallel()

	svc := services.EmptyAgentSummaryService()
	messages := []services.CompactMessage{
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Type: services.MessageTypeAssistant, Content: "first"},
		{Role: types.RoleUser, Content: "follow-up"},
		{Role: types.RoleAssistant, Type: services.MessageTypeAssistant, Content: "final"},
	}
	summary := svc.SummarizeMessages(context.Background(), messages, "fallback")
	if summary != "final" {
		t.Fatalf("unexpected summary: %q", summary)
	}

	summary = svc.SummarizeMessages(context.Background(), []services.CompactMessage{{Role: types.RoleUser, Content: "hello"}}, "fallback")
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
	if out := svc.SummarizeSingleTool("grep", "boom", true); out != "grep failed: boom" {
		t.Fatalf("unexpected error summary: %q", out)
	}
	if out := svc.SummarizeSingleTool("grep", "", false); out != "grep completed" {
		t.Fatalf("unexpected empty summary: %q", out)
	}
	if out := svc.SummarizeSingleTool("grep", "2 matches", false); out != "grep: 2 matches" {
		t.Fatalf("unexpected content summary: %q", out)
	}
}

