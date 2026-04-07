// Package components provides reusable TUI components
// list_item.go implements ListItem matching src/components/design-system/ListItem.tsx
package components

import (
	"fmt"
	"strings"
)

// ListItemConfig holds configuration for a list item
type ListItemConfig struct {
	// IsFocused indicates keyboard selection - shows pointer (❯)
	IsFocused bool

	// IsSelected indicates the item is chosen/checked - shows checkmark (✓)
	IsSelected bool

	// Content is the main text content
	Content string

	// Description is optional secondary text below main content
	Description string

	// ShowScrollDown shows down arrow instead of pointer (for scroll hints)
	ShowScrollDown bool

	// ShowScrollUp shows up arrow instead of pointer (for scroll hints)
	ShowScrollUp bool

	// Styled applies automatic color styling based on focus/selection state
	// Default: true
	Styled bool

	// Disabled dims the text and hides indicators
	Disabled bool
}

// ListItem color constants matching theme.go
var listItemColors = struct {
	success    rgb
	suggestion rgb
	inactive   rgb
}{
	success:    rgb{78, 186, 101},  // success green
	suggestion: rgb{115, 198, 255}, // info blue (used as suggestion)
	inactive:   rgb{134, 145, 160}, // muted gray
}

type rgb struct {
	r, g, b int
}

// RenderListItem renders a list item with indicators and styling
// Matches the behavior of src/components/design-system/ListItem.tsx
func RenderListItem(cfg ListItemConfig) string {
	var lines []string

	// Main row: indicator + content + checkmark
	mainRow := renderListItemMainRow(cfg)
	lines = append(lines, mainRow)

	// Description row (if present)
	if cfg.Description != "" {
		descRow := renderListItemDescription(cfg.Description)
		lines = append(lines, descRow)
	}

	return strings.Join(lines, "\n")
}

// renderListItemMainRow renders the main row with indicator, content, and checkmark
func renderListItemMainRow(cfg ListItemConfig) string {
	var parts []string

	// Indicator
	indicator := renderListItemIndicator(cfg)
	parts = append(parts, indicator)

	// Content with styling
	content := renderListItemContent(cfg)
	parts = append(parts, content)

	// Checkmark (if selected and not disabled)
	if cfg.IsSelected && !cfg.Disabled {
		parts = append(parts, colorFg(listItemColors.success, CheckMark))
	}

	return strings.Join(parts, " ")
}

// renderListItemIndicator returns the left indicator character
func renderListItemIndicator(cfg ListItemConfig) string {
	if cfg.Disabled {
		return " "
	}

	if cfg.IsFocused {
		return colorFg(listItemColors.suggestion, PointerLarge)
	}

	if cfg.ShowScrollDown {
		return dimText(ArrowDown)
	}

	if cfg.ShowScrollUp {
		return dimText(ArrowUp)
	}

	return " "
}

// renderListItemContent renders the content with appropriate styling
func renderListItemContent(cfg ListItemConfig) string {
	content := cfg.Content

	if cfg.Disabled {
		return colorFg(listItemColors.inactive, content)
	}

	// If not using automatic styling, return as-is
	if !cfg.Styled {
		return content
	}

	// Apply color based on state
	if cfg.IsSelected {
		return colorFg(listItemColors.success, content)
	}

	if cfg.IsFocused {
		return colorFg(listItemColors.suggestion, content)
	}

	return content
}

// renderListItemDescription renders the description line with padding
func renderListItemDescription(desc string) string {
	// Padding left of 2 spaces (matching Ink paddingLeft={2})
	return "  " + colorFg(listItemColors.inactive, desc)
}

// colorFg applies foreground color using ANSI escape codes
func colorFg(c rgb, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", c.r, c.g, c.b, text)
}

// dimText applies dim styling
func dimText(text string) string {
	return fmt.Sprintf("\033[2m%s\033[0m", text)
}

// ListItemBatch renders multiple list items for a selection list
// Handles scroll indicators automatically based on visible range
func ListItemBatch(items []ListItemConfig, focusIndex, startVisible, visibleCount int) []string {
	var result []string

	endVisible := startVisible + visibleCount
	if endVisible > len(items) {
		endVisible = len(items)
	}

	for i := startVisible; i < endVisible; i++ {
		item := items[i]

		// Add scroll indicators for first/last visible items
		if i == startVisible && startVisible > 0 {
			item.ShowScrollUp = true
		}
		if i == endVisible-1 && endVisible < len(items) {
			item.ShowScrollDown = true
		}

		// Set focus state
		item.IsFocused = (i == focusIndex)

		result = append(result, RenderListItem(item))
	}

	return result
}

// DefaultListItemConfig returns a ListItemConfig with default values
func DefaultListItemConfig() ListItemConfig {
	return ListItemConfig{
		Styled: true, // Default to styled
	}
}
