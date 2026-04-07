package components

import (
	"fmt"
	"strings"
)

// DialogConfig holds configuration for rendering a dialog
type DialogConfig struct {
	Title         string
	Subtitle      string
	Content       []string // Lines of content
	Color         RGB      // Border/title color
	Width         int      // Dialog width (0 = auto)
	HideBorder    bool
	HideInputHint bool
	InputHint     string // Custom input hint (default: "Enter to confirm, Esc to cancel")
}

// RGB represents an RGB color
type RGB struct {
	R, G, B int
}

// Default dialog colors matching theme.go
var (
	DialogColorPermission = RGB{177, 185, 249}
	DialogColorClaude     = RGB{215, 119, 87}
	DialogColorSuccess    = RGB{78, 186, 101}
	DialogColorError      = RGB{255, 107, 128}
	DialogColorWarning    = RGB{255, 193, 7}
	DialogColorInfo       = RGB{115, 198, 255}
	DialogColorMuted      = RGB{134, 145, 160}
	DialogColorText       = RGB{255, 255, 255}
)

// RenderDialog renders a dialog box
// Matches src/components/design-system/Dialog.tsx
func RenderDialog(cfg DialogConfig) string {
	if cfg.Width <= 0 {
		cfg.Width = 60
	}
	if cfg.Width < 20 {
		cfg.Width = 20
	}
	if cfg.Color == (RGB{}) {
		cfg.Color = DialogColorPermission
	}

	innerWidth := cfg.Width - 2

	var lines []string

	if !cfg.HideBorder {
		// Top border with rounded corners
		lines = append(lines, renderLine(cfg.Color, "╭"+strings.Repeat("─", innerWidth)+"╮"))
	}

	// Title (bold, colored)
	if cfg.Title != "" {
		titleLine := renderColoredText(cfg.Color, cfg.Title, true)
		titlePadded := padToWidth(titleLine, innerWidth)
		if !cfg.HideBorder {
			lines = append(lines, renderLine(cfg.Color, "│")+titlePadded+renderLine(cfg.Color, "│"))
		} else {
			lines = append(lines, titleLine)
		}
	}

	// Subtitle (dimmed)
	if cfg.Subtitle != "" {
		subtitleLine := renderDimText(cfg.Subtitle)
		subtitlePadded := padToWidth(subtitleLine, innerWidth)
		if !cfg.HideBorder {
			lines = append(lines, renderLine(cfg.Color, "│")+subtitlePadded+renderLine(cfg.Color, "│"))
		} else {
			lines = append(lines, subtitleLine)
		}
	}

	// Empty line after title/subtitle if we have content
	if (cfg.Title != "" || cfg.Subtitle != "") && len(cfg.Content) > 0 {
		emptyLine := strings.Repeat(" ", innerWidth)
		if !cfg.HideBorder {
			lines = append(lines, renderLine(cfg.Color, "│")+emptyLine+renderLine(cfg.Color, "│"))
		} else {
			lines = append(lines, "")
		}
	}

	// Content lines
	for _, content := range cfg.Content {
		wrapped := wrapText(content, innerWidth)
		for _, line := range wrapped {
			linePadded := padToWidth(line, innerWidth)
			if !cfg.HideBorder {
				lines = append(lines, renderLine(cfg.Color, "│")+linePadded+renderLine(cfg.Color, "│"))
			} else {
				lines = append(lines, line)
			}
		}
	}

	// Input hint
	if !cfg.HideInputHint {
		hint := cfg.InputHint
		if hint == "" {
			hint = "Enter to confirm · Esc to cancel"
		}

		// Empty line before hint
		emptyLine := strings.Repeat(" ", innerWidth)
		if !cfg.HideBorder {
			lines = append(lines, renderLine(cfg.Color, "│")+emptyLine+renderLine(cfg.Color, "│"))
		} else {
			lines = append(lines, "")
		}

		hintLine := renderDimItalicText(hint)
		hintPadded := padToWidth(hintLine, innerWidth)
		if !cfg.HideBorder {
			lines = append(lines, renderLine(cfg.Color, "│")+hintPadded+renderLine(cfg.Color, "│"))
		} else {
			lines = append(lines, hintLine)
		}
	}

	if !cfg.HideBorder {
		// Bottom border
		lines = append(lines, renderLine(cfg.Color, "╰"+strings.Repeat("─", innerWidth)+"╯"))
	}

	return strings.Join(lines, "\n")
}

// Pane renders a simple bordered pane (matches Pane.tsx)
func RenderPane(content []string, color RGB, width int) string {
	if width <= 0 {
		width = 60
	}
	innerWidth := width - 2

	var lines []string

	// Top border
	lines = append(lines, renderLine(color, "╭"+strings.Repeat("─", innerWidth)+"╮"))

	// Content
	for _, line := range content {
		linePadded := padToWidth(line, innerWidth)
		lines = append(lines, renderLine(color, "│")+linePadded+renderLine(color, "│"))
	}

	// Bottom border
	lines = append(lines, renderLine(color, "╰"+strings.Repeat("─", innerWidth)+"╯"))

	return strings.Join(lines, "\n")
}

// Helper functions

func renderLine(color RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderColoredText(color RGB, text string, bold bool) string {
	if bold {
		return fmt.Sprintf("\033[1m\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderDimText(text string) string {
	return fmt.Sprintf("\033[2m%s\033[0m", text)
}

func renderDimItalicText(text string) string {
	return fmt.Sprintf("\033[2;3m%s\033[0m", text)
}

// padToWidth pads a string with spaces to reach the desired width
// Accounts for ANSI escape codes when calculating visible width
func padToWidth(text string, width int) string {
	visWidth := visibleWidth(text)
	if visWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-visWidth)
}

// visibleWidth calculates the visible width of a string, ignoring ANSI codes
func visibleWidth(s string) int {
	// Strip ANSI codes for width calculation
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	currentLine := ""

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
