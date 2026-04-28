package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadNormalizesLegacyClaudeCodeGoPaths(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	content := strings.Join([]string{
		"CLAUDE_CODE_API_KEY=test-key",
		"CLAUDE_CODE_BASE_URL=https://example.com/v1/chat/completions",
		"CLAUDE_CODE_SESSION_DIR=.claude-code-go/sessions",
		"CLAUDE_CODE_MCP_CONFIG=.claude-code-go/mcp.json",
		"CLAUDE_CODE_PLUGINS_CONFIG=.claude-code-go/plugins.json",
		"CLAUDE_CODE_HOOKS_CONFIG=.claude-code-go/hooks.json",
		"",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	unsetClaudeConfigEnv(t)
	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	assertNoLegacyPath(t, cfg.SessionDir)
	assertNoLegacyPath(t, cfg.MCPConfigPath)
	assertNoLegacyPath(t, cfg.PluginsConfigPath)
	assertNoLegacyPath(t, cfg.HooksConfigPath)
	if cfg.MCPConfigPath != filepath.Join(".claude-go", "mcp.json") {
		t.Fatalf("unexpected MCP path: %s", cfg.MCPConfigPath)
	}
	if cfg.PluginsConfigPath != filepath.Join(".claude-go", "plugins.json") {
		t.Fatalf("unexpected plugins path: %s", cfg.PluginsConfigPath)
	}
	if cfg.HooksConfigPath != filepath.Join(".claude-go", "hooks.json") {
		t.Fatalf("unexpected hooks path: %s", cfg.HooksConfigPath)
	}
}

func TestLoadAppliesActiveAPIProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &APIProfilesStore{Active: "one", Profiles: map[string]APIProfile{}}
	if err := store.Upsert(APIProfile{
		Name:         "one",
		APIKey:       "profile-key",
		BaseURL:      "https://profile.example/v1/chat/completions",
		Model:        "profile-model",
		SummaryModel: "summary-model",
	}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if err := SaveAPIProfiles(store); err != nil {
		t.Fatalf("save profiles: %v", err)
	}

	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	content := strings.Join([]string{
		"CLAUDE_CODE_API_KEY=env-key",
		"CLAUDE_CODE_BASE_URL=https://env.example/v1/chat/completions",
		"CLAUDE_CODE_MODEL=env-model",
		"",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	unsetClaudeConfigEnv(t)
	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.APIKey != "profile-key" || cfg.BaseURL != "https://profile.example/v1/chat/completions" || cfg.Model != "profile-model" || cfg.SummaryModel != "summary-model" {
		t.Fatalf("active profile was not applied: %#v", cfg)
	}
}

func TestLoadCanUseActiveAPIProfileWithoutEnvCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &APIProfilesStore{Active: "standalone", Profiles: map[string]APIProfile{}}
	if err := store.Upsert(APIProfile{
		Name:    "standalone",
		APIKey:  "profile-key",
		BaseURL: "https://profile.example/v1/chat/completions",
		Model:   "profile-model",
	}); err != nil {
		t.Fatalf("upsert profile: %v", err)
	}
	if err := SaveAPIProfiles(store); err != nil {
		t.Fatalf("save profiles: %v", err)
	}

	unsetClaudeConfigEnv(t)
	cfg, err := Load(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("load config from active profile: %v", err)
	}
	if cfg.APIKey != "profile-key" || cfg.BaseURL != "https://profile.example/v1/chat/completions" || cfg.Model != "profile-model" {
		t.Fatalf("active profile was not used as standalone config: %#v", cfg)
	}
}

func TestLoadAllowsMissingCredentialsForInteractiveSetup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	unsetClaudeConfigEnv(t)

	cfg, err := Load(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("load config without credentials: %v", err)
	}
	if cfg.APIKey != "" || cfg.BaseURL != "" {
		t.Fatalf("expected empty credentials before setup, got %#v", cfg)
	}
	if cfg.Model == "" {
		t.Fatalf("expected default model to remain available")
	}
}

func assertNoLegacyPath(t *testing.T, path string) {
	t.Helper()
	if strings.Contains(path, ".claude-code-go") || strings.Contains(path, ".Claude-Go") {
		t.Fatalf("legacy path was not normalized: %s", path)
	}
}

func unsetClaudeConfigEnv(t *testing.T) {
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
		"CLAUDE_CODE_SUMMARY_MODEL",
	} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}
}
