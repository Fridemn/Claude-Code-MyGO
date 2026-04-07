package tests

import (
	"claude-code-go/internal/tool/file"
	"claude-code-go/internal/tool/search"

	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/command"
	cmddev "claude-code-go/internal/command/dev"
	cmdfiles "claude-code-go/internal/command/files"
	cmdmemory "claude-code-go/internal/command/memory"
	cmdsession "claude-code-go/internal/command/session"
	cmdstats "claude-code-go/internal/command/stats"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/session"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

func TestFileAndSessionCommands(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "demo.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nTODO test\n"), 0o644); err != nil {
		t.Fatalf("write demo file: %v", err)
	}

	registry := command.EmptyRegistry()
	cmdfiles.Register(registry)
	cmdsession.Register(registry)
	cmdmemory.Register(registry)
	cmddev.Register(registry)

	toolsRegistry := tool.EmptyRegistry()
	file.RegisterFileTools(toolsRegistry)
	search.RegisterSearchTools(toolsRegistry)
	toolsRegistry.Register(fakeExecCommandTool{})

	eng := createTestEngineWithMessages(t, []types.Message{
		{Role: types.RoleSystem, Content: "system prompt"},
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Content: "world"},
	})

	runtime := command.Runtime{
		Engine: eng,
		Tools:  toolsRegistry,
		Config: config.Config{
			AppName:    "Claude-Code-Go",
			Model:      "test-model",
			BaseURL:    "https://example.com/v1/chat/completions",
			MaxTurns:   8,
			SessionDir: root,
		},
	}

	checkCommandContains(t, registry, runtime, "/files "+root, "demo.txt")
	checkCommandContains(t, registry, runtime, "/read "+filePath+" 1 2", "line2")
	checkCommandContains(t, registry, runtime, "/grep TODO "+root, "demo.txt")
	checkCommandContains(t, registry, runtime, "/history", "ASSISTANT")
	checkCommandContains(t, registry, runtime, "/history", "world")
	checkCommandContains(t, registry, runtime, "/prompt", "system prompt")
	checkCommandContains(t, registry, runtime, "/config", "app=Claude-Code-Go")
	checkCommandContains(t, registry, runtime, "/memory", "recent_memory:")
}

func TestStatsAndDevCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdstats.Register(registry)
	cmddev.Register(registry)
	cmdfiles.Register(registry)

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.SetSessionID("session-1")
	store.RecordTurn("test-model", 10*time.Millisecond)
	store.RecordToolCall(5 * time.Millisecond)
	store.RecordError(context.Canceled)

	toolsRegistry := tool.EmptyRegistry()
	file.RegisterFileTools(toolsRegistry)
	toolsRegistry.Register(fakeExecCommandTool{})

	runtime := command.Runtime{
		State: store,
		Tools: toolsRegistry,
		Config: config.Config{
			AppName:    "Claude-Code-Go",
			Model:      "test-model",
			BaseURL:    "https://example.com/v1/chat/completions",
			SessionDir: ".claude-code-go/sessions",
		},
	}

	checkCommandContains(t, registry, runtime, "/usage", "models:")
	checkCommandContains(t, registry, runtime, "/stats", "tool_calls=1")
	checkCommandContains(t, registry, runtime, "/cost", "total_cost_usd=")
	checkCommandContains(t, registry, runtime, "/tools", "list_files")
	checkCommandContains(t, registry, runtime, "/tool list_files", "name=list_files")
	checkCommandContains(t, registry, runtime, "/model", "model=test-model")
	checkCommandContains(t, registry, runtime, "/doctor", "git=true")
	checkCommandContains(t, registry, runtime, "/diff .", "diff --git")
}

func checkCommandContains(t *testing.T, registry *command.Registry, runtime command.Runtime, line string, want string) {
	t.Helper()
	out, ok, err := registry.Execute(context.Background(), line, runtime)
	if err != nil || !ok {
		t.Fatalf("command %q failed: ok=%t err=%v", line, ok, err)
	}
	if !strings.Contains(out.Value, want) {
		t.Fatalf("command %q output missing %q:\n%s", line, want, out.Value)
	}
}

func createTestEngineWithMessages(t *testing.T, messages []types.Message) *engine.Engine {
	t.Helper()
	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider:        &scriptedProvider{},
		Tools:           tool.EmptyRegistry(),
		Sessions:        sessions,
		InitialMessages: messages,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return eng
}

type fakeExecCommandTool struct{}

func (fakeExecCommandTool) Name() string              { return "exec_command" }
func (fakeExecCommandTool) Description() string       { return "fake exec tool for command tests" }
func (fakeExecCommandTool) IsReadOnly(tool.Input) bool { return true }
func (fakeExecCommandTool) Call(_ context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	commandStr, _ := in["command"].(string)
	switch {
	case strings.Contains(commandStr, "git rev-parse --is-inside-work-tree"):
		return tool.Result{Content: "true"}, nil
	case strings.Contains(commandStr, "git diff --stat"):
		return tool.Result{Content: "diff --git a/demo b/demo\n+added line"}, nil
	default:
		return tool.Result{Content: "ok"}, nil
	}
}
