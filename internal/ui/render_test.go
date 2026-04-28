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

func TestRenderMarkdownWithGlamourRemovesLiteralHashesFromNestedHeadings(t *testing.T) {
	t.Parallel()

	lines := renderMarkdownWithGlamour("## 二级标题\n\n### 3. 技术架构调整\n\n#### 四级标题", 80)
	if len(lines) == 0 {
		t.Fatalf("expected nested headings to render")
	}

	joined := ansiRE.ReplaceAllString(strings.Join(lines, "\n"), "")
	for _, unexpected := range []string{"## 二级标题", "### 3. 技术架构调整", "#### 四级标题"} {
		if strings.Contains(joined, unexpected) {
			t.Fatalf("expected rendered heading to omit literal markdown markers %q, got %q", unexpected, joined)
		}
	}
	for _, expected := range []string{"二级标题", "3. 技术架构调整", "四级标题"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected rendered heading text %q, got %q", expected, joined)
		}
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

func TestRenderMarkdownWithGlamourTableHasCompactSpacing(t *testing.T) {
	t.Parallel()

	md := `### 表格

| 功能 | 状态 | 备注 |
| --- | --- | --- |
| 基础格式 | ✅ | 已完成 |
| 代码高亮 | ✅ | 已完成 |
| 表格支持 | ✅ | 已完成 |`

	lines := renderMarkdownWithGlamour(md, 120)
	plain := strings.Trim(ansiRE.ReplaceAllString(strings.Join(lines, "\n"), ""), "\n")
	if plain == "" {
		t.Fatalf("expected non-empty table render")
	}

	maxBlankRun := 0
	currentBlankRun := 0
	for _, line := range strings.Split(plain, "\n") {
		if strings.TrimSpace(line) == "" {
			currentBlankRun++
			if currentBlankRun > maxBlankRun {
				maxBlankRun = currentBlankRun
			}
			continue
		}
		currentBlankRun = 0
	}

	// Keep at most one blank separator line to avoid visually "stretched" rows.
	if maxBlankRun > 1 {
		t.Fatalf("expected compact markdown spacing, max blank run=%d, output:\n%s", maxBlankRun, plain)
	}
}
