package tests

import (
	"strings"
	"testing"

	"claude-go/cmd"
)

func TestRendererCreation(t *testing.T) {
	t.Parallel()

	r := cmd.NewRenderer(80)
	if r == nil {
		t.Fatal("NewRenderer returned nil")
	}
}

func TestRenderMarkdown(t *testing.T) {
	t.Parallel()

	r := cmd.NewRenderer(80)

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "plain text",
			input:    "Hello, world!",
			contains: "Hello, world!",
		},
		{
			name:     "bold",
			input:    "**bold text**",
			contains: "bold text",
		},
		{
			name:     "italic",
			input:    "*italic text*",
			contains: "italic text",
		},
		{
			name:     "code",
			input:    "`code`",
			contains: "code",
		},
		{
			name:     "header",
			input:    "# Header",
			contains: "Header",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered := r.RenderMarkdown(tc.input)
			if !strings.Contains(rendered, tc.contains) {
				t.Errorf("RenderMarkdown(%q) = %q, should contain %q", tc.input, rendered, tc.contains)
			}
		})
	}
}

func TestRenderStreamingChunk(t *testing.T) {
	t.Parallel()

	r := cmd.NewRenderer(80)

	// Streaming chunks should pass through unchanged
	chunk := "Hello"
	result := r.RenderStreamingChunk(chunk, true, false)
	if result != chunk {
		t.Errorf("RenderStreamingChunk should pass through unchanged, got %q", result)
	}
}

func TestStripANSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no ANSI",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "with ANSI",
			input:    "\x1b[31mred\x1b[0m",
			expected: "red",
		},
		{
			name:     "multiple ANSI",
			input:    "\x1b[1m\x1b[32mbold green\x1b[0m",
			expected: "bold green",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := cmd.StripANSI(tc.input)
			if result != tc.expected {
				t.Errorf("StripANSI(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
