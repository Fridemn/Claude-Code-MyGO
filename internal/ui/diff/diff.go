package diff

import (
	"fmt"
	"strings"

	"claude-code-go/internal/ui/components"
)

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldNum  int // Line number in old file (0 if not applicable)
	NewNum  int // Line number in new file (0 if not applicable)
}

// DiffLineType represents the type of a diff line
type DiffLineType int

const (
	DiffLineContext  DiffLineType = iota // Unchanged line
	DiffLineAdded                        // Added line
	DiffLineRemoved                      // Removed line
	DiffLineModified                     // Modified line (for word-level diffs)
	DiffLineHeader                       // File header
	DiffLineHunk                         // Hunk header (@@ ... @@)
)

// DiffHunk represents a hunk in a diff
type DiffHunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// FileDiff represents a diff for a single file
type FileDiff struct {
	OldPath   string
	NewPath   string
	Hunks     []DiffHunk
	IsNew     bool
	IsDeleted bool
	IsBinary  bool
}

// Theme colors for diff
var (
	colorAdded   = components.RGB{78, 186, 101}  // success/green
	colorRemoved = components.RGB{255, 107, 128} // error/red
	colorContext = components.RGB{134, 145, 160} // muted
	colorHeader  = components.RGB{177, 185, 249} // permission/purple
	colorHunk    = components.RGB{115, 198, 255} // info/blue
	colorText    = components.RGB{255, 255, 255}
)

// DiffConfig holds configuration for rendering diffs
type DiffConfig struct {
	Width           int
	ShowLineNums    bool
	Context         int  // Lines of context around changes
	Unified         bool // Unified diff format (vs side-by-side)
	SyntaxHighlight bool
}

// RenderFileDiff renders a file diff
// Matches src/components/StructuredDiff.tsx and FileEditToolDiff.tsx
func RenderFileDiff(diff FileDiff, cfg DiffConfig) string {
	if cfg.Width <= 0 {
		cfg.Width = 80
	}

	var lines []string

	// File header
	header := renderFileHeader(diff)
	lines = append(lines, header)

	// Binary file indicator
	if diff.IsBinary {
		lines = append(lines, renderMuted("  Binary file"))
		return strings.Join(lines, "\n")
	}

	// Render hunks
	for _, hunk := range diff.Hunks {
		hunkLines := renderHunk(hunk, cfg)
		lines = append(lines, hunkLines...)
	}

	return strings.Join(lines, "\n")
}

// renderFileHeader renders the file diff header
func renderFileHeader(diff FileDiff) string {
	var header string
	if diff.IsNew {
		header = fmt.Sprintf("+ %s (new file)", diff.NewPath)
		return renderColored(colorAdded, header)
	}
	if diff.IsDeleted {
		header = fmt.Sprintf("- %s (deleted)", diff.OldPath)
		return renderColored(colorRemoved, header)
	}
	if diff.OldPath != diff.NewPath {
		header = fmt.Sprintf("  %s → %s", diff.OldPath, diff.NewPath)
	} else {
		header = fmt.Sprintf("  %s", diff.NewPath)
	}
	return renderColored(colorHeader, header)
}

// renderHunk renders a diff hunk
func renderHunk(hunk DiffHunk, cfg DiffConfig) []string {
	var lines []string

	// Hunk header
	hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
	lines = append(lines, renderColored(colorHunk, "  "+hunkHeader))

	// Diff lines
	lineNumWidth := 4
	if cfg.ShowLineNums {
		maxNum := max(hunk.OldStart+hunk.OldCount, hunk.NewStart+hunk.NewCount)
		lineNumWidth = len(fmt.Sprintf("%d", maxNum))
	}

	for _, line := range hunk.Lines {
		rendered := renderDiffLine(line, cfg, lineNumWidth)
		lines = append(lines, rendered)
	}

	return lines
}

// renderDiffLine renders a single diff line
func renderDiffLine(line DiffLine, cfg DiffConfig, lineNumWidth int) string {
	var prefix string
	var content string
	var color components.RGB

	switch line.Type {
	case DiffLineAdded:
		prefix = "+"
		color = colorAdded
	case DiffLineRemoved:
		prefix = "-"
		color = colorRemoved
	case DiffLineContext:
		prefix = " "
		color = colorContext
	case DiffLineModified:
		prefix = "~"
		color = colorHeader
	default:
		prefix = " "
		color = colorText
	}

	content = line.Content

	// Add line numbers if requested
	if cfg.ShowLineNums {
		oldNum := "    "
		newNum := "    "
		if line.OldNum > 0 {
			oldNum = fmt.Sprintf("%*d", lineNumWidth, line.OldNum)
		}
		if line.NewNum > 0 {
			newNum = fmt.Sprintf("%*d", lineNumWidth, line.NewNum)
		}
		prefix = fmt.Sprintf("%s %s %s", oldNum, newNum, prefix)
	}

	return renderColored(color, "  "+prefix+content)
}

// RenderDiffList renders a list of file diffs
// Matches src/components/StructuredDiffList.tsx
func RenderDiffList(diffs []FileDiff, cfg DiffConfig) string {
	var sections []string

	// Summary header
	added, removed, modified := countChanges(diffs)
	summary := renderDiffSummary(added, removed, modified, len(diffs))
	sections = append(sections, summary)

	// Each file diff
	for _, diff := range diffs {
		sections = append(sections, RenderFileDiff(diff, cfg))
	}

	return strings.Join(sections, "\n\n")
}

// countChanges counts added, removed, and modified files
func countChanges(diffs []FileDiff) (added, removed, modified int) {
	for _, diff := range diffs {
		if diff.IsNew {
			added++
		} else if diff.IsDeleted {
			removed++
		} else {
			modified++
		}
	}
	return
}

// renderDiffSummary renders a summary of changes
func renderDiffSummary(added, removed, modified, totalFiles int) string {
	var parts []string
	if added > 0 {
		parts = append(parts, renderColored(colorAdded, fmt.Sprintf("+%d added", added)))
	}
	if removed > 0 {
		parts = append(parts, renderColored(colorRemoved, fmt.Sprintf("-%d removed", removed)))
	}
	if modified > 0 {
		parts = append(parts, renderColored(colorHeader, fmt.Sprintf("~%d modified", modified)))
	}

	summary := strings.Join(parts, ", ")
	return fmt.Sprintf("%d files changed: %s", totalFiles, summary)
}

// RenderInlineDiff renders an inline diff preview (for file edits)
// Matches FileEditToolDiff.tsx behavior
func RenderInlineDiff(oldContent, newContent string, width int) string {
	// Simple line-by-line diff
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var result []string

	// Find common lines and differences
	// This is a simplified diff - real implementation would use LCS algorithm
	maxLen := max(len(oldLines), len(newLines))

	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine == newLine {
			// Context line
			if oldLine != "" {
				result = append(result, renderColored(colorContext, "  "+oldLine))
			}
		} else {
			// Changed lines
			if oldLine != "" {
				result = append(result, renderColored(colorRemoved, "- "+oldLine))
			}
			if newLine != "" {
				result = append(result, renderColored(colorAdded, "+ "+newLine))
			}
		}
	}

	return strings.Join(result, "\n")
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderMuted(text string) string {
	return renderColored(colorContext, text)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
