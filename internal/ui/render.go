package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func boxWithWidth(width int, title string, body []string, border rgb, titleFg rgb, bodyFg rgb, bodyBg *rgb) string {
	if width < 32 {
		width = 32
	}
	top := style(&border, nil, "┌"+line(width-2, "─")+"┐", false)
	bottom := style(&border, nil, "└"+line(width-2, "─")+"┘", false)
	out := []string{top}

	if strings.TrimSpace(title) != "" {
		left := style(&border, nil, "│ ", false)
		right := style(&border, nil, "│", false)
		content := style(&titleFg, bodyBg, padVisible(title, width-3), true)
		out = append(out, left+content+right)
		out = append(out, style(&border, nil, "├"+line(width-2, "─")+"┤", false))
	}

	if len(body) == 0 {
		body = []string{""}
	}
	for _, row := range body {
		left := style(&border, nil, "│ ", false)
		right := style(&border, nil, "│", false)
		content := style(&bodyFg, bodyBg, padVisible(row, width-3), false)
		out = append(out, left+content+right)
	}
	out = append(out, bottom)
	return strings.Join(out, "\n")
}

func framedBoxWithTitle(width int, title string, body []string, border rgb, titleFg rgb, bodyFg rgb, bodyBg *rgb) string {
	if width < 32 {
		width = 32
	}
	inner := width - 2
	titlePlain := " " + title + " "
	titleWidth := visibleWidth(titlePlain)
	if titleWidth > inner {
		titlePlain = truncateVisible(titlePlain, inner)
		titleWidth = visibleWidth(titlePlain)
	}

	leftRule := "─"
	if title != "" {
		leftRule = "╭"
	}
	leftWidth := 0
	if title != "" {
		leftWidth = 2
	}
	rightWidth := inner - leftWidth - titleWidth
	if rightWidth < 0 {
		rightWidth = 0
	}

	var out []string
	if title == "" {
		out = append(out, style(&border, nil, "╭"+strings.Repeat("─", inner)+"╮", false))
	} else {
		top := style(&border, nil, leftRule+strings.Repeat("─", leftWidth), false) +
			style(&titleFg, bodyBg, titlePlain, true) +
			style(&border, nil, strings.Repeat("─", rightWidth)+"╮", false)
		out = append(out, top)
	}

	if len(body) == 0 {
		body = []string{""}
	}
	for _, row := range body {
		left := style(&border, nil, "│", false)
		right := style(&border, nil, "│", false)
		content := style(&bodyFg, bodyBg, padVisible(row, inner), false)
		out = append(out, left+content+right)
	}
	out = append(out, style(&border, nil, "╰"+strings.Repeat("─", inner)+"╯", false))
	return strings.Join(out, "\n")
}

func truncateVisible(v string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(v) <= width {
		return v
	}
	plain := ansiRE.ReplaceAllString(v, "")
	if width == 1 {
		return truncateRunesToWidth(plain, 1)
	}
	if width <= 3 {
		return truncateRunesToWidth(plain, width)
	}
	return truncateRunesToWidth(plain, width-1) + "…"
}

func renderMarkdown(content string, width int) []string {
	if shouldUseGlamour(content) {
		if lines := renderMarkdownWithGlamour(content, width); len(lines) > 0 {
			return lines
		}
	}
	lines := splitLinesRaw(strings.TrimSpace(content))
	if len(lines) == 0 {
		return []string{""}
	}

	var out []string
	inCode := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			label := "code"
			if lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```")); lang != "" {
				label = "code: " + lang
			}
			out = append(out, style(&dark.permission, &dark.commandBackground, " "+label+" ", true))
			continue
		}
		switch {
		case inCode:
			for _, wrapped := range wrapText(line, width-2) {
				out = append(out, style(&dark.text, &dark.commandBackground, " "+wrapped, false))
			}
		case isRule(trimmed):
			out = append(out, style(&dark.subtle, nil, strings.Repeat("─", max(12, width-2)), false))
		case strings.HasPrefix(trimmed, "# "):
			out = append(out, style(&dark.claude, nil, strings.TrimPrefix(trimmed, "# "), true))
		case strings.HasPrefix(trimmed, "## "):
			out = append(out, style(&dark.permission, nil, strings.TrimPrefix(trimmed, "## "), true))
		case isNumberedList(trimmed):
			prefix, item := splitNumberedList(trimmed)
			wrapped := wrapText(item, width-6)
			for i, w := range wrapped {
				lead := "   "
				if i == 0 {
					lead = prefix + " "
				}
				out = append(out, lead+w)
			}
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			item := strings.TrimSpace(trimmed[2:])
			wrapped := wrapText(item, width-4)
			for i, w := range wrapped {
				prefix := "  "
				if i == 0 {
					prefix = "• "
				}
				out = append(out, prefix+w)
			}
		case strings.HasPrefix(trimmed, ">"):
			quote := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			for _, wrapped := range wrapText(quote, width-4) {
				out = append(out, style(&dark.subtle, nil, "│ "+wrapped, false))
			}
		case trimmed == "":
			out = append(out, "")
		default:
			out = append(out, wrapText(line, width)...)
		}
	}
	return out
}

func renderPanelContent(content string, width int) []string {
	lines := splitLinesRaw(strings.TrimSpace(content))
	if len(lines) == 0 {
		return []string{""}
	}

	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, "")
		case isPanelSection(trimmed):
			accent := panelSectionColor(trimmed)
			out = append(out,
				style(accent, &dark.panelBackground, " "+trimmed+" ", true),
				style(&dark.panelBorder, &dark.panelBackground, strings.Repeat("─", max(12, width-2)), false),
			)
		case strings.HasPrefix(trimmed, "- /"):
			for _, wrapped := range wrapText(trimmed, width-2) {
				out = append(out, style(&dark.permission, &dark.panelBackground, " → "+wrapped, true))
			}
		case strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, ":"):
			label, detail, _ := strings.Cut(strings.TrimPrefix(trimmed, "- "), ":")
			row := style(panelListColor(label), &dark.panelBackground, " "+label+":", true) + style(panelListColor(detail), &dark.panelBackground, detail, false)
			for _, wrapped := range wrapTextPreservingANSI(row, width) {
				out = append(out, wrapped)
			}
		case isCalloutLine(trimmed):
			prefix := " ! "
			if strings.Contains(strings.ToLower(trimmed), "note") || strings.Contains(strings.ToLower(trimmed), "tip") {
				prefix = " i "
			}
			for _, wrapped := range wrapText(trimmed, width-6) {
				out = append(out, style(panelListColor(trimmed), &dark.panelBackground, prefix+wrapped, true))
			}
		case strings.HasPrefix(line, "  "):
			for _, wrapped := range wrapText(strings.TrimSpace(line), width-4) {
				out = append(out, style(&dark.muted, &dark.panelBackground, "  "+wrapped, false))
			}
		case strings.HasPrefix(trimmed, "- "):
			for _, wrapped := range wrapText(trimmed, width-2) {
				out = append(out, style(panelListColor(trimmed), &dark.panelBackground, " "+wrapped, false))
			}
		case isKeyValueLine(trimmed):
			key, value, _ := strings.Cut(trimmed, "=")
			row := style(&dark.subtle, &dark.panelBackground, key+"=", false) + style(panelValueColor(key, value), &dark.panelBackground, value, false)
			for _, wrapped := range wrapTextPreservingANSI(row, width) {
				out = append(out, wrapped)
			}
		default:
			out = append(out, renderMarkdown(line, width)...)
		}
	}
	return out
}

func isPanelSection(line string) bool {
	return strings.HasSuffix(line, ":") && !strings.Contains(line, "=") && !strings.HasPrefix(line, "- ")
}

func panelSectionColor(line string) *rgb {
	lower := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(line), ":"))
	switch lower {
	case "actions":
		return &dark.permission
	case "overview", "summary", "results":
		return &dark.info
	case "warnings", "errors":
		return &dark.warning
	default:
		return &dark.panelAccent
	}
}

func isKeyValueLine(line string) bool {
	if strings.HasPrefix(line, "- ") {
		return false
	}
	key, value, ok := strings.Cut(line, "=")
	return ok && strings.TrimSpace(key) != "" && strings.TrimSpace(value) != ""
}

func isCalloutLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "warning") || strings.HasPrefix(lower, "error") || strings.HasPrefix(lower, "note") || strings.HasPrefix(lower, "tip")
}

func wrapTextPreservingANSI(text string, width int) []string {
	if width <= 0 || visibleWidth(text) <= width {
		return []string{text}
	}
	plain := ansiRE.ReplaceAllString(text, "")
	return wrapText(plain, width)
}

func panelValueColor(key, value string) *rgb {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	switch {
	case lowerValue == "true" || lowerValue == "enabled" || lowerValue == "configured" || lowerValue == "ready":
		return &dark.success
	case lowerValue == "connected" || lowerValue == "active":
		return &dark.info
	case lowerValue == "false" || lowerValue == "disabled":
		return &dark.warning
	case strings.Contains(lowerKey, "error") || strings.Contains(lowerKey, "failed") || lowerValue == "failed" || lowerValue == "error":
		return &dark.error
	case strings.Contains(lowerKey, "status") || strings.Contains(lowerKey, "mode"):
		return &dark.panelAccent
	default:
		return &dark.text
	}
}

func panelListColor(line string) *rgb {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.Contains(lower, "/"):
		return &dark.permission
	case strings.Contains(lower, " enabled=true"), strings.Contains(lower, " result=ok"), strings.Contains(lower, ": ok"), strings.Contains(lower, " configured"), strings.Contains(lower, " ready"):
		return &dark.success
	case strings.Contains(lower, " connected"), strings.Contains(lower, " active"):
		return &dark.info
	case strings.Contains(lower, " enabled=false"), strings.Contains(lower, " disabled"):
		return &dark.warning
	case strings.Contains(lower, " failed"), strings.Contains(lower, " error"), strings.Contains(lower, "missing "), strings.Contains(lower, "duplicate "), strings.Contains(lower, "negative "), strings.Contains(lower, "issues"):
		return &dark.error
	default:
		return &dark.text
	}
}

func isRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	return strings.Trim(line, "-*") == ""
}

func isNumberedList(line string) bool {
	for i, r := range line {
		if r >= '0' && r <= '9' {
			continue
		}
		return i > 0 && r == '.' && i+1 < len(line) && line[i+1] == ' '
	}
	return false
}

func splitNumberedList(line string) (string, string) {
	for i, r := range line {
		if r == '.' && i+1 < len(line) && line[i+1] == ' ' {
			return line[:i+1], strings.TrimSpace(line[i+1:])
		}
	}
	return "1.", line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	text = strings.ReplaceAll(text, "\t", "  ")
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return []string{""}
	}

	var out []string
	current := ""
	for _, token := range tokens {
		tokenParts := wrapToken(token, width)
		for _, part := range tokenParts {
			if current == "" {
				current = part
				continue
			}
			if visibleWidth(current)+1+visibleWidth(part) <= width {
				current += " " + part
				continue
			}
			out = append(out, current)
			current = part
		}
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func splitLinesRaw(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.Split(v, "\n")
}

func countLines(v string) int {
	if v == "" {
		return 0
	}
	return len(strings.Split(v, "\n"))
}

func visibleWidth(v string) int {
	clean := ansiRE.ReplaceAllString(v, "")
	width := 0
	for _, r := range clean {
		width += runeCellWidth(r)
	}
	return width
}

func padVisible(v string, width int) string {
	length := visibleWidth(v)
	if length >= width {
		return v
	}
	return v + strings.Repeat(" ", width-length)
}

func truncateRunesToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var out strings.Builder
	used := 0
	for _, r := range text {
		rw := runeCellWidth(r)
		if rw == 0 {
			out.WriteRune(r)
			continue
		}
		if used+rw > width {
			break
		}
		out.WriteRune(r)
		used += rw
	}
	return out.String()
}

func wrapToken(token string, width int) []string {
	if visibleWidth(token) <= width {
		return []string{token}
	}

	var out []string
	var current strings.Builder
	currentWidth := 0

	for _, r := range token {
		rw := runeCellWidth(r)
		if rw == 0 {
			current.WriteRune(r)
			continue
		}
		if currentWidth > 0 && currentWidth+rw > width {
			out = append(out, current.String())
			current.Reset()
			currentWidth = 0
		}
		current.WriteRune(r)
		currentWidth += rw
	}

	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

func runeCellWidth(r rune) int {
	switch {
	case r == 0:
		return 0
	case r < 32 || (r >= 0x7f && r < 0xa0):
		return 0
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r):
		return 0
	case isWideRune(r):
		return 2
	default:
		return 1
	}
}

func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f:
		return true
	case r == 0x2329 || r == 0x232a:
		return true
	case r >= 0x2e80 && r <= 0xa4cf:
		return true
	case r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff:
		return true
	case r >= 0xfe10 && r <= 0xfe19:
		return true
	case r >= 0xfe30 && r <= 0xfe6f:
		return true
	case r >= 0xff00 && r <= 0xff60:
		return true
	case r >= 0xffe0 && r <= 0xffe6:
		return true
	case r >= 0x1f300 && r <= 0x1faff:
		return true
	case r >= 0x20000 && r <= 0x3fffd:
		return true
	default:
		return false
	}
}

// renderToolUseBlock renders a tool_use entry with tool name and parameters
func renderToolUseBlock(width int, entry TranscriptEntry, mode ViewMode) string {
	ctx := RenderContext{Mode: mode}
	return renderToolUseBlockWithContext(width, entry, ctx)
}

// renderToolUseBlockWithContext renders a tool_use entry with full context
// Matches TS ToolUseLoader.tsx and AssistantToolUseMessage.tsx behavior
func renderToolUseBlockWithContext(width int, entry TranscriptEntry, ctx RenderContext) string {
	if entry.ToolName == "" {
		return ""
	}

	// Determine if this tool is currently in progress
	isInProgress := entry.IsActive
	if ctx.InProgressToolIDs != nil && entry.ToolUseID != "" {
		isInProgress = ctx.InProgressToolIDs[entry.ToolUseID]
	}

	// Determine indicator style based on state
	// TS ToolUseLoader: unresolved+animating=blinking, resolved+ok=green, resolved+error=red
	var indicator string
	if isInProgress {
		// Blinking animation - alternate between ● and space every 600ms
		// SpinnerTick is in milliseconds
		frame := (ctx.SpinnerTick / 600) % 2
		if frame == 0 {
			indicator = style(&dark.claude, nil, "● ", false)
		} else {
			indicator = "  " // space to maintain alignment
		}
	} else if entry.Subtype == "error" {
		indicator = style(&dark.error, nil, "● ", false)
	} else {
		indicator = style(&dark.success, nil, "● ", false)
	}

	// Tool name - bold when active
	var label string
	if isInProgress {
		label = indicator + style(&dark.text, nil, entry.ToolName, true)
	} else {
		label = indicator + style(&dark.text, nil, entry.ToolName, false)
	}

	// Show parameters in all modes (TS shows condensed details in normal mode too)
	if entry.Content != "" {
		paramWidth := width - visibleWidth(entry.ToolName) - 8
		if paramWidth < 16 {
			paramWidth = 16
		}
		label += style(&dark.muted, nil, " ("+truncateVisible(entry.Content, paramWidth)+")", false)
	}

	if isInProgress {
		// TS parity: queued and permission wait statuses show dedicated sub-lines.
		status := strings.TrimSpace(ctx.StatusText)
		if entry.ToolUseID != "" && ctx.ActiveToolUseID != "" && entry.ToolUseID == ctx.ActiveToolUseID {
			lower := strings.ToLower(status)
			if strings.Contains(lower, "classifier checking") {
				return label + "\n" + style(&dark.subtle, nil, "  "+status, false)
			}
			if strings.Contains(strings.ToLower(status), "permission") {
				return label + "\n" + style(&dark.subtle, nil, "  Waiting for permission…", false)
			}
			if strings.Contains(lower, "queued") || strings.Contains(lower, "waiting") {
				return label + "\n" + style(&dark.subtle, nil, "  Waiting…", false)
			}
			if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "denied") {
				return label + "\n" + style(&dark.error, nil, "  "+truncateVisible(status, width-4), false)
			}
			// For Bash-style statuses, show detailed status text with elapsed.
			if (entry.ToolName == "Bash" || entry.ToolName == "PowerShell") && status != "" && !strings.Contains(lower, "running tool:") && !strings.Contains(lower, "receiving response") {
				progressText := "  " + truncateVisible(status, width-4) + " (" + formatElapsedDurationLabel(ctx.SpinnerTick) + ")"
				return label + "\n" + style(&dark.subtle, nil, progressText, false)
			}
		}

		// TS parity: active Bash tool shows a progress row ("Running…")
		if entry.ToolName == "Bash" || entry.ToolName == "PowerShell" {
			progress := style(&dark.subtle, nil, "  Running… "+formatElapsedDurationLabel(ctx.SpinnerTick), false)
			return label + "\n" + progress
		}
		// Tool-specific progress phrasing for better TS parity.
		return label + "\n" + style(&dark.subtle, nil, "  "+activeToolProgressLabel(entry.ToolName), false)
	}

	return label
}

// renderGroupedToolUseBlock renders a grouped_tool_use entry
// Matches TS logic from src/components/messages/GroupedToolUseMessage.tsx
func renderGroupedToolUseBlock(width int, entry TranscriptEntry, mode ViewMode) string {
	ctx := RenderContext{Mode: mode}
	return renderGroupedToolUseBlockWithContext(width, entry, ctx)
}

func renderGroupedToolUseBlockWithContext(width int, entry TranscriptEntry, ctx RenderContext) string {
	if entry.ToolName == "" {
		return ""
	}

	count := len(entry.Meta.GroupMessages)
	if count == 0 {
		count = 1 // Fallback if count not available
	}

	// Check if any tool in the group is in progress
	isInProgress := entry.IsActive
	if ctx.InProgressToolIDs != nil {
		for _, msg := range entry.Meta.GroupMessages {
			if ctx.InProgressToolIDs[msg.ToolUseID] {
				isInProgress = true
				break
			}
		}
	}

	var indicator string
	if isInProgress {
		frame := (ctx.SpinnerTick / 600) % 2
		if frame == 0 {
			indicator = style(&dark.claude, nil, "● ", false)
		} else {
			indicator = "  "
		}
	} else {
		indicator = style(&dark.success, nil, "● ", false)
	}

	// Agent grouped display follows TS phrasing ("Running N agents…", "N agents finished").
	if entry.ToolName == "Agent" {
		var summary string
		if isInProgress {
			summary = fmt.Sprintf("Running %d agents…", count)
		} else {
			summary = fmt.Sprintf("%d agents finished", count)
		}
		lines := []string{
			indicator + style(&dark.text, nil, summary, isInProgress),
		}
		// Add compact per-agent detail lines from grouped tool-use summaries.
		// Keep lines deduplicated and capped to avoid noisy grouped output.
		seen := map[string]bool{}
		detailCount := 0
		const maxDetailLines = 4
		for _, msg := range entry.Meta.GroupMessages {
			if msg.Kind != "tool_use" || strings.TrimSpace(msg.Content) == "" {
				continue
			}
			detail, ok := normalizeAgentGroupedDetail(msg.Content)
			if !ok {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(detail))
			if seen[key] {
				continue
			}
			seen[key] = true
			detailCount++
			if detailCount > maxDetailLines {
				continue
			}
			detail = truncateVisible(detail, width-8)
			lines = append(lines, style(&dark.subtle, nil, "  ↳ "+detail, false))
		}
		if detailCount > maxDetailLines {
			rest := detailCount - maxDetailLines
			lines = append(lines, style(&dark.muted, nil, fmt.Sprintf("  ↳ +%d more", rest), false))
		}
		return strings.Join(lines, "\n")
	}

	// Default grouped format with tool-aware operation label + latest hint.
	label := indicator + style(&dark.text, nil, entry.ToolName, isInProgress)
	opText := groupedOperationText(entry.ToolName, count)
	label += style(&dark.muted, nil, " ("+opText+")", false)
	if hint, ok := groupedLatestHint(entry.ToolName, entry.Meta.GroupMessages); ok {
		return label + "\n" + style(&dark.subtle, nil, "  ⎿ "+truncateVisible(hint, width-8), false)
	}
	return label
}

// renderToolResultBlock renders a tool_result entry
func renderToolResultBlock(width int, entry TranscriptEntry, mode ViewMode, latestBashOutputUUID string) string {
	// Determine if this should show full output
	shouldShowFull := mode == ViewModeVerbose || mode == ViewModeTranscript || entry.UUID == latestBashOutputUUID

	parsed := parseToolResultEnvelope(entry.Content)

	// For Edit tool, always show diff preview (not just in verbose mode)
	// Matches TS behavior where file edit shows diff in all modes
	if entry.ToolName == "Edit" {
		return renderEditToolResultWithDiff(width, parsed)
	}

	content := summarizeToolResult(entry.ToolName, parsed, width, shouldShowFull)

	// Style based on success/error
	var color *rgb
	isError := strings.Contains(entry.Subtype, "error") || parsed.Status == "error" || strings.TrimSpace(parsed.Error) != ""
	if isError {
		color = &dark.error
	} else {
		color = &dark.muted
	}

	return style(color, nil, "  ⎿ "+content, false)
}

// renderEditToolResultWithDiff renders Edit tool result with diff preview
// Shows diff in all modes, matching TS FileEditToolUpdatedMessage behavior
func renderEditToolResultWithDiff(width int, parsed parsedToolResult) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return style(&dark.muted, nil, "  ⎿ "+truncateToolContent(parsed.Body, width-4), false)
	}

	filePath := getString(obj, "filePath")
	displayPath := displayFilePathForSummary(filePath)

	// Build result with diff preview
	var lines []string

	// Count additions and removals from structuredPatch
	patches := getArray(obj, "structuredPatch")
	numAdditions := 0
	numRemovals := 0
	for _, patch := range patches {
		patchMap, ok := patch.(map[string]any)
		if !ok {
			continue
		}
		patchLines := getArray(patchMap, "lines")
		for _, l := range patchLines {
			lineStr, ok := l.(string)
			if !ok {
				continue
			}
			if strings.HasPrefix(lineStr, "+") {
				numAdditions++
			} else if strings.HasPrefix(lineStr, "-") {
				numRemovals++
			}
		}
	}

	// Header line with summary (matches TS behavior)
	header := "Updated " + displayPath
	if getBool(obj, "replaceAll") {
		header = "Applied edits to " + displayPath
	}
	// Add line counts like TS: "Added X lines / Removed X lines"
	if numAdditions > 0 || numRemovals > 0 {
		var countParts []string
		if numAdditions > 0 {
			countParts = append(countParts, fmt.Sprintf("Added %d %s", numAdditions, pluralize(numAdditions, "line", "lines")))
		}
		if numRemovals > 0 {
			countParts = append(countParts, fmt.Sprintf("Removed %d %s", numRemovals, pluralize(numRemovals, "line", "lines")))
		}
		header += " · " + strings.Join(countParts, " / ")
	}

	// Check for error status
	if parsed.Status == "error" || strings.TrimSpace(parsed.Error) != "" {
		lines = append(lines, style(&dark.error, nil, "  ⎿ "+truncateToolContent(parsed.Error, width-4), false))
		return strings.Join(lines, "\n")
	}

	lines = append(lines, style(&dark.success, nil, "  ⎿ "+header, false))

	// Render diff preview from structuredPatch with git-style background colors
	if len(patches) > 0 {
		// Limit hunks to avoid overwhelming output
		maxHunks := 2
		displayPatches := patches
		if len(patches) > maxHunks {
			displayPatches = patches[:maxHunks]
		}

		for _, patch := range displayPatches {
			patchMap, ok := patch.(map[string]any)
			if !ok {
				continue
			}
			diffLines := renderStructuredPatchDiff(patchMap, width-8)
			lines = append(lines, diffLines...)
		}

		if len(patches) > maxHunks {
			lines = append(lines, style(&dark.muted, nil, "    … (more hunks)", false))
		}
	}

	return strings.Join(lines, "\n")
}

// renderStructuredPatchDiff renders lines from a structured patch hunk with git-style backgrounds
func renderStructuredPatchDiff(patch map[string]any, width int) []string {
	var lines []string

	patchLines, ok := patch["lines"].([]any)
	if !ok {
		return lines
	}

	// Limit display to avoid overwhelming output
	maxLines := 15
	displayLines := patchLines
	if len(patchLines) > maxLines {
		displayLines = patchLines[:maxLines]
	}

	for _, l := range displayLines {
		lineStr, ok := l.(string)
		if !ok {
			continue
		}
		// Truncate long lines
		if len(lineStr) > width {
			lineStr = lineStr[:width-3] + "…"
		}

		// Apply git-style background colors based on prefix
		if strings.HasPrefix(lineStr, "+") {
			// Green background for additions
			lines = append(lines, renderWithDiffBackground(40, 80, 50, "    "+lineStr))
		} else if strings.HasPrefix(lineStr, "-") {
			// Red background for deletions
			lines = append(lines, renderWithDiffBackground(80, 40, 50, "    "+lineStr))
		} else if strings.HasPrefix(lineStr, " ") {
			lines = append(lines, style(&dark.muted, nil, "    "+lineStr, false))
		} else {
			lines = append(lines, style(&dark.muted, nil, "    "+lineStr, false))
		}
	}

	if len(patchLines) > maxLines {
		lines = append(lines, style(&dark.muted, nil, "    … (more changes)", false))
	}

	return lines
}

// renderWithDiffBackground renders text with a colored background (git diff style)
func renderWithDiffBackground(r, g, b int, text string) string {
	return fmt.Sprintf("\033[48;2;%d;%d;%dm\033[38;2;%d;%d;%dm%s\033[0m",
		r, g, b, 255, 255, 255, text)
}

func getArray(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

type parsedToolResult struct {
	Tool   string
	Status string
	Error  string
	Body   string
	Raw    string
}

func parseToolResultEnvelope(raw string) parsedToolResult {
	out := parsedToolResult{Raw: raw, Body: strings.TrimSpace(raw)}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	lines := strings.Split(raw, "\n")
	bodyStart := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			bodyStart = i + 1
			break
		}
		if !strings.Contains(trimmed, "=") {
			break
		}
		key, val, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "tool":
			out.Tool = strings.TrimSpace(val)
		case "status":
			out.Status = strings.TrimSpace(val)
		case "error":
			out.Error = strings.TrimSpace(val)
		}
	}
	if bodyStart < len(lines) {
		out.Body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	}
	return out
}

func summarizeToolResult(toolName string, parsed parsedToolResult, width int, full bool) string {
	name := toolName
	if name == "" {
		name = parsed.Tool
	}
	if full {
		return truncateToolContent(parsed.Body, width-4)
	}
	if msg, ok := summarizeToolResultError(name, parsed); ok {
		return msg
	}
	switch name {
	case "Read":
		return summarizeReadResult(parsed, width)
	case "Write":
		return summarizeWriteResult(parsed, width)
	case "Edit":
		return summarizeEditResult(parsed, width)
	case "Grep":
		return summarizeGrepResult(parsed, width)
	case "Glob":
		return summarizeGlobResult(parsed, width)
	case "Bash":
		return summarizeBashResult(parsed, width)
	case "Agent":
		return summarizeAgentResult(parsed, width)
	default:
		return truncateToolContent(parsed.Body, width-4)
	}
}

func summarizeToolResultError(toolName string, parsed parsedToolResult) (string, bool) {
	if parsed.Status != "error" && strings.TrimSpace(parsed.Error) == "" {
		return "", false
	}
	errText := strings.TrimSpace(parsed.Error)
	if errText == "" {
		errText = extractTagged(parsed.Body, "tool_use_error")
	}
	lower := strings.ToLower(errText)
	switch toolName {
	case "Read":
		if strings.Contains(lower, "not found") {
			return "File not found", true
		}
		return "Error reading file", true
	case "Write":
		return "Error writing file", true
	case "Edit":
		if strings.Contains(lower, "file has not been read yet") {
			return "File must be read first", true
		}
		if strings.Contains(lower, "not found") {
			return "File not found", true
		}
		return "Error editing file", true
	case "Grep", "Glob":
		if strings.Contains(lower, "not found") {
			return "File not found", true
		}
		return "Error searching files", true
	case "Bash", "PowerShell":
		if strings.TrimSpace(errText) != "" {
			return truncateToolContent(strings.Split(strings.TrimSpace(errText), "\n")[0], 96), true
		}
		return "Command failed", true
	default:
		return "", false
	}
}

func extractTagged(raw, tag string) string {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(tag) == "" {
		return ""
	}
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(raw, open)
	if start < 0 {
		return ""
	}
	start += len(open)
	end := strings.Index(raw[start:], close)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(raw[start : start+end])
}

func normalizeAgentGroupedDetail(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	left, right, ok := strings.Cut(s, ":")
	if ok {
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		switch {
		case left == "" && right == "":
			return "", false
		case left == "":
			return right, true
		case right == "":
			return left, true
		default:
			return left + " · " + right, true
		}
	}
	return s, true
}

func groupedOperationText(toolName string, count int) string {
	if count < 1 {
		count = 1
	}
	switch toolName {
	case "Read":
		return fmt.Sprintf("%d %s", count, pluralize(count, "read", "reads"))
	case "Grep", "Glob":
		return fmt.Sprintf("%d %s", count, pluralize(count, "search", "searches"))
	case "Write", "Edit":
		return fmt.Sprintf("%d %s", count, pluralize(count, "update", "updates"))
	case "Bash", "PowerShell":
		return fmt.Sprintf("%d %s", count, pluralize(count, "command", "commands"))
	default:
		return fmt.Sprintf("%d operations", count)
	}
}

func groupedLatestHint(toolName string, group []TranscriptEntry) (string, bool) {
	for i := len(group) - 1; i >= 0; i-- {
		msg := group[i]
		if msg.Kind != "tool_use" {
			continue
		}
		hint := strings.TrimSpace(msg.Content)
		if hint != "" {
			return normalizeGroupedHint(toolName, hint), true
		}
	}
	return "", false
}

func normalizeGroupedHint(toolName, raw string) string {
	hint := strings.TrimSpace(raw)
	if hint == "" {
		return ""
	}
	switch toolName {
	case "Read":
		// Keep path prominent for read hints ("path · lines x-y" -> "path").
		if left, _, ok := strings.Cut(hint, " · "); ok && strings.TrimSpace(left) != "" {
			return strings.TrimSpace(left)
		}
		return hint
	case "Grep", "Glob":
		// Search hint should be quote-like for quick visual scanning.
		if strings.HasPrefix(hint, "\"") || strings.HasPrefix(hint, "'") {
			return hint
		}
		return "\"" + hint + "\""
	case "Bash", "PowerShell":
		// Match TS command-as-hint spirit.
		if strings.HasPrefix(hint, "$ ") {
			return hint
		}
		return "$ " + hint
	default:
		return hint
	}
}

func summarizeReadResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	typ := getString(obj, "type")
	file := getMap(obj, "file")
	switch typ {
	case "text":
		numLines := getInt(file, "numLines")
		return fmt.Sprintf("Read %d %s", numLines, pluralize(numLines, "line", "lines"))
	case "image":
		size := getInt(file, "originalSize")
		if size > 0 {
			return fmt.Sprintf("Read image (%s)", formatBytes(size))
		}
		return "Read image"
	case "notebook":
		cells := getSliceLen(file, "cells")
		if cells > 0 {
			return fmt.Sprintf("Read %d %s", cells, pluralize(cells, "cell", "cells"))
		}
		return "No cells found in notebook"
	case "pdf":
		size := getInt(file, "originalSize")
		if size > 0 {
			return fmt.Sprintf("Read PDF (%s)", formatBytes(size))
		}
		return "Read PDF"
	case "parts":
		count := getInt(file, "count")
		size := getInt(file, "originalSize")
		if count > 0 {
			if size > 0 {
				return fmt.Sprintf("Read %d %s (%s)", count, pluralize(count, "page", "pages"), formatBytes(size))
			}
			return fmt.Sprintf("Read %d %s", count, pluralize(count, "page", "pages"))
		}
	case "file_unchanged":
		return "Unchanged since last read"
	}
	return truncateToolContent(parsed.Body, width-4)
}

func summarizeWriteResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	filePath := getString(obj, "filePath")
	if isPlanPathForSummary(filePath) {
		return "/plan to preview"
	}
	displayPath := displayFilePathForSummary(filePath)
	content := getString(obj, "content")
	lines := countLogicalLines(content)
	verb := "Wrote"
	if getString(obj, "type") == "update" {
		verb = "Updated"
	}
	if displayPath != "" && lines > 0 {
		return fmt.Sprintf("%s %d %s to %s", verb, lines, pluralize(lines, "line", "lines"), displayPath)
	}
	if displayPath != "" {
		return fmt.Sprintf("%s %s", verb, displayPath)
	}
	return truncateToolContent(parsed.Body, width-4)
}

func summarizeEditResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	filePath := getString(obj, "filePath")
	if isPlanPathForSummary(filePath) {
		return "/plan to preview"
	}
	displayPath := displayFilePathForSummary(filePath)
	replaceAll := getBool(obj, "replaceAll")
	if displayPath == "" {
		return "Applied edit"
	}
	if replaceAll {
		return "Applied edits to " + displayPath
	}
	return "Updated " + displayPath
}

func summarizeGrepResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	mode := getString(obj, "mode")
	numFiles := getInt(obj, "numFiles")
	switch mode {
	case "count":
		numMatches := getInt(obj, "numMatches")
		if numMatches > 0 {
			return fmt.Sprintf("Found %d %s in %d %s", numMatches, pluralize(numMatches, "match", "matches"), numFiles, pluralize(numFiles, "file", "files"))
		}
		if numFiles > 0 {
			return fmt.Sprintf("Found 0 matches in %d %s", numFiles, pluralize(numFiles, "file", "files"))
		}
		return "Found 0 matches"
	case "content":
		numLines := getInt(obj, "numLines")
		if numLines > 0 {
			return fmt.Sprintf("Found %d matching %s", numLines, pluralize(numLines, "line", "lines"))
		}
		return "Found 0 matching lines"
	default:
		if numFiles > 0 {
			return fmt.Sprintf("Found %d %s", numFiles, pluralize(numFiles, "file", "files"))
		}
		return "Found 0 files"
	}
}

func summarizeGlobResult(parsed parsedToolResult, width int) string {
	body := strings.TrimSpace(parsed.Body)
	if body == "" || body == "No files found" {
		return "Found 0 files"
	}
	lines := splitLinesRaw(body)
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "(Results are truncated") {
			continue
		}
		count++
	}
	if count > 0 {
		return fmt.Sprintf("Found %d %s", count, pluralize(count, "file", "files"))
	}
	return "Found 0 files"
}

func summarizeBashResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	if getBool(obj, "isImage") {
		return "[Image data detected and sent to Claude]"
	}
	if id := getString(obj, "backgroundTaskId"); id != "" {
		return "Started background task " + id
	}
	stdout := getString(obj, "stdout")
	stderr := getString(obj, "stderr")
	stderr = removeTaggedSection(stderr, "sandbox_violations")
	stderr, _ = stripCwdResetWarning(stderr)
	returnCodeInterpretation := getString(obj, "returnCodeInterpretation")
	if parsed.Status == "error" && strings.TrimSpace(stderr) != "" {
		return truncateToolContent(strings.Split(strings.TrimSpace(stderr), "\n")[0], width-4)
	}
	if getBool(obj, "interrupted") {
		return "Command interrupted"
	}
	if strings.TrimSpace(stdout) == "" && strings.TrimSpace(stderr) == "" {
		if strings.TrimSpace(returnCodeInterpretation) != "" {
			return returnCodeInterpretation
		}
		if getBool(obj, "noOutputExpected") {
			return "Done"
		}
		return "(No output)"
	}
	if strings.TrimSpace(stderr) != "" && strings.TrimSpace(stdout) == "" {
		return truncateToolContent(strings.Split(strings.TrimSpace(stderr), "\n")[0], width-4)
	}
	lines := countLogicalLines(stdout)
	if lines > 1 {
		return fmt.Sprintf("%d %s output", lines, pluralize(lines, "line", "lines"))
	}
	return truncateToolContent(strings.TrimSpace(stdout), width-4)
}

func removeTaggedSection(raw, tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.TrimSpace(raw) == "" {
		return raw
	}
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	start := strings.Index(raw, open)
	if start < 0 {
		return raw
	}
	end := strings.Index(raw[start+len(open):], close)
	if end < 0 {
		return raw
	}
	end += start + len(open) + len(close)
	return strings.TrimSpace(raw[:start] + raw[end:])
}

func stripCwdResetWarning(stderr string) (string, bool) {
	const marker = "Shell cwd was reset to "
	lines := splitLinesRaw(stderr)
	if len(lines) == 0 {
		return strings.TrimSpace(stderr), false
	}
	out := make([]string, 0, len(lines))
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, marker) {
			found = true
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n")), found
}

func isPlanPathForSummary(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	plansDir := filepath.Join(home, ".claude", "plans")
	cleanPath := filepath.Clean(path)
	cleanPlansDir := filepath.Clean(plansDir)
	return strings.HasPrefix(cleanPath, cleanPlansDir)
}

func displayFilePathForSummary(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		return path
	}
	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, absPath)
	}
	if rel, relErr := filepath.Rel(cwd, absPath); relErr == nil && rel != "" && rel != "." {
		return rel
	}
	return path
}

func summarizeAgentResult(parsed parsedToolResult, width int) string {
	obj := parseJSONMap(parsed.Body)
	if len(obj) == 0 {
		return truncateToolContent(parsed.Body, width-4)
	}
	status := getString(obj, "status")
	agentType := getString(obj, "agent_type")
	if agentType == "" {
		agentType = getString(obj, "subagent_type")
	}
	if agentType == "" {
		agentType = "Agent"
	}
	if status == "remote_launched" {
		taskID := getString(obj, "taskId")
		if taskID == "" {
			taskID = getString(obj, "task_id")
		}
		if taskID != "" {
			return "Remote agent launched · " + taskID
		}
		return "Remote agent launched"
	}
	if status == "async_launched" {
		return "Backgrounded agent"
	}
	if status == "completed" {
		toolUses := getInt(obj, "totalToolUseCount")
		tokens := getInt(obj, "totalTokens")
		durationMs := getInt(obj, "totalDurationMs")
		_, hasToolUses := obj["totalToolUseCount"]
		_, hasTokens := obj["totalTokens"]
		_, hasDuration := obj["totalDurationMs"]
		if hasToolUses || hasTokens || hasDuration {
			parts := make([]string, 0, 3)
			parts = append(parts, fmt.Sprintf("%d %s", toolUses, pluralize(toolUses, "tool use", "tool uses")))
			if hasTokens {
				parts = append(parts, fmt.Sprintf("%s tokens", formatWithCommas(tokens)))
			}
			if hasDuration && durationMs > 0 {
				parts = append(parts, formatElapsedMs(durationMs))
			}
			if len(parts) > 0 {
				return "Done (" + strings.Join(parts, " · ") + ")"
			}
		}
		summary := getString(obj, "summary")
		if strings.TrimSpace(summary) != "" {
			return truncateToolContent(summary, width-4)
		}
		return agentType + " agent completed"
	}
	return truncateToolContent(parsed.Body, width-4)
}

func formatElapsedMs(ms int) string {
	if ms <= 0 {
		return "0s"
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	rem := seconds % 60
	if rem == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, rem)
}

func formatWithCommas(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign = "-"
		s = s[1:]
	}
	rem := len(s) % 3
	var b strings.Builder
	b.WriteString(sign)
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteString(",")
		}
	}
	for i := rem; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteString(",")
		}
	}
	return b.String()
}

func truncateToolContent(content string, width int) string {
	if width <= 0 {
		width = 40
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return truncateVisible(content, width)
}

func parseJSONMap(raw string) map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func getMap(m map[string]any, key string) map[string]any {
	v, ok := m[key]
	if !ok {
		return nil
	}
	out, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return out
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func getBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func getInt(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil {
			return n
		}
	}
	return 0
}

func getSliceLen(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	s, ok := v.([]any)
	if !ok {
		return 0
	}
	return len(s)
}

func countLogicalLines(content string) int {
	if content == "" {
		return 0
	}
	lines := strings.Split(content, "\n")
	if strings.HasSuffix(content, "\n") {
		return len(lines) - 1
	}
	return len(lines)
}

func formatBytes(size int) string {
	const kb = 1024
	const mb = 1024 * 1024
	switch {
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func formatElapsedDurationLabel(ms int) string {
	if ms <= 0 {
		return "0s"
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm%02ds", seconds/60, seconds%60)
}

func activeToolProgressLabel(toolName string) string {
	switch toolName {
	case "Read":
		return "Reading…"
	case "Write", "Edit":
		return "Updating…"
	case "Glob", "Grep":
		return "Searching…"
	case "Agent":
		return "Running agents…"
	case "WebFetch":
		return "Fetching…"
	case "WebSearch":
		return "Searching web…"
	default:
		return "Running…"
	}
}

// renderCollapsedBlock renders a collapsed group summary
func renderCollapsedBlock(width int, entry TranscriptEntry, mode ViewMode) string {
	ctx := RenderContext{Mode: mode}
	return renderCollapsedBlockWithContext(width, entry, ctx)
}

// renderCollapsedBlockWithContext renders a collapsed group summary with full context
// Matches TS CollapsedReadSearchContent.tsx behavior
func renderCollapsedBlockWithContext(width int, entry TranscriptEntry, ctx RenderContext) string {
	// Build summary text
	var parts []string
	meta := entry.Meta

	// Memory operations first
	if meta.MemoryReadCount > 0 {
		verb := "Recalled"
		if entry.IsActive {
			verb = "Recalling"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.MemoryReadCount, pluralize(meta.MemoryReadCount, "memory", "memories")))
	}
	if meta.MemoryWriteCount > 0 {
		verb := "Wrote"
		if entry.IsActive {
			verb = "Writing"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.MemoryWriteCount, pluralize(meta.MemoryWriteCount, "memory", "memories")))
	}

	// Search operations
	if meta.SearchCount > 0 {
		verb := "Searched for"
		if entry.IsActive {
			verb = "Searching for"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.SearchCount, pluralize(meta.SearchCount, "pattern", "patterns")))
	}

	// Read operations
	if meta.ReadCount > 0 {
		verb := "Read"
		if entry.IsActive {
			verb = "Reading"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.ReadCount, pluralize(meta.ReadCount, "file", "files")))
	}

	// List operations
	if meta.ListCount > 0 {
		verb := "Listed"
		if entry.IsActive {
			verb = "Listing"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.ListCount, pluralize(meta.ListCount, "directory", "directories")))
	}

	// Bash operations
	if meta.BashCount > 0 {
		verb := "Ran"
		if entry.IsActive {
			verb = "Running"
		}
		parts = append(parts, fmt.Sprintf("%s %d bash %s", verb, meta.BashCount, pluralize(meta.BashCount, "command", "commands")))
	}

	// MCP operations
	if meta.MCPCallCount > 0 {
		verb := "Queried"
		if entry.IsActive {
			verb = "Querying"
		}
		serverNames := strings.Join(meta.MCPServerNames, ", ")
		if serverNames != "" {
			parts = append(parts, fmt.Sprintf("%s %s", verb, serverNames))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d MCP %s", verb, meta.MCPCallCount, pluralize(meta.MCPCallCount, "tool", "tools")))
		}
	}

	// Git operations
	if len(meta.Commits) > 0 {
		for _, c := range meta.Commits {
			parts = append(parts, "committed "+c.SHA[:7])
		}
	}
	if len(meta.PRs) > 0 {
		for _, pr := range meta.PRs {
			if pr.Action == "created" {
				parts = append(parts, fmt.Sprintf("created PR #%d", pr.Number))
			}
		}
	}

	summary := strings.Join(parts, ", ")
	if entry.IsActive {
		summary += "…"
	}

	// Render the summary line with blinking indicator for active groups
	var lines []string

	// Add blinking indicator for active groups (matches TS ToolUseLoader)
	var prefix string
	if entry.IsActive && ctx.Busy {
		frame := (ctx.SpinnerTick / 600) % 2
		if frame == 0 {
			prefix = style(&dark.claude, nil, "● ", false)
		} else {
			prefix = "  "
		}
	} else if entry.IsActive {
		prefix = style(&dark.claude, nil, "● ", false)
	} else {
		prefix = style(&dark.success, nil, "● ", false)
	}

	color := &dark.claude
	if !entry.IsActive {
		color = &dark.muted
	}
	lines = append(lines, prefix+style(color, nil, summary, false))

	// Add hint if present (current operation being performed)
	if meta.DisplayHint != "" && entry.IsActive {
		lines = append(lines, style(&dark.subtle, nil, "  ⎿ "+truncateVisible(meta.DisplayHint, width-8), false))
	}

	// Add Ctrl+O hint in normal mode
	if ctx.Mode == ViewModeNormal {
		lines = append(lines, style(&dark.subtle, nil, "    Ctrl+O to expand", false))
	}

	return strings.Join(lines, "\n")
}

// renderThinkingBlock renders a thinking/redacted_thinking entry
// Matches TS AssistantThinkingMessage.tsx behavior
func renderThinkingBlock(width int, entry TranscriptEntry, mode ViewMode) string {
	// TS normal mode: shows "∴ Thinking" + "Ctrl+O to expand" (collapsed)
	// TS transcript/verbose mode: shows full thinking content
	// Ref: src/components/messages/AssistantThinkingMessage.tsx:39-57

	if mode == ViewModeNormal {
		// Show collapsed label in normal mode (TS line 40-47)
		label := style(&dark.subtle, nil, "∴ Thinking", false)
		hint := style(&dark.subtle, nil, " (Ctrl+O to expand)", false)
		return label + hint
	}

	// In verbose/transcript mode, show full thinking content (TS line 58+)
	if entry.Kind == "redacted_thinking" {
		// Redacted thinking shows just label
		label := style(&dark.subtle, nil, "∴ Thinking…", false)
		return label
	}

	// Full thinking: label + content
	label := style(&dark.subtle, nil, "∴ Thinking…", false)
	// Render thinking content with markdown (TS uses Markdown component)
	// For now, show raw content indented
	lines := strings.Split(entry.Content, "\n")
	var rendered []string
	rendered = append(rendered, label)
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			rendered = append(rendered, style(&dark.subtle, nil, "  "+line, false))
		}
	}
	return strings.Join(rendered, "\n")
}

// pluralize returns singular or plural form
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
