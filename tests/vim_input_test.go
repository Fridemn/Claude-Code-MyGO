package tests

import (
	"testing"

	"claude-go/internal/ui/input"
)

func TestPromptInputVimDisabledByDefault(t *testing.T) {
	s := input.PromptInputStateFor(40)
	if s.VimEnabled {
		t.Fatalf("expected vim mode disabled by default")
	}
}

func TestPromptInputVimNormalInsertFlow(t *testing.T) {
	s := input.PromptInputStateFor(40)
	s.EnableVimMode(true)
	s.SetValue("hello")
	s.MoveEnd()

	handled, swallow := s.HandleVimKey("esc")
	if !handled || !swallow || s.VimInsertMode {
		t.Fatalf("expected esc to enter normal mode")
	}

	handled, swallow = s.HandleVimKey("h")
	if !handled || !swallow {
		t.Fatalf("expected h to be handled in normal mode")
	}

	before := s.CursorPos
	handled, swallow = s.HandleVimKey("i")
	if !handled || !swallow || !s.VimInsertMode {
		t.Fatalf("expected i to enter insert mode")
	}
	if s.CursorPos != before {
		t.Fatalf("expected cursor unchanged on i")
	}
}

