package components

import (
	"fmt"
	"strings"
)

// Tab represents a single tab in a tab bar
type Tab struct {
	Label    string
	Active   bool
	Disabled bool
	Badge    string // Optional badge text (e.g., count)
}

// TabConfig holds configuration for rendering tabs
type TabConfig struct {
	Tabs          []Tab
	ActiveColor   RGB
	InactiveColor RGB
	DisabledColor RGB
	Separator     string // Default: " │ "
	ShowUnderline bool   // Show underline for active tab
}

// Default tab colors
var (
	TabColorActive   = RGB{177, 185, 249} // permission color
	TabColorInactive = RGB{134, 145, 160} // muted
	TabColorDisabled = RGB{80, 80, 80}    // subtle
)

// RenderTabs renders a horizontal tab bar
// Matches src/components/design-system/Tabs.tsx
func RenderTabs(cfg TabConfig) string {
	if len(cfg.Tabs) == 0 {
		return ""
	}

	// Set defaults
	if cfg.ActiveColor == (RGB{}) {
		cfg.ActiveColor = TabColorActive
	}
	if cfg.InactiveColor == (RGB{}) {
		cfg.InactiveColor = TabColorInactive
	}
	if cfg.DisabledColor == (RGB{}) {
		cfg.DisabledColor = TabColorDisabled
	}
	if cfg.Separator == "" {
		cfg.Separator = " │ "
	}

	var parts []string

	for _, tab := range cfg.Tabs {
		var tabText string

		// Add label
		label := tab.Label
		if tab.Badge != "" {
			label = fmt.Sprintf("%s (%s)", label, tab.Badge)
		}

		// Apply styling based on state
		if tab.Disabled {
			tabText = renderTabText(label, cfg.DisabledColor, false, false)
		} else if tab.Active {
			tabText = renderTabText(label, cfg.ActiveColor, true, cfg.ShowUnderline)
		} else {
			tabText = renderTabText(label, cfg.InactiveColor, false, false)
		}

		parts = append(parts, tabText)
	}

	// Join with separator
	separator := fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m",
		cfg.InactiveColor.R, cfg.InactiveColor.G, cfg.InactiveColor.B, cfg.Separator)

	return strings.Join(parts, separator)
}

// renderTabText renders a single tab with appropriate styling
func renderTabText(text string, color RGB, bold, underline bool) string {
	var codes []string
	if bold {
		codes = append(codes, "1")
	}
	if underline {
		codes = append(codes, "4")
	}

	codeStr := ""
	if len(codes) > 0 {
		codeStr = strings.Join(codes, ";") + ";"
	}

	return fmt.Sprintf("\033[%s38;2;%d;%d;%dm%s\033[0m", codeStr, color.R, color.G, color.B, text)
}

// TagTabs renders tabs as tags/pills (matches TagTabs.tsx)
type TagTab struct {
	Label  string
	Active bool
	Color  RGB // Custom color for this tag
}

// RenderTagTabs renders tabs as colored tags
func RenderTagTabs(tabs []TagTab, gap int) string {
	if len(tabs) == 0 {
		return ""
	}

	if gap <= 0 {
		gap = 1
	}

	var parts []string
	for _, tab := range tabs {
		color := tab.Color
		if color == (RGB{}) {
			if tab.Active {
				color = TabColorActive
			} else {
				color = TabColorInactive
			}
		}

		var text string
		if tab.Active {
			// Active: colored background with white text
			text = fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;255;255;255m %s \033[0m",
				color.R, color.G, color.B, tab.Label)
		} else {
			// Inactive: just colored text
			text = fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m",
				color.R, color.G, color.B, tab.Label)
		}

		parts = append(parts, text)
	}

	return strings.Join(parts, strings.Repeat(" ", gap))
}

// TabBarState manages tab bar state with keyboard navigation
// Matches src/components/design-system/Tabs.tsx behavior
type TabBarState struct {
	Tabs       []string
	ActiveIdx  int
	OnChange   func(idx int)
	IsFocused  bool // Whether the tab header is focused (vs content)
	IsDisabled bool // Disable navigation
}

// TabBarState creates a new tab bar state
func NewTabBarState(tabs []string, initialActive int) *TabBarState {
	return TabBarStateFor(tabs, initialActive)
}

func TabBarStateFor(tabs []string, initialActive int) *TabBarState {
	if initialActive < 0 || initialActive >= len(tabs) {
		initialActive = 0
	}
	return &TabBarState{
		Tabs:      tabs,
		ActiveIdx: initialActive,
		IsFocused: true,
	}
}

// Next moves to the next tab (tabs:next keybinding)
func (s *TabBarState) Next() {
	if len(s.Tabs) == 0 || s.IsDisabled {
		return
	}
	s.ActiveIdx = (s.ActiveIdx + 1) % len(s.Tabs)
	s.IsFocused = true
	if s.OnChange != nil {
		s.OnChange(s.ActiveIdx)
	}
}

// Prev moves to the previous tab (tabs:previous keybinding)
func (s *TabBarState) Prev() {
	if len(s.Tabs) == 0 || s.IsDisabled {
		return
	}
	s.ActiveIdx = (s.ActiveIdx - 1 + len(s.Tabs)) % len(s.Tabs)
	s.IsFocused = true
	if s.OnChange != nil {
		s.OnChange(s.ActiveIdx)
	}
}

// SetActive sets the active tab by index
func (s *TabBarState) SetActive(idx int) {
	if idx < 0 || idx >= len(s.Tabs) || s.IsDisabled {
		return
	}
	s.ActiveIdx = idx
	if s.OnChange != nil {
		s.OnChange(idx)
	}
}

// SetActiveByName sets the active tab by name
func (s *TabBarState) SetActiveByName(name string) {
	for i, tab := range s.Tabs {
		if tab == name {
			s.SetActive(i)
			return
		}
	}
}

// GetActive returns the active tab name
func (s *TabBarState) GetActive() string {
	if s.ActiveIdx >= 0 && s.ActiveIdx < len(s.Tabs) {
		return s.Tabs[s.ActiveIdx]
	}
	return ""
}

// HandleKey handles keyboard input for tab navigation
// Returns true if the key was handled
func (s *TabBarState) HandleKey(key string) bool {
	if s.IsDisabled {
		return false
	}

	switch key {
	case "left", "shift+tab":
		s.Prev()
		return true
	case "right", "tab":
		s.Next()
		return true
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Number keys for quick tab access
		idx := int(key[0] - '1')
		if idx < len(s.Tabs) {
			s.SetActive(idx)
			return true
		}
	}
	return false
}

// FocusHeader focuses the tab header row
func (s *TabBarState) FocusHeader() {
	s.IsFocused = true
}

// BlurHeader blurs the tab header (focus moves to content)
func (s *TabBarState) BlurHeader() {
	s.IsFocused = false
}

// Render renders the tab bar
func (s *TabBarState) Render() string {
	tabs := make([]Tab, len(s.Tabs))
	for i, label := range s.Tabs {
		tabs[i] = Tab{
			Label:  label,
			Active: i == s.ActiveIdx,
		}
	}
	return RenderTabs(TabConfig{Tabs: tabs, ShowUnderline: s.IsFocused})
}

// RenderWithHint renders the tab bar with navigation hint
func (s *TabBarState) RenderWithHint() string {
	tabBar := s.Render()
	hint := fmt.Sprintf("\033[38;2;%d;%d;%dm←/→ switch tabs\033[0m",
		TabColorInactive.R, TabColorInactive.G, TabColorInactive.B)
	return tabBar + "  " + hint
}
