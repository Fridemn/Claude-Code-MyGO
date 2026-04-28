package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	styles "github.com/charmbracelet/glamour/styles"
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
		glamour.WithStyles(glamourStyleConfig()),
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

func glamourStyleConfig() ansi.StyleConfig {
	style := styles.DarkStyleConfig
	zero := uint(0)

	// Tighten document/paragraph spacing to match TS CLI compact transcript style.
	// Glamour's dark preset adds block prefixes/suffixes that create visible blank
	// lines between markdown paragraphs and table rows in terminal rendering.
	style.Document.BlockPrefix = ""
	style.Document.BlockSuffix = ""
	style.Document.Margin = &zero
	style.Paragraph.BlockPrefix = ""
	style.Paragraph.BlockSuffix = ""
	style.Paragraph.Margin = &zero
	style.Table.BlockPrefix = ""
	style.Table.BlockSuffix = ""
	style.Table.Margin = &zero

	style.H2.Prefix = ""
	style.H3.Prefix = ""
	style.H4.Prefix = ""
	style.H5.Prefix = ""
	style.H6.Prefix = ""
	return style
}
