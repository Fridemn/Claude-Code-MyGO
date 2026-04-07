package tests

import (
	"testing"

	"claude-code-go/internal/ui/input"
)

func TestPromptInputHistoryPrevNext(t *testing.T) {
	s := input.PromptInputStateFor(40)
	s.AddHistory("first")
	s.AddHistory("second")

	if !s.PrevHistory() || s.Value != "second" {
		t.Fatalf("expected first prev to load latest history item, got %q", s.Value)
	}
	if !s.PrevHistory() || s.Value != "first" {
		t.Fatalf("expected second prev to load older history item, got %q", s.Value)
	}
	if !s.NextHistory() || s.Value != "second" {
		t.Fatalf("expected next to move forward in history, got %q", s.Value)
	}
	if !s.NextHistory() || s.Value != "" {
		t.Fatalf("expected next at tail to restore draft, got %q", s.Value)
	}
}

func TestPromptInputHistoryDedupesConsecutive(t *testing.T) {
	s := input.PromptInputStateFor(40)
	s.AddHistory("same")
	s.AddHistory("same")
	if len(s.History) != 1 {
		t.Fatalf("expected consecutive duplicate not to be added, len=%d", len(s.History))
	}
}

