package tests

import (
	"context"
	"testing"

	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/tool"
	"claude-go/internal/types"
)

type staticProvider struct {
	response engine.Response
}

func (p staticProvider) Complete(context.Context, engine.Request) (engine.Response, error) {
	return p.response, nil
}

func TestEnginePersistSession_AlsoRecordsTranscript(t *testing.T) {
	t.Setenv("USER_TYPE", "cli")

	dir := t.TempDir()
	sessions, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("create session manager: %v", err)
	}
	transcripts, err := session.CreateEnhancedManager(dir)
	if err != nil {
		t.Fatalf("create transcript manager: %v", err)
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider:    staticProvider{response: engine.Response{Text: "ok"}},
		Tools:       tool.EmptyRegistry(),
		Sessions:    sessions,
		Transcripts: transcripts,
	})
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}

	if _, err := eng.Submit(context.Background(), "hello"); err != nil {
		t.Fatalf("submit: %v", err)
	}

	messages, err := transcripts.ReadTranscript(eng.SessionID())
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected persisted transcript messages")
	}

	var foundUser, foundAssistant bool
	for _, msg := range messages {
		if msg.Role == types.RoleUser && msg.Content == "hello" {
			foundUser = true
		}
		if msg.Role == types.RoleAssistant && msg.Content == "ok" {
			foundAssistant = true
		}
	}
	if !foundUser || !foundAssistant {
		t.Fatalf("expected transcript to contain user+assistant messages, got %#v", messages)
	}
}
