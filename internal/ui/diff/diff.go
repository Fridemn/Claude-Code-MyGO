package diff

import (
	"fmt"
	"strings"

	"claude-go/internal/ui/components"
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

// Theme colors for diff (foreground for text, backgrounds for git-style highlighting)
var (
	colorAddedFG   = components.RGB{R: 78, G: 186, B: 101}  // success/green text
	colorRemovedFG = components.RGB{R: 255, G: 107, B: 128} // error/red text
	colorAddedBG   = components.RGB{R: 34, G: 92, B: 43}    // green background (git style)
	colorRemovedBG = components.RGB{R: 122, G: 51, B: 54}   // red background (git style)
	colorContext   = components.RGB{R: 134, G: 145, B: 160} // muted
	colorHeader    = components.RGB{R: 177, G: 185, B: 249} // permission/purple
	colorHunk      = components.RGB{R: 115, G: 198, B: 255} // info/blue
	colorText      = components.RGB{R: 255, G: 255, B: 255}
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
		return renderColored(colorAddedFG, header)
	}
	if diff.IsDeleted {
		header = fmt.Sprintf("- %s (deleted)", diff.OldPath)
		return renderColored(colorRemovedFG, header)
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

// renderDiffLine renders a single diff line with git-style background highlighting
func renderDiffLine(line DiffLine, cfg DiffConfig, lineNumWidth int) string {
	var prefix string
	var content string

	switch line.Type {
	case DiffLineAdded:
		prefix = "+"
		content = line.Content
		// Git-style: green background with white text
		return renderWithBackground(colorAddedBG, "  "+prefix+content)
	case DiffLineRemoved:
		prefix = "-"
		content = line.Content
		// Git-style: red background with white text
		return renderWithBackground(colorRemovedBG, "  "+prefix+content)
	case DiffLineContext:
		prefix = " "
		content = line.Content
		return renderColored(colorContext, "  "+prefix+content)
	case DiffLineModified:
		prefix = "~"
		content = line.Content
		return renderColored(colorHeader, "  "+prefix+content)
	default:
		prefix = " "
		content = line.Content
		return renderColored(colorText, "  "+prefix+content)
	}
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
		parts = append(parts, renderColored(colorAddedFG, fmt.Sprintf("+%d added", added)))
	}
	if removed > 0 {
		parts = append(parts, renderColored(colorRemovedFG, fmt.Sprintf("-%d removed", removed)))
	}
	if modified > 0 {
		parts = append(parts, renderColored(colorHeader, fmt.Sprintf("~%d modified", modified)))
	}

	summary := strings.Join(parts, ", ")
	return fmt.Sprintf("%d files changed: %s", totalFiles, summary)
}

// RenderInlineDiff renders an inline diff preview (for file edits)
// Matches FileEditToolDiff.tsx behavior with git-style background colors
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
			// Changed lines - use background colors for git style
			if oldLine != "" {
				result = append(result, renderWithBackground(colorRemovedBG, "- "+oldLine))
			}
			if newLine != "" {
				result = append(result, renderWithBackground(colorAddedBG, "+ "+newLine))
			}
		}
	}

	return strings.Join(result, "\n")
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

// renderWithBackground renders text with background color (git diff style)
// Uses white text on colored background
func renderWithBackground(bgColor components.RGB, text string) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;%d;%d;%dm%s\033[0m",
		bgColor.R, bgColor.G, bgColor.B,
		255, 255, 255, // White text
		text)
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

// GenerateEditDiff generates a structured diff for a file edit operation
// This is used for permission request previews to show what will change
func GenerateEditDiff(filePath, oldContent, newContent string, contextLines int) FileDiff {
	if contextLines <= 0 {
		contextLines = 4
	}

	diff := FileDiff{
		OldPath: filePath,
		NewPath: filePath,
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Find the changes using a simple LCS approach
	hunk := findEditHunk(oldLines, newLines, contextLines)
	if hunk != nil {
		diff.Hunks = []DiffHunk{*hunk}
	}

	return diff
}

// findEditHunk finds the changed region and builds a diff hunk with context
func findEditHunk(oldLines, newLines []string, contextLines int) *DiffHunk {
	// Find where changes start
	changeStart := 0
	maxLen := max(len(oldLines), len(newLines))
	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			changeStart = i
			break
		}
		// If we reach the end and all lines matched, no changes
		if i >= len(oldLines)-1 && i >= len(newLines)-1 {
			return nil
		}
	}

	// Find where changes end
	changeEndOld := len(oldLines)
	changeEndNew := len(newLines)
	for i := changeStart; i < maxLen; i++ {
		// Find first matching line after changes
		if i < len(oldLines) && i < len(newLines) && oldLines[i] == newLines[i] {
			changeEndOld = i
			changeEndNew = i
			break
		}
	}

	// Add context before
	contextStart := changeStart - contextLines
	if contextStart < 0 {
		contextStart = 0
	}

	// Add context after
	contextEndOld := changeEndOld + contextLines
	if contextEndOld > len(oldLines) {
		contextEndOld = len(oldLines)
	}
	contextEndNew := changeEndNew + contextLines
	if contextEndNew > len(newLines) {
		contextEndNew = len(newLines)
	}

	// Build the hunk
	hunk := &DiffHunk{
		OldStart: contextStart + 1, // 1-indexed
		OldCount: changeEndOld - contextStart,
		NewStart: contextStart + 1,
		NewCount: changeEndNew - contextStart,
		Lines:    make([]DiffLine, 0),
	}

	// Add context before changes
	for i := contextStart; i < changeStart; i++ {
		line := ""
		if i < len(oldLines) {
			line = oldLines[i]
		}
		hunk.Lines = append(hunk.Lines, DiffLine{
			Type:    DiffLineContext,
			Content: line,
			OldNum:  i + 1,
			NewNum:  i + 1,
		})
	}

	// Add removed lines
	for i := changeStart; i < changeEndOld && i < len(oldLines); i++ {
		hunk.Lines = append(hunk.Lines, DiffLine{
			Type:    DiffLineRemoved,
			Content: oldLines[i],
			OldNum:  i + 1,
			NewNum:  0,
		})
	}

	// Add added lines
	for i := changeStart; i < changeEndNew && i < len(newLines); i++ {
		// Check if this line was actually added (not just shifted)
		isNew := i >= len(oldLines) || (i < changeEndOld && newLines[i] != oldLines[i])
		if isNew || i >= changeEndOld {
			hunk.Lines = append(hunk.Lines, DiffLine{
				Type:    DiffLineAdded,
				Content: newLines[i],
				OldNum:  0,
				NewNum:  i + 1,
			})
		}
	}

	// Add context after changes
	for i := changeEndNew; i < contextEndNew && i < len(newLines); i++ {
		hunk.Lines = append(hunk.Lines, DiffLine{
			Type:    DiffLineContext,
			Content: newLines[i],
			OldNum:  i + 1 - (changeEndNew - changeEndOld), // Adjust for deletions
			NewNum:  i + 1,
		})
	}

	return hunk
}

// RenderEditDiffPreview renders a compact diff preview for file edit permission
// This matches the TS FileEditToolDiff behavior for permission requests
func RenderEditDiffPreview(filePath, oldString, newString string, width int) string {
	if width <= 0 {
		width = 80
	}

	// Truncate very large diffs
	maxDiffLines := 30
	if len(strings.Split(oldString, "\n")) > maxDiffLines || len(strings.Split(newString, "\n")) > maxDiffLines {
		// Show only a preview of the change
		oldPreview := truncateForDiffPreview(oldString, 10)
		newPreview := truncateForDiffPreview(newString, 10)
		return renderCompactDiff(filePath, oldPreview, newPreview, width, true)
	}

	return renderCompactDiff(filePath, oldString, newString, width, false)
}

// truncateForDiffPreview truncates content for diff preview display
func truncateForDiffPreview(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n") + "\n… (truncated)"
}

// renderCompactDiff renders a compact diff suitable for permission dialogs
func renderCompactDiff(filePath, oldString, newString string, width int, truncated bool) string {
	diff := GenerateEditDiff(filePath, oldString, newString, 4)

	var lines []string

	// File header (dimmed)
	lines = append(lines, renderMuted("  File: "+filePath))
	if truncated {
		lines = append(lines, renderMuted("  (showing partial content)"))
	}
	lines = append(lines, "")

	// Render hunks
	cfg := DiffConfig{
		Width:        width,
		ShowLineNums: true,
		Context:      4,
		Unified:      true,
	}

	for _, hunk := range diff.Hunks {
		// Hunk header
		hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
		lines = append(lines, renderColored(colorHunk, "  "+hunkHeader))

		// Diff lines (limit display)
		displayLines := hunk.Lines
		if len(displayLines) > 25 {
			displayLines = displayLines[:25]
			lines = append(lines, renderMuted("  … (more changes below)"))
		}

		for _, line := range displayLines {
			rendered := renderDiffLine(line, cfg, 4)
			lines = append(lines, rendered)
		}
	}

	if len(diff.Hunks) == 0 {
		// No changes detected - show inline comparison
		lines = append(lines, renderInlineComparison(oldString, newString, width))
	}

	return strings.Join(lines, "\n")
}

// renderInlineComparison renders simple inline comparison when hunks can't be computed
func renderInlineComparison(oldString, newString string, width int) string {
	var lines []string

	if oldString != "" {
		lines = append(lines, renderWithBackground(colorRemovedBG, "- "+truncateString(oldString, width-4)))
	}
	if newString != "" {
		lines = append(lines, renderWithBackground(colorAddedBG, "+ "+truncateString(newString, width-4)))
	}

	return strings.Join(lines, "\n")
}

// truncateString truncates a string to maxLen
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "…"
}
