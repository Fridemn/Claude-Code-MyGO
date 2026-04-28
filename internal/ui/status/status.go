package status

import (
	"fmt"
	"strings"

	"claude-go/internal/ui/components"
)

// StatusLineConfig holds configuration for the status line
type StatusLineConfig struct {
	Model          string
	PermissionMode string
	CWD            string
	SessionName    string
	TotalCost      float64
	TokenUsage     TokenUsage
	RateLimits     *RateLimits
	Width          int
	ShowCost       bool
	ShowTokens     bool
	ShowRateLimits bool
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	InputTokens       int
	OutputTokens      int
	ContextWindowSize int
	UsedPercentage    float64
}

// RateLimits represents rate limit information
type RateLimits struct {
	FiveHourUsed   float64
	FiveHourResets string
	SevenDayUsed   float64
	SevenDayResets string
}

// Theme colors
var (
	colorMuted      = components.RGB{134, 145, 160}
	colorText       = components.RGB{255, 255, 255}
	colorSuccess    = components.RGB{78, 186, 101}
	colorWarning    = components.RGB{255, 193, 7}
	colorError      = components.RGB{255, 107, 128}
	colorPermission = components.RGB{177, 185, 249}
	colorInfo       = components.RGB{115, 198, 255}
)

// RenderStatusLine renders the bottom status line
// Matches src/components/StatusLine.tsx
func RenderStatusLine(cfg StatusLineConfig) string {
	if cfg.Width <= 0 {
		cfg.Width = 80
	}

	var leftParts []string
	var rightParts []string

	// Left side: model and permission mode
	if cfg.Model != "" {
		leftParts = append(leftParts, renderMuted(cfg.Model))
	}
	if cfg.PermissionMode != "" && cfg.PermissionMode != "default" {
		modeColor := colorPermission
		if cfg.PermissionMode == "auto" {
			modeColor = colorSuccess
		} else if cfg.PermissionMode == "bypass" {
			modeColor = colorError
		}
		leftParts = append(leftParts, renderColored(modeColor, "["+cfg.PermissionMode+"]"))
	}

	// Right side: costs, tokens, rate limits
	if cfg.ShowCost && cfg.TotalCost > 0 {
		rightParts = append(rightParts, fmt.Sprintf("$%.2f", cfg.TotalCost))
	}

	if cfg.ShowTokens && cfg.TokenUsage.ContextWindowSize > 0 {
		usageText := formatTokenUsage(cfg.TokenUsage)
		rightParts = append(rightParts, usageText)
	}

	if cfg.ShowRateLimits && cfg.RateLimits != nil {
		if cfg.RateLimits.FiveHourUsed > 0 {
			color := colorMuted
			if cfg.RateLimits.FiveHourUsed > 80 {
				color = colorWarning
			}
			if cfg.RateLimits.FiveHourUsed > 95 {
				color = colorError
			}
			rightParts = append(rightParts, renderColored(color, fmt.Sprintf("5h: %.0f%%", cfg.RateLimits.FiveHourUsed)))
		}
	}

	// Build the status line
	leftStr := strings.Join(leftParts, " · ")
	rightStr := strings.Join(rightParts, " · ")

	// Calculate padding
	leftWidth := visibleWidth(leftStr)
	rightWidth := visibleWidth(rightStr)
	padding := cfg.Width - leftWidth - rightWidth - 2

	if padding < 1 {
		// Not enough space, just show left side
		return leftStr
	}

	return leftStr + strings.Repeat(" ", padding) + renderMuted(rightStr)
}

// formatTokenUsage formats token usage for display
func formatTokenUsage(usage TokenUsage) string {
	total := usage.InputTokens + usage.OutputTokens
	return fmt.Sprintf("%s/%s (%.0f%%)",
		formatNumber(total),
		formatNumber(usage.ContextWindowSize),
		usage.UsedPercentage)
}

// RenderStatusNotice renders a status notice
// Matches src/components/StatusNotices.tsx
type NoticeType int

const (
	NoticeInfo NoticeType = iota
	NoticeWarning
	NoticeError
	NoticeSuccess
)

type StatusNotice struct {
	Type    NoticeType
	Title   string
	Message string
	Action  string // Optional action hint
}

func RenderStatusNotice(notice StatusNotice, width int) string {
	var icon string
	var color components.RGB

	switch notice.Type {
	case NoticeSuccess:
		icon = "✓"
		color = colorSuccess
	case NoticeWarning:
		icon = "⚠"
		color = colorWarning
	case NoticeError:
		icon = "✗"
		color = colorError
	default:
		icon = "ℹ"
		color = colorInfo
	}

	var lines []string

	// Title line
	titleLine := renderColored(color, icon+" "+notice.Title)
	lines = append(lines, titleLine)

	// Message
	if notice.Message != "" {
		for _, line := range wrapText(notice.Message, width-4) {
			lines = append(lines, "  "+renderMuted(line))
		}
	}

	// Action hint
	if notice.Action != "" {
		lines = append(lines, "  "+renderMuted("→ "+notice.Action))
	}

	return strings.Join(lines, "\n")
}

// RenderTokenWarning renders a token usage warning
// Matches src/components/TokenWarning.tsx
func RenderTokenWarning(usage TokenUsage, width int) string {
	if usage.UsedPercentage < 75 {
		return "" // No warning needed
	}

	var level string

	if usage.UsedPercentage >= 95 {
		level = "critical"
	} else if usage.UsedPercentage >= 90 {
		level = "high"
	} else {
		level = "elevated"
	}

	message := fmt.Sprintf("Context window usage %s (%.0f%%)", level, usage.UsedPercentage)

	notice := StatusNotice{
		Type:    NoticeWarning,
		Title:   "Token Usage Warning",
		Message: message,
		Action:  "Consider using /compact to summarize the conversation",
	}

	return RenderStatusNotice(notice, width)
}

// RenderMemoryUsage renders a memory usage indicator
// Matches src/components/MemoryUsageIndicator.tsx
func RenderMemoryUsage(usedMB, totalMB int, width int) string {
	percentage := float64(usedMB) / float64(totalMB) * 100

	color := colorMuted
	if percentage >= 90 {
		color = colorError
	} else if percentage >= 75 {
		color = colorWarning
	}

	bar := components.ProgressBarSimple(percentage/100, 10)
	text := fmt.Sprintf("%dMB/%dMB", usedMB, totalMB)

	return renderColored(color, bar+" "+text)
}

// RenderIdeStatus renders IDE connection status
// Matches src/components/IdeStatusIndicator.tsx
type IdeStatus int

const (
	IdeDisconnected IdeStatus = iota
	IdeConnecting
	IdeConnected
)

func RenderIdeStatus(status IdeStatus, ideName string) string {
	var icon string
	var text string
	var color components.RGB

	switch status {
	case IdeConnected:
		icon = "●"
		text = ideName + " connected"
		color = colorSuccess
	case IdeConnecting:
		icon = "○"
		text = "Connecting to " + ideName + "..."
		color = colorWarning
	default:
		icon = "○"
		text = "IDE not connected"
		color = colorMuted
	}

	return renderColored(color, icon+" "+text)
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderMuted(text string) string {
	return renderColored(colorMuted, text)
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

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

func wrapText(text string, width int) []string {
	if width <= 0 || len(text) <= width {
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
