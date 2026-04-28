package tests

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"claude-go/internal/ui"
	"claude-go/internal/ui/dialogs"
)

func TestQuickOpenTabInsertsMention(t *testing.T) {
	s := dialogs.QuickOpenStateFor(100, 30)
	s.ListFiles = func() ([]string, error) {
		return []string{"src/main.tsx", "README.md"}, nil
	}
	if err := s.ReloadForTest(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	_ = s.HandleKey("s")
	act := s.HandleKey("tab")
	if !act.Done {
		t.Fatalf("expected tab to close quick open")
	}
	if !strings.Contains(act.Insert, "@src/main.tsx") {
		t.Fatalf("expected mention insert, got %q", act.Insert)
	}
}

func TestQuickOpenOverlayInModelFlow(t *testing.T) {
	m := ui.ModelFor()
	modelAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	model := modelAny.(ui.Model)
	out := model.View()
	if !strings.Contains(out, "Quick Open") {
		t.Fatalf("expected quick open overlay, got:\n%s", out)
	}
}

