package components

import (
	"math"
	"strings"
)

// Progress bar block characters (same as TS BLOCKS array)
// Matches src/components/design-system/ProgressBar.tsx
var progressBlocks = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉', '█'}

// ProgressBarConfig holds configuration for rendering a progress bar
type ProgressBarConfig struct {
	Ratio      float64 // [0, 1] - how much progress to display
	Width      int     // how many characters wide
	FillColor  string  // ANSI color for filled portion (optional)
	EmptyColor string  // ANSI color for empty portion (optional)
}

// RenderProgressBar renders a unicode block-based progress bar
// Matches the behavior of src/components/design-system/ProgressBar.tsx
func RenderProgressBar(cfg ProgressBarConfig) string {
	// Clamp ratio to [0, 1]
	ratio := math.Min(1, math.Max(0, cfg.Ratio))

	// Calculate whole blocks
	whole := int(math.Floor(ratio * float64(cfg.Width)))

	var segments []string

	// Add filled blocks
	if whole > 0 {
		fullBlock := string(progressBlocks[len(progressBlocks)-1])
		segments = append(segments, strings.Repeat(fullBlock, whole))
	}

	// Add partial block and empty space if not completely filled
	if whole < cfg.Width {
		// Calculate partial block
		remainder := ratio*float64(cfg.Width) - float64(whole)
		middleIndex := int(math.Floor(remainder * float64(len(progressBlocks))))
		if middleIndex >= len(progressBlocks) {
			middleIndex = len(progressBlocks) - 1
		}
		segments = append(segments, string(progressBlocks[middleIndex]))

		// Add empty blocks
		empty := cfg.Width - whole - 1
		if empty > 0 {
			emptyBlock := string(progressBlocks[0])
			segments = append(segments, strings.Repeat(emptyBlock, empty))
		}
	}

	result := strings.Join(segments, "")

	// Apply colors if provided
	if cfg.FillColor != "" || cfg.EmptyColor != "" {
		result = applyProgressColors(result, cfg.FillColor, cfg.EmptyColor)
	}

	return result
}

// applyProgressColors applies foreground and background colors to progress bar text
func applyProgressColors(text, fgColor, bgColor string) string {
	var b strings.Builder

	if fgColor != "" {
		b.WriteString(fgColor)
	}
	if bgColor != "" {
		b.WriteString(bgColor)
	}

	b.WriteString(text)

	if fgColor != "" || bgColor != "" {
		b.WriteString("\033[0m") // Reset
	}

	return b.String()
}

// ProgressBarSimple renders a simple progress bar with default styling
func ProgressBarSimple(ratio float64, width int) string {
	return RenderProgressBar(ProgressBarConfig{
		Ratio: ratio,
		Width: width,
	})
}

// ProgressBarStyled renders a progress bar with fill and empty colors
func ProgressBarStyled(ratio float64, width int, fillColor, emptyColor string) string {
	return RenderProgressBar(ProgressBarConfig{
		Ratio:      ratio,
		Width:      width,
		FillColor:  fillColor,
		EmptyColor: emptyColor,
	})
}
