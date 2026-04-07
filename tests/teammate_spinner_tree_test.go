package tests

import (
	"strings"
	"testing"

	"claude-code-go/internal/ui/components"
)

func TestRenderTeammateSpinnerTreeSelection(t *testing.T) {
	t.Parallel()

	out := components.RenderTeammateSpinnerTree(components.TeammateSpinnerTreeConfig{
		Width:            100,
		SelectionMode:    true,
		SelectedIndex:    -1,
		LeaderVerb:       "analyzing",
		LeaderTokenCount: 1200,
		Teammates: []components.TeammateTask{
			{Name: "researcher", Activity: "searching", ToolUseCount: 2, TokenCount: 530},
			{Name: "coder", Activity: "editing", ToolUseCount: 1, TokenCount: 300},
		},
	})

	if !strings.Contains(out, "team-lead") {
		t.Fatalf("expected leader row, got: %q", out)
	}
	if !strings.Contains(out, "❯ ╭═ team-lead") {
		t.Fatalf("expected selected leader glyph, got: %q", out)
	}
	if !strings.Contains(out, "@researcher") {
		t.Fatalf("expected teammate name, got: %q", out)
	}
	if !strings.Contains(out, "1,200 tokens") {
		t.Fatalf("expected formatted tokens, got: %q", out)
	}
	if !strings.Contains(out, "hide") {
		t.Fatalf("expected hide row in selection mode, got: %q", out)
	}
}

func TestRenderTeammateSpinnerTreeResponsiveNameHide(t *testing.T) {
	t.Parallel()

	out := components.RenderTeammateSpinnerTree(components.TeammateSpinnerTreeConfig{
		Width:          48,
		SelectionMode:  false,
		SelectedIndex:  0,
		LeaderIdleText: "idle for 3s",
		Teammates: []components.TeammateTask{
			{Name: "long-agent-name", Activity: "working", ToolUseCount: 2, TokenCount: 1500},
		},
	})

	if strings.Contains(out, "@long-agent-name") {
		t.Fatalf("expected teammate name hidden on narrow width, got: %q", out)
	}
	if !strings.Contains(out, "working") {
		t.Fatalf("expected activity still shown, got: %q", out)
	}
}
