package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
)

var glamourMarkdownRE = regexp.MustCompile("(?m)(^#{1,6}\\s|^\\s*[-*+]\\s|^\\s*\\d+\\.\\s|^\\s*>\\s|^\\s*```|^\\s*\\|.*\\||\\[[^\\]]+\\]\\([^)]+\\)|\\*\\*[^*]+\\*\\*|_[^_]+_)")

func shouldUseGlamour(content string) bool {
	return glamourMarkdownRE.MatchString(content)
}

func renderMarkdownWithGlamour(content string, width int) []string {
	if width <= 0 {
		width = 80
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithWordWrap(width),
		glamour.WithStylePath("dark"),
	)
	if err != nil {
		return nil
	}
	out, err := renderer.Render(strings.TrimSpace(content))
	if err != nil {
		return nil
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return []string{""}
	}
	return strings.Split(out, "\n")
}
