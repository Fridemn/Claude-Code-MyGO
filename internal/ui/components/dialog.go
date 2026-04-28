package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	TopBorderOnly bool   // Only show top border (matches TS PermissionDialog)
}

// RGB represents an RGB color
type RGB struct {
	R, G, B int
}

// Default dialog colors matching theme.ts dark theme
var (
	DialogColorPermission = RGB{177, 185, 249} // Light blue-purple (permission/suggestion)
	DialogColorClaude     = RGB{215, 119, 87}  // Claude orange
	DialogColorSuccess    = RGB{78, 186, 101}  // Bright green
	DialogColorError      = RGB{255, 107, 128} // Bright red
	DialogColorWarning    = RGB{255, 193, 7}   // Bright amber
	DialogColorInfo       = RGB{177, 185, 249} // Same as permission (suggestion color)
	DialogColorInactive   = RGB{153, 153, 153} // Light gray (inactive)
	DialogColorText       = RGB{255, 255, 255} // White
)

// Figures symbols matching TS 'figures' package
var (
	FigurePointer = "›" // figures.pointer - selection indicator
	FigureTick    = "✔" // figures.tick - checkbox selected
	FigureArrowUp = "↑" // figures.arrowUp
	FigureArrowDown = "↓" // figures.arrowDown
)

// RenderDialog renders a dialog box
// Matches src/components/design-system/Dialog.tsx and PermissionDialog.tsx
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

	// Use lipgloss for consistent styling
	borderColor := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", cfg.Color.R, cfg.Color.G, cfg.Color.B))

	// For top-border-only mode (matches TS PermissionDialog)
	if cfg.TopBorderOnly {
		// Top border with rounded corners, spanning full width
		topBorder := "╭" + strings.Repeat("─", cfg.Width-2) + "╮"
		lines = append(lines, lipgloss.NewStyle().Foreground(borderColor).Render(topBorder))

		// Title area with padding
		if cfg.Title != "" {
			titleStyle := lipgloss.NewStyle().
				Foreground(borderColor).
				Bold(true).
				PaddingLeft(1).PaddingRight(1)
			titleLine := titleStyle.Render(cfg.Title)

			// Add subtitle on same line if present
			if cfg.Subtitle != "" {
				subtitleStyle := lipgloss.NewStyle().Faint(true).PaddingLeft(1)
				titleLine = titleLine + subtitleStyle.Render(cfg.Subtitle)
			}
			lines = append(lines, titleLine)
		}

		// Content lines with horizontal padding
		contentStyle := lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
		for _, content := range cfg.Content {
			wrapped := wrapText(content, innerWidth-2)
			for _, line := range wrapped {
				lines = append(lines, contentStyle.Render(line))
			}
		}

		// Input hint - matches TS: <Text color="inactive" dimColor>
		if !cfg.HideInputHint {
			hint := cfg.InputHint
			if hint == "" {
				hint = "Enter to confirm · Esc to cancel"
			}
			// Use inactive color with dimmed style (matches TS QuestionView keyboard hint)
			inactiveColor := lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", DialogColorInactive.R, DialogColorInactive.G, DialogColorInactive.B))
			hintStyle := lipgloss.NewStyle().Foreground(inactiveColor).Faint(true).PaddingLeft(1).PaddingRight(1)
			lines = append(lines, hintStyle.Render(hint))
		}

		return strings.Join(lines, "\n")
	}

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
		subtitleLine := RenderDimText(cfg.Subtitle)
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

// RenderDimText renders text with dimmed style (matches TS: <Text dimColor>)
func RenderDimText(text string) string {
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
