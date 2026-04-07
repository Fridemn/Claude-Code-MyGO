// Package components provides UI components for the CLI.
// fullscreen_layout.go implements the main fullscreen layout with modal support.
// Matches src/components/FullscreenLayout.tsx
package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Layout colors
var (
	layoutMuted  = RGB{134, 145, 160}
	layoutPill   = RGB{215, 119, 87}
	layoutPillBg = RGB{42, 42, 42}
)

// fgColor returns ANSI foreground color escape code
func fgColor(c RGB) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

// bgColor returns ANSI background color escape code
func bgColor(c RGB) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

const ansiReset = "\033[0m"

// FullscreenLayoutConfig configures the fullscreen layout
type FullscreenLayoutConfig struct {
	Width  int
	Height int

	// Sticky prompt header (when scrolled away from prompt)
	StickyPrompt *StickyPrompt
	HideSticky   bool

	// Scroll pill ("N new" or "Jump to bottom")
	NewMessageCount int
	HidePill        bool

	// Modal dialog (fullscreen overlay)
	Modal      string // rendered modal content
	ModalTitle string
}

// FullscreenLayoutRegions contains the different content regions
type FullscreenLayoutRegions struct {
	// Scrollable content (messages)
	Scrollable string

	// Bottom pinned content (input, spinner, permissions)
	Bottom string

	// Overlay content (rendered inside scroll area after messages)
	Overlay string

	// Floating content (bottom-right, e.g., companion bubble)
	BottomFloat string

	// Modal content (fullscreen pane over everything)
	Modal string
}

// FullscreenLayoutState manages the layout state
type FullscreenLayoutState struct {
	Width  int
	Height int

	// Scroll management
	ScrollOffset    int
	ViewportHeight  int
	ContentHeight   int
	IsStickyScroll  bool // pinned to bottom
	DividerY        int  // position of "N new" divider
	NewMessageCount int

	// Sticky prompt
	StickyPrompt *StickyPrompt
	HideSticky   bool

	// Modal state
	ShowModal  bool
	ModalTitle string
}

// Modal peek rows (context visible above modal)
const ModalTranscriptPeek = 2

// FullscreenLayoutState creates a new layout state
func FullscreenLayoutStateFor(width, height int) *FullscreenLayoutState {
	return &FullscreenLayoutState{
		Width:          width,
		Height:         height,
		ViewportHeight: height - 3, // reserve for bottom section
		IsStickyScroll: true,
	}
}

// SetSize updates the layout dimensions
func (s *FullscreenLayoutState) SetSize(width, height int) {
	s.Width = width
	s.Height = height
	s.ViewportHeight = height - 3
}

// SetContentHeight updates the content height
func (s *FullscreenLayoutState) SetContentHeight(h int) {
	s.ContentHeight = h
	// Clamp scroll offset
	maxScroll := max(0, h-s.ViewportHeight)
	if s.ScrollOffset > maxScroll {
		s.ScrollOffset = maxScroll
	}
}

// ScrollTo scrolls to a specific offset
func (s *FullscreenLayoutState) ScrollTo(offset int) {
	maxScroll := max(0, s.ContentHeight-s.ViewportHeight)
	s.ScrollOffset = clamp(offset, 0, maxScroll)
	s.IsStickyScroll = s.ScrollOffset >= maxScroll
}

// ScrollBy scrolls by a delta
func (s *FullscreenLayoutState) ScrollBy(delta int) {
	s.ScrollTo(s.ScrollOffset + delta)
}

// ScrollToBottom scrolls to the bottom and enables sticky scroll
func (s *FullscreenLayoutState) ScrollToBottom() {
	maxScroll := max(0, s.ContentHeight-s.ViewportHeight)
	s.ScrollOffset = maxScroll
	s.IsStickyScroll = true
	s.ClearNewDivider()
}

// OnScrollAway is called when the user scrolls away from bottom
// It snapshots the current position for the "N new" divider
func (s *FullscreenLayoutState) OnScrollAway() {
	if s.DividerY == 0 && !s.IsStickyScroll {
		s.DividerY = s.ContentHeight
	}
}

// ClearNewDivider clears the "N new" divider
func (s *FullscreenLayoutState) ClearNewDivider() {
	s.DividerY = 0
	s.NewMessageCount = 0
}

// ShouldShowPill returns true if the "N new" pill should be shown
func (s *FullscreenLayoutState) ShouldShowPill() bool {
	if s.DividerY == 0 || s.IsStickyScroll {
		return false
	}
	// Show if we haven't scrolled past the divider
	viewportBottom := s.ScrollOffset + s.ViewportHeight
	return viewportBottom < s.DividerY
}

// HandleKey handles keyboard input
func (s *FullscreenLayoutState) HandleKey(key string) bool {
	switch key {
	case "pgup", "ctrl+b":
		s.ScrollBy(-s.ViewportHeight + 2)
		s.OnScrollAway()
		return true
	case "pgdown", "ctrl+f":
		s.ScrollBy(s.ViewportHeight - 2)
		return true
	case "ctrl+u":
		s.ScrollBy(-s.ViewportHeight / 2)
		s.OnScrollAway()
		return true
	case "ctrl+d":
		s.ScrollBy(s.ViewportHeight / 2)
		return true
	case "home", "ctrl+home":
		s.ScrollTo(0)
		s.OnScrollAway()
		return true
	case "end", "ctrl+end":
		s.ScrollToBottom()
		return true
	}
	return false
}

// HandleMouse handles mouse input
func (s *FullscreenLayoutState) HandleMouse(msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		s.ScrollBy(-3)
		s.OnScrollAway()
		return true
	case tea.MouseButtonWheelDown:
		s.ScrollBy(3)
		return true
	}
	return false
}

// RenderFullscreenLayout renders the full layout
func RenderFullscreenLayout(state *FullscreenLayoutState, regions FullscreenLayoutRegions) string {
	var sections []string

	// If modal is shown, render it instead
	if state.ShowModal && regions.Modal != "" {
		return renderModalLayout(state, regions)
	}

	// Sticky prompt header (if scrolled away)
	if !state.HideSticky && state.StickyPrompt != nil && !state.IsStickyScroll {
		sections = append(sections, renderStickyHeader(state.StickyPrompt, state.Width))
	}

	// Scrollable content area
	scrollContent := renderScrollArea(state, regions.Scrollable)
	sections = append(sections, scrollContent)

	// Overlay (inside scroll area)
	if regions.Overlay != "" {
		sections = append(sections, regions.Overlay)
	}

	// Bottom float (if any)
	// Note: In a real implementation this would use absolute positioning

	// Bottom pinned section
	if regions.Bottom != "" {
		sections = append(sections, regions.Bottom)
	}

	// Scroll pill
	if state.ShouldShowPill() {
		pill := renderScrollPill(state)
		sections = append(sections, pill)
	}

	return strings.Join(sections, "\n")
}

// renderScrollArea renders the scrollable content with viewport clipping
func renderScrollArea(state *FullscreenLayoutState, content string) string {
	if content == "" {
		// Return empty lines for viewport
		return strings.Repeat("\n", state.ViewportHeight-1)
	}

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Apply scroll offset
	startLine := state.ScrollOffset
	endLine := startLine + state.ViewportHeight

	if startLine >= totalLines {
		startLine = max(0, totalLines-1)
	}
	if endLine > totalLines {
		endLine = totalLines
	}

	visibleLines := lines[startLine:endLine]

	// Pad to fill viewport
	for len(visibleLines) < state.ViewportHeight {
		visibleLines = append(visibleLines, "")
	}

	return strings.Join(visibleLines, "\n")
}

// renderStickyHeader renders the sticky prompt header
func renderStickyHeader(prompt *StickyPrompt, width int) string {
	if prompt == nil || prompt.Text == "" {
		return ""
	}

	// Truncate long prompts
	text := prompt.Text
	maxLen := width - 4
	if len(text) > maxLen {
		text = text[:maxLen-1] + "…"
	}

	// Render with muted color and top border
	header := fgColor(layoutMuted) + "▔" + strings.Repeat("▔", width-2) + ansiReset + "\n"
	header += fgColor(layoutMuted) + "❯ " + text + ansiReset

	return header
}

// renderScrollPill renders the "N new" or "Jump to bottom" pill
func renderScrollPill(state *FullscreenLayoutState) string {
	var text string
	if state.NewMessageCount > 0 {
		if state.NewMessageCount == 1 {
			text = "1 new message"
		} else {
			text = fmt.Sprintf("%d new messages", state.NewMessageCount)
		}
	} else {
		text = "Jump to bottom"
	}

	// Render pill style
	pill := bgColor(layoutPillBg) + fgColor(layoutPill) + fmt.Sprintf(" ↓ %s ", text) + ansiReset

	// Center the pill
	padding := (state.Width - len(text) - 5) / 2
	if padding > 0 {
		pill = strings.Repeat(" ", padding) + pill
	}

	return pill
}

// renderModalLayout renders the layout with modal overlay
func renderModalLayout(state *FullscreenLayoutState, regions FullscreenLayoutRegions) string {
	var sections []string

	// Show a few rows of transcript context above the modal
	if state.ContentHeight > 0 {
		contextLines := min(ModalTranscriptPeek, state.ContentHeight)
		// In a real implementation, extract last N lines of content
		sections = append(sections, strings.Repeat("…\n", contextLines))
	}

	// Modal divider
	divider := fgColor(layoutMuted) + "▔" + strings.Repeat("▔", state.Width-2) + ansiReset
	sections = append(sections, divider)

	// Modal content
	modalHeight := state.Height - ModalTranscriptPeek - 2
	modalContent := padToHeight(regions.Modal, modalHeight)
	sections = append(sections, modalContent)

	return strings.Join(sections, "\n")
}

// padToHeight pads content to fill the specified height
func padToHeight(content string, height int) string {
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
