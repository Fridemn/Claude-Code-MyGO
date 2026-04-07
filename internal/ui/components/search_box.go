// Package components provides reusable TUI components
// search_box.go implements SearchBox matching src/components/SearchBox.tsx
package components

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// SearchBoxConfig holds configuration for a search box
type SearchBoxConfig struct {
	// Query is the current search text
	Query string

	// Placeholder shown when query is empty
	// Default: "Search…"
	Placeholder string

	// IsFocused indicates if the search box is focused
	IsFocused bool

	// IsTerminalFocused indicates if the terminal window is focused
	IsTerminalFocused bool

	// Prefix shown before the search text
	// Default: "⌕"
	Prefix string

	// Width is the total width of the search box (0 = auto)
	Width int

	// CursorOffset is the cursor position in the query
	// Default: len(Query)
	CursorOffset int

	// Borderless removes the rounded border
	Borderless bool
}

// SearchBox color constants
var searchBoxColors = struct {
	suggestion rgb
	border     rgb
	borderDim  rgb
}{
	suggestion: rgb{115, 198, 255}, // info blue
	border:     rgb{115, 198, 255}, // info blue when focused
	borderDim:  rgb{80, 80, 80},    // subtle gray when not focused
}

// RenderSearchBox renders a search box with query, cursor, and optional border
// Matches src/components/SearchBox.tsx behavior
func RenderSearchBox(cfg SearchBoxConfig) string {
	// Apply defaults
	if cfg.Placeholder == "" {
		cfg.Placeholder = "Search…"
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "⌕"
	}

	offset := cfg.CursorOffset
	if offset == 0 && cfg.Query != "" {
		offset = len([]rune(cfg.Query))
	}

	// Build the content
	content := renderSearchBoxContent(cfg, offset)

	// Apply prefix with dimming based on focus
	var prefix string
	if cfg.IsFocused {
		prefix = cfg.Prefix + " "
	} else {
		prefix = dimText(cfg.Prefix + " ")
	}

	inner := prefix + content

	// If borderless, just return the content
	if cfg.Borderless {
		return inner
	}

	// Render with rounded border
	return renderSearchBoxWithBorder(inner, cfg)
}

// renderSearchBoxContent renders the query/placeholder with cursor
func renderSearchBoxContent(cfg SearchBoxConfig, offset int) string {
	if cfg.IsFocused {
		if cfg.Query != "" {
			if cfg.IsTerminalFocused {
				// Show cursor at offset position
				runes := []rune(cfg.Query)
				if offset > len(runes) {
					offset = len(runes)
				}

				before := string(runes[:offset])
				var cursor string
				var after string

				if offset < len(runes) {
					cursor = inverseText(string(runes[offset]))
					after = string(runes[offset+1:])
				} else {
					cursor = inverseText(" ")
					after = ""
				}

				return before + cursor + after
			}
			return cfg.Query
		}

		// Empty query - show placeholder with cursor on first char
		if cfg.IsTerminalFocused {
			runes := []rune(cfg.Placeholder)
			return inverseText(string(runes[0])) + dimText(string(runes[1:]))
		}
		return dimText(cfg.Placeholder)
	}

	// Not focused
	if cfg.Query != "" {
		return cfg.Query
	}
	return cfg.Placeholder
}

// renderSearchBoxWithBorder renders the content with a rounded border
func renderSearchBoxWithBorder(content string, cfg SearchBoxConfig) string {
	// Calculate content width
	contentWidth := runewidth.StringWidth(stripAnsi(content))

	// Determine box width
	boxWidth := cfg.Width
	if boxWidth == 0 {
		boxWidth = contentWidth + 4 // 2 for padding, 2 for borders
	}

	innerWidth := boxWidth - 2 // Subtract border chars

	// Pad content to fill inner width
	padded := content + strings.Repeat(" ", max(0, innerWidth-contentWidth-2))

	// Choose border color
	var borderColor rgb
	if cfg.IsFocused {
		borderColor = searchBoxColors.suggestion
	} else {
		borderColor = searchBoxColors.borderDim
	}

	// Build the box with rounded corners
	topBorder := colorFg(borderColor, CornerTopLeft+strings.Repeat(Line, innerWidth)+CornerTopRight)
	bottomBorder := colorFg(borderColor, CornerBottomLeft+strings.Repeat(Line, innerWidth)+CornerBottomRight)

	leftBorder := colorFg(borderColor, VerticalLine)
	rightBorder := colorFg(borderColor, VerticalLine)

	middleRow := leftBorder + " " + padded + " " + rightBorder

	return strings.Join([]string{topBorder, middleRow, bottomBorder}, "\n")
}

// inverseText applies inverse (swap fg/bg) styling
func inverseText(text string) string {
	return fmt.Sprintf("\033[7m%s\033[0m", text)
}

// stripAnsi removes ANSI escape codes from a string
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// SearchBoxState maintains the state of a search box for interactive use
type SearchBoxState struct {
	Query        string
	CursorOffset int
}

// SearchBoxState creates a new search box state
func NewSearchBoxState() *SearchBoxState {
	return SearchBoxStateFor()
}

func SearchBoxStateFor() *SearchBoxState {
	return &SearchBoxState{}
}

// Insert inserts a character at the cursor position
func (s *SearchBoxState) Insert(char string) {
	runes := []rune(s.Query)
	if s.CursorOffset > len(runes) {
		s.CursorOffset = len(runes)
	}

	newRunes := make([]rune, 0, len(runes)+len([]rune(char)))
	newRunes = append(newRunes, runes[:s.CursorOffset]...)
	newRunes = append(newRunes, []rune(char)...)
	newRunes = append(newRunes, runes[s.CursorOffset:]...)

	s.Query = string(newRunes)
	s.CursorOffset += len([]rune(char))
}

// Backspace deletes the character before the cursor
func (s *SearchBoxState) Backspace() {
	if s.CursorOffset == 0 {
		return
	}

	runes := []rune(s.Query)
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:s.CursorOffset-1]...)
	newRunes = append(newRunes, runes[s.CursorOffset:]...)

	s.Query = string(newRunes)
	s.CursorOffset--
}

// Delete deletes the character at the cursor
func (s *SearchBoxState) Delete() {
	runes := []rune(s.Query)
	if s.CursorOffset >= len(runes) {
		return
	}

	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:s.CursorOffset]...)
	newRunes = append(newRunes, runes[s.CursorOffset+1:]...)

	s.Query = string(newRunes)
}

// MoveLeft moves the cursor left
func (s *SearchBoxState) MoveLeft() {
	if s.CursorOffset > 0 {
		s.CursorOffset--
	}
}

// MoveRight moves the cursor right
func (s *SearchBoxState) MoveRight() {
	runes := []rune(s.Query)
	if s.CursorOffset < len(runes) {
		s.CursorOffset++
	}
}

// MoveHome moves the cursor to the beginning
func (s *SearchBoxState) MoveHome() {
	s.CursorOffset = 0
}

// MoveEnd moves the cursor to the end
func (s *SearchBoxState) MoveEnd() {
	s.CursorOffset = len([]rune(s.Query))
}

// Clear clears the query and resets the cursor
func (s *SearchBoxState) Clear() {
	s.Query = ""
	s.CursorOffset = 0
}

// Set sets the query and cursor position
func (s *SearchBoxState) Set(query string) {
	s.Query = query
	s.CursorOffset = len([]rune(query))
}
