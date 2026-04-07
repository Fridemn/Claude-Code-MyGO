package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"claude-code-go/internal/bootstrap"
)

func TestWrapInputForDisplayBreaksLongInputAcrossLines(t *testing.T) {
	t.Parallel()

	got := wrapInputForDisplay("这是一个很长的中文输入内容", 6, 6)
	if len(got) < 2 {
		t.Fatalf("expected wrapped input to span multiple lines, got %#v", got)
	}
}

func TestRenderInputPanelKeepsBuddyVisibleWhenInputWraps(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(72, bootstrap.State{}, "这是一个很长的中文输入内容用于测试buddy稳定显示", nil, -1, false, 0, "", "", time.Time{}, "", 0, 0, nil, "", 0)
	if !strings.Contains(panel, "Gravy") {
		t.Fatalf("expected buddy label to remain visible, got:\n%s", panel)
	}
}

func TestRenderInputPanelShowsSpinnerWhenBusy(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(72, bootstrap.State{}, "", nil, -1, true, 0, "", "Waiting for model response", time.Now().Add(-3*time.Second), "Working", 0, 0, nil, "", 0)
	if !strings.Contains(panel, "Ctrl+C to stop") {
		t.Fatalf("expected busy control hint, got:\n%s", panel)
	}
	if !strings.Contains(panel, "Waiting for model response") {
		t.Fatalf("expected status row when busy, got:\n%s", panel)
	}
	if !strings.Contains(panel, "Working") {
		t.Fatalf("expected spinner label when busy, got:\n%s", panel)
	}
}

func TestRenderInputPanelShowsTeammateTreeWhenBusy(t *testing.T) {
	t.Parallel()

	panel := RenderInputPanel(80, bootstrap.State{}, "", nil, -1, true, 0, "", "Waiting for model response", time.Now().Add(-2*time.Second), "Working", 100, 0, []TeammateSpinnerNode{
		{
			Name:         "researcher",
			Activity:     "searching docs",
			ToolUseCount: 2,
			TokenCount:   350,
		},
	}, "analyzing", 100)
	if !strings.Contains(panel, "team-lead") {
		t.Fatalf("expected teammate tree leader row, got:\n%s", panel)
	}
	if !strings.Contains(panel, "@researcher") {
		t.Fatalf("expected teammate row, got:\n%s", panel)
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
