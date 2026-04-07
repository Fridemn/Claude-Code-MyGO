package bash


import (
	"encoding/base64"
	"regexp"
	"strings"
)

// DetectBlockedSleepPattern detects blocking sleep commands
// that should run in background instead of blocking the conversation.
// Returns a description of the blocked pattern, or empty string if not blocked.
func DetectBlockedSleepPattern(command string) string {
	// Split command by common separators
	parts := splitCommandForSleepCheck(command)
	if len(parts) == 0 {
		return ""
	}

	first := strings.TrimSpace(parts[0])
	if first == "" {
		return ""
	}

	// Check for bare sleep N or sleep N.N as first subcommand
	// Float durations < 2s are allowed (legit pacing, not polls)
	sleepRegex := regexp.MustCompile(`^sleep\s+(\d+)(?:\.\d+)?\s*$`)
	m := sleepRegex.FindStringSubmatch(first)
	if m == nil {
		return ""
	}

	// Parse the integer part
	secs := 0
	for _, c := range m[1] {
		secs = secs*10 + int(c-'0')
	}

	if secs < 2 {
		// Sub-2s sleeps are fine (rate limiting, pacing)
		return ""
	}

	// Check if this is in a pipeline (pipeline is fine)
	for i := 1; i < len(parts); i++ {
		if parts[i] == "|" {
			return "" // sleep in pipeline is fine
		}
	}

	// Check if this is in a subshell (subshell is fine)
	if strings.HasPrefix(command, "(") || strings.HasPrefix(command, "{") {
		return ""
	}

	// sleep N alone → "what are you waiting for?"
	// sleep N && check → "use Monitor tool"
	rest := ""
	if len(parts) > 1 {
		restParts := []string{}
		for i := 1; i < len(parts); i++ {
			restParts = append(restParts, parts[i])
		}
		rest = strings.TrimSpace(strings.Join(restParts, " "))
	}

	if rest != "" {
		return "sleep " + m[1] + " followed by: " + rest
	}
	return "standalone sleep " + m[1]
}

// splitCommandForSleepCheck splits a command for sleep pattern detection
func splitCommandForSleepCheck(command string) []string {
	var parts []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := rune(command[i])

		if escaped {
			current.WriteRune(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingleQuote {
			escaped = true
			current.WriteRune(c)
			continue
		}

		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteRune(c)
			continue
		}

		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(c)
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			// Check for command separators
			if i+1 < len(command) {
				twoChar := command[i : i+2]
				if twoChar == "&&" || twoChar == "||" {
					if current.Len() > 0 {
						parts = append(parts, strings.TrimSpace(current.String()))
						current.Reset()
					}
					parts = append(parts, twoChar)
					i++
					continue
				}
			}
			if c == ';' || c == '|' || c == '\n' {
				if current.Len() > 0 {
					parts = append(parts, strings.TrimSpace(current.String()))
					current.Reset()
				}
				if c == ';' || c == '|' {
					parts = append(parts, string(c))
				}
				continue
			}
		}

		current.WriteRune(c)
	}

	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}

	return parts
}

// StripEmptyLines strips leading and trailing lines that contain only whitespace/newlines.
// Unlike trim(), this preserves whitespace within content lines and only removes
// completely empty lines from the beginning and end.
func StripEmptyLines(content string) string {
	lines := strings.Split(content, "\n")

	// Find the first non-empty line
	startIndex := 0
	for startIndex < len(lines) && strings.TrimSpace(lines[startIndex]) == "" {
		startIndex++
	}

	// Find the last non-empty line
	endIndex := len(lines) - 1
	for endIndex >= 0 && strings.TrimSpace(lines[endIndex]) == "" {
		endIndex--
	}

	// If all lines are empty, return empty string
	if startIndex > endIndex {
		return ""
	}

	// Return the slice with non-empty lines
	return strings.Join(lines[startIndex:endIndex+1], "\n")
}

// ImageOutput constants
const (
	MaxOutputLength     = 30000
	MaxOutputUpperLimit = 150000
	MaxImageFileSize    = 20 * 1024 * 1024 // 20 MB
)

// IsImageOutput checks if content is a base64 encoded image data URL
func IsImageOutput(content string) bool {
	imageDataURLRegex := regexp.MustCompile(`^data:image/[a-z0-9.+_-]+;base64,`)
	return imageDataURLRegex.MatchString(content)
}

// DataURI represents a parsed data URI
type DataURI struct {
	MediaType string
	Data      string
}

// ParseDataUri parses a data-URI string into its media type and base64 payload
func ParseDataUri(s string) *DataURI {
	dataURIRegex := regexp.MustCompile(`^data:([^;]+);base64,(.+)$`)
	s = strings.TrimSpace(s)

	m := dataURIRegex.FindStringSubmatch(s)
	if m == nil || len(m) < 3 {
		return nil
	}

	return &DataURI{
		MediaType: m[1],
		Data:      m[2],
	}
}

// buildImageToolResult builds an image tool result block from shell stdout
func buildImageToolResult(stdout string, toolUseID string) *ImageToolResult {
	parsed := ParseDataUri(stdout)
	if parsed == nil {
		return nil
	}

	return &ImageToolResult{
		ToolUseID:  toolUseID,
		Type:       "image",
		MediaType:  parsed.MediaType,
		Base64Data: parsed.Data,
	}
}

// ImageToolResult represents an image tool result
type ImageToolResult struct {
	ToolUseID  string
	Type       string
	MediaType  string
	Base64Data string
}

// FormatOutput formats output content with truncation if needed
func FormatOutput(content string) OutputFormat {
	isImage := IsImageOutput(content)
	if isImage {
		return OutputFormat{
			TotalLines:       1,
			TruncatedContent: content,
			IsImage:          true,
		}
	}

	maxOutputLength := getMaxOutputLength()
	if len(content) <= maxOutputLength {
		return OutputFormat{
			TotalLines:       CountLines(content),
			TruncatedContent: content,
			IsImage:          false,
		}
	}

	truncatedPart := content[:maxOutputLength]
	remainingLines := CountLines(content[maxOutputLength:])
	truncated := truncatedPart + "\n\n... [" + Itoa(remainingLines) + " lines truncated] ..."

	return OutputFormat{
		TotalLines:       CountLines(content),
		TruncatedContent: truncated,
		IsImage:          false,
	}
}

// OutputFormat represents formatted output
type OutputFormat struct {
	TotalLines       int
	TruncatedContent string
	IsImage          bool
}

// CountLines counts the number of lines in content
func CountLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// getMaxOutputLength returns the maximum output length from environment
func getMaxOutputLength() int {
	// Default value, can be overridden by environment
	return MaxOutputLength
}

// resizeShellImageOutput resizes image output from shell tool
// Returns the re-encoded data URI on success, or empty string if failed
func resizeShellImageOutput(stdout string, outputFilePath string, outputFileSize int64) string {
	// If output spilled to disk, read from there
	source := stdout
	if outputFilePath != "" {
		size := outputFileSize
		if size > MaxImageFileSize {
			return "" // Too large
		}
		// Read from file would happen here in real implementation
		// For now, we just use stdout
	}

	parsed := ParseDataUri(source)
	if parsed == nil {
		return ""
	}

	// Decode base64
	_, err := base64.StdEncoding.DecodeString(parsed.Data)
	if err != nil {
		return ""
	}

	// Resize logic would be implemented here
	// For now, return original
	return stdout
}

// ClaudeCodeHint represents a Claude Code hint from shell output
type ClaudeCodeHint struct {
	V             int    // Spec version
	Type          string // Hint type (e.g., "plugin")
	Value         string // Hint payload
	SourceCommand string // First token of command that produced hint
}

// hintTagRegex matches Claude Code hint tags
var hintTagRegex = regexp.MustCompile(`^[ \t]*<claude-code-hint\s+([^>]*?)\s*\/>[ \t]*$`)

// attrRegex matches attributes in hint tags
var attrRegex = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|([^\s/>]+))`)

// Supported versions and types
var supportedHintVersions = map[int]bool{1: true}
var supportedHintTypes = map[string]bool{"plugin": true}

// ExtractClaudeCodeHints scans shell output for hint tags
// Returns parsed hints and output with hint lines removed
func ExtractClaudeCodeHints(output string, command string) ClaudeCodeHintsResult {
	// Fast path: no tag sequence → no work
	if !strings.Contains(output, "<claude-code-hint") {
		return ClaudeCodeHintsResult{Hints: []ClaudeCodeHint{}, Stripped: output}
	}

	sourceCommand := FirstCommandToken(command)
	hints := []ClaudeCodeHint{}
	lines := strings.Split(output, "\n")
	strippedLines := make([]string, 0, len(lines))

	for _, rawLine := range lines {
		match := hintTagRegex.FindStringSubmatch(rawLine)
		if match == nil {
			strippedLines = append(strippedLines, rawLine)
			continue
		}

		attrs := parseHintAttrs(match[1])
		v := 0
		if attrs["v"] != "" {
			for _, c := range attrs["v"] {
				v = v*10 + int(c-'0')
			}
		}
		hintType := attrs["type"]
		value := attrs["value"]

		if !supportedHintVersions[v] {
			continue
		}
		if hintType == "" || !supportedHintTypes[hintType] {
			continue
		}
		if value == "" {
			continue
		}

		hints = append(hints, ClaudeCodeHint{
			V:             v,
			Type:          hintType,
			Value:         value,
			SourceCommand: sourceCommand,
		})
		strippedLines = append(strippedLines, "")
	}

	stripped := strings.Join(strippedLines, "\n")

	// Collapse multiple blank lines
	if len(hints) > 0 || stripped != output {
		blankLineRegex := regexp.MustCompile(`\n{3,}`)
		stripped = blankLineRegex.ReplaceAllString(stripped, "\n\n")
	}

	return ClaudeCodeHintsResult{Hints: hints, Stripped: stripped}
}

// ClaudeCodeHintsResult represents extracted hints
type ClaudeCodeHintsResult struct {
	Hints    []ClaudeCodeHint
	Stripped string
}

// parseHintAttrs parses attributes from a hint tag
func parseHintAttrs(tagBody string) map[string]string {
	attrs := map[string]string{}
	matches := attrRegex.FindAllStringSubmatch(tagBody, -1)
	for _, m := range matches {
		if len(m) >= 4 {
			key := m[1]
			value := m[2]
			if value == "" && len(m) >= 3 {
				value = m[3]
			}
			attrs[key] = value
		}
	}
	return attrs
}

// FirstCommandToken extracts the first whitespace-separated token from command
func FirstCommandToken(command string) string {
	trimmed := strings.TrimSpace(command)
	spaceIdx := strings.IndexFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n'
	})
	if spaceIdx == -1 {
		return trimmed
	}
	return trimmed[:spaceIdx]
}
