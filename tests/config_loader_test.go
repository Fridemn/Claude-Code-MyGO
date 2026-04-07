package tests

import (
	"os"
	"path/filepath"
	"testing"

	"claude-code-go/internal/config"
)

func TestConfigLoad_FromEnvFile(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	content := "" +
		"CLAUDE_CODE_API_KEY=test-key\n" +
		"CLAUDE_CODE_BASE_URL=https://example.com/v1/chat/completions\n" +
		"CLAUDE_CODE_MODEL=test-model\n" +
		"CLAUDE_CODE_MAX_TURNS=9\n" +
		"CLAUDE_CODE_SESSION_DIR=.sessions\n" +
		"CLAUDE_CODE_MCP_CONFIG=.mcp.json\n" +
		"CLAUDE_CODE_PLUGINS_CONFIG=.plugins.json\n" +
		"CLAUDE_CODE_HOOKS_CONFIG=.hooks.json\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	unsetClaudeEnv(t)
	cfg, err := config.Load(envPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.APIKey != "test-key" || cfg.Model != "test-model" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.MaxTurns != 9 {
		t.Fatalf("unexpected max_turns: %d", cfg.MaxTurns)
	}
	if cfg.SessionDir != ".sessions" {
		t.Fatalf("unexpected session dir: %s", cfg.SessionDir)
	}
	if cfg.MCPConfigPath != ".mcp.json" || cfg.PluginsConfigPath != ".plugins.json" || cfg.HooksConfigPath != ".hooks.json" {
		t.Fatalf("unexpected config paths: %#v", cfg)
	}
}

func TestConfigLoad_ExistingEnvWinsOverFile(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("CLAUDE_CODE_API_KEY=file-key\nCLAUDE_CODE_BASE_URL=https://file.url/v1/chat/completions\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	unsetClaudeEnv(t)
	t.Setenv("CLAUDE_CODE_API_KEY", "env-key")
	t.Setenv("CLAUDE_CODE_BASE_URL", "https://env.url/v1/chat/completions")
	cfg, err := config.Load(envPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("expected env key to win, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://env.url/v1/chat/completions" {
		t.Fatalf("expected env base url to win, got %q", cfg.BaseURL)
	}
}

func TestConfigLoad_InvalidMaxTurns(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("CLAUDE_CODE_API_KEY=test-key\nCLAUDE_CODE_BASE_URL=https://test.url/v1/chat/completions\nCLAUDE_CODE_MAX_TURNS=0\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	unsetClaudeEnv(t)
	_, err := config.Load(envPath)
	if err == nil {
		t.Fatalf("expected invalid max turns error")
	}
}

func unsetClaudeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CLAUDE_CODE_API_KEY",
		"CLAUDE_CODE_BASE_URL",
		"CLAUDE_CODE_MODEL",
		"CLAUDE_CODE_APP_NAME",
		"CLAUDE_CODE_MAX_TURNS",
		"CLAUDE_CODE_SESSION_DIR",
		"CLAUDE_CODE_SYSTEM_PROMPT",
		"CLAUDE_CODE_MCP_CONFIG",
		"CLAUDE_CODE_PLUGINS_CONFIG",
		"CLAUDE_CODE_HOOKS_CONFIG",
	} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}
}
