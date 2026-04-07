package tests

import (
	"strings"
	"testing"
	"time"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/ui"
)

func TestRenderInputPanelHighlightsSelectedSuggestion(t *testing.T) {
	panel := ui.RenderInputPanel(80, bootstrap.State{}, "/he", []ui.SlashSuggestion{
		{Command: "/help", Description: "show help"},
		{Command: "/hello", Description: "say hello"},
	}, 1, false, 0, "", "", time.Time{}, "", 0, 0, nil, "", 0)

	if !strings.Contains(panel, "› ") {
		t.Fatalf("expected selected suggestion indicator, got:\n%s", panel)
	}
	if !strings.Contains(panel, "/hello") {
		t.Fatalf("expected selected command to render, got:\n%s", panel)
	}
}
