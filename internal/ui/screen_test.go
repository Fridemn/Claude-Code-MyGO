package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"claude-go/internal/bootstrap"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestWrapInputForDisplayBreaksLongInputAcrossLines(t *testing.T) {
	t.Parallel()

	got := wrapInputForDisplay("这是一个很长的中文输入内容", 6, 6)
	if len(got) < 2 {
		t.Fatalf("expected wrapped input to span multiple lines, got %#v", got)
	}
}

func TestRenderInputPanelShowsSpinnerWhenBusy(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(72, bootstrap.State{}, "", 0, nil, -1, true, 0, "", "Waiting for model response", time.Now().Add(-3*time.Second), "Working", 0, 0, nil, "", 0, "", "")
	if !strings.Contains(panel, "Ctrl+C to stop") {
		t.Fatalf("expected busy control hint, got:\n%s", panel)
	}
	if !strings.Contains(panel, "Waiting for model response") {
		t.Fatalf("expected status row when busy, got:\n%s", panel)
	}
	if !strings.Contains(panel, "Working") {
		t.Fatalf("expected spinner label when busy, got:\n%s", panel)
	}
	if !strings.Contains(panel, "esc to interrupt") {
		t.Fatalf("expected busy footer interrupt hint, got:\n%s", panel)
	}
}

func TestRenderInputPanelShowsTeammateTreeWhenBusy(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(80, bootstrap.State{}, "", 0, nil, -1, true, 0, "", "Waiting for model response", time.Now().Add(-2*time.Second), "Working", 100, 0, []TeammateSpinnerNode{
		{
			Name:         "researcher",
			Activity:     "searching docs",
			ToolUseCount: 2,
			TokenCount:   350,
		},
	}, "analyzing", 100, "", "")
	if !strings.Contains(panel, "team-lead") {
		t.Fatalf("expected teammate tree leader row, got:\n%s", panel)
	}
	if !strings.Contains(panel, "@researcher") {
		t.Fatalf("expected teammate row, got:\n%s", panel)
	}
}

func TestRenderInputPanelShowsPermissionModeFooter(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(100, bootstrap.State{}, "", 0, nil, -1, false, 0, "", "", time.Time{}, "", 0, 0, nil, "", 0, "acceptEdits", "")
	plain := ansiRegexp.ReplaceAllString(panel, "")

	if !strings.Contains(plain, "accept edits on") {
		t.Fatalf("expected permission mode footer text, got:\n%s", plain)
	}
	if !strings.Contains(plain, "shift+tab to cycle") {
		t.Fatalf("expected mode cycle hint, got:\n%s", plain)
	}
}

func TestRenderInputPanelShowsAutoCompactHintOnRight(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(100, bootstrap.State{}, "", 0, nil, -1, false, 0, "", "", time.Time{}, "", 0, 0, nil, "", 0, "acceptEdits", "5% until auto-compact")
	plain := ansiRegexp.ReplaceAllString(panel, "")

	if !strings.Contains(plain, "5% until auto-compact") {
		t.Fatalf("expected auto-compact right hint, got:\n%s", plain)
	}
}

func TestRenderScreenScrollsWholePage(t *testing.T) {
	t.Parallel()

	entries := make([]TranscriptEntry, 0, 60)
	for i := 1; i <= 60; i++ {
		entries = append(entries, TranscriptEntry{
			Kind:    "assistant",
			Title:   "Claude",
			Content: fmt.Sprintf("entry %02d", i),
		})
	}

	bottom := RenderScreen(ScreenState{
		Version: "v-test",
		Width:   100,
		Height:  24,
		State: bootstrap.State{
			SessionID: "session-1",
			TurnCount: 1,
		},
		CurrentInput: "",
		Entries:      entries,
	})

	if strings.Contains(bottom, "Welcome back!") {
		t.Fatalf("expected bottom viewport not to include header when content overflows, got:\n%s", bottom)
	}

	top := RenderScreen(ScreenState{
		Version: "v-test",
		Width:   100,
		Height:  24,
		State: bootstrap.State{
			SessionID: "session-1",
			TurnCount: 1,
		},
		CurrentInput:     "",
		Entries:          entries,
		TranscriptScroll: 1000,
	})

	if !strings.Contains(top, "Welcome back!") {
		t.Fatalf("expected scrolled viewport to include header, got:\n%s", top)
	}
}

func TestRenderTranscriptCoreMessageKindsUseInlineIndicators(t *testing.T) {
	t.Parallel()

	out := RenderTranscript(100, 0, []TranscriptEntry{
		{Kind: "assistant", Title: "Claude", Content: "让我继续移动测试文件。"},
		{Kind: "user", Title: "You", Content: "这是用户消息"},
		{Kind: "system", Title: "System", Content: "这是系统消息"},
	}, ViewModeNormal, 0, "", "")

	plain := ansiRegexp.ReplaceAllString(out, "")

	for _, disallowed := range []string{"Claude\n", "System\n", "Claude\r\n", "System\r\n", "这是系统消息"} {
		if strings.Contains(plain, disallowed) {
			t.Fatalf("expected hidden system message and no standalone role labels, got:\n%s", plain)
		}
	}

	for _, want := range []string{
		"● 让我继续移动测试文件。",
		"⏵ 这是用户消息",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("expected inline role indicator %q, got:\n%s", want, plain)
		}
	}

	if !strings.Contains(out, "\x1b[38;2;255;255;255m● ") {
		t.Fatalf("expected assistant dot to use white color, got:\n%s", out)
	}
	if !strings.Contains(out, "\x1b[48;2;55;55;55m") {
		t.Fatalf("expected user transcript message to include gray background, got:\n%s", out)
	}
}
