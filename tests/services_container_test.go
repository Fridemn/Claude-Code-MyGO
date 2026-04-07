package tests

import (
	"context"
	"path/filepath"
	"testing"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/config"
	"claude-code-go/internal/services"
)

func TestServicesContainerBootstrap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		APIKey:            "test-key",
		BaseURL:           "https://example.com/v1/chat/completions",
		Model:             "test-model",
		AppName:           "Claude-Code-Go",
		MaxTurns:          8,
		SessionDir:        filepath.Join(root, "sessions"),
		MCPConfigPath:     filepath.Join(root, "mcp.json"),
		PluginsConfigPath: filepath.Join(root, "plugins.json"),
		HooksConfigPath:   filepath.Join(root, "hooks.json"),
	}

	state, err := bootstrap.CreateStore(cfg)
	if err != nil {
		t.Fatalf("new bootstrap store: %v", err)
	}

	container, err := services.Create(context.Background(), cfg, state)
	if err != nil {
		t.Fatalf("new services container: %v", err)
	}

	if container.Engine() == nil || container.Commands() == nil || container.Agents() == nil {
		t.Fatalf("expected core services to be initialized")
	}
	if container.Tools() == nil || container.Sessions() == nil || container.Provider() == nil {
		t.Fatalf("expected runtime services to be initialized")
	}
	if container.MCP() == nil || container.Plugins() == nil || container.Hooks() == nil {
		t.Fatalf("expected integration services to be initialized")
	}

	snapshot := state.Snapshot()
	if snapshot.CurrentModel != cfg.Model {
		t.Fatalf("expected current model %q, got %q", cfg.Model, snapshot.CurrentModel)
	}
	if snapshot.SessionID == "" {
		t.Fatalf("expected session id to be set during container bootstrap")
	}
	if container.Config().SystemPrompt == "" {
		t.Fatalf("expected system prompt to be initialized")
	}
	for _, toolName := range []string{
		"list_files",
		"Read",
		"Grep",
		"Bash",
		"TaskList",
		"WebSearch",
	} {
		if _, ok := container.Tools().Get(toolName); !ok {
			t.Fatalf("expected builtin tool %q to be registered", toolName)
		}
	}

	if len(container.MCP().Servers()) == 0 {
		t.Fatalf("expected default MCP servers")
	}
	if len(container.Plugins().List()) == 0 {
		t.Fatalf("expected default plugins")
	}
	if len(container.Hooks().List()) == 0 {
		t.Fatalf("expected default hooks")
	}
}
