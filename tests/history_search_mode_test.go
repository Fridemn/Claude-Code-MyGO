package tests

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"claude-go/internal/ui"
	"claude-go/internal/ui/messages"
)

func TestModelCtrlRHistorySearchAcceptsMatch(t *testing.T) {
	m := ui.ModelFor()
	modelAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	mm := modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = modelAny.(ui.Model)

	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = modelAny.(ui.Model)

	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = modelAny.(ui.Model)

	out := mm.View()
	if !strings.Contains(out, "first") {
		t.Fatalf("expected accepted history match in input, got:\n%s", out)
	}
}

func TestModelCtrlRHistorySearchCancelRestoresDraft(t *testing.T) {
	m := ui.ModelFor()
	m.AddMessage(messages.Message{Type: messages.MessageTypeAssistant, Content: "x"})
	modelAny, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm := modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	mm = modelAny.(ui.Model)

	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	mm = modelAny.(ui.Model)
	modelAny, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = modelAny.(ui.Model)

	out := mm.View()
	if !strings.Contains(out, "draft") {
		t.Fatalf("expected draft restored after cancel, got:\n%s", out)
	}
}

