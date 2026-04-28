package tests

import (
	"strings"
	"testing"

	"claude-go/internal/ui"
)

func TestStreamingToFinalNoDuplicationAndNoThinkTags(t *testing.T) {
	t.Parallel()

	stream := ui.TranscriptEntry{
		Kind:    "assistant_streaming",
		Title:   "Claude",
		Content: "<think>internal reasoning</think>Hello world",
		UUID:    "streaming",
	}
	final := ui.TranscriptEntry{
		Kind:    "assistant",
		Title:   "Claude",
		Content: "Hello world",
		UUID:    "final",
	}

	streamRendered := ui.RenderTranscript(100, 0, []ui.TranscriptEntry{stream}, ui.ViewModeNormal, 0, "", "")
	finalRendered := ui.RenderTranscript(100, 0, []ui.TranscriptEntry{final}, ui.ViewModeNormal, 0, "", "")
	combined := streamRendered + "\n" + finalRendered

	if strings.Contains(combined, "<think>") || strings.Contains(combined, "</think>") {
		t.Fatalf("think tags should be stripped from rendered output: %q", combined)
	}
	if strings.Count(combined, "Hello world") != 2 {
		t.Fatalf("expected one streaming + one final render, got: %q", combined)
	}
}

