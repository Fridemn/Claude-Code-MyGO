// Package components provides UI components for the CLI.
// virtual_list.go implements a virtualized list with scroll support.
// Matches src/components/VirtualMessageList.tsx and src/hooks/useVirtualScroll.ts
package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// VirtualListItem represents an item in the virtual list
type VirtualListItem interface {
	// Key returns a unique identifier for caching
	Key() string
	// Render returns the rendered content
	Render(width int, focused bool) string
	// Height returns the height in rows (0 = auto-measure)
	Height() int
}

// VirtualListConfig configures the virtual list
type VirtualListConfig struct {
	Items         []VirtualListItem
	Width         int
	Height        int // viewport height in rows
	Overscan      int // extra rows to render above/below viewport
	FocusedIndex  int
	SelectedIndex int
	ShowPointer   bool
	OnItemClick   func(idx int, item VirtualListItem)
}

// VirtualListState manages the state of a virtual list
type VirtualListState struct {
	Items        []VirtualListItem
	ScrollOffset int // scroll position in rows
	ViewportH    int // viewport height
	FocusedIdx   int
	SelectedIdx  int
	Width        int

	// Height cache: key -> measured height
	heightCache map[string]int
	// Computed offsets: item index -> cumulative row offset
	offsets []int
	// Total content height
	totalHeight int
}

// VirtualListConstants define virtualization parameters
// Matches useVirtualScroll.ts defaults
const (
	DefaultEstimateHeight = 3  // estimated height for unmeasured items
	OverscanRows          = 40 // extra rows rendered above/below viewport
	ColdStartCount        = 30 // items rendered before layout
	MaxMountedItems       = 300
	SlideStep             = 25 // max new items per render
)

// VirtualListState creates a new virtual list state
func VirtualListStateFor(items []VirtualListItem, width, viewportH int) *VirtualListState {
	state := &VirtualListState{
		Items:       items,
		ViewportH:   viewportH,
		Width:       width,
		heightCache: make(map[string]int),
	}
	state.recomputeOffsets()
	return state
}

// SetItems updates the items and recomputes offsets
func (s *VirtualListState) SetItems(items []VirtualListItem) {
	s.Items = items
	s.recomputeOffsets()
	// Clamp focus
	if s.FocusedIdx >= len(items) {
		s.FocusedIdx = max(0, len(items)-1)
	}
}

// SetViewport updates the viewport dimensions
func (s *VirtualListState) SetViewport(width, height int) {
	if width != s.Width {
		// Width changed - invalidate height cache
		s.heightCache = make(map[string]int)
	}
	s.Width = width
	s.ViewportH = height
	s.recomputeOffsets()
}

// SetHeight caches a measured height for an item
func (s *VirtualListState) SetHeight(key string, h int) {
	if old, ok := s.heightCache[key]; ok && old == h {
		return
	}
	s.heightCache[key] = h
	s.recomputeOffsets()
}

// recomputeOffsets builds the cumulative offset array
func (s *VirtualListState) recomputeOffsets() {
	n := len(s.Items)
	s.offsets = make([]int, n+1)
	offset := 0
	for i, item := range s.Items {
		s.offsets[i] = offset
		h := s.getItemHeight(item)
		offset += h
	}
	s.offsets[n] = offset
	s.totalHeight = offset
}

// getItemHeight returns cached or estimated height
func (s *VirtualListState) getItemHeight(item VirtualListItem) int {
	if h, ok := s.heightCache[item.Key()]; ok {
		return h
	}
	// Check if item provides its own height
	if h := item.Height(); h > 0 {
		return h
	}
	return DefaultEstimateHeight
}

// GetVisibleRange returns the range of items to render [start, end)
func (s *VirtualListState) GetVisibleRange() (start, end int) {
	n := len(s.Items)
	if n == 0 || s.ViewportH <= 0 {
		return 0, min(n, ColdStartCount)
	}

	// Find first visible item
	start = s.findItemAtOffset(s.ScrollOffset - OverscanRows)
	start = max(0, start)

	// Find last visible item
	endOffset := s.ScrollOffset + s.ViewportH + OverscanRows
	end = s.findItemAtOffset(endOffset) + 1
	end = min(n, end)

	// Apply max mounted limit
	if end-start > MaxMountedItems {
		// Center around current scroll position
		midOffset := s.ScrollOffset + s.ViewportH/2
		midItem := s.findItemAtOffset(midOffset)
		start = max(0, midItem-MaxMountedItems/2)
		end = min(n, start+MaxMountedItems)
	}

	return start, end
}

// findItemAtOffset finds the item index at a given row offset
func (s *VirtualListState) findItemAtOffset(offset int) int {
	if offset <= 0 {
		return 0
	}
	// Binary search
	lo, hi := 0, len(s.Items)
	for lo < hi {
		mid := (lo + hi) / 2
		if s.offsets[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return max(0, lo-1)
}

// GetTopSpacer returns the height of space before rendered items
func (s *VirtualListState) GetTopSpacer() int {
	start, _ := s.GetVisibleRange()
	if start >= len(s.offsets) {
		return 0
	}
	return s.offsets[start]
}

// GetBottomSpacer returns the height of space after rendered items
func (s *VirtualListState) GetBottomSpacer() int {
	_, end := s.GetVisibleRange()
	if end >= len(s.offsets) {
		return 0
	}
	return s.totalHeight - s.offsets[end]
}

// GetItemTop returns the row offset of an item
func (s *VirtualListState) GetItemTop(idx int) int {
	if idx < 0 || idx >= len(s.offsets) {
		return -1
	}
	return s.offsets[idx]
}

// GetItemHeightByIndex returns the height of item at index
func (s *VirtualListState) GetItemHeightByIndex(idx int) int {
	if idx < 0 || idx >= len(s.Items) {
		return DefaultEstimateHeight
	}
	return s.getItemHeight(s.Items[idx])
}

// ScrollTo scrolls to a specific row offset
func (s *VirtualListState) ScrollTo(offset int) {
	maxOffset := max(0, s.totalHeight-s.ViewportH)
	s.ScrollOffset = clamp(offset, 0, maxOffset)
}

// ScrollToItem scrolls to make an item visible
func (s *VirtualListState) ScrollToItem(idx int, headroom int) {
	if idx < 0 || idx >= len(s.Items) {
		return
	}
	itemTop := s.offsets[idx]
	itemBottom := itemTop + s.getItemHeight(s.Items[idx])

	// Check if item is already visible
	viewTop := s.ScrollOffset
	viewBottom := viewTop + s.ViewportH

	if itemTop < viewTop+headroom {
		// Scroll up to show item with headroom
		s.ScrollTo(itemTop - headroom)
	} else if itemBottom > viewBottom {
		// Scroll down to show item
		s.ScrollTo(itemBottom - s.ViewportH)
	}
}

// ScrollToBottom scrolls to the bottom
func (s *VirtualListState) ScrollToBottom() {
	s.ScrollTo(max(0, s.totalHeight-s.ViewportH))
}

// IsAtBottom returns true if scrolled to bottom
func (s *VirtualListState) IsAtBottom() bool {
	maxOffset := max(0, s.totalHeight-s.ViewportH)
	return s.ScrollOffset >= maxOffset
}

// MoveUp moves focus up
func (s *VirtualListState) MoveUp() {
	if s.FocusedIdx > 0 {
		s.FocusedIdx--
		s.ScrollToItem(s.FocusedIdx, 3)
	}
}

// MoveDown moves focus down
func (s *VirtualListState) MoveDown() {
	if s.FocusedIdx < len(s.Items)-1 {
		s.FocusedIdx++
		s.ScrollToItem(s.FocusedIdx, 3)
	}
}

// PageUp scrolls up by viewport height
func (s *VirtualListState) PageUp() {
	s.ScrollTo(s.ScrollOffset - s.ViewportH + 2)
	// Move focus to first visible item
	first := s.findItemAtOffset(s.ScrollOffset)
	s.FocusedIdx = clamp(first, 0, len(s.Items)-1)
}

// PageDown scrolls down by viewport height
func (s *VirtualListState) PageDown() {
	s.ScrollTo(s.ScrollOffset + s.ViewportH - 2)
	// Move focus to first visible item
	first := s.findItemAtOffset(s.ScrollOffset)
	s.FocusedIdx = clamp(first, 0, len(s.Items)-1)
}

// HandleKey processes keyboard input, returns true if handled
func (s *VirtualListState) HandleKey(key string) bool {
	switch key {
	case "up", "k":
		s.MoveUp()
		return true
	case "down", "j":
		s.MoveDown()
		return true
	case "pgup", "ctrl+b":
		s.PageUp()
		return true
	case "pgdown", "ctrl+f":
		s.PageDown()
		return true
	case "home", "g":
		s.FocusedIdx = 0
		s.ScrollTo(0)
		return true
	case "end", "G":
		s.FocusedIdx = max(0, len(s.Items)-1)
		s.ScrollToBottom()
		return true
	}
	return false
}

// HandleMouse processes mouse input
func (s *VirtualListState) HandleMouse(msg tea.MouseMsg) bool {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		s.ScrollTo(s.ScrollOffset - 3)
		return true
	case tea.MouseButtonWheelDown:
		s.ScrollTo(s.ScrollOffset + 3)
		return true
	}
	return false
}

// Render renders the virtual list
func RenderVirtualList(state *VirtualListState, cfg VirtualListConfig) string {
	if len(state.Items) == 0 {
		return ""
	}

	start, end := state.GetVisibleRange()
	var lines []string

	// Top spacer
	if topSpacer := state.GetTopSpacer(); topSpacer > 0 {
		lines = append(lines, strings.Repeat("\n", topSpacer-1))
	}

	// Rendered items
	for i := start; i < end; i++ {
		item := state.Items[i]
		isFocused := i == state.FocusedIdx && cfg.ShowPointer
		rendered := item.Render(cfg.Width, isFocused)

		// Measure actual height
		actualH := strings.Count(rendered, "\n") + 1
		state.SetHeight(item.Key(), actualH)

		lines = append(lines, rendered)
	}

	// Bottom spacer
	if bottomSpacer := state.GetBottomSpacer(); bottomSpacer > 0 {
		lines = append(lines, strings.Repeat("\n", bottomSpacer-1))
	}

	return strings.Join(lines, "\n")
}

// StickyPrompt represents a sticky prompt header
// Matches src/components/VirtualMessageList.tsx StickyPrompt
type StickyPrompt struct {
	Text     string
	ScrollTo func()
}

// ScrollIndicator represents scroll position indicators
type ScrollIndicator struct {
	ShowUp    bool // can scroll up
	ShowDown  bool // can scroll down
	Position  int  // current position (0-100)
	AtBottom  bool // pinned to bottom (sticky scroll)
	NewCount  int  // count of new messages since scroll
	NewOffset int  // offset where new messages start
}

// GetScrollIndicator returns scroll indicators for the current state
func (s *VirtualListState) GetScrollIndicator() ScrollIndicator {
	maxOffset := max(0, s.totalHeight-s.ViewportH)
	position := 0
	if maxOffset > 0 {
		position = (s.ScrollOffset * 100) / maxOffset
	}

	return ScrollIndicator{
		ShowUp:   s.ScrollOffset > 0,
		ShowDown: s.ScrollOffset < maxOffset,
		Position: position,
		AtBottom: s.IsAtBottom(),
	}
}

// Helper functions

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
