package tests

import (
	"errors"
	"strings"
	"testing"
	"time"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/components"
	"claude-code-go/internal/config"
	"claude-code-go/internal/prompt"
	"claude-code-go/internal/tool"
)

func TestPromptSystemAndWithTools(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	defaultPrompt := strings.TrimSpace(prompt.System(cfg))
	if !strings.Contains(defaultPrompt, "Claude-Code-Go") {
		t.Fatalf("unexpected default prompt: %q", defaultPrompt)
	}
	if !strings.Contains(defaultPrompt, "Primary working directory:") {
		t.Fatalf("expected cwd context in default prompt: %q", defaultPrompt)
	}
	if !strings.Contains(defaultPrompt, "Use '.' for the current repository root") {
		t.Fatalf("expected sibling repository guidance in default prompt: %q", defaultPrompt)
	}

	custom := strings.TrimSpace(prompt.System(config.Config{SystemPrompt: "  custom system  "}))
	if custom != "custom system" {
		t.Fatalf("expected trimmed custom prompt, got %q", custom)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	withTools := prompt.WithTools("base prompt", registry.List())
	if !strings.Contains(withTools, "base prompt") {
		t.Fatalf("expected base prompt to be preserved: %q", withTools)
	}
	if !strings.Contains(withTools, "Available tools:") || !strings.Contains(withTools, "echo_tool") {
		t.Fatalf("expected tool fragment to be appended: %q", withTools)
	}
}

func TestBootstrapStoreLifecycle(t *testing.T) {
	t.Parallel()

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	store.SetSessionID("session-1")
	store.SetCurrentModel("model-2")
	store.RecordTurn("model-2", 25*time.Millisecond)
	store.RecordToolCall(5 * time.Millisecond)
	store.RecordError(errors.New("boom"))

	snapshot := store.Snapshot()
	if snapshot.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %q", snapshot.SessionID)
	}
	if snapshot.CurrentModel != "model-2" || snapshot.MainLoopModel != "model-2" {
		t.Fatalf("unexpected model state: %#v", snapshot)
	}
	if snapshot.TurnCount != 1 || snapshot.ToolCallCount != 1 {
		t.Fatalf("unexpected counters: %#v", snapshot)
	}
	if snapshot.TotalAPIDuration != 25*time.Millisecond || snapshot.TotalToolDuration != 5*time.Millisecond {
		t.Fatalf("unexpected durations: %#v", snapshot)
	}
	if snapshot.ModelUsage["model-2"] != 1 {
		t.Fatalf("unexpected model usage: %#v", snapshot.ModelUsage)
	}
	if snapshot.LastError != "boom" || len(snapshot.InMemoryErrorLog) != 1 {
		t.Fatalf("unexpected error log state: %#v", snapshot)
	}
}

func TestComponentsChatRenderAndPromptLabel(t *testing.T) {
	t.Parallel()

	app := components.ChatAppFor()
	rendered := app.Render(components.ChatProps{
		Version: "test-version",
		Config: config.Config{
			AppName: "Claude-Code-Go",
			Model:   "test-model",
			BaseURL: "https://example.com/v1/chat/completions",
		},
		State: bootstrap.State{
			SessionID: "session-1",
			TurnCount: 3,
		},
		Entries: []components.TranscriptEntry{
			{Kind: "user", Title: "", Content: "hello"},
			{Kind: "assistant", Title: "Claude", Content: "world"},
		},
	})

	for _, want := range []string{
		"Claude-Code-Go",
		"test-version",
		"session-1",
		"test-model",
		"hello",
		"world",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered screen to contain %q", want)
		}
	}

	label := app.PromptLabel()
	if !strings.Contains(label, "⏵") {
		t.Fatalf("unexpected prompt label: %q", label)
	}
}
