package tests

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"claude-go/internal/ui"
	"claude-go/internal/ui/dialogs"
)

func TestModelPickerSelectsFocusedModel(t *testing.T) {
	s := dialogs.ModelPickerStateFor("gpt-4.1")
	_ = s.HandleKey("down")
	act := s.HandleKey("enter")
	if !act.Done {
		t.Fatalf("expected picker to close on enter")
	}
	if act.Selected == "" {
		t.Fatalf("expected selected model on enter")
	}
}

func TestModelPickerEscCancels(t *testing.T) {
	s := dialogs.ModelPickerStateFor("gpt-4.1")
	act := s.HandleKey("esc")
	if !act.Done {
		t.Fatalf("expected esc to close picker")
	}
	if act.Selected != "" {
		t.Fatalf("expected no selection on esc")
	}
}

func TestModelPickerOverlayInModelFlow(t *testing.T) {
	m := ui.ModelFor()
	modelAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	model := modelAny.(ui.Model)
	out := model.View()
	if !strings.Contains(out, "Select model") {
		t.Fatalf("expected model picker view, got:\n%s", out)
	}
}
