package memory

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"claude-go/internal/types"
)

// Frontmatter represents parsed YAML frontmatter from a memory file.
type Frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Type        string   `yaml:"type"`
	Paths       string   `yaml:"paths"` // Comma or newline separated glob patterns
	Globs       []string `yaml:"-"`     // Parsed glob patterns
}

var (
	// frontmatterRegex matches YAML frontmatter between --- delimiters.
	frontmatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?`)
	// keyValueRegex matches key: value pairs in frontmatter.
	keyValueRegex = regexp.MustCompile(`^(\w+):\s*(.*)$`)
)

// ParseFrontmatter extracts frontmatter from raw content and returns
// the frontmatter struct and the content without frontmatter.
func ParseFrontmatter(rawContent string) (Frontmatter, string) {
	var fm Frontmatter

	matches := frontmatterRegex.FindStringSubmatch(rawContent)
	if len(matches) < 2 {
		return fm, rawContent
	}

	frontmatterText := matches[1]
	content := strings.TrimPrefix(rawContent, matches[0])

	// Parse key: value pairs
	scanner := bufio.NewScanner(strings.NewReader(frontmatterText))
	var currentKey string
	var currentValue strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if matches := keyValueRegex.FindStringSubmatch(line); len(matches) == 3 {
			// Save previous key/value
			if currentKey != "" {
				setFrontmatterValue(&fm, currentKey, strings.TrimSpace(currentValue.String()))
			}
			currentKey = matches[1]
			currentValue.Reset()
			currentValue.WriteString(matches[2])
		} else if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
			// Continuation of previous value
			currentValue.WriteString("\n")
			currentValue.WriteString(strings.TrimSpace(line))
		} else if currentKey != "" && strings.TrimSpace(line) != "" {
			// Part of a multi-line value
			currentValue.WriteString("\n")
			currentValue.WriteString(line)
		}
	}
	// Save last key/value
	if currentKey != "" {
		setFrontmatterValue(&fm, currentKey, strings.TrimSpace(currentValue.String()))
	}

	// Parse paths into globs
	if fm.Paths != "" {
		fm.Globs = splitPathInFrontmatter(fm.Paths)
	}

	return fm, content
}

func setFrontmatterValue(fm *Frontmatter, key, value string) {
	switch key {
	case "name":
		fm.Name = value
	case "description":
		fm.Description = value
	case "type":
		fm.Type = value
	case "paths":
		fm.Paths = value
	}
}

// splitPathInFrontmatter splits a paths string into individual glob patterns.
// Handles comma-separated, newline-separated, and array-style paths.
func splitPathInFrontmatter(paths string) []string {
	// Remove array brackets if present
	paths = strings.TrimPrefix(paths, "[")
	paths = strings.TrimSuffix(paths, "]")

	// Split by comma or newline
	parts := strings.FieldsFunc(paths, func(r rune) bool {
		return r == ',' || r == '\n'
	})

	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Remove quotes
		p = strings.Trim(p, `"'`)
		if p != "" {
			// Remove /** suffix - ignore library treats 'path' as matching both
			// the path itself and everything inside it
			p = strings.TrimSuffix(p, "/**")
			result = append(result, p)
		}
	}

	return result
}

// StripHTMLComments removes block-level HTML comments from markdown content.
// Uses a simple parser to avoid removing comments inside code blocks.
func StripHTMLComments(content string) (string, bool) {
	if !strings.Contains(content, "<!--") {
		return content, false
	}

	// Simple state machine to track code blocks
	var result strings.Builder
	inCodeBlock := false
	inCodeSpan := false
	stripped := false

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for fenced code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			result.WriteString(line)
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
			continue
		}

		// Skip processing inside code blocks
		if inCodeBlock {
			result.WriteString(line)
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
			continue
		}

		// Process line for HTML comments (but not inside code spans)
		processed := processLineForHTMLComments(line, &inCodeSpan)
		if processed != line {
			stripped = true
		}
		result.WriteString(processed)
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String(), stripped
}

func processLineForHTMLComments(line string, inCodeSpan *bool) string {
	var result strings.Builder
	i := 0

	for i < len(line) {
		// Check for code span toggle
		if i < len(line)-1 && line[i:i+1] == "`" {
			// Check if it's escaped
			if i > 0 && line[i-1] == '\\' {
				result.WriteByte(line[i])
				i++
				continue
			}
			*inCodeSpan = !*inCodeSpan
			result.WriteByte(line[i])
			i++
			continue
		}

		// Don't strip comments inside code spans
		if *inCodeSpan {
			result.WriteByte(line[i])
			i++
			continue
		}

		// Check for HTML comment start
		if i < len(line)-3 && line[i:i+4] == "<!--" {
			// Find comment end
			endIdx := strings.Index(line[i:], "-->")
			if endIdx != -1 {
				stripped := true
				_ = stripped // Mark as used
				i += endIdx + 3 // Skip past -->
				continue
			}
			// Unclosed comment - leave as is
			result.WriteString(line[i:])
			break
		}

		result.WriteByte(line[i])
		i++
	}

	return result.String()
}

// TruncateEntrypointContent truncates MEMORY.md content to line AND byte caps.
// Returns truncation info with details about what was truncated.
func TruncateEntrypointContent(raw string) types.MemoryEntrypointTruncation {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return types.MemoryEntrypointTruncation{
			Content:   "",
			LineCount: 0,
			ByteCount: 0,
		}
	}

	contentLines := strings.Split(trimmed, "\n")
	lineCount := len(contentLines)
	byteCount := len(trimmed)

	wasLineTruncated := lineCount > MaxEntrypointLines
	wasByteTruncated := byteCount > MaxEntrypointBytes

	if !wasLineTruncated && !wasByteTruncated {
		return types.MemoryEntrypointTruncation{
			Content:           trimmed,
			LineCount:         lineCount,
			ByteCount:         byteCount,
			WasLineTruncated:  false,
			WasByteTruncated:  false,
		}
	}

	var truncated string
	if wasLineTruncated {
		truncated = strings.Join(contentLines[:MaxEntrypointLines], "\n")
	} else {
		truncated = trimmed
	}

	if len(truncated) > MaxEntrypointBytes {
		cutAt := bytes.LastIndexByte([]byte(truncated[:MaxEntrypointBytes]), '\n')
		if cutAt > 0 {
			truncated = truncated[:cutAt]
		} else {
			truncated = truncated[:MaxEntrypointBytes]
		}
	}

	return types.MemoryEntrypointTruncation{
		Content:           truncated,
		LineCount:         lineCount,
		ByteCount:         byteCount,
		WasLineTruncated:  wasLineTruncated,
		WasByteTruncated:  wasByteTruncated,
	}
}

// ParseMemoryFileContent parses raw memory file content into a MemoryFileInfo.
// This is a pure function - no I/O.
func ParseMemoryFileContent(rawContent string, filePath string, memType types.MemoryType) (*types.MemoryFileInfo, []string) {
	// Skip non-text files
	ext := getFileExtension(filePath)
	if ext != "" && !IsTextFileExtension(ext) {
		return nil, nil
	}

	// Parse frontmatter
	fm, content := ParseFrontmatter(rawContent)

	// Strip HTML comments
	strippedContent, wasStripped := StripHTMLComments(content)
	_ = wasStripped // Track for contentDiffersFromDisk

	// Extract @include paths
	includePaths := extractIncludePaths(strippedContent, filePath)

	// Truncate MEMORY.md entrypoints
	finalContent := strippedContent
	if memType == types.MemoryTypeAutoMem || memType == types.MemoryTypeTeamMem {
		trunc := TruncateEntrypointContent(strippedContent)
		finalContent = trunc.Content
	}

	contentDiffersFromDisk := finalContent != rawContent

	return &types.MemoryFileInfo{
		Path:                   filePath,
		Type:                   memType,
		Content:                finalContent,
		Globs:                  fm.Globs,
		ContentDiffersFromDisk: contentDiffersFromDisk,
		RawContent:             conditionalString(contentDiffersFromDisk, rawContent),
	}, includePaths
}

func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return strings.ToLower(path[i:])
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}

func conditionalString(condition bool, value string) string {
	if condition {
		return value
	}
	return ""
}

// includePathRegex matches @path references in markdown text.
var includePathRegex = regexp.MustCompile(`(?:^|\s)@((?:[^\s\\]|\\ )+)`)

// extractIncludePaths extracts @path include references from content.
// Returns resolved absolute paths.
func extractIncludePaths(content string, basePath string) []string {
	// This is a simplified implementation - the full version uses a markdown lexer
	// to avoid matching inside code blocks
	var paths []string
	matches := includePathRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			path := match[1]
			// Strip fragment identifiers
			if idx := strings.Index(path, "#"); idx != -1 {
				path = path[:idx]
			}
			if path != "" {
				// TODO: Resolve relative to basePath
				paths = append(paths, path)
			}
		}
	}
	return paths
}