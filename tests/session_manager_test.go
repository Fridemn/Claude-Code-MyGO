package tests

import (
	"context"
	"testing"

	"claude-code-go/internal/session"
	"claude-code-go/internal/types"
)

func TestSessionManagerSaveLoadAndCreateWithID(t *testing.T) {
	t.Parallel()

	manager, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	s, err := manager.Create(context.Background())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if s.ID == "" {
		t.Fatalf("expected generated session id")
	}

	s.Messages = append(s.Messages, types.Message{
		Role:    types.RoleUser,
		Content: "hello",
	})
	if err := manager.Save(s); err != nil {
		t.Fatalf("save session: %v", err)
	}

	loaded, err := manager.Load(s.ID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if len(loaded.Messages) != 1 || loaded.Messages[0].Content != "hello" {
		t.Fatalf("unexpected loaded session: %#v", loaded)
	}

	fixed := manager.CreateWithID("fixed-id")
	if fixed.ID != "fixed-id" {
		t.Fatalf("unexpected fixed id: %s", fixed.ID)
	}
}

