package tests

import (
	"context"
	"testing"
	"time"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/session"
	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
)

func TestAgentManagerSpawnAndContinue(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	manager := agent.CreateManager(
		config.Config{
			Model:      "test-model",
			MaxTurns:   8,
			SessionDir: t.TempDir(),
		},
		&scriptedProvider{
			responses: []engine.Response{
				{Text: "first response"},
				{Text: "second response"},
			},
		},
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	first, err := manager.Spawn(context.Background(), agent.SpawnInput{
		Description:  "test task",
		Prompt:       "hello",
		SubagentType: "general-purpose",
		Background:   false,
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if first.Task == nil || first.Task.Status != task.StatusCompleted {
		t.Fatalf("unexpected first task state: %#v", first.Task)
	}
	if first.Task.SessionID == "" {
		t.Fatalf("expected session id on first task")
	}

	second, err := manager.Continue(context.Background(), first.Task.ID, "follow-up", false)
	if err != nil {
		t.Fatalf("continue: %v", err)
	}
	if second.Task == nil || second.Task.Status != task.StatusCompleted {
		t.Fatalf("unexpected second task state: %#v", second.Task)
	}
	if second.Task.ID != first.Task.ID {
		t.Fatalf("expected continuation to reuse task id")
	}
	if second.Task.SessionID != first.Task.SessionID {
		t.Fatalf("expected continuation to reuse session id")
	}
	if second.Task.LastUserPrompt != "follow-up" {
		t.Fatalf("unexpected last user prompt: %q", second.Task.LastUserPrompt)
	}
	if second.Task.LastAssistantReply != "second response" {
		t.Fatalf("unexpected last assistant reply: %q", second.Task.LastAssistantReply)
	}
	if second.Task.TurnCount != 2 {
		t.Fatalf("expected turn_count=2, got %d", second.Task.TurnCount)
	}
}

func TestAgentManagerStopBackgroundTask(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	manager := agent.CreateManager(
		config.Config{
			Model:      "test-model",
			MaxTurns:   8,
			SessionDir: t.TempDir(),
		},
		&blockingProvider{},
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	result, err := manager.Spawn(context.Background(), agent.SpawnInput{
		Description:  "background test",
		Prompt:       "run forever",
		SubagentType: "verification",
		Background:   true,
	})
	if err != nil {
		t.Fatalf("spawn background: %v", err)
	}
	if result.Task == nil {
		t.Fatalf("expected task")
	}

	time.Sleep(50 * time.Millisecond)
	if err := manager.Stop(result.Task.ID); err != nil {
		t.Fatalf("stop task: %v", err)
	}

	finalTask, err := manager.WaitForTask(result.Task.ID)
	if err != nil {
		t.Fatalf("wait for task: %v", err)
	}
	if finalTask.Status != task.StatusKilled {
		t.Fatalf("expected killed status, got %s", finalTask.Status)
	}
	if finalTask.Error == "" {
		t.Fatalf("expected stop reason to be recorded")
	}
}

type blockingProvider struct{}

func (p *blockingProvider) Complete(ctx context.Context, _ engine.Request) (engine.Response, error) {
	<-ctx.Done()
	return engine.Response{}, ctx.Err()
}

