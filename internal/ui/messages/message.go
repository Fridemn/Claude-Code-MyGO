package messages

import (
	"fmt"
	"strings"
	"time"

	"claude-go/internal/ui/components"
)

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeUser           MessageType = "user"
	MessageTypeAssistant      MessageType = "assistant"
	MessageTypeSystem         MessageType = "system"
	MessageTypeToolUse        MessageType = "tool_use"
	MessageTypeToolResult     MessageType = "tool_result"
	MessageTypeAttachment     MessageType = "attachment"
	MessageTypeCollapsed      MessageType = "collapsed"
	MessageTypeThinking       MessageType = "thinking"
	MessageTypeCompactSummary MessageType = "compact_summary"
)

// Message represents a chat message for rendering
type Message struct {
	Type      MessageType
	Content   string
	UUID      string
	Timestamp time.Time

	// Tool-specific fields
	ToolName   string
	ToolUseID  string
	ToolInput  string
	ToolOutput string
	IsError    bool

	// Status fields
	IsActive    bool
	IsStreaming bool
	IsCollapsed bool
	IsTruncated bool

	// Metadata
	TokenCount int
	Meta       MessageMeta

	// Assistant advisor result compatibility (TS: advisor_tool_result/advisor_result)
	// Kept minimal and behavior-safe: only used for clickability gating.
	IsAdvisorResult bool
}

// IsClickableForExpand matches the transcript click-toggle rule used by UI:
// only entries that can reveal additional hidden/truncated detail should toggle.
func IsClickableForExpand(msg Message) bool {
	switch msg.Type {
	case MessageTypeCollapsed:
		return true
	case MessageTypeToolResult:
		return !msg.IsError && msg.IsTruncated
	case MessageTypeAssistant:
		return msg.IsAdvisorResult
	default:
		return false
	}
}

// MessageMeta contains metadata for collapsed groups and special messages
type MessageMeta struct {
	SearchCount      int
	ReadCount        int
	ListCount        int
	BashCount        int
	MemoryReadCount  int
	MemoryWriteCount int
	MCPCallCount     int
	MCPServerNames   []string
	DisplayHint      string
	GroupMessages    []Message
}

// Theme colors for messages
var (
	colorClaude     = components.RGB{215, 119, 87}
	colorUser       = components.RGB{122, 180, 232}
	colorMuted      = components.RGB{134, 145, 160}
	colorSuccess    = components.RGB{78, 186, 101}
	colorError      = components.RGB{255, 107, 128}
	colorWarning    = components.RGB{255, 193, 7}
	colorText       = components.RGB{255, 255, 255}
	colorPermission = components.RGB{177, 185, 249}
)

// RenderMessage renders a message based on its type
func RenderMessage(msg Message, width int, verbose bool) string {
	switch msg.Type {
	case MessageTypeUser:
		return RenderUserMessage(msg, width)
	case MessageTypeAssistant:
		return RenderAssistantMessage(msg, width)
	case MessageTypeSystem:
		return RenderSystemMessage(msg, width)
	case MessageTypeToolUse:
		return RenderToolUseMessage(msg, width, verbose)
	case MessageTypeToolResult:
		return RenderToolResultMessage(msg, width, verbose)
	case MessageTypeCollapsed:
		return RenderCollapsedMessage(msg, width)
	case MessageTypeThinking:
		return RenderThinkingMessage(msg, width, verbose)
	case MessageTypeCompactSummary:
		return RenderCompactSummary(msg, width)
	default:
		return msg.Content
	}
}

// RenderUserMessage renders a user message
// Matches src/components/messages/UserTextMessage.tsx
func RenderUserMessage(msg Message, width int) string {
	// User label with colored prompt
	label := renderColoredBold(colorUser, "⏵") + " "

	content := msg.Content
	if msg.IsTruncated && len(content) > width-4 {
		content = content[:width-7] + "..."
	}

	return label + content
}

// RenderAssistantMessage renders an assistant text message
// Matches src/components/messages/AssistantTextMessage.tsx
func RenderAssistantMessage(msg Message, width int) string {
	// Assistant messages are shown with claude color indicator
	indicator := components.BlackCircle
	if msg.IsStreaming {
		indicator = "●" // Active indicator
	}

	prefix := renderColored(colorClaude, indicator) + " "

	// Wrap content to width
	lines := wrapText(msg.Content, width-2)

	var result []string
	for i, line := range lines {
		if i == 0 {
			result = append(result, prefix+line)
		} else {
			result = append(result, "  "+line)
		}
	}

	return strings.Join(result, "\n")
}

// RenderSystemMessage renders a system message
// Matches src/components/messages/SystemTextMessage.tsx
func RenderSystemMessage(msg Message, width int) string {
	// Check for compact boundary - render as special compact boundary message
	if strings.Contains(msg.Content, "[compact boundary") {
		return RenderCompactBoundary(width)
	}
	// System messages are dimmed
	return renderDim(msg.Content)
}

// RenderToolUseMessage renders a tool use message
// Matches src/components/messages/AssistantToolUseMessage.tsx
func RenderToolUseMessage(msg Message, width int, verbose bool) string {
	// Status indicator
	var indicator string
	if msg.IsActive {
		indicator = renderColored(colorClaude, components.BlackCircle)
	} else if msg.IsError {
		indicator = renderColored(colorError, components.BlackCircle)
	} else {
		indicator = renderColored(colorSuccess, components.BlackCircle)
	}

	// Tool name
	toolName := renderBold(msg.ToolName)

	// Build the line
	line := indicator + " " + toolName

	// In verbose mode, show input parameters
	if verbose && msg.ToolInput != "" {
		input := truncateText(msg.ToolInput, width-len(msg.ToolName)-10)
		line += " " + renderMuted("("+input+")")
	}

	return line
}

// RenderToolResultMessage renders a tool result message
// Matches src/components/messages/UserToolResultMessage
func RenderToolResultMessage(msg Message, width int, verbose bool) string {
	// Tree branch indicator
	prefix := renderMuted("  ⎿ ")

	content := msg.Content
	if !verbose && len(content) > width-6 {
		content = truncateText(content, width-6)
	}

	// Color based on success/error
	if msg.IsError {
		return prefix + renderColored(colorError, content)
	}

	return prefix + renderMuted(content)
}

// RenderCollapsedMessage renders a collapsed group summary
// Matches src/components/messages/CollapsedReadSearchContent.tsx
func RenderCollapsedMessage(msg Message, width int) string {
	meta := msg.Meta

	// Build summary parts
	var parts []string

	// Memory operations first
	if meta.MemoryReadCount > 0 {
		verb := "Recalled"
		if msg.IsActive {
			verb = "Recalling"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.MemoryReadCount, pluralize(meta.MemoryReadCount, "memory", "memories")))
	}
	if meta.MemoryWriteCount > 0 {
		verb := "Wrote"
		if msg.IsActive {
			verb = "Writing"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.MemoryWriteCount, pluralize(meta.MemoryWriteCount, "memory", "memories")))
	}

	// Search operations
	if meta.SearchCount > 0 {
		verb := "Searched for"
		if msg.IsActive {
			verb = "Searching for"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.SearchCount, pluralize(meta.SearchCount, "pattern", "patterns")))
	}

	// Read operations
	if meta.ReadCount > 0 {
		verb := "Read"
		if msg.IsActive {
			verb = "Reading"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.ReadCount, pluralize(meta.ReadCount, "file", "files")))
	}

	// List operations
	if meta.ListCount > 0 {
		verb := "Listed"
		if msg.IsActive {
			verb = "Listing"
		}
		parts = append(parts, fmt.Sprintf("%s %d %s", verb, meta.ListCount, pluralize(meta.ListCount, "directory", "directories")))
	}

	// Bash operations
	if meta.BashCount > 0 {
		verb := "Ran"
		if msg.IsActive {
			verb = "Running"
		}
		parts = append(parts, fmt.Sprintf("%s %d bash %s", verb, meta.BashCount, pluralize(meta.BashCount, "command", "commands")))
	}

	// MCP operations
	if meta.MCPCallCount > 0 {
		verb := "Queried"
		if msg.IsActive {
			verb = "Querying"
		}
		if len(meta.MCPServerNames) > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", verb, strings.Join(meta.MCPServerNames, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%s %d MCP %s", verb, meta.MCPCallCount, pluralize(meta.MCPCallCount, "tool", "tools")))
		}
	}

	summary := strings.Join(parts, ", ")
	if msg.IsActive {
		summary += "…"
	}

	// Render the summary line
	var lines []string
	color := colorClaude
	if !msg.IsActive {
		color = colorMuted
	}
	lines = append(lines, renderColored(color, "  ⎿ "+summary))

	// Add hint if present
	if meta.DisplayHint != "" {
		hint := truncateText(meta.DisplayHint, width-8)
		lines = append(lines, renderMuted("    "+hint))
	}

	// Add Ctrl+O hint
	lines = append(lines, renderMuted("    Ctrl+O to expand"))

	return strings.Join(lines, "\n")
}

// RenderThinkingMessage renders a thinking block
// Matches src/components/messages/AssistantThinkingMessage.tsx
func RenderThinkingMessage(msg Message, width int, verbose bool) string {
	if !verbose {
		return "" // Hidden in normal mode
	}

	content := truncateText(msg.Content, width-16)
	return renderMuted("  [thinking: " + content + "]")
}

// RenderCompactSummary renders a compact summary message
// Matches src/components/CompactSummary.tsx
func RenderCompactSummary(msg Message, width int) string {
	indicator := renderColored(colorText, components.BlackCircle)
	title := renderBold("Compact summary")
	hint := renderMuted(" (Ctrl+O to expand)")

	return indicator + " " + title + hint
}

// RenderCompactBoundary renders a compact boundary message
// Matches src/components/messages/CompactBoundaryMessage.tsx
// Shows "✻ Conversation compacted (Ctrl+O for history)"
func RenderCompactBoundary(width int) string {
	return renderDim("✻ Conversation compacted (Ctrl+O for history)")
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderColoredBold(color components.RGB, text string) string {
	return fmt.Sprintf("\033[1m\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderBold(text string) string {
	return "\033[1m" + text + "\033[0m"
}

func renderDim(text string) string {
	return "\033[2m" + text + "\033[0m"
}

func renderMuted(text string) string {
	return renderColored(colorMuted, text)
}

func truncateText(text string, maxLen int) string {
	if maxLen <= 3 {
		return "…"
	}
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-1] + "…"
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

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
