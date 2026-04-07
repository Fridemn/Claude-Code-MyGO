// Package components provides reusable TUI components
// fuzzy_picker.go implements FuzzyPicker matching src/components/design-system/FuzzyPicker.tsx
package components

import (
	"fmt"
	"strings"

	"github.com/sahilm/fuzzy"
)

// FuzzyPickerItem represents an item in the fuzzy picker
type FuzzyPickerItem struct {
	ID          string
	Label       string
	Description string
	// Custom render content (if empty, uses Label)
	RenderContent string
}

// FuzzyPickerAction represents a Tab/Shift+Tab action
type FuzzyPickerAction struct {
	Action  string // Hint label, e.g. "mention"
	Handler func(item *FuzzyPickerItem)
}

// FuzzyPickerConfig holds configuration for the fuzzy picker
type FuzzyPickerConfig struct {
	// Title shown at the top
	Title string

	// Placeholder for the search box
	// Default: "Type to search…"
	Placeholder string

	// Items to select from
	Items []FuzzyPickerItem

	// VisibleCount is the number of items to show
	// Default: 8
	VisibleCount int

	// Direction: "down" (default) or "up"
	// "up" puts items[0] at the bottom (atuin-style)
	Direction string

	// PreviewPosition: "bottom" (default) or "right"
	PreviewPosition string

	// EmptyMessage shown when no items match
	// Default: "No results"
	EmptyMessage string

	// MatchLabel shows match count, e.g. "42 matches"
	MatchLabel string

	// SelectAction label for Enter key
	// Default: "select"
	SelectAction string

	// OnTab action
	OnTab *FuzzyPickerAction

	// OnShiftTab action
	OnShiftTab *FuzzyPickerAction

	// ExtraHints appended to the hint row
	ExtraHints string

	// Width of the picker (0 = auto)
	Width int
}

// FuzzyPickerState holds the mutable state of a fuzzy picker
type FuzzyPickerState struct {
	Query         string
	CursorOffset  int
	FocusedIndex  int
	FilteredItems []FuzzyPickerItem
	AllItems      []FuzzyPickerItem
}

// FuzzyPickerState creates a new fuzzy picker state
func NewFuzzyPickerState(items []FuzzyPickerItem) *FuzzyPickerState {
	return FuzzyPickerStateFor(items)
}

func FuzzyPickerStateFor(items []FuzzyPickerItem) *FuzzyPickerState {
	return &FuzzyPickerState{
		AllItems:      items,
		FilteredItems: items,
	}
}

// UpdateQuery updates the search query and filters items
func (s *FuzzyPickerState) UpdateQuery(query string) {
	s.Query = query
	s.CursorOffset = len([]rune(query))

	if query == "" {
		s.FilteredItems = s.AllItems
	} else {
		// Use fuzzy matching
		s.FilteredItems = filterItemsFuzzy(s.AllItems, query)
	}

	// Reset focus to first item
	s.FocusedIndex = 0
	if s.FocusedIndex >= len(s.FilteredItems) {
		s.FocusedIndex = len(s.FilteredItems) - 1
	}
	if s.FocusedIndex < 0 {
		s.FocusedIndex = 0
	}
}

// MoveUp moves focus up (visually)
func (s *FuzzyPickerState) MoveUp(direction string) {
	if direction == "up" {
		s.FocusedIndex++
	} else {
		s.FocusedIndex--
	}
	s.clampFocus()
}

// MoveDown moves focus down (visually)
func (s *FuzzyPickerState) MoveDown(direction string) {
	if direction == "up" {
		s.FocusedIndex--
	} else {
		s.FocusedIndex++
	}
	s.clampFocus()
}

func (s *FuzzyPickerState) clampFocus() {
	if s.FocusedIndex < 0 {
		s.FocusedIndex = 0
	}
	if s.FocusedIndex >= len(s.FilteredItems) {
		s.FocusedIndex = len(s.FilteredItems) - 1
	}
	if s.FocusedIndex < 0 {
		s.FocusedIndex = 0
	}
}

// GetFocused returns the currently focused item, or nil if none
func (s *FuzzyPickerState) GetFocused() *FuzzyPickerItem {
	if s.FocusedIndex >= 0 && s.FocusedIndex < len(s.FilteredItems) {
		return &s.FilteredItems[s.FocusedIndex]
	}
	return nil
}

// InsertChar inserts a character at the cursor position
func (s *FuzzyPickerState) InsertChar(char string) {
	runes := []rune(s.Query)
	if s.CursorOffset > len(runes) {
		s.CursorOffset = len(runes)
	}

	newRunes := make([]rune, 0, len(runes)+len([]rune(char)))
	newRunes = append(newRunes, runes[:s.CursorOffset]...)
	newRunes = append(newRunes, []rune(char)...)
	newRunes = append(newRunes, runes[s.CursorOffset:]...)

	s.UpdateQuery(string(newRunes))
}

// Backspace deletes the character before the cursor
func (s *FuzzyPickerState) Backspace() {
	if s.CursorOffset == 0 {
		return
	}

	runes := []rune(s.Query)
	newRunes := make([]rune, 0, len(runes)-1)
	newRunes = append(newRunes, runes[:s.CursorOffset-1]...)
	newRunes = append(newRunes, runes[s.CursorOffset:]...)

	s.CursorOffset--
	s.UpdateQuery(string(newRunes))
}

// filterItemsFuzzy filters items using fuzzy matching
func filterItemsFuzzy(items []FuzzyPickerItem, query string) []FuzzyPickerItem {
	// Create a string slice for fuzzy matching
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}

	// Perform fuzzy match
	matches := fuzzy.Find(query, labels)

	// Build result list in match order
	result := make([]FuzzyPickerItem, 0, len(matches))
	for _, match := range matches {
		result = append(result, items[match.Index])
	}

	return result
}

// FuzzyPicker colors
var fuzzyPickerColors = struct {
	permission rgb
	title      rgb
	hint       rgb
}{
	permission: rgb{177, 185, 249}, // permission purple
	title:      rgb{177, 185, 249}, // permission purple
	hint:       rgb{134, 145, 160}, // muted
}

// RenderFuzzyPicker renders the fuzzy picker
func RenderFuzzyPicker(cfg FuzzyPickerConfig, state *FuzzyPickerState, isFocused, isTerminalFocused bool) string {
	// Apply defaults
	if cfg.Placeholder == "" {
		cfg.Placeholder = "Type to search…"
	}
	if cfg.VisibleCount == 0 {
		cfg.VisibleCount = 8
	}
	if cfg.Direction == "" {
		cfg.Direction = "down"
	}
	if cfg.EmptyMessage == "" {
		cfg.EmptyMessage = "No results"
	}
	if cfg.SelectAction == "" {
		cfg.SelectAction = "select"
	}

	var lines []string

	// Title
	title := boldText(colorFg(fuzzyPickerColors.title, cfg.Title))
	lines = append(lines, title)
	lines = append(lines, "")

	// Search box (if direction is "down", show above list)
	searchBox := RenderSearchBox(SearchBoxConfig{
		Query:             state.Query,
		Placeholder:       cfg.Placeholder,
		IsFocused:         isFocused,
		IsTerminalFocused: isTerminalFocused,
		CursorOffset:      state.CursorOffset,
	})

	if cfg.Direction != "up" {
		lines = append(lines, searchBox)
		lines = append(lines, "")
	}

	// List
	listLines := renderFuzzyPickerList(cfg, state)
	lines = append(lines, listLines...)

	// Match label
	if cfg.MatchLabel != "" {
		lines = append(lines, dimText(cfg.MatchLabel))
	}

	// Search box (if direction is "up", show below list)
	if cfg.Direction == "up" {
		lines = append(lines, "")
		lines = append(lines, searchBox)
	}

	// Hints
	lines = append(lines, "")
	hints := renderFuzzyPickerHints(cfg)
	lines = append(lines, dimText(hints))

	// Wrap in pane with permission color border
	content := strings.Join(lines, "\n")
	return renderFuzzyPickerPane(content, cfg.Width)
}

// renderFuzzyPickerList renders the list of items
func renderFuzzyPickerList(cfg FuzzyPickerConfig, state *FuzzyPickerState) []string {
	items := state.FilteredItems
	focusedIndex := state.FocusedIndex
	visibleCount := cfg.VisibleCount

	if len(items) == 0 {
		// Empty state - pad to maintain height
		result := []string{dimText(cfg.EmptyMessage)}
		for i := 1; i < visibleCount; i++ {
			result = append(result, "")
		}
		return result
	}

	// Calculate visible window
	windowStart := focusedIndex - visibleCount + 1
	if windowStart < 0 {
		windowStart = 0
	}
	if windowStart > len(items)-visibleCount {
		windowStart = len(items) - visibleCount
	}
	if windowStart < 0 {
		windowStart = 0
	}

	windowEnd := windowStart + visibleCount
	if windowEnd > len(items) {
		windowEnd = len(items)
	}

	visible := items[windowStart:windowEnd]

	var result []string

	// Render visible items
	for i, item := range visible {
		globalIndex := windowStart + i
		isFocusedItem := globalIndex == focusedIndex

		// Determine scroll indicators
		showScrollUp := i == 0 && windowStart > 0
		showScrollDown := i == len(visible)-1 && windowEnd < len(items)

		content := item.Label
		if item.RenderContent != "" {
			content = item.RenderContent
		}

		listItem := RenderListItem(ListItemConfig{
			Content:        content,
			Description:    item.Description,
			IsFocused:      isFocusedItem,
			ShowScrollUp:   showScrollUp,
			ShowScrollDown: showScrollDown,
			Styled:         true,
		})

		result = append(result, listItem)
	}

	// Pad to maintain consistent height
	for len(result) < visibleCount {
		result = append(result, "")
	}

	// Reverse if direction is "up"
	if cfg.Direction == "up" {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}

	return result
}

// renderFuzzyPickerHints renders the keyboard hints
func renderFuzzyPickerHints(cfg FuzzyPickerConfig) string {
	hints := []string{
		"↑/↓ navigate",
		"Enter " + cfg.SelectAction,
	}

	if cfg.OnTab != nil {
		hints = append(hints, "Tab "+cfg.OnTab.Action)
	}

	if cfg.OnShiftTab != nil {
		hints = append(hints, "shift+tab "+cfg.OnShiftTab.Action)
	}

	hints = append(hints, "Esc cancel")

	if cfg.ExtraHints != "" {
		hints = append(hints, cfg.ExtraHints)
	}

	return strings.Join(hints, " · ")
}

// renderFuzzyPickerPane wraps content in a pane with colored border
func renderFuzzyPickerPane(content string, width int) string {
	if width == 0 {
		width = 60
	}

	lines := strings.Split(content, "\n")
	borderColor := fuzzyPickerColors.permission

	// Top border with divider line
	divider := colorFg(borderColor, strings.Repeat("─", width))
	result := []string{divider}

	// Content with left padding
	for _, line := range lines {
		result = append(result, "  "+line)
	}

	return strings.Join(result, "\n")
}

// boldText applies bold styling
func boldText(text string) string {
	return fmt.Sprintf("\033[1m%s\033[0m", text)
}

// FuzzyPickerModel is a Bubble Tea compatible model for the fuzzy picker
type FuzzyPickerModel struct {
	Config FuzzyPickerConfig
	State  *FuzzyPickerState

	// Callbacks
	OnSelect   func(item *FuzzyPickerItem)
	OnCancel   func()
	OnTab      func(item *FuzzyPickerItem)
	OnShiftTab func(item *FuzzyPickerItem)

	// Focus state
	IsFocused         bool
	IsTerminalFocused bool
}

// FuzzyPickerModel creates a new fuzzy picker model
func NewFuzzyPickerModel(cfg FuzzyPickerConfig) *FuzzyPickerModel {
	return FuzzyPickerModelFor(cfg)
}

func FuzzyPickerModelFor(cfg FuzzyPickerConfig) *FuzzyPickerModel {
	return &FuzzyPickerModel{
		Config:            cfg,
		State:             FuzzyPickerStateFor(cfg.Items),
		IsFocused:         true,
		IsTerminalFocused: true,
	}
}

// View renders the fuzzy picker
func (m *FuzzyPickerModel) View() string {
	return RenderFuzzyPicker(m.Config, m.State, m.IsFocused, m.IsTerminalFocused)
}

// HandleKey handles a key press and returns true if the picker should close
func (m *FuzzyPickerModel) HandleKey(key string) bool {
	switch key {
	case "up", "ctrl+p":
		m.State.MoveUp(m.Config.Direction)
	case "down", "ctrl+n":
		m.State.MoveDown(m.Config.Direction)
	case "enter":
		if item := m.State.GetFocused(); item != nil && m.OnSelect != nil {
			m.OnSelect(item)
			return true
		}
	case "tab":
		if item := m.State.GetFocused(); item != nil && m.OnTab != nil {
			m.OnTab(item)
			return true
		} else if item != nil && m.OnSelect != nil {
			m.OnSelect(item)
			return true
		}
	case "shift+tab":
		if item := m.State.GetFocused(); item != nil && m.OnShiftTab != nil {
			m.OnShiftTab(item)
			return true
		} else if item != nil && m.OnTab != nil {
			m.OnTab(item)
			return true
		}
	case "esc", "ctrl+c":
		if m.OnCancel != nil {
			m.OnCancel()
		}
		return true
	case "backspace":
		m.State.Backspace()
	default:
		// Single character input
		if len(key) == 1 && key >= " " {
			m.State.InsertChar(key)
		}
	}
	return false
}
