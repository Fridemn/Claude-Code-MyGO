package memory

import (
	"path/filepath"
	"strings"
)

// IncludeResolver resolves @include paths in memory files.
type IncludeResolver struct {
	basePath    string
	processed   map[string]bool
	maxDepth    int
	homeDir     string
	originalCwd string
}

// IncludeResolver creates a new include resolver.
func CreateIncludeResolver(basePath, homeDir, originalCwd string) *IncludeResolver {
	return &IncludeResolver{
		basePath:    basePath,
		processed:   make(map[string]bool),
		maxDepth:    MaxIncludeDepth,
		homeDir:     homeDir,
		originalCwd: originalCwd,
	}
}

// ResolveIncludePaths extracts and resolves all @include paths from content.
// Returns a list of resolved absolute paths.
func (r *IncludeResolver) ResolveIncludePaths(content string) []string {
	// Use markdown-aware extraction
	tokens := tokenizeMarkdown(content)
	return r.extractFromTokens(tokens)
}

// extractFromTokens extracts @include paths from markdown tokens,
// avoiding code blocks and code spans.
func (r *IncludeResolver) extractFromTokens(tokens []MarkdownToken) []string {
	var paths []string
	r.extractFromTokensRecursive(tokens, &paths)
	return paths
}

func (r *IncludeResolver) extractFromTokensRecursive(tokens []MarkdownToken, paths *[]string) {
	for _, token := range tokens {
		// Skip code blocks and code spans
		if token.Type == "code_block" || token.Type == "code_span" {
			continue
		}

		// Process text content
		if token.Text != "" {
			r.extractFromText(token.Text, paths)
		}

		// Recurse into children
		if len(token.Children) > 0 {
			r.extractFromTokensRecursive(token.Children, paths)
		}
	}
}

// extractFromText extracts @include paths from text content.
func (r *IncludeResolver) extractFromText(text string, paths *[]string) {
	// Pattern: @path, @./path, @~/path, @/path
	// Also handles escaped spaces: @path\ with\ spaces

	for i := 0; i < len(text); i++ {
		// Look for @ at word boundary
		if text[i] == '@' && (i == 0 || isWhitespace(text[i-1])) {
			// Extract the path after @
			path := extractPath(text[i+1:])
			if path == "" {
				continue
			}

			// Strip fragment identifiers
			if idx := strings.Index(path, "#"); idx != -1 {
				path = path[:idx]
			}

			if path == "" {
				continue
			}

			// Unescape spaces
			path = strings.ReplaceAll(path, "\\ ", " ")

			// Validate and resolve the path
			if isValidIncludePath(path) {
				resolved := r.resolvePath(path)
				if resolved != "" {
					*paths = append(*paths, resolved)
				}
			}
		}
	}
}

// resolvePath resolves a relative/absolute/home path to an absolute path.
func (r *IncludeResolver) resolvePath(path string) string {
	switch {
	case strings.HasPrefix(path, "~/"):
		// Home directory path
		if r.homeDir == "" {
			return ""
		}
		return filepath.Join(r.homeDir, path[2:])

	case strings.HasPrefix(path, "/"):
		// Absolute path
		return path

	case strings.HasPrefix(path, "./"):
		// Relative to current file
		return filepath.Join(filepath.Dir(r.basePath), path[2:])

	default:
		// Relative to current file (implicit ./)
		return filepath.Join(filepath.Dir(r.basePath), path)
	}
}

// MarkdownToken represents a parsed markdown token.
type MarkdownToken struct {
	Type     string
	Text     string
	Children []MarkdownToken
}

// tokenizeMarkdown parses markdown content into tokens.
// This is a simplified implementation that handles the most common cases.
func tokenizeMarkdown(content string) []MarkdownToken {
	var tokens []MarkdownToken
	lines := strings.Split(content, "\n")

	inCodeBlock := false
	var codeBlockContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle fenced code blocks
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// End code block
				tokens = append(tokens, MarkdownToken{
					Type: "code_block",
					Text: codeBlockContent.String(),
				})
				codeBlockContent.Reset()
				inCodeBlock = false
			} else {
				// Start code block
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			codeBlockContent.WriteString(line)
			codeBlockContent.WriteString("\n")
			continue
		}

		// Handle headings
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for i := 0; i < len(trimmed) && i < 6; i++ {
				if trimmed[i] == '#' {
					level++
				} else {
					break
				}
			}
			tokens = append(tokens, MarkdownToken{
				Type: "heading",
				Text: strings.TrimSpace(trimmed[level:]),
			})
			continue
		}

		// Handle list items
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			text := strings.TrimPrefix(trimmed, "- ")
			text = strings.TrimPrefix(text, "* ")
			// Check for inline code spans
			children := parseInlineElements(text)
			tokens = append(tokens, MarkdownToken{
				Type:     "list_item",
				Text:     text,
				Children: children,
			})
			continue
		}

		// Regular paragraph
		if trimmed != "" {
			children := parseInlineElements(trimmed)
			tokens = append(tokens, MarkdownToken{
				Type:     "paragraph",
				Text:     trimmed,
				Children: children,
			})
		}
	}

	return tokens
}

// parseInlineElements parses inline elements like code spans, links, etc.
func parseInlineElements(text string) []MarkdownToken {
	var tokens []MarkdownToken
	var currentText strings.Builder
	inCodeSpan := false

	for i := 0; i < len(text); i++ {
		// Check for code span
		if text[i] == '`' {
			if inCodeSpan {
				// End code span
				tokens = append(tokens, MarkdownToken{
					Type: "code_span",
					Text: currentText.String(),
				})
				currentText.Reset()
				inCodeSpan = false
			} else {
				// Save current text if any
				if currentText.Len() > 0 {
					tokens = append(tokens, MarkdownToken{
						Type: "text",
						Text: currentText.String(),
					})
					currentText.Reset()
				}
				inCodeSpan = true
			}
			continue
		}

		currentText.WriteByte(text[i])
	}

	// Save remaining text
	if currentText.Len() > 0 {
		if inCodeSpan {
			// Unclosed code span, treat as text
			tokens = append(tokens, MarkdownToken{
				Type: "text",
				Text: "`" + currentText.String(),
			})
		} else {
			tokens = append(tokens, MarkdownToken{
				Type: "text",
				Text: currentText.String(),
			})
		}
	}

	return tokens
}

// extractPath extracts the path after @, handling escaped spaces.
func extractPath(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && s[i+1] == ' ' {
			// Escaped space
			result.WriteByte(' ')
			i++
			continue
		}
		if isWhitespace(c) || isPunctuation(c) {
			break
		}
		result.WriteByte(c)
	}
	return result.String()
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isPunctuation(c byte) bool {
	return c == ',' || c == '.' || c == ';' || c == ':' || c == '!' || c == '?'
}

// isValidIncludePath checks if a path is valid for @include.
func isValidIncludePath(path string) bool {
	if path == "" {
		return false
	}

	// Skip if it looks like an email or mention
	if strings.Contains(path, "@") {
		return false
	}

	// Skip if it starts with special characters (likely not a path)
	if len(path) > 0 && (path[0] == '#' || path[0] == '^' || path[0] == '%' || path[0] == '&' || path[0] == '*') {
		return false
	}

	// Check for text file extension
	ext := filepath.Ext(path)
	if ext != "" && !IsTextFileExtension(ext) {
		return false
	}

	return true
}
