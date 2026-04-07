package tests

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"claude-code-go/internal/ui"
	"claude-code-go/internal/ui/messages"
)

func TestMessageClickabilityRule(t *testing.T) {
	if !messages.IsClickableForExpand(messages.Message{Type: messages.MessageTypeCollapsed}) {
		t.Fatalf("collapsed message must be clickable")
	}
	if !messages.IsClickableForExpand(messages.Message{Type: messages.MessageTypeToolResult, IsTruncated: true, IsError: false}) {
		t.Fatalf("truncated successful tool result must be clickable")
	}
	if messages.IsClickableForExpand(messages.Message{Type: messages.MessageTypeToolResult, IsTruncated: true, IsError: true}) {
		t.Fatalf("error tool result must not be clickable")
	}
	if messages.IsClickableForExpand(messages.Message{Type: messages.MessageTypeAssistant}) {
		t.Fatalf("assistant plain text must not be clickable")
	}
	if !messages.IsClickableForExpand(messages.Message{Type: messages.MessageTypeAssistant, IsAdvisorResult: true}) {
		t.Fatalf("assistant advisor result must be clickable")
	}
}

func TestModelScrollKeysStillWork(t *testing.T) {
	m := ui.ModelFor()
	for i := 0; i < 50; i++ {
		m.AddMessage(messages.Message{
			Type:    messages.MessageTypeAssistant,
			Content: "message line",
		})
	}

	modelAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := modelAny.(ui.Model)
	if model.View() == "" {
		t.Fatalf("expected non-empty view after key update")
	}

	modelAny, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = modelAny.(ui.Model)
	if model.View() == "" {
		t.Fatalf("expected non-empty view after page-down update")
	}
}

func TestModelViewWithZoneMarkers(t *testing.T) {
	m := ui.ModelFor()
	m.AddMessage(messages.Message{
		Type:    messages.MessageTypeAssistant,
		Content: "hello",
	})
	out := m.View()
	if out == "" {
		t.Fatalf("expected non-empty view")
	}
}
