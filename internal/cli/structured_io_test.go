package cli

import "testing"

func TestAppendUTF8ByteBuildsChineseRune(t *testing.T) {
	t.Parallel()

	current := ""
	pending := []byte{}
	for _, b := range []byte("你") {
		current, pending = appendUTF8Byte(current, pending, b)
	}

	if current != "你" {
		t.Fatalf("current = %q, want %q", current, "你")
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %v, want empty", pending)
	}
}

func TestDropLastRuneRemovesWholeChineseRune(t *testing.T) {
	t.Parallel()

	if got := dropLastRune("你好a"); got != "你好" {
		t.Fatalf("dropLastRune(%q) = %q, want %q", "你好a", got, "你好")
	}
	if got := dropLastRune("你好"); got != "你" {
		t.Fatalf("dropLastRune(%q) = %q, want %q", "你好", got, "你")
	}
}

func TestRenderRewritePrefixTracksPreviousFrameHeight(t *testing.T) {
	t.Parallel()

	if got, want := renderedLineCount("first\nsecond"), 2; got != want {
		t.Fatalf("renderedLineCount = %d, want %d", got, want)
	}
	if got, want := renderRewritePrefix(2), "\r\033[1A\033[J"; got != want {
		t.Fatalf("renderRewritePrefix = %q, want %q", got, want)
	}
}
