package tests

import (
	"claude-go/internal/tool/file"
	"claude-go/internal/tool/search"

	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claude-go/internal/bootstrap"
	"claude-go/internal/command"
	cmddev "claude-go/internal/command/dev"
	cmdfiles "claude-go/internal/command/files"
	cmdmemory "claude-go/internal/command/memory"
	cmdsession "claude-go/internal/command/session"
	cmdstats "claude-go/internal/command/stats"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/tool"
	"claude-go/internal/types"
	tea "github.com/charmbracelet/bubbletea"
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
			AppName:    "Claude-Go",
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
	checkCommandContains(t, registry, runtime, "/config", "app=Claude-Go")
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
			AppName:    "Claude-Go",
			Model:      "test-model",
			BaseURL:    "https://example.com/v1/chat/completions",
			SessionDir: ".claude-go/sessions",
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

func TestUsageCommandLoadModel(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdstats.Register(registry)

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.RecordTurn("test-model", 5*time.Millisecond)
	store.RecordTurn("test-model", 5*time.Millisecond)

	closed := false
	model, _, handled, err := registry.LoadModel(context.Background(), "/usage", command.Runtime{
		State: store,
		OnExit: func() {
			closed = true
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /usage load model handled, handled=%t model=%T", handled, model)
	}
	if !strings.Contains(model.View(), "Usage") {
		t.Fatalf("expected usage model view, got %q", model.View())
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from usage model")
	}
	if !closed {
		t.Fatal("expected usage model to trigger OnExit")
	}
}

func TestToolsCommandLoadModel(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdstats.Register(registry)

	toolsRegistry := tool.EmptyRegistry()
	file.RegisterFileTools(toolsRegistry)

	closed := false
	model, _, handled, err := registry.LoadModel(context.Background(), "/tools", command.Runtime{
		Tools: toolsRegistry,
		OnExit: func() {
			closed = true
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /tools load model handled, handled=%t model=%T", handled, model)
	}
	if !strings.Contains(model.View(), "list_files") {
		t.Fatalf("expected tools model to include list_files, got %q", model.View())
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from tools model")
	}
	if !closed {
		t.Fatal("expected tools model to trigger OnExit")
	}
}

func TestConfigCommandLoadModelViaSettingsAlias(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdsession.Register(registry)

	eng := createTestEngineWithMessages(t, []types.Message{
		{Role: types.RoleSystem, Content: "system prompt"},
		{Role: types.RoleUser, Content: "hello"},
	})

	closed := false
	model, _, handled, err := registry.LoadModel(context.Background(), "/settings", command.Runtime{
		Engine: eng,
		Config: config.Config{
			AppName:    "Claude-Go",
			Model:      "test-model",
			BaseURL:    "https://example.com/v1/chat/completions",
			MaxTurns:   8,
			SessionDir: ".claude-go/sessions",
		},
		OnExit: func() {
			closed = true
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /settings load model handled, handled=%t model=%T", handled, model)
	}
	if !strings.Contains(model.View(), "Settings") || !strings.Contains(model.View(), "app=Claude-Go") {
		t.Fatalf("unexpected config model view: %q", model.View())
	}

	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from config model")
	}
	if !closed {
		t.Fatal("expected config model to trigger OnExit")
	}
}

func TestMemoryCommandLoadModelCancelUsesOnDone(t *testing.T) {
	registry := command.EmptyRegistry()
	cmdmemory.Register(registry)

	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	store.SetCWD(root)

	eng := createTestEngineWithMessages(t, []types.Message{
		{Role: types.RoleSystem, Content: "system prompt"},
		{Role: types.RoleUser, Content: "hello"},
		{Role: types.RoleAssistant, Content: "world"},
	})

	var doneResult string
	var doneDisplay string
	model, _, handled, err := registry.LoadModel(context.Background(), "/memory", command.Runtime{
		Engine: eng,
		State:  store,
		OnLocalJSXDone: func(result string, options command.LocalJSXDoneOptions) {
			doneResult = result
			doneDisplay = options.Display
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /memory load model handled, handled=%t model=%T", handled, model)
	}
	if !strings.Contains(model.View(), "Memory") {
		t.Fatalf("expected memory model view, got %q", model.View())
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from memory model")
	}
	if doneResult != "Cancelled memory editing" {
		t.Fatalf("unexpected onDone result: %q", doneResult)
	}
	if doneDisplay != "system" {
		t.Fatalf("expected system display, got %q", doneDisplay)
	}
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

func (fakeExecCommandTool) Name() string               { return "exec_command" }
func (fakeExecCommandTool) Description() string        { return "fake exec tool for command tests" }
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
