package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-code-go/internal/api"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/types"
)

func TestLiveAPIFromDotEnv(t *testing.T) {
	if os.Getenv("CLAUDE_CODE_RUN_LIVE_API_TEST") != "1" {
		t.Skip("set CLAUDE_CODE_RUN_LIVE_API_TEST=1 to run live API smoke using ../.env")
	}

	cfg, err := config.Load(filepath.Join("..", ".env"))
	if err != nil {
		t.Fatalf("load ../.env: %v", err)
	}
	client := api.CreateOpenAICompatibleClient(cfg)
	resp, err := client.Complete(context.Background(), engine.Request{
		Model: cfg.Model,
		Messages: []types.Message{
			{Role: types.RoleUser, Content: "Reply with exactly: live api ok"},
		},
	})
	if err != nil {
		t.Fatalf("live api request failed: %v", err)
	}
	if resp.Text == "" {
		t.Fatalf("expected non-empty live api response")
	}
}

