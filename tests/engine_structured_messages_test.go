package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/tool"
	"claude-go/internal/types"
)

func TestEngineSubmitProducesStructuredStopHookSummary(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: "done"},
		},
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider: provider,
		Tools:    tool.EmptyRegistry(),
		Hooks: hookRunnerFunc(func(_ context.Context, event engine.HookEvent) ([]engine.HookExecution, error) {
			if event.Name != string(types.HookEventStop) {
				return nil, nil
			}
			return []engine.HookExecution{
				{
					Event:      event.Name,
					Hook:       "Stop",
					Command:    "echo ok",
					Result:     "ok",
					Output:     "hook output",
					DurationMs: 120,
				},
				{
					Event:      event.Name,
					Hook:       "Stop",
					Command:    "echo fail",
					Result:     "error",
					Error:      "hook failed",
					DurationMs: 300,
				},
			}, nil
		}),
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if _, err := eng.Submit(context.Background(), "hello"); err != nil {
		t.Fatalf("submit: %v", err)
	}

	var summaryPayload map[string]any
	seenHookAttachment := false
	for _, msg := range eng.Messages() {
		if msg.Type == types.MessageTypeAttachment && strings.Contains(msg.Content, `"hookEvent":"Stop"`) {
			seenHookAttachment = true
		}
		if msg.Role != types.RoleSystem {
			continue
		}
		if !strings.Contains(msg.Content, `"subtype":"stop_hook_summary"`) {
			continue
		}
		if err := json.Unmarshal([]byte(msg.Content), &summaryPayload); err != nil {
			t.Fatalf("unmarshal summary payload: %v", err)
		}
	}

	if len(summaryPayload) == 0 {
		t.Fatal("expected structured stop_hook_summary system message")
	}
	if got, ok := summaryPayload["hookCount"].(float64); !ok || int(got) != 2 {
		t.Fatalf("expected hookCount=2, got %#v", summaryPayload["hookCount"])
	}
	if got, ok := summaryPayload["totalDurationMs"].(float64); !ok || int(got) != 420 {
		t.Fatalf("expected totalDurationMs=420, got %#v", summaryPayload["totalDurationMs"])
	}
	if errors, ok := summaryPayload["hookErrors"].([]any); !ok || len(errors) != 1 {
		t.Fatalf("expected one hook error entry, got %#v", summaryPayload["hookErrors"])
	}
	if !seenHookAttachment {
		t.Fatal("expected structured hook attachment messages for stop hooks")
	}
}

func TestEngineSubmitProducesRelevantMemoriesAttachment(t *testing.T) {
	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	projectRoot := t.TempDir()
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	memoryDir := filepath.Join(projectRoot, ".claude", "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}

	memoryPath := filepath.Join(memoryDir, "migration-notes.md")
	memoryContent := `---
name: migration notes
description: tips for migration flow
type: project
---

Prefer structured summary payloads for hook UI.`
	if err := os.WriteFile(memoryPath, []byte(memoryContent), 0o644); err != nil {
		t.Fatalf("write memory file: %v", err)
	}

	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: "done"},
		},
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider: provider,
		Tools:    tool.EmptyRegistry(),
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if _, err := eng.Submit(context.Background(), "check migration UI behavior"); err != nil {
		t.Fatalf("submit: %v", err)
	}

	found := false
	for _, msg := range eng.Messages() {
		if msg.Type != types.MessageTypeAttachment {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
			continue
		}
		if payload["type"] != "relevant_memories" {
			continue
		}
		memories, ok := payload["memories"].([]any)
		if !ok || len(memories) == 0 {
			t.Fatalf("expected non-empty memories payload, got %#v", payload["memories"])
		}
		first, _ := memories[0].(map[string]any)
		path, _ := first["path"].(string)
		if !strings.Contains(path, "migration-notes.md") {
			t.Fatalf("expected memory path in payload, got %q", path)
		}
		found = true
		break
	}

	if !found {
		t.Fatal("expected structured relevant_memories attachment")
	}
}
