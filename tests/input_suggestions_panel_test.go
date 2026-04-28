package tests

import (
	"strings"
	"testing"
	"time"

	"claude-go/internal/bootstrap"
	"claude-go/internal/ui"
)

func TestRenderInputPanelHighlightsSelectedSuggestion(t *testing.T) {
	panel := ui.RenderInputPanel(80, bootstrap.State{}, "/he", 3, []ui.SlashSuggestion{
		{Command: "/help", Description: "show help"},
		{Command: "/hello", Description: "say hello"},
	}, 1, false, 0, "", "", time.Time{}, "", 0, 0, nil, "", 0, "", "")

	if !strings.Contains(panel, "› ") {
		t.Fatalf("expected selected suggestion indicator, got:\n%s", panel)
	}
	if !strings.Contains(panel, "/hello") {
		t.Fatalf("expected selected command to render, got:\n%s", panel)
	}
}
