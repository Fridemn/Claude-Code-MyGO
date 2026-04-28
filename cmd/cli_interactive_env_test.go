package cmd

import (
	"os"
	"testing"
)

func TestApplyInteractiveCompatEnvBridgesLegacyAPIKey(t *testing.T) {
	t.Setenv("CLAUDE_CODE_API_KEY", "")
	t.Setenv("CLAUDE_CODE_BASE_URL", "")

	cfg := &CLIConfig{
		APIKey: "legacy-key",
	}

	restore, err := applyInteractiveCompatEnv(cfg)
	if err != nil {
		t.Fatalf("applyInteractiveCompatEnv failed: %v", err)
	}
	defer restore()

	if got := os.Getenv("CLAUDE_CODE_API_KEY"); got != "legacy-key" {
		t.Fatalf("expected CLAUDE_CODE_API_KEY to be bridged, got %q", got)
	}
}

func TestApplyInteractiveCompatEnvHonorsExplicitOverrides(t *testing.T) {
	t.Setenv("CLAUDE_CODE_API_KEY", "from-env")
	t.Setenv("CLAUDE_CODE_BASE_URL", "https://env.example/v1")
	t.Setenv("CLAUDE_CODE_MODEL", "env-model")
	t.Setenv("CLAUDE_CODE_SYSTEM_PROMPT", "env-prompt")
	t.Setenv("CLAUDE_CODE_MAX_TURNS", "22")

	cfg := &CLIConfig{
		APIKey:               "flag-key",
		BaseURL:              "https://flag.example/v1",
		Model:                "flag-model",
		SystemPrompt:         "flag-prompt",
		MaxTurns:             88,
		apiKeyExplicit:       true,
		baseURLExplicit:      true,
		modelExplicit:        true,
		systemPromptExplicit: true,
		maxTurnsExplicit:     true,
	}

	restore, err := applyInteractiveCompatEnv(cfg)
	if err != nil {
		t.Fatalf("applyInteractiveCompatEnv failed: %v", err)
	}
	defer restore()

	if got := os.Getenv("CLAUDE_CODE_API_KEY"); got != "flag-key" {
		t.Fatalf("expected explicit api key override, got %q", got)
	}
	if got := os.Getenv("CLAUDE_CODE_BASE_URL"); got != "https://flag.example/v1" {
		t.Fatalf("expected explicit base URL override, got %q", got)
	}
	if got := os.Getenv("CLAUDE_CODE_MODEL"); got != "flag-model" {
		t.Fatalf("expected explicit model override, got %q", got)
	}
	if got := os.Getenv("CLAUDE_CODE_SYSTEM_PROMPT"); got != "flag-prompt" {
		t.Fatalf("expected explicit system prompt override, got %q", got)
	}
	if got := os.Getenv("CLAUDE_CODE_MAX_TURNS"); got != "88" {
		t.Fatalf("expected explicit max turns override, got %q", got)
	}
}

func TestApplyInteractiveCompatEnvRespectsExistingBaseURLWhenNotExplicit(t *testing.T) {
	t.Setenv("CLAUDE_CODE_BASE_URL", "https://keep.example/v1")

	cfg := &CLIConfig{
		BaseURL: "",
	}

	restore, err := applyInteractiveCompatEnv(cfg)
	if err != nil {
		t.Fatalf("applyInteractiveCompatEnv failed: %v", err)
	}
	defer restore()

	if got := os.Getenv("CLAUDE_CODE_BASE_URL"); got != "https://keep.example/v1" {
		t.Fatalf("expected existing base URL to remain unchanged, got %q", got)
	}
}
