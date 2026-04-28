package cmd

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// Renderer handles Markdown and color rendering for CLI output
type Renderer struct {
	mdRenderer *glamour.TermRenderer
	width      int
}

// NewRenderer creates a new CLI renderer
func NewRenderer(width int) *Renderer {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		// Fallback to default renderer
		r, _ = glamour.NewTermRenderer()
	}

	return &Renderer{
		mdRenderer: r,
		width:      width,
	}
}

// RenderMarkdown renders Markdown content to styled terminal output
func (r *Renderer) RenderMarkdown(content string) string {
	if r.mdRenderer == nil {
		return content
	}

	rendered, err := r.mdRenderer.Render(content)
	if err != nil {
		return content
	}

	// glamour adds trailing newlines, clean them up
	return strings.TrimRight(rendered, "\n")
}

// RenderStreamingChunk renders a streaming chunk with optional styling
func (r *Renderer) RenderStreamingChunk(chunk string, isFirst, isCodeBlock bool) string {
	if chunk == "" {
		return ""
	}

	// For streaming, we output raw chunks to maintain flow
	// Full Markdown rendering happens on complete messages
	return chunk
}

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	// Simple ANSI stripping for cleaner comparison
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
