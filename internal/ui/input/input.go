package input

import (
	"fmt"
	"regexp"
	"strings"

	"claude-go/internal/ui/components"
	"claude-go/internal/ui/paste"
)

// InputState represents the state of a text input
type InputState struct {
	Value       string
	CursorPos   int
	Placeholder string
	IsFocused   bool
	IsDisabled  bool
	IsError     bool
	Width       int
}

// Theme colors for input
var (
	colorText       = components.RGB{255, 255, 255}
	colorMuted      = components.RGB{134, 145, 160}
	colorBorder     = components.RGB{136, 136, 136}
	colorClaude     = components.RGB{215, 119, 87}
	colorPermission = components.RGB{177, 185, 249}
	colorError      = components.RGB{255, 107, 128}
	colorUserLabel  = components.RGB{122, 180, 232}
)

var inlineHighlightPattern = regexp.MustCompile(`(/[A-Za-z0-9_-]+|@[A-Za-z0-9._-]+)`)

// InputState creates a new input state
func NewInputState(placeholder string, width int) *InputState {
	return InputStateFor(placeholder, width)
}

func InputStateFor(placeholder string, width int) *InputState {
	return &InputState{
		Placeholder: placeholder,
		Width:       width,
		CursorPos:   0,
	}
}

// Insert inserts text at the cursor position
func (s *InputState) Insert(text string) {
	if s.IsDisabled {
		return
	}
	before := s.Value[:s.CursorPos]
	after := s.Value[s.CursorPos:]
	s.Value = before + text + after
	s.CursorPos += len(text)
}

// Backspace deletes the character before the cursor
func (s *InputState) Backspace() {
	if s.IsDisabled || s.CursorPos == 0 {
		return
	}
	before := s.Value[:s.CursorPos-1]
	after := s.Value[s.CursorPos:]
	s.Value = before + after
	s.CursorPos--
}

// Delete deletes the character at the cursor
func (s *InputState) Delete() {
	if s.IsDisabled || s.CursorPos >= len(s.Value) {
		return
	}
	before := s.Value[:s.CursorPos]
	after := s.Value[s.CursorPos+1:]
	s.Value = before + after
}

// MoveLeft moves the cursor left
func (s *InputState) MoveLeft() {
	if s.CursorPos > 0 {
		s.CursorPos--
	}
}

// MoveRight moves the cursor right
func (s *InputState) MoveRight() {
	if s.CursorPos < len(s.Value) {
		s.CursorPos++
	}
}

// MoveHome moves the cursor to the start
func (s *InputState) MoveHome() {
	s.CursorPos = 0
}

// MoveEnd moves the cursor to the end
func (s *InputState) MoveEnd() {
	s.CursorPos = len(s.Value)
}

// Clear clears the input
func (s *InputState) Clear() {
	s.Value = ""
	s.CursorPos = 0
}

// SetValue sets the input value
func (s *InputState) SetValue(value string) {
	s.Value = value
	if s.CursorPos > len(value) {
		s.CursorPos = len(value)
	}
}

// RenderInput renders a basic text input
// Matches src/components/TextInput.tsx
func RenderInput(state *InputState) string {
	if state.Width <= 0 {
		state.Width = 60
	}

	text := state.Value
	if text == "" && state.Placeholder != "" && !state.IsFocused {
		// Show placeholder when empty and not focused
		return renderMuted(state.Placeholder)
	}

	if !state.IsFocused {
		// Just show the text
		return renderInlineHighlights(text)
	}

	// Show text with cursor
	if state.CursorPos >= len(text) {
		// Cursor at end
		return renderInlineHighlights(text) + renderCursor(" ")
	}

	// Cursor in middle
	before := text[:state.CursorPos]
	cursor := text[state.CursorPos : state.CursorPos+1]
	after := text[state.CursorPos+1:]
	return renderInlineHighlights(before) + renderCursor(cursor) + renderInlineHighlights(after)
}

// renderCursor renders the cursor character with inverted colors
func renderCursor(char string) string {
	return fmt.Sprintf("\033[7m%s\033[0m", char)
}

// PromptInputState represents the state of the main prompt input
type PromptInputState struct {
	*InputState
	ShowPromptLabel    bool
	PromptLabel        string // e.g., "⏵" or custom
	Suggestions        []string
	SelectedSuggestion int
	IsBusy             bool
	Mode               string // "normal", "command", "search"
	VimEnabled         bool
	VimInsertMode      bool
	History            []string
	HistoryIndex       int // -1 means editing current draft
	historyDraft       string
	// Paste support
	PasteManager *paste.Manager
}

// PromptInputState creates a new prompt input state
func NewPromptInputState(width int) *PromptInputState {
	return PromptInputStateFor(width)
}

func PromptInputStateFor(width int) *PromptInputState {
	return &PromptInputState{
		InputState:         InputStateFor("Ask Claude...", width),
		ShowPromptLabel:    true,
		PromptLabel:        "⏵",
		SelectedSuggestion: -1,
		VimEnabled:         false,
		VimInsertMode:      true,
		HistoryIndex:       -1,
	}
}

// RenderPromptInput renders the main prompt input
// Matches src/components/PromptInput/PromptInput.tsx
func RenderPromptInput(state *PromptInputState) string {
	var lines []string

	// Render the main input line
	inputLine := ""
	if state.ShowPromptLabel {
		inputLine = renderColored(colorUserLabel, state.PromptLabel) + " "
	}
	inputLine += RenderInput(state.InputState)
	if state.VimEnabled && !state.VimInsertMode {
		inputLine += " " + renderMuted("-- NORMAL --")
	}
	lines = append(lines, inputLine)

	// Render suggestions if available
	if len(state.Suggestions) > 0 && state.IsFocused {
		lines = append(lines, renderSuggestions(state.Suggestions, state.SelectedSuggestion, state.InputState.Width))
	}

	return strings.Join(lines, "\n")
}

// EnableVimMode toggles vim-style editing for the prompt input.
func (s *PromptInputState) EnableVimMode(enabled bool) {
	s.VimEnabled = enabled
	if !enabled {
		s.VimInsertMode = true
	}
}

// HandleVimKey processes a key in vim mode.
// handled=true means key was applied by vim logic.
// swallow=true means caller should stop further processing.
func (s *PromptInputState) HandleVimKey(key string) (handled bool, swallow bool) {
	if !s.VimEnabled {
		return false, false
	}

	if s.VimInsertMode {
		if key == "esc" {
			s.VimInsertMode = false
			return true, true
		}
		return false, false
	}

	switch key {
	case "i":
		s.VimInsertMode = true
		return true, true
	case "a":
		s.MoveRight()
		s.VimInsertMode = true
		return true, true
	case "I":
		s.MoveHome()
		s.VimInsertMode = true
		return true, true
	case "A":
		s.MoveEnd()
		s.VimInsertMode = true
		return true, true
	case "h", "left":
		s.MoveLeft()
		return true, true
	case "l", "right":
		s.MoveRight()
		return true, true
	case "x", "delete":
		s.Delete()
		return true, true
	case "0", "home":
		s.MoveHome()
		return true, true
	case "$", "end":
		s.MoveEnd()
		return true, true
	case "esc":
		return true, true
	default:
		return false, true
	}
}

// AddHistory records a submitted prompt in input history.
func (s *PromptInputState) AddHistory(value string) {
	v := strings.TrimSpace(value)
	if v == "" {
		return
	}
	if n := len(s.History); n > 0 && s.History[n-1] == v {
		s.HistoryIndex = -1
		s.historyDraft = ""
		return
	}
	s.History = append(s.History, v)
	s.HistoryIndex = -1
	s.historyDraft = ""
}

// PrevHistory navigates to older history (like shell up-arrow).
func (s *PromptInputState) PrevHistory() bool {
	if len(s.History) == 0 {
		return false
	}
	if s.HistoryIndex == -1 {
		s.historyDraft = s.Value
		s.HistoryIndex = len(s.History) - 1
	} else if s.HistoryIndex > 0 {
		s.HistoryIndex--
	}
	s.SetValue(s.History[s.HistoryIndex])
	s.MoveEnd()
	return true
}

// NextHistory navigates to newer history (like shell down-arrow).
func (s *PromptInputState) NextHistory() bool {
	if len(s.History) == 0 || s.HistoryIndex == -1 {
		return false
	}
	if s.HistoryIndex < len(s.History)-1 {
		s.HistoryIndex++
		s.SetValue(s.History[s.HistoryIndex])
		s.MoveEnd()
		return true
	}
	s.HistoryIndex = -1
	s.SetValue(s.historyDraft)
	s.MoveEnd()
	return true
}

// renderSuggestions renders command suggestions
func renderSuggestions(suggestions []string, selected int, width int) string {
	if len(suggestions) == 0 {
		return ""
	}

	var lines []string
	for i, suggestion := range suggestions {
		prefix := "  "
		text := suggestion
		if i == selected {
			// Highlight selected suggestion
			text = renderColoredBg(colorPermission, suggestion)
			prefix = renderColored(colorPermission, "› ")
		} else {
			text = renderMuted(suggestion)
		}
		lines = append(lines, prefix+text)
	}
	return strings.Join(lines, "\n")
}

// RenderPromptFooter renders the footer with hints and status
// Matches src/components/PromptInput/PromptInputFooter.tsx
func RenderPromptFooter(width int, hints []string, status string) string {
	status = strings.TrimSpace(status)
	hintText := strings.TrimSpace(strings.Join(hints, " · "))
	if status == "" && hintText == "" {
		return ""
	}
	if width <= 0 {
		if status == "" {
			return renderMuted(hintText)
		}
		if hintText == "" {
			return renderMuted(status)
		}
		return renderMuted(status + "  " + hintText)
	}

	if status == "" {
		return renderMuted(truncateRunes(hintText, width))
	}
	if hintText == "" {
		return renderMuted(truncateRunes(status, width))
	}

	statusLen := runeLen(status)
	hintLen := runeLen(hintText)
	if statusLen+hintLen <= width {
		return renderMuted(status + strings.Repeat(" ", width-statusLen-hintLen) + hintText)
	}

	minGap := 2
	maxHintWidth := width - statusLen - minGap
	if maxHintWidth > 0 {
		return renderMuted(status + strings.Repeat(" ", minGap) + truncateRunes(hintText, maxHintWidth))
	}
	return renderMuted(truncateRunes(status, width))
}

func runeLen(s string) int {
	return len([]rune(s))
}

func truncateRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

// SearchBoxState represents the state of a search input
type SearchBoxState struct {
	*InputState
	Label       string
	Results     []string
	SelectedIdx int
}

// SearchBoxState creates a new search box state
func NewSearchBoxState(label, placeholder string, width int) *SearchBoxState {
	return SearchBoxStateFor(label, placeholder, width)
}

func SearchBoxStateFor(label, placeholder string, width int) *SearchBoxState {
	return &SearchBoxState{
		InputState:  InputStateFor(placeholder, width),
		Label:       label,
		SelectedIdx: -1,
	}
}

// RenderSearchBox renders a search input box
// Matches src/components/SearchBox.tsx
func RenderSearchBox(state *SearchBoxState) string {
	var lines []string

	// Label and input
	labelLine := ""
	if state.Label != "" {
		labelLine = renderBold(state.Label + ": ")
	}
	labelLine += RenderInput(state.InputState)
	lines = append(lines, labelLine)

	// Results
	if len(state.Results) > 0 {
		for i, result := range state.Results {
			prefix := "  "
			text := result
			if i == state.SelectedIdx {
				text = renderColored(colorPermission, result)
				prefix = renderColored(colorPermission, "› ")
			} else {
				text = renderMuted(result)
			}
			lines = append(lines, prefix+text)
		}
	}

	return strings.Join(lines, "\n")
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderColoredBg(color components.RGB, text string) string {
	// White text on colored background
	return fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;255;255;255m%s\033[0m", color.R, color.G, color.B, text)
}

func renderBold(text string) string {
	return "\033[1m" + text + "\033[0m"
}

func renderMuted(text string) string {
	return renderColored(colorMuted, text)
}

func renderInlineHighlights(text string) string {
	if text == "" {
		return ""
	}
	return inlineHighlightPattern.ReplaceAllStringFunc(text, func(token string) string {
		if strings.HasPrefix(token, "/") {
			return renderColored(colorPermission, token)
		}
		if strings.HasPrefix(token, "@") {
			return renderColored(colorClaude, token)
		}
		return token
	})
}

// Paste support methods

// ensurePasteManager initializes the paste manager if needed.
func (s *PromptInputState) ensurePasteManager() {
	if s.PasteManager == nil {
		s.PasteManager = paste.NewManager()
	}
}

// HandlePaste handles pasted text, returning the text to insert.
// For short text (< PasteThreshold), returns the text as-is.
// For long text, stores it and returns a collapsed reference.
func (s *PromptInputState) HandlePaste(text string) string {
	s.ensurePasteManager()
	return s.PasteManager.AddPaste(text)
}

// ExpandInput expands all paste references in the current input value.
func (s *PromptInputState) ExpandInput() string {
	if s.PasteManager == nil {
		return s.Value
	}
	return s.PasteManager.ExpandInput(s.Value)
}

// ClearPastes clears all stored paste content.
func (s *PromptInputState) ClearPastes() {
	if s.PasteManager != nil {
		s.PasteManager.Clear()
	}
}

// GetPastedContent returns the stored paste content for a given ID.
func (s *PromptInputState) GetPastedContent(id int) *paste.PastedContent {
	if s.PasteManager == nil {
		return nil
	}
	return s.PasteManager.GetContent(id)
}

// GetPasteManager returns the paste manager, initializing if needed.
func (s *PromptInputState) GetPasteManager() *paste.Manager {
	s.ensurePasteManager()
	return s.PasteManager
}
