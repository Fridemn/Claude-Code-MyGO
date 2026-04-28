package ui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"claude-go/internal/bootstrap"
	"claude-go/internal/config"
	"claude-go/internal/ui/components"
)

// RenderContext holds rendering state passed through the render tree
type RenderContext struct {
	Mode                 ViewMode
	LatestBashOutputUUID string
	InProgressToolIDs    map[string]bool // Tool IDs currently executing
	ActiveToolUseID      string
	StatusText           string
	SpinnerTick          int  // For animation (ms elapsed)
	Busy                 bool // Whether assistant is processing
}

// ViewMode controls how messages are displayed and collapsed
type ViewMode int

const (
	ViewModeNormal     ViewMode = 0 // Default: apply all collapse functions
	ViewModeVerbose    ViewMode = 1 // Skip grouping, show more detail
	ViewModeTranscript ViewMode = 2 // Skip all collapse, show full history
)

// GitCommit represents a git commit from a bash operation
type GitCommit struct {
	SHA  string
	Kind string // "normal", "amend", "fixup"
}

// GitPR represents a PR created/updated from a bash operation
type GitPR struct {
	Number int
	URL    string
	Action string // "created", "updated"
}

// GitBranch represents a branch operation
type GitBranch struct {
	Ref    string
	Action string // "created", "deleted", "switched"
}

// GitPush represents a push operation
type GitPush struct {
	Branch string
}

// EntryMeta contains metadata for collapsed groups and special entries
type EntryMeta struct {
	SearchCount       int
	ReadCount         int
	ListCount         int
	MemorySearchCount int
	MemoryReadCount   int
	MemoryWriteCount  int
	MCPCallCount      int
	BashCount         int
	HookCount         int
	HookTotalMs       int
	Commits           []GitCommit
	PRs               []GitPR
	Branches          []GitBranch
	Pushes            []GitPush
	DisplayHint       string            // Most recent operation hint (file path, pattern, etc.)
	GroupMessages     []TranscriptEntry // For verbose expansion of collapsed groups
	MCPServerNames    []string          // MCP servers queried
}

// TranscriptEntry represents a single entry in the chat transcript
type TranscriptEntry struct {
	Kind      string    // user, assistant, assistant_streaming, tool_use, tool_result, collapsed, system, etc.
	Title     string    // Display label
	Content   string    // Primary text content
	UUID      string    // Unique identifier for matching/collapse logic
	Timestamp time.Time // For metadata display in transcript mode
	Subtype   string    // System message subtype: stop_hook_summary, task_status, etc.
	Data      string    // Raw structured payload for system/attachment entries
	ToolName  string    // For tool_use/tool_result entries
	ToolInput string    // Raw JSON tool input for classification/collapse parity
	ToolUseID string    // Links tool_result to tool_use
	IsActive  bool      // For collapsed groups - indicates in-progress operations
	Meta      EntryMeta // Collapsed counts, hook info, git ops
}

type SlashSuggestion struct {
	Command     string
	Description string
	Details     []string
}

type ScreenState struct {
	Version              string
	Config               config.Config
	Width                int
	Height               int
	State                bootstrap.State
	SessionID            string
	Turn                 int
	Entries              []TranscriptEntry
	CurrentInput         string
	Suggestions          []SlashSuggestion
	SelectedSuggestion   int
	Busy                 bool
	SpinnerTick          int
	StreamingText        string
	ToolName             string
	ToolCallID           string
	StatusText           string
	StartedAt            time.Time
	Verb                 string   // Randomly selected verb, constant during request
	TokenCount           int      // Current token count for display
	Mode                 ViewMode // Current view mode (normal, verbose, transcript)
	TranscriptScroll     int      // Scroll offset from transcript bottom, in lines
	LastThinkingBlockID  string   // ID of last thinking block for visibility logic
	LatestBashOutputUUID string   // UUID of most recent bash output for auto-expand
	Teammates            []TeammateSpinnerNode
	TeammateLeaderVerb   string
	TeammateLeaderTokens int
	PermissionMode       string
	FooterRightHint      string
	// InProgressToolIDs tracks tool_use IDs that are currently executing (no result yet)
	InProgressToolIDs map[string]bool
}

type TeammateSpinnerNode struct {
	Name           string
	Activity       string
	ToolUseCount   int
	TokenCount     int
	IsIdle         bool
	IsForegrounded bool
}

func RenderScreen(state ScreenState) string {
	size := terminalSize{Width: state.Width, Height: state.Height}
	if size.Width <= 0 || size.Height <= 0 {
		size = getTerminalSize()
	}
	width := clamp(size.Width, 72, 140)
	_ = clamp(size.Height, 24, 80)

	// Build render context for passing state through render tree
	ctx := RenderContext{
		Mode:                 state.Mode,
		LatestBashOutputUUID: state.LatestBashOutputUUID,
		InProgressToolIDs:    state.InProgressToolIDs,
		ActiveToolUseID:      state.ToolCallID,
		StatusText:           state.StatusText,
		SpinnerTick:          state.SpinnerTick,
		Busy:                 state.Busy,
	}

	header := RenderHeader(width, state.Version, state.Config, state.State, state.SessionID, state.Turn)
	transcript := RenderTranscriptWithContext(width, 0, state.Entries, state.TranscriptScroll, state.LastThinkingBlockID, ctx)
	input := RenderInputPanel(
		width,
		state.State,
		state.CurrentInput,
		len(state.CurrentInput),
		-1,
		-1,
		state.Suggestions,
		state.SelectedSuggestion,
		state.Busy,
		state.SpinnerTick,
		state.ToolName,
		state.StatusText,
		state.StartedAt,
		state.Verb,
		state.TokenCount,
		state.TranscriptScroll,
		state.Teammates,
		state.TeammateLeaderVerb,
		state.TeammateLeaderTokens,
		state.PermissionMode,
		state.FooterRightHint,
	)

	parts := []string{
		header,
	}
	if transcript != "" {
		parts = append(parts, transcript)
	}
	parts = append(parts, input)
	full := strings.Join(parts, "\n")
	if size.Height <= 0 {
		return full
	}

	lines := strings.Split(full, "\n")
	if len(lines) <= size.Height {
		return full
	}

	offset := state.TranscriptScroll
	if offset < 0 {
		offset = 0
	}
	maxOffset := max(0, len(lines)-size.Height)
	if offset > maxOffset {
		offset = maxOffset
	}

	start := max(0, len(lines)-size.Height-offset)
	end := min(len(lines), start+size.Height)
	return strings.Join(lines[start:end], "\n")
}

func RenderHeader(width int, version string, cfg config.Config, state bootstrap.State, sessionID string, turn int) string {
	title := style(&dark.claude, nil, " "+fallbackValue(cfg.AppName, "Claude Code")+" ", true) + style(&dark.muted, nil, "v"+version, false)
	innerWidth := width - 2
	leftWidth := clamp(width/4+6, 26, 42)
	rightWidth := innerWidth - leftWidth - 3
	if rightWidth < 28 {
		rightWidth = 28
		leftWidth = innerWidth - rightWidth - 3
	}

	welcomeText := "Welcome back!"
	modelLine := fallbackValue(cfg.Model, "unset") + " · " + summarizeBilling(cfg.BaseURL)
	pathLine := summarizePath(strings.TrimSpace(state.CWD), leftWidth-2)
	if pathLine == "" {
		pathLine = summarizePath(".", leftWidth-2)
	}

	leftLines := make([]string, 0, 8)
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, centerText(style(&dark.text, nil, welcomeText, true), leftWidth))
	leftLines = append(leftLines, "")
	for _, row := range centeredLogoGlyph(leftWidth) {
		leftLines = append(leftLines, row)
	}
	leftLines = append(leftLines, "")
	leftLines = append(leftLines, centerText(style(&dark.muted, nil, truncateVisible(modelLine, leftWidth), false), leftWidth))
	leftLines = append(leftLines, centerText(style(&dark.muted, nil, truncateVisible(pathLine, leftWidth), false), leftWidth))

	rightLines := renderFeedColumn(rightWidth, cfg, state, sessionID, turn)
	rows := max(len(leftLines), len(rightLines))
	body := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		left := ""
		right := ""
		if i < len(leftLines) {
			left = padVisible(leftLines[i], leftWidth)
		} else {
			left = strings.Repeat(" ", leftWidth)
		}
		if i < len(rightLines) {
			right = padVisible(rightLines[i], rightWidth)
		} else {
			right = strings.Repeat(" ", rightWidth)
		}
		divider := style(&dark.claudeDim, nil, "│", false)
		body = append(body, left+" "+divider+" "+right)
	}

	return framedBoxWithTitle(width, title, body, dark.claude, dark.claude, dark.text, nil)
}

func RenderInputPanel(
	width int,
	state bootstrap.State,
	currentInput string,
	cursorPos int,
	selectionStart int,
	selectionEnd int,
	suggestions []SlashSuggestion,
	selectedSuggestion int,
	busy bool,
	spinnerTick int,
	toolName, statusText string,
	startedAt time.Time,
	verb string,
	tokenCount int,
	transcriptScroll int,
	teammates []TeammateSpinnerNode,
	teammateLeaderVerb string,
	teammateLeaderTokens int,
	permissionMode string,
	footerRightHint string,
) string {
	rule := style(&dark.inputLine, nil, strings.Repeat("─", width), false)

	// Show spinner if busy
	var inputRows []string
	if busy {
		// Calculate elapsed time in ms
		var elapsedMs int
		if !startedAt.IsZero() {
			elapsedMs = int(time.Since(startedAt).Milliseconds())
		}

		// Use verb if provided, otherwise use a default
		displayVerb := verb
		if displayVerb == "" {
			displayVerb = "Working"
		}

		// Render spinner row with frame animation (120ms per frame)
		// spinnerTick is in ms, frame = tick / 120
		spinnerText := RenderSpinnerRow(spinnerTick, displayVerb, toolName, elapsedMs, tokenCount, width, SpinnerModeResponding, false)

		// Build status row
		var statusParts []string
		if strings.TrimSpace(statusText) != "" {
			statusParts = append(statusParts, statusText)
		}
		// Show /btw hint for side questions during busy state
		statusParts = append(statusParts, "/btw for side question · Ctrl+C to stop")

		statusRow := strings.Join(statusParts, " · ")
		inputRows = []string{
			spinnerText,
			style(&dark.muted, nil, truncateVisible(statusRow, width), false),
		}
		if len(teammates) > 0 {
			nodes := make([]components.TeammateTask, 0, len(teammates))
			for _, teammate := range teammates {
				nodes = append(nodes, components.TeammateTask{
					Name:           teammate.Name,
					Activity:       teammate.Activity,
					ToolUseCount:   teammate.ToolUseCount,
					TokenCount:     teammate.TokenCount,
					IsIdle:         teammate.IsIdle,
					IsForegrounded: teammate.IsForegrounded,
				})
			}
			tree := components.RenderTeammateSpinnerTree(components.TeammateSpinnerTreeConfig{
				Width:            width,
				SelectionMode:    false,
				SelectedIndex:    -1,
				LeaderVerb:       strings.TrimSpace(teammateLeaderVerb),
				LeaderTokenCount: teammateLeaderTokens,
				LeaderIdleText:   "idle",
				Teammates:        nodes,
			})
			for _, line := range strings.Split(tree, "\n") {
				inputRows = append(inputRows, line)
			}
		}
		// Always show input field at bottom even when busy (for immediate commands like /btw)
		// Show a dimmed input prompt to indicate user can type side commands
		inputRows = append(inputRows, "")
		inputRows = append(inputRows, style(&dark.muted, nil, "❯ "+currentInput, false))
	} else {
		inputRows = renderInputRows(width, currentInput, cursorPos, selectionStart, selectionEnd)
	}

	hint, rightHint := renderFooterHints(width, state, busy, transcriptScroll, permissionMode, footerRightHint)

	rows := []string{rule}
	rows = append(rows, inputRows...)
	rows = append(rows, rule)
	if len(suggestions) == 0 || busy {
		rows = append(rows, renderFooterLine(width, hint, rightHint))
	} else {
		for i, suggestion := range suggestions {
			cmdWidth := visibleWidth(suggestion.Command)
			descWidth := max(10, width-cmdWidth-2)
			prefix := "  "
			cmdColor := &dark.permission
			descColor := &dark.muted
			if i == selectedSuggestion {
				prefix = style(&dark.permission, nil, "› ", true)
				cmdColor = &dark.text
				descColor = &dark.text
			}
			row := prefix +
				style(cmdColor, nil, suggestion.Command, true) +
				"  " +
				style(descColor, nil, truncateVisible(suggestion.Description, descWidth), false)
			rows = append(rows, padVisible(row, width))
			for _, detail := range suggestion.Details {
				detail = strings.TrimSpace(detail)
				if detail == "" {
					continue
				}
				detailPrefix := "    "
				if i == selectedSuggestion {
					detailPrefix = "  " + style(&dark.permission, nil, "│ ", false)
				}
				rows = append(rows, padVisible(detailPrefix+style(&dark.muted, nil, truncateVisible(detail, max(10, width-visibleWidth(detailPrefix))), false), width))
			}
		}
	}
	rows = append(rows, strings.Repeat(" ", width))
	return strings.Join(rows, "\n")
}

func renderInputRows(width int, currentInput string, cursorPos int, selectionStart int, selectionEnd int) []string {
	prompt := style(&dark.text, nil, "❯", true) + " "
	continuation := "  "
	cursor := style(&dark.text, nil, "█", false)

	// ANSI codes for selection highlighting
	const (
		reverseVideoOn  = "\x1b[7m"
		reverseVideoOff = "\x1b[27m"
	)

	firstWidth := max(1, width-visibleWidth(prompt)-visibleWidth(cursor))
	nextWidth := max(1, width-visibleWidth(continuation)-visibleWidth(cursor))

	// Track clamped selection boundaries for local use
	hasSelection := selectionStart >= 0 && selectionEnd > selectionStart && selectionStart < len(currentInput)
	selStart := selectionStart
	selEnd := selectionEnd
	if selEnd > len(currentInput) {
		selEnd = len(currentInput)
	}

	// Single-pass wrapping with both selection highlighting and cursor tracking
	type lineInfo struct {
		text      string // rendered text (may contain ANSI codes)
		cursorIdx int    // byte index in rendered text where cursor should be inserted, -1 if not this line
	}

	var lines []lineInfo
	var current strings.Builder
	currentWidth := 0
	lineWidth := firstWidth
	byteIdx := 0
	cursorLine := -1
	cursorCol := 0
	inSelection := false

	// Helper to flush current line
	flushLine := func() {
		lines = append(lines, lineInfo{text: current.String(), cursorIdx: -1})
		current.Reset()
		currentWidth = 0
		lineWidth = nextWidth
	}

	for _, r := range inputRunes(currentInput) {
		runeLen := utf8.RuneLen(r)

		// Track cursor position (before the character)
		if byteIdx == cursorPos {
			cursorLine = len(lines)
			cursorCol = current.Len()
		}

		// Check selection boundaries
		if byteIdx == selStart && hasSelection && !inSelection {
			current.WriteString(reverseVideoOn)
			inSelection = true
		}
		if byteIdx == selEnd && hasSelection && inSelection {
			current.WriteString(reverseVideoOff)
			inSelection = false
		}

		if r == '\n' {
			if inSelection {
				current.WriteString(reverseVideoOff)
			}
			if cursorLine == len(lines) {
				lines = append(lines, lineInfo{text: current.String(), cursorIdx: cursorCol})
				cursorLine = len(lines) - 1
			} else {
				flushLine()
			}
			current.Reset()
			currentWidth = 0
			lineWidth = nextWidth
			if inSelection {
				current.WriteString(reverseVideoOn)
			}
			byteIdx += runeLen
			continue
		}
		rw := runeCellWidth(r)
		if rw == 0 {
			current.WriteRune(r)
			byteIdx += runeLen
			continue
		}
		if currentWidth > 0 && currentWidth+rw > lineWidth {
			if inSelection {
				current.WriteString(reverseVideoOff)
			}
			if cursorLine == len(lines) {
				lines = append(lines, lineInfo{text: current.String(), cursorIdx: cursorCol})
				cursorLine = len(lines) - 1
			} else {
				flushLine()
			}
			current.Reset()
			currentWidth = 0
			lineWidth = nextWidth
			if inSelection {
				current.WriteString(reverseVideoOn)
			}
		}
		current.WriteRune(r)
		currentWidth += rw
		byteIdx += runeLen
	}

	// Close selection at end if still open
	if inSelection {
		current.WriteString(reverseVideoOff)
	}

	// Handle cursor at end of input
	if cursorPos >= len(currentInput) {
		cursorLine = len(lines)
		cursorCol = current.Len()
	}

	// Flush remaining
	if current.Len() > 0 || len(lines) == 0 {
		if cursorLine == len(lines) {
			lines = append(lines, lineInfo{text: current.String(), cursorIdx: cursorCol})
			cursorLine = len(lines) - 1
		} else {
			lines = append(lines, lineInfo{text: current.String(), cursorIdx: -1})
		}
	}

	// Build final rows with prefix and cursor
	// When there's a selection, skip cursor block to avoid ANSI reset canceling reverse video
	showCursor := !hasSelection
	cursorWidth := visibleWidth(cursor)
	rows := make([]string, 0, len(lines))
	for i, line := range lines {
		prefix := continuation
		lWidth := nextWidth
		if i == 0 {
			prefix = prompt
			lWidth = firstWidth
		}
		// Calculate padding width
		padWidth := visibleWidth(prefix) + lWidth
		if showCursor {
			padWidth += cursorWidth
		}
		if showCursor && i == cursorLine && line.cursorIdx >= 0 {
			ci := line.cursorIdx
			text := line.text
			if ci <= len(text) {
				rendered := prefix + text[:ci] + cursor + text[ci:]
				rows = append(rows, padVisible(rendered, padWidth))
			} else {
				rendered := prefix + text + cursor
				rows = append(rows, padVisible(rendered, padWidth))
			}
		} else {
			rendered := prefix + line.text
			rows = append(rows, padVisible(rendered, padWidth))
		}
	}
	return rows
}

// inputRunes returns the runes of a string for iteration
func inputRunes(s string) []rune {
	return []rune(s)
}

type footerSegment struct {
	Text  string
	Color *rgb
	Bold  bool
}

func renderFooterHints(width int, state bootstrap.State, busy bool, transcriptScroll int, permissionMode, footerRightHint string) (string, string) {
	if strings.TrimSpace(state.LastError) != "" {
		return style(&dark.error, nil, truncateVisible("last error: "+state.LastError, width), false), ""
	}
	if transcriptScroll > 0 {
		return style(&dark.info, nil, fmt.Sprintf("Wheel/PgUp/PgDn/↑/↓ scroll · Home/End jump · %d lines above", transcriptScroll), false), ""
	}

	var leftParts []footerSegment
	if modePart := modeFooterSegment(permissionMode); modePart != nil {
		leftParts = append(leftParts, *modePart)
		leftParts = append(leftParts, footerSegment{
			Text:  "(shift+tab to cycle)",
			Color: &dark.muted,
			Bold:  false,
		})
	}
	if busy {
		leftParts = append(leftParts, footerSegment{
			Text:  "esc to interrupt",
			Color: &dark.muted,
			Bold:  false,
		})
	}
	if len(leftParts) == 0 {
		leftParts = append(leftParts, footerSegment{
			Text:  "? for shortcuts",
			Color: &dark.muted,
			Bold:  false,
		})
	}

	leftHint := styleFooterSegments(leftParts, width)
	rightHint := ""
	if strings.TrimSpace(footerRightHint) != "" {
		rightHint = style(&dark.muted, nil, truncateVisible(footerRightHint, width), false)
	}
	return leftHint, rightHint
}

func modeFooterSegment(permissionMode string) *footerSegment {
	mode := strings.TrimSpace(permissionMode)
	switch mode {
	case "acceptEdits":
		return &footerSegment{Text: "⏵⏵ accept edits on", Color: &dark.permission, Bold: true}
	case "bypassPermissions":
		return &footerSegment{Text: "⏵⏵ bypass permissions on", Color: &dark.error, Bold: true}
	case "dontAsk":
		return &footerSegment{Text: "⏵⏵ don't ask on", Color: &dark.error, Bold: true}
	case "plan":
		return &footerSegment{Text: "⏸ plan mode on", Color: &dark.warning, Bold: true}
	case "auto":
		return &footerSegment{Text: "⏵⏵ auto mode on", Color: &dark.warning, Bold: true}
	default:
		return nil
	}
}

func styleFooterSegments(parts []footerSegment, width int) string {
	if len(parts) == 0 {
		return ""
	}
	sep := style(&dark.muted, nil, " · ", false)
	plainSep := " · "

	var plainParts []string
	for _, p := range parts {
		plainParts = append(plainParts, p.Text)
	}
	plainJoined := strings.Join(plainParts, plainSep)
	if visibleWidth(plainJoined) > width {
		return style(&dark.muted, nil, truncateVisible(plainJoined, width), false)
	}

	styled := make([]string, 0, len(parts))
	for _, p := range parts {
		styled = append(styled, style(p.Color, nil, p.Text, p.Bold))
	}
	return strings.Join(styled, sep)
}

func renderFooterLine(width int, left, right string) string {
	if strings.TrimSpace(right) == "" {
		return padVisible(left, width)
	}

	leftWidth := visibleWidth(left)
	rightWidth := visibleWidth(right)
	required := leftWidth + 1 + rightWidth
	if required > width {
		leftBudget := max(1, width-rightWidth-1)
		leftPlain := ansiRE.ReplaceAllString(left, "")
		left = style(&dark.muted, nil, truncateVisible(leftPlain, leftBudget), false)
		leftWidth = visibleWidth(left)
		required = leftWidth + 1 + rightWidth
	}
	if required > width {
		rightBudget := max(1, width-leftWidth-1)
		rightPlain := ansiRE.ReplaceAllString(right, "")
		right = style(&dark.muted, nil, truncateVisible(rightPlain, rightBudget), false)
		rightWidth = visibleWidth(right)
	}
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// wrapInputWithCursor wraps input for display and tracks cursor position across lines
// cursorPos is a byte position in the input string
func wrapInputWithCursor(input string, cursorPos int, firstWidth, continuationWidth int) ([]string, int, int) {
	if firstWidth <= 0 {
		firstWidth = 1
	}
	if continuationWidth <= 0 {
		continuationWidth = 1
	}
	if input == "" {
		return []string{""}, 0, 0
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0
	lineWidth := firstWidth
	cursorLine := 0
	cursorCol := 0
	byteIdx := 0 // Track byte position

	for _, r := range input {
		runeLen := utf8.RuneLen(r)

		if byteIdx == cursorPos {
			// Record cursor position before this character
			cursorLine = len(lines)
			cursorCol = current.Len()
		}

		if r == '\n' {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
			lineWidth = continuationWidth
			byteIdx += runeLen
			continue
		}
		rw := runeCellWidth(r)
		if rw == 0 {
			current.WriteRune(r)
			byteIdx += runeLen
			continue
		}
		if currentWidth > 0 && currentWidth+rw > lineWidth {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
			lineWidth = continuationWidth
		}
		current.WriteRune(r)
		currentWidth += rw
		byteIdx += runeLen
	}

	// Handle cursor at end of input
	if cursorPos >= len(input) {
		cursorLine = len(lines)
		cursorCol = current.Len()
	}

	lines = append(lines, current.String())
	return lines, cursorLine, cursorCol
}

func wrapInputForDisplay(input string, firstWidth, continuationWidth int) []string {
	if firstWidth <= 0 {
		firstWidth = 1
	}
	if continuationWidth <= 0 {
		continuationWidth = 1
	}
	if input == "" {
		return []string{""}
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0
	lineWidth := firstWidth
	for _, r := range input {
		if r == '\n' {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
			lineWidth = continuationWidth
			continue
		}
		rw := runeCellWidth(r)
		if rw == 0 {
			current.WriteRune(r)
			continue
		}
		if currentWidth > 0 && currentWidth+rw > lineWidth {
			lines = append(lines, current.String())
			current.Reset()
			currentWidth = 0
			lineWidth = continuationWidth
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	lines = append(lines, current.String())
	return lines
}

func formatElapsed(elapsed time.Duration) string {
	if elapsed < time.Second {
		return "0s"
	}
	seconds := int(elapsed.Seconds())
	if seconds < 60 {
		return strconv.Itoa(seconds) + "s"
	}
	return strconv.Itoa(seconds/60) + "m" + strconv.Itoa(seconds%60) + "s"
}

func RenderTranscript(width int, maxHeight int, entries []TranscriptEntry, mode ViewMode, transcriptScroll int, lastThinkingBlockID string, latestBashOutputUUID string) string {
	ctx := RenderContext{
		Mode:                 mode,
		LatestBashOutputUUID: latestBashOutputUUID,
	}
	return RenderTranscriptWithContext(width, maxHeight, entries, transcriptScroll, lastThinkingBlockID, ctx)
}

// RenderTranscriptWithContext renders transcript entries with full render context
func RenderTranscriptWithContext(width int, maxHeight int, entries []TranscriptEntry, transcriptScroll int, lastThinkingBlockID string, ctx RenderContext) string {
	if len(entries) == 0 {
		return ""
	}

	var blocks []string
	for _, entry := range entries {
		// Handle thinking visibility based on mode
		if entry.Kind == "thinking" || entry.Kind == "redacted_thinking" {
			if ctx.Mode == ViewModeNormal {
				continue // Hide thinking in normal mode
			}
			// In transcript mode, only show last thinking block
			if ctx.Mode == ViewModeTranscript && entry.UUID != lastThinkingBlockID {
				continue
			}
		}

		// Handle collapsed groups based on mode
		if entry.Kind == "collapsed" {
			if ctx.Mode == ViewModeVerbose || ctx.Mode == ViewModeTranscript {
				// In verbose/transcript mode, expand the group
				if len(entry.Meta.GroupMessages) > 0 {
					for _, groupEntry := range entry.Meta.GroupMessages {
						blocks = append(blocks, renderEntryWithContext(width, groupEntry, ctx))
					}
					continue
				}
			}
		}

		blocks = append(blocks, renderEntryWithContext(width, entry, ctx))
	}
	lines := flattenRenderedBlocks(blocks)
	if len(lines) == 0 {
		return ""
	}
	if maxHeight <= 0 {
		return strings.Join(lines, "\n")
	}

	offset := transcriptScroll
	if offset < 0 {
		offset = 0
	}
	maxOffset := max(0, len(lines)-maxHeight)
	if offset > maxOffset {
		offset = maxOffset
	}

	start := max(0, len(lines)-maxHeight-offset)
	end := min(len(lines), start+maxHeight)
	window := append([]string(nil), lines[start:end]...)

	return strings.Join(window, "\n")
}

func renderEntry(width int, entry TranscriptEntry, mode ViewMode, latestBashOutputUUID string) string {
	ctx := RenderContext{
		Mode:                 mode,
		LatestBashOutputUUID: latestBashOutputUUID,
	}
	return renderEntryWithContext(width, entry, ctx)
}

func renderEntryWithContext(width int, entry TranscriptEntry, ctx RenderContext) string {
	switch entry.Kind {
	case "assistant", "assistant_streaming":
		// Strip <think> tags from content (some models output thinking as raw XML)
		textContent, thinkingContent := stripThinkingTags(entry.Content)
		var parts []string

		// Render thinking as collapsed block if present
		if thinkingContent != "" {
			thinkEntry := TranscriptEntry{
				Kind:    "thinking",
				Content: thinkingContent,
				UUID:    entry.UUID + "-think",
			}
			parts = append(parts, renderThinkingBlock(width, thinkEntry, ctx.Mode))
		}

		// Render text content if present
		if textContent != "" {
			parts = append(parts, renderAssistantTextBlock(width, textContent))
		}

		if len(parts) == 0 {
			return "" // No content to render
		}
		return strings.Join(parts, "\n")
	case "user":
		return renderUserTextBlock(width, entry.Content)
	case "compact_summary":
		// Compact summary renders with markdown enabled
		return renderCompactSummaryBlock(width, entry.Content)
	case "system":
		// Compact boundary: "✻ Conversation compacted (Ctrl+O for history)"
		if strings.Contains(entry.Content, "[compact boundary") {
			return components.RenderDimText("✻ Conversation compacted (Ctrl+O for history)")
		}
		return ""
	case "progress":
		if ctx.Mode != ViewModeTranscript {
			return ""
		}
		content := strings.TrimSpace(entry.Content)
		if content == "" {
			return ""
		}
		label := "progress"
		if strings.TrimSpace(entry.Subtype) != "" {
			label = strings.TrimSpace(entry.Subtype)
		}
		return style(&dark.subtle, nil, "["+label+"] "+content, false)
	case "tool_use":
		return renderToolUseBlockWithContext(width, entry, ctx)
	case "grouped_tool_use":
		return renderGroupedToolUseBlockWithContext(width, entry, ctx)
	case "tool_result":
		return renderToolResultBlock(width, entry, ctx.Mode, ctx.LatestBashOutputUUID)
	case "collapsed":
		return renderCollapsedBlockWithContext(width, entry, ctx)
	case "thinking", "redacted_thinking":
		return renderThinkingBlock(width, entry, ctx.Mode)
	case "panel":
		ruleWidth := width - 8
		if ruleWidth < 12 {
			ruleWidth = 12
		}
		lines := []string{
			style(&dark.panelAccent, &dark.panelBackground, " PANEL ", true) + " " + style(&dark.text, &dark.panelBackground, entry.Title, true),
			style(&dark.muted, &dark.panelBackground, "Structured command output", false),
			style(&dark.panelBorder, &dark.panelBackground, strings.Repeat("─", ruleWidth), false),
			style(&dark.subtle, &dark.panelBackground, "source=local-jsx  mode=panel  layout=sectioned", false),
			"",
		}
		lines = append(lines, renderPanelContent(entry.Content, width-6)...)
		return boxWithWidth(width, entry.Title, lines, dark.panelBorder, dark.panelAccent, dark.text, &dark.panelBackground)
	case "command":
		return renderMessageBlock(width, entry.Title, &dark.permission, entry.Content)
	case "error":
		return renderMessageBlock(width, entry.Title, &dark.error, entry.Content)
	case "notice":
		if entry.Title != "" {
			return renderMessageBlock(width, entry.Title, &dark.info, entry.Content)
		}
		return style(&dark.subtle, nil, entry.Content, false)
	default:
		if entry.Title != "" {
			return renderMessageBlock(width, entry.Title, &dark.warning, entry.Content)
		}
		return renderUserTextBlock(width, entry.Content)
	}
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func summarizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "not configured"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	return parsed.Host + path
}

func summarizePath(path string, width int) string {
	path = strings.TrimSpace(path)
	if visibleWidth(path) <= width || width < 16 {
		return path
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return ".../" + strings.Join(parts[len(parts)-3:], "/")
}

func summarizeBilling(raw string) string {
	base := summarizeBaseURL(raw)
	if base == "not configured" {
		return "API Usage Billing"
	}
	return base
}

func renderMessageBlock(width int, label string, color *rgb, content string) string {
	// Match TS: messages are rendered WITHOUT boxes, just with simple label and content
	// See AssistantTextMessage.tsx and UserTextMessage.tsx
	var lines []string

	// Add label line (matching TS pattern)
	switch label {
	case "Claude":
		lines = append(lines, style(color, nil, " "+label+" ", true))
	case "⏵":
		lines = append(lines, style(color, nil, " "+label+" ", true))
	default:
		lines = append(lines, style(color, nil, " "+label+" ", true))
	}

	// Add content lines
	for _, line := range renderMarkdown(content, width-2) {
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func renderAssistantTextBlock(width int, content string) string {
	return renderInlineMessageBlock(width, components.BlackCircle+" ", &dark.text, content, true, nil, nil)
}

func renderUserTextBlock(width int, content string) string {
	return renderInlineMessageBlock(width, "⏵ ", &dark.userLabel, content, false, &dark.text, &dark.userMessageBackground)
}

func renderCompactSummaryBlock(width int, content string) string {
	return renderInlineMessageBlock(width, components.BlackCircle+" ", &dark.text, content, true, nil, nil)
}

func renderSystemTextBlock(width int, entry TranscriptEntry) string {
	prefixColor := &dark.subtle
	contentColor := &dark.subtle
	subtype := strings.ToLower(strings.TrimSpace(entry.Subtype))
	switch {
	case strings.Contains(subtype, "error"):
		prefixColor = &dark.error
		contentColor = &dark.error
	case strings.Contains(subtype, "warning"):
		prefixColor = &dark.warning
		contentColor = &dark.warning
	}
	return renderInlineMessageBlock(width, components.BlackCircle+" ", prefixColor, entry.Content, false, contentColor, nil)
}

func renderInlineMessageBlock(width int, prefix string, prefixColor *rgb, content string, markdown bool, contentColor *rgb, backgroundColor *rgb) string {
	prefixWidth := visibleWidth(prefix)
	if prefixWidth <= 0 {
		prefixWidth = 1
	}

	contentWidth := max(1, width-prefixWidth)
	var contentLines []string
	if markdown {
		contentLines = renderMarkdown(content, contentWidth)
	} else {
		rawLines := splitLinesRaw(strings.TrimSpace(content))
		if len(rawLines) == 0 {
			rawLines = []string{""}
		}
		for _, raw := range rawLines {
			wrapped := wrapText(raw, contentWidth)
			if len(wrapped) == 0 {
				wrapped = []string{""}
			}
			contentLines = append(contentLines, wrapped...)
		}
	}
	if len(contentLines) == 0 {
		contentLines = []string{""}
	}

	prefixStyled := style(prefixColor, backgroundColor, prefix, false)
	indent := strings.Repeat(" ", prefixWidth)
	lines := make([]string, 0, len(contentLines))

	firstLine := contentLines[0]
	if contentColor != nil || backgroundColor != nil {
		firstLine = style(contentColor, backgroundColor, firstLine, false)
	}
	lines = append(lines, prefixStyled+firstLine)

	indentSegment := indent
	if backgroundColor != nil {
		indentSegment = style(nil, backgroundColor, indent, false)
	}

	for _, line := range contentLines[1:] {
		if contentColor != nil || backgroundColor != nil {
			line = style(contentColor, backgroundColor, line, false)
		}
		lines = append(lines, indentSegment+line)
	}

	return strings.Join(lines, "\n")
}

func countRenderLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func flattenRenderedBlocks(blocks []string) []string {
	lines := make([]string, 0, len(blocks)*2)
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}
		lines = append(lines, strings.Split(block, "\n")...)
	}
	return lines
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func joinStatus(width int, left, center, right string) string {
	plainLeft := visibleWidth(left)
	plainCenter := visibleWidth(center)
	plainRight := visibleWidth(right)
	if width < plainLeft+plainCenter+plainRight+4 {
		return left + "  " + center + "  " + right
	}
	spaces := width - plainLeft - plainCenter - plainRight
	leftPad := spaces / 2
	rightPad := spaces - leftPad
	return left + strings.Repeat(" ", leftPad) + center + strings.Repeat(" ", rightPad) + right
}

func condensedLogoGlyph() []string {
	return []string{
		style(&dark.claude, nil, "  ██████  ", false),
		style(&dark.claude, nil, " ██▄██▄██ ", false),
		style(&dark.claude, nil, "  ██████  ", false),
		style(&dark.muted, nil, "   ▀  ▀   ", false),
	}
}

func centeredLogoGlyph(width int) []string {
	// Claude Code Go mascot - Gopher-inspired, 4 rows with half-height blocks
	// ▀ = top half, ▄ = bottom half, █ = full block
	c := &GoBlue           // Go blue body
	bg := &dark.background // dark for eyes
	w := &dark.text        // white for teeth/hands/feet
	rows := []string{
		"  " + style(c, nil, "▄█", false) + "  " + style(c, nil, "█▄", false) + "  ",                                                                   // ears (symmetric)
		style(c, nil, "██", false) + style(bg, nil, "▀", false) + style(c, nil, "██", false) + style(bg, nil, "▀", false) + style(c, nil, "██", false), // eyes (wider body)
		style(w, nil, "▀", false) + style(c, nil, "███", false) + style(w, nil, "▄▄", false) + style(c, nil, "███", false) + style(w, nil, "▀", false), // hands (▀) + teeth
		"  " + style(w, nil, "▀", false) + "    " + style(w, nil, "▀", false) + "  ",                                                                   // feet (white)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, centerText(row, width))
	}
	return out
}

func centerText(v string, width int) string {
	plain := visibleWidth(v)
	if plain >= width {
		return v
	}
	left := (width - plain) / 2
	right := width - plain - left
	return strings.Repeat(" ", left) + v + strings.Repeat(" ", right)
}

func renderFeedColumn(width int, cfg config.Config, state bootstrap.State, sessionID string, turn int) []string {
	tipsTitle := style(&dark.claude, nil, "Tips for getting started", true)
	recentTitle := style(&dark.claudeDim, nil, "Recent activity", true)

	cwd := strings.TrimSpace(state.CWD)
	tipLines := []string{
		"Run /init to create a CLAUDE.md file with instructions for Claude.",
	}
	if cwd != "" {
		tipLines = append(tipLines, "Note: Claude works best when launched inside your project directory.")
	}

	// Load recent activity from sessions in current project directory
	recentSessions := loadRecentActivity(cwd, sessionID)

	lines := []string{tipsTitle}
	for _, line := range tipLines {
		lines = append(lines, style(&dark.text, nil, truncateVisible(line, width), false))
	}
	lines = append(lines, style(&dark.claudeDim, nil, strings.Repeat("─", width), false))
	lines = append(lines, recentTitle)

	// Show recent sessions or "No recent activity"
	if len(recentSessions) > 0 {
		for _, s := range recentSessions {
			// Format: session_id_short · date · title preview
			idShort := s.SessionID
			if len(idShort) > 8 {
				idShort = idShort[:8]
			}
			title := s.Title
			if len(title) > 30 {
				title = title[:30] + "..."
			}
			date := s.Date
			lineText := fmt.Sprintf("%s · %s · %s", idShort, date, title)
			lines = append(lines, style(&dark.muted, nil, truncateVisible(lineText, width), false))
		}
	} else {
		lines = append(lines, style(&dark.muted, nil, "No recent activity", false))
	}

	lines = append(lines, "")
	lines = append(lines, style(&dark.subtle, nil, truncateVisible("session "+sessionID+" · turn "+strconv.Itoa(turn), width), false))
	return lines
}

// RecentSessionInfo holds info for recent activity display
type RecentSessionInfo struct {
	SessionID string
	Title     string
	Date      string
}

// loadRecentActivity loads recent sessions from the current project directory
// Uses Go CLI independent storage: ~/.claude-go/projects/<project>/
func loadRecentActivity(cwd, currentSessionID string) []RecentSessionInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	// Sanitize cwd to get project directory name
	sanitized := sanitizePathForUI(cwd)
	// Use Go CLI specific projects directory
	projectsDir := filepath.Join(home, ".claude-go", "projects", sanitized)

	// Check if project directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil
	}

	// List session files
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}

	var sessions []RecentSessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")

		// Skip current session
		if sessionID == currentSessionID {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Extract first prompt from session file
		title := extractFirstPromptFromSession(filepath.Join(projectsDir, entry.Name()))
		if title == "" {
			title = "(no prompt)"
		}
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		sessions = append(sessions, RecentSessionInfo{
			SessionID: sessionID,
			Title:     title,
			Date:      info.ModTime().Format("Jan 02"),
		})
	}

	// Sort by modification time (most recent first) and take top 3
	sort.Slice(sessions, func(i, j int) bool {
		// Parse dates and compare (simplified: just compare SessionID timestamps embedded)
		return sessions[i].SessionID > sessions[j].SessionID // UUIDs have timestamp component
	})

	if len(sessions) > 3 {
		sessions = sessions[:3]
	}

	return sessions
}

// sanitizePathForUI converts a path to a safe directory name for UI display
func sanitizePathForUI(path string) string {
	var result []rune
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}
	s := string(result)
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// extractFirstPromptFromSession reads first user message from session file
func extractFirstPromptFromSession(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// JSONL format: {"type": "message", "message": {"role": "user", "content": "..."}}
		if entry["type"] == "message" {
			if msg, ok := entry["message"].(map[string]interface{}); ok {
				if role, _ := msg["role"].(string); role == "user" {
					if content, ok := msg["content"].(string); ok {
						if len(content) > 100 {
							content = content[:100]
						}
						return content
					}
				}
			}
		}
	}
	return ""
}

func renderBodyFiller(width, height int) string {
	if height <= 0 {
		return ""
	}
	var lines []string
	for i := 0; i < height; i++ {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func mergeColumns(left, right []string, leftWidth, rightWidth int) []string {
	rows := max(len(left), len(right))
	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		l := ""
		r := ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, padVisible(l, leftWidth)+"  "+padVisible(r, rightWidth))
	}
	return out
}

// stripThinkingTags removes <think>...</think> tags from text content.
// Returns the text content without thinking tags and the extracted thinking content.
// This handles models that output thinking as raw XML tags rather than structured blocks.
func stripThinkingTags(content string) (textContent string, thinkingContent string) {
	// Match both <think>...</think> and <thinking>...</thinking> patterns
	// Some models use different tag names
	patterns := []struct {
		start string
		end   string
	}{
		{"<think>", "</think>"},
		{"<thinking>", "</thinking>"},
	}

	textContent = content
	var thinkParts []string

	for _, pat := range patterns {
		for {
			startIdx := strings.Index(textContent, pat.start)
			if startIdx == -1 {
				break
			}
			endIdx := strings.Index(textContent[startIdx:], pat.end)
			if endIdx == -1 {
				// Incomplete tag - keep searching from after start tag
				// This handles streaming where end tag hasn't arrived yet
				thinkParts = append(thinkParts, textContent[startIdx+len(pat.start):])
				textContent = textContent[:startIdx]
				break
			}
			// Extract thinking content
			thinkEnd := startIdx + endIdx
			thinkText := textContent[startIdx+len(pat.start) : thinkEnd]
			thinkParts = append(thinkParts, thinkText)
			// Remove the entire tag block from text content
			textContent = textContent[:startIdx] + textContent[thinkEnd+len(pat.end):]
		}
	}

	if len(thinkParts) > 0 {
		thinkingContent = strings.TrimSpace(strings.Join(thinkParts, "\n"))
	}
	textContent = strings.TrimSpace(textContent)
	return
}
