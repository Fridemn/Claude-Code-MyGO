package components

import (
	"fmt"
	"strings"
)

// KeyboardShortcutHint renders a keyboard shortcut hint
// Matches src/components/design-system/KeyboardShortcutHint.tsx
type KeyboardShortcutHint struct {
	Shortcut   string // e.g., "Enter", "Ctrl+C", "Esc"
	Action     string // e.g., "confirm", "cancel", "exit"
	ShowParens bool   // wrap in parentheses
}

// Render renders the keyboard shortcut hint
func (h KeyboardShortcutHint) Render() string {
	shortcutText := h.formatShortcut()

	if h.Action != "" {
		result := fmt.Sprintf("%s to %s", shortcutText, h.Action)
		if h.ShowParens {
			return "(" + result + ")"
		}
		return result
	}

	if h.ShowParens {
		return "(" + shortcutText + ")"
	}
	return shortcutText
}

// formatShortcut formats the shortcut key for display
func (h KeyboardShortcutHint) formatShortcut() string {
	// Format common shortcuts nicely
	switch strings.ToLower(h.Shortcut) {
	case "ctrl+c", "^c":
		return "Ctrl+C"
	case "ctrl+d", "^d":
		return "Ctrl+D"
	case "ctrl+o", "^o":
		return "Ctrl+O"
	case "ctrl+r", "^r":
		return "Ctrl+R"
	case "ctrl+z", "^z":
		return "Ctrl+Z"
	case "escape", "esc":
		return "Esc"
	case "enter", "return":
		return "Enter"
	case "space":
		return "Space"
	case "tab":
		return "Tab"
	case "backspace":
		return "Backspace"
	case "delete", "del":
		return "Delete"
	case "up", "arrowup":
		return "↑"
	case "down", "arrowdown":
		return "↓"
	case "left", "arrowleft":
		return "←"
	case "right", "arrowright":
		return "→"
	default:
		return h.Shortcut
	}
}

// RenderByline renders a series of keyboard hints separated by "·"
// Matches src/components/design-system/Byline.tsx
func RenderByline(hints []KeyboardShortcutHint) string {
	var parts []string
	for _, hint := range hints {
		parts = append(parts, hint.Render())
	}
	return strings.Join(parts, " · ")
}

// ConfigurableShortcutHint renders a shortcut hint with configurable action
// Matches src/components/ConfigurableShortcutHint.tsx
type ConfigurableShortcutHint struct {
	Action      string // Keybinding action name
	Context     string // Keybinding context
	Fallback    string // Fallback shortcut if not configured
	Description string // Action description
	ShowParens  bool   // Wrap in parentheses
}

// Render renders the configurable shortcut hint
func (h ConfigurableShortcutHint) Render() string {
	// In Go, we don't have keybinding context, so use fallback
	shortcut := h.Fallback

	hint := KeyboardShortcutHint{
		Shortcut:   shortcut,
		Action:     h.Description,
		ShowParens: h.ShowParens,
	}

	return hint.Render()
}

// Common shortcut hint constructors

// HintEnterConfirm creates an "Enter to confirm" hint
func HintEnterConfirm() KeyboardShortcutHint {
	return KeyboardShortcutHint{Shortcut: "Enter", Action: "confirm"}
}

// HintEscCancel creates an "Esc to cancel" hint
func HintEscCancel() KeyboardShortcutHint {
	return KeyboardShortcutHint{Shortcut: "Esc", Action: "cancel"}
}

// HintCtrlCExit creates a "Ctrl+C to exit" hint
func HintCtrlCExit() KeyboardShortcutHint {
	return KeyboardShortcutHint{Shortcut: "Ctrl+C", Action: "exit"}
}

// HintCtrlOExpand creates a "Ctrl+O to expand" hint
func HintCtrlOExpand() KeyboardShortcutHint {
	return KeyboardShortcutHint{Shortcut: "Ctrl+O", Action: "expand"}
}

// HintUpDown creates an "↑↓ to navigate" hint
func HintUpDown() string {
	return "↑↓ to navigate"
}

// HintLeftRight creates a "←→ to navigate" hint
func HintLeftRight() string {
	return "←→ to navigate"
}

// RenderDimHint renders a hint in dim style
func RenderDimHint(text string) string {
	return fmt.Sprintf("\033[2m%s\033[0m", text)
}

// RenderItalicHint renders a hint in italic style
func RenderItalicHint(text string) string {
	return fmt.Sprintf("\033[3m%s\033[0m", text)
}

// RenderDimItalicHint renders a hint in dim italic style
func RenderDimItalicHint(text string) string {
	return fmt.Sprintf("\033[2;3m%s\033[0m", text)
}

// StandardDialogHints returns the standard dialog input hints
func StandardDialogHints() string {
	return RenderByline([]KeyboardShortcutHint{
		HintEnterConfirm(),
		HintEscCancel(),
	})
}

// ExitPendingHint returns the "Press X again to exit" hint
func ExitPendingHint(keyName string) string {
	return fmt.Sprintf("Press %s again to exit", keyName)
}
