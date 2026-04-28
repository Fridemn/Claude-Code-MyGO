package tests

import (
	"strings"
	"testing"

	"claude-go/internal/ui/input"
)

func TestRenderInputHighlightsCommandAndMention(t *testing.T) {
	s := input.InputStateFor("", 80)
	s.IsFocused = true
	s.SetValue("/help @agent")
	s.MoveEnd()

	out := input.RenderInput(s)
	if !strings.Contains(out, "/help") {
		t.Fatalf("expected /help in output")
	}
	if !strings.Contains(out, "@agent") {
		t.Fatalf("expected @agent in output")
	}
	if !strings.Contains(out, "\x1b[38;2;177;185;249m/help\x1b[0m") {
		t.Fatalf("expected command token to be highlighted, got: %q", out)
	}
}

