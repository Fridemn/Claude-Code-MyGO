package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/config"
	"claude-code-go/internal/ui/components"
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
	ToolName  string    // For tool_use/tool_result entries
	ToolUseID string    // Links tool_result to tool_use
	IsActive  bool      // For collapsed groups - indicates in-progress operations
	Meta      EntryMeta // Collapsed counts, hook info, git ops
}

type SlashSuggestion struct {
	Command     string
	Description string
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
	input := RenderInputPanel(width, state.State, state.CurrentInput, state.Suggestions, state.SelectedSuggestion, state.Busy, state.SpinnerTick, state.ToolName, state.StatusText, state.StartedAt, state.Verb, state.TokenCount, state.TranscriptScroll, state.Teammates, state.TeammateLeaderVerb, state.TeammateLeaderTokens)

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

func RenderInputPanel(width int, state bootstrap.State, currentInput string, suggestions []SlashSuggestion, selectedSuggestion int, busy bool, spinnerTick int, toolName, statusText string, startedAt time.Time, verb string, tokenCount int, transcriptScroll int, teammates []TeammateSpinnerNode, teammateLeaderVerb string, teammateLeaderTokens int) string {
	sprite := buddyDuckSprite()
	spriteWidth := 0
	for _, line := range sprite {
		if w := visibleWidth(line); w > spriteWidth {
			spriteWidth = w
		}
	}
	leftWidth := width - spriteWidth - 3
	if leftWidth < 32 {
		leftWidth = width
		sprite = nil
		spriteWidth = 0
	}

	rule := style(&dark.inputLine, nil, strings.Repeat("─", leftWidth), false)

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
		spinnerText := RenderSpinnerRow(spinnerTick, displayVerb, toolName, elapsedMs, tokenCount, leftWidth, SpinnerModeResponding, false)

		// Build status row
		var statusParts []string
		if strings.TrimSpace(statusText) != "" {
			statusParts = append(statusParts, statusText)
		}
		statusParts = append(statusParts, "Ctrl+C to stop")

		statusRow := strings.Join(statusParts, " · ")
		inputRows = []string{
			spinnerText,
			style(&dark.muted, nil, truncateVisible(statusRow, leftWidth), false),
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
				Width:            leftWidth,
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
	} else {
		inputRows = renderInputRows(leftWidth, currentInput)
	}

	hint := style(&dark.muted, nil, "? for shortcuts", false)
	if strings.TrimSpace(state.LastError) != "" {
		hint = style(&dark.error, nil, truncateVisible("last error: "+state.LastError, leftWidth), false)
	} else if transcriptScroll > 0 {
		hint = style(&dark.info, nil, fmt.Sprintf("Wheel/PgUp/PgDn/↑/↓ scroll · Home/End jump · %d lines above", transcriptScroll), false)
	}

	leftRows := []string{rule}
	leftRows = append(leftRows, inputRows...)
	leftRows = append(leftRows, rule)
	if len(suggestions) == 0 || busy {
		leftRows = append(leftRows, padVisible(hint, leftWidth))
	} else {
		for i, suggestion := range suggestions {
			cmdWidth := visibleWidth(suggestion.Command)
			descWidth := max(10, leftWidth-cmdWidth-2)
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
			leftRows = append(leftRows, padVisible(row, leftWidth))
		}
	}
	leftRows = append(leftRows, strings.Repeat(" ", leftWidth))
	if sprite == nil {
		return strings.Join(leftRows, "\n")
	}

	totalRows := max(len(leftRows), len(sprite))
	rows := make([]string, 0, totalRows)
	leftOffset := max(0, totalRows-len(leftRows))
	spriteOffset := max(0, totalRows-len(sprite))
	for i := 0; i < totalRows; i++ {
		left := strings.Repeat(" ", leftWidth)
		if i >= leftOffset && i-leftOffset < len(leftRows) {
			left = padVisible(leftRows[i-leftOffset], leftWidth)
		}
		right := ""
		if i >= spriteOffset && i-spriteOffset < len(sprite) {
			right = sprite[i-spriteOffset]
		}
		rows = append(rows, left+"   "+right)
	}
	return strings.Join(rows, "\n")
}

func renderInputRows(width int, currentInput string) []string {
	prompt := style(&dark.text, nil, "❯", true) + " "
	continuation := "  "
	cursor := style(&dark.text, nil, "█", false)

	firstWidth := max(1, width-visibleWidth(prompt)-visibleWidth(cursor))
	nextWidth := max(1, width-visibleWidth(continuation)-visibleWidth(cursor))
	inputLines := wrapInputForDisplay(currentInput, firstWidth, nextWidth)
	rows := make([]string, 0, len(inputLines))
	for i, line := range inputLines {
		prefix := continuation
		lineWidth := nextWidth
		if i == 0 {
			prefix = prompt
			lineWidth = firstWidth
		}
		rendered := prefix + line
		if i == len(inputLines)-1 {
			rendered += cursor
		}
		rows = append(rows, padVisible(rendered, visibleWidth(prefix)+lineWidth+visibleWidth(cursor)))
	}
	return rows
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
		return renderMessageBlock(width, "Claude", &dark.claude, entry.Content)
	case "user":
		return renderMessageBlock(width, "⏵", &dark.userLabel, entry.Content)
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
		return renderMessageBlock(width, "⏵", &dark.userLabel, entry.Content)
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

	lines := []string{tipsTitle}
	for _, line := range tipLines {
		lines = append(lines, style(&dark.text, nil, truncateVisible(line, width), false))
	}
	lines = append(lines, style(&dark.claudeDim, nil, strings.Repeat("─", width), false))
	lines = append(lines, recentTitle)
	lines = append(lines, style(&dark.muted, nil, "No recent activity", false))
	lines = append(lines, "")
	lines = append(lines, style(&dark.subtle, nil, truncateVisible("session "+sessionID+" · turn "+strconv.Itoa(turn), width), false))
	return lines
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

func buddyDuckSprite() []string {
	bodyColor := &dark.success
	nameColor := &dark.muted
	return []string{
		style(bodyColor, nil, "    __      ", false),
		style(bodyColor, nil, "  <(o )___  ", false),
		style(bodyColor, nil, "   (  ._>   ", false),
		style(bodyColor, nil, "    `--´    ", false),
		style(nameColor, nil, "   Gravy    ", true),
	}
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
