package ui

import (
	"strings"
	"testing"
)

func TestVisibleWidthTreatsChineseAsDoubleWidth(t *testing.T) {
	t.Parallel()

	if got := visibleWidth("你好abc"); got != 7 {
		t.Fatalf("visibleWidth(%q) = %d, want 7", "你好abc", got)
	}
}

func TestWrapTextBreaksWideCharactersWithoutSpaces(t *testing.T) {
	t.Parallel()

	got := wrapText("你好世界编程", 6)
	want := []string{"你好世", "界编程"}
	if len(got) != len(want) {
		t.Fatalf("wrapText returned %d lines, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrapText line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTruncateVisibleUsesDisplayWidth(t *testing.T) {
	t.Parallel()

	if got := truncateVisible("你好世界", 5); got != "你好…" {
		t.Fatalf("truncateVisible(%q, 5) = %q, want %q", "你好世界", got, "你好…")
	}
}

func TestRenderMarkdownWithGlamourBasic(t *testing.T) {
	t.Parallel()

	lines := renderMarkdownWithGlamour("# Title\n\n- a\n- b", 60)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty glamour render lines")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Title") {
		t.Fatalf("expected glamour output to include markdown title, got %q", joined)
	}
}

func TestShouldUseGlamourDetection(t *testing.T) {
	t.Parallel()

	if !shouldUseGlamour("# Title\ncontent") {
		t.Fatalf("expected heading markdown to enable glamour")
	}
	if !shouldUseGlamour("visit [x](https://example.com)") {
		t.Fatalf("expected markdown link to enable glamour")
	}
	if shouldUseGlamour("plain sentence without markdown tokens") {
		t.Fatalf("expected plain text to keep legacy renderer")
	}
}
