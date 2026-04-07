package components

import (
	"fmt"
)

// Status icon constants matching TS src/constants/figures.js
const (
	BlackCircle       = "●"
	CheckMark         = "✓"
	CrossMark         = "✗"
	WarningSign       = "⚠"
	InfoSign          = "ℹ"
	ArrowRight        = "→"
	ArrowLeft         = "←"
	ArrowUp           = "↑"
	ArrowDown         = "↓"
	Bullet            = "•"
	Ellipsis          = "…"
	PointerSmall      = "›"
	PointerLarge      = "❯"
	Line              = "─"
	VerticalLine      = "│"
	CornerTopLeft     = "╭"
	CornerTopRight    = "╮"
	CornerBottomLeft  = "╰"
	CornerBottomRight = "╯"
	Tee               = "├"
	TeeEnd            = "└"
	Spinner           = "◠"
)

// StatusIconType represents different status states
type StatusIconType int

const (
	StatusPending StatusIconType = iota
	StatusSuccess
	StatusError
	StatusWarning
	StatusInfo
	StatusActive
)

// StatusIcon represents a status indicator icon with color
type StatusIcon struct {
	Icon  string
	Type  StatusIconType
	Color string // ANSI color code
}

// RGB color helpers matching theme.go palette
var statusColors = map[StatusIconType]struct{ r, g, b int }{
	StatusPending: {134, 145, 160}, // muted
	StatusSuccess: {78, 186, 101},  // success
	StatusError:   {255, 107, 128}, // error
	StatusWarning: {255, 193, 7},   // warning
	StatusInfo:    {115, 198, 255}, // info
	StatusActive:  {215, 119, 87},  // claude
}

// RenderStatusIcon renders a status icon with appropriate color
// Matches ToolUseLoader.tsx behavior
func RenderStatusIcon(status StatusIconType, dimmed bool) string {
	icon := BlackCircle

	switch status {
	case StatusSuccess:
		icon = CheckMark
	case StatusError:
		icon = CrossMark
	case StatusWarning:
		icon = WarningSign
	case StatusInfo:
		icon = InfoSign
	case StatusActive:
		icon = BlackCircle
	case StatusPending:
		icon = BlackCircle
	}

	color := statusColors[status]

	if dimmed {
		return fmt.Sprintf("\033[2m\033[38;2;%d;%d;%dm%s\033[0m", color.r, color.g, color.b, icon)
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.r, color.g, color.b, icon)
}

// RenderToolUseLoader renders the tool use loading indicator
// Matches src/components/ToolUseLoader.tsx
func RenderToolUseLoader(isError, isUnresolved, shouldAnimate, isBlinking bool) string {
	// Determine what to show
	if !shouldAnimate || isBlinking || isError || !isUnresolved {
		// Show the circle
		if isUnresolved {
			// Pending state - dimmed
			return RenderStatusIcon(StatusPending, true)
		} else if isError {
			return RenderStatusIcon(StatusError, false)
		} else {
			return RenderStatusIcon(StatusSuccess, false)
		}
	}

	// Return space when not showing (blinking off state)
	return " "
}

// ToolUseLoaderState holds state for animated tool use loader
type ToolUseLoaderState struct {
	IsError      bool
	IsUnresolved bool
	BlinkTick    int
}

// ShouldBlink returns true if the loader should be in "blink off" state
// Blink interval matches useBlink.ts (530ms on, 530ms off = ~1Hz)
func (s *ToolUseLoaderState) ShouldBlink(tickMs int) bool {
	// 530ms cycle
	cycleMs := tickMs % 1060
	return cycleMs >= 530
}

// Render renders the tool use loader for the current state
func (s *ToolUseLoaderState) Render(tickMs int, shouldAnimate bool) string {
	isBlinking := s.ShouldBlink(tickMs)
	return RenderToolUseLoader(s.IsError, s.IsUnresolved, shouldAnimate, isBlinking)
}

// StatusBadge renders a status badge with background color
func StatusBadge(text string, status StatusIconType) string {
	color := statusColors[status]
	// Dark background with colored text
	return fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;255;255;255m %s \033[0m",
		color.r/4, color.g/4, color.b/4, // Darken for background
		text)
}
