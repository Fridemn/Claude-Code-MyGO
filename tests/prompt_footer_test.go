package tests

import (
	"strings"
	"testing"

	"claude-go/internal/ui/input"
)

func TestRenderPromptFooterStatusLeftHintsRight(t *testing.T) {
	got := input.RenderPromptFooter(40, []string{"Enter send", "Esc cancel"}, "INSERT")
	if !strings.Contains(got, "INSERT") || !strings.Contains(got, "Enter send") {
		t.Fatalf("expected both status and hints, got %q", got)
	}
	if strings.Index(got, "INSERT") > strings.Index(got, "Enter send") {
		t.Fatalf("expected status on left and hints on right, got %q", got)
	}
}

func TestRenderPromptFooterTruncatesWhenNarrow(t *testing.T) {
	got := input.RenderPromptFooter(14, []string{"Enter send", "Esc cancel"}, "NORMAL")
	if !strings.Contains(got, "NORMAL") {
		t.Fatalf("expected status preserved under narrow width, got %q", got)
	}
}

func TestRenderPromptFooterHandlesSingleSide(t *testing.T) {
	leftOnly := input.RenderPromptFooter(20, nil, "SEARCH")
	if !strings.Contains(leftOnly, "SEARCH") {
		t.Fatalf("expected left-only status render, got %q", leftOnly)
	}
	rightOnly := input.RenderPromptFooter(20, []string{"? help"}, "")
	if !strings.Contains(rightOnly, "? help") {
		t.Fatalf("expected right-only hints render, got %q", rightOnly)
	}
}

