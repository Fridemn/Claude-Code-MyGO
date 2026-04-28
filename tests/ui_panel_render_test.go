package tests

import (
	"fmt"
	"strings"
	"testing"

	"claude-go/internal/bootstrap"
	"claude-go/internal/components"
	"claude-go/internal/config"
)

func TestChatAppRender_PanelEntryContainsSectionsAndActions(t *testing.T) {
	t.Parallel()

	app := components.ChatAppFor()
	props := components.ChatProps{
		Version: "test-version",
		Config: config.Config{
			AppName: "Claude-Go",
			Model:   "test-model",
			BaseURL: "https://example.com/v1/chat/completions",
		},
		State: bootstrap.State{
			SessionID: "session-1",
			TurnCount: 2,
		},
		Height: 72,
		Entries: []components.TranscriptEntry{
			{
				Kind:  "panel",
				Title: "Panel /mcp",
				Content: strings.Join([]string{
					"overview:",
					"status=connected",
					"enabled=true",
					"",
					"entries:",
					"- local-workspace [sdk] connected enabled=true",
					"  description line",
					"",
					"actions:",
					"- /mcp-connect local-workspace",
				}, "\n"),
			},
		},
	}

	topScreen := app.Render(props)

	for _, want := range []string{
		"Panel /mcp",
		"overview:",
		"status=",
		"connected",
	} {
		if !strings.Contains(topScreen, want) {
			t.Fatalf("expected top-of-panel render to contain %q, got:\n%s", want, topScreen)
		}
	}

	props.TranscriptScroll = 9
	scrolledScreen := app.Render(props)

	for _, want := range []string{
		"entries:",
		"local-workspace [sdk] connected enabled=true",
		"actions:",
		"/mcp-connect local-workspace",
	} {
		if !strings.Contains(scrolledScreen, want) {
			t.Fatalf("expected scrolled panel render to contain %q, got:\n%s", want, scrolledScreen)
		}
	}
}

func TestChatAppRender_TranscriptShowsHeaderSummaryAndScrollHint(t *testing.T) {
	t.Parallel()

	app := components.ChatAppFor()
	entries := make([]components.TranscriptEntry, 0, 22)
	for i := 0; i < 22; i++ {
		kind := "assistant"
		if i%2 == 0 {
			kind = "user"
		}
		entries = append(entries, components.TranscriptEntry{
			Kind:    kind,
			Title:   "entry",
			Content: "message content",
		})
	}

	screen := app.Render(components.ChatProps{
		Version: "test-version",
		Config: config.Config{
			AppName: "Claude-Go",
			Model:   "test-model",
			BaseURL: "https://example.com/v1/chat/completions",
		},
		State: bootstrap.State{
			SessionID: "session-1",
			TurnCount: 22,
		},
		Height:           24,
		Entries:          entries,
		TranscriptScroll: 1,
	})

	for _, want := range []string{
		"session session-1",
		"turn 22",
		"example.com/v1/ch", // URL is truncated in display
		"Wheel/PgUp/PgDn/↑/↓ scroll",
		"lines above",
	} {
		if !strings.Contains(screen, want) {
			t.Fatalf("expected rendered screen to contain %q, got:\n%s", want, screen)
		}
	}
}

func TestChatAppRender_TranscriptScrollFromBottom_WithSingleLargeBlock(t *testing.T) {
	t.Parallel()

	app := components.ChatAppFor()
	lines := make([]string, 0, 80)
	for i := 1; i <= 80; i++ {
		lines = append(lines, fmt.Sprintf("line %02d", i))
	}
	content := strings.Join(lines, "\n")

	render := func(scroll int) string {
		return app.Render(components.ChatProps{
			Version: "test-version",
			Config: config.Config{
				AppName: "Claude-Go",
				Model:   "test-model",
				BaseURL: "https://example.com/v1/chat/completions",
			},
			State: bootstrap.State{
				SessionID: "session-1",
				TurnCount: 1,
			},
			Height: 22,
			Entries: []components.TranscriptEntry{
				{
					Kind:    "assistant",
					Title:   "Claude",
					Content: content,
				},
			},
			TranscriptScroll: scroll,
		})
	}

	bottom := render(0)
	up := render(60)

	if strings.Contains(bottom, "line 01") {
		t.Fatalf("expected bottom render not to include earliest line before scrolling up, got:\n%s", bottom)
	}

	if !strings.Contains(up, "line 01") {
		t.Fatalf("expected scrolled render to include early lines after scrolling up, got:\n%s", up)
	}
}
