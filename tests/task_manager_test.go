package tests

import (
	"errors"
	"testing"

	"claude-code-go/internal/task"
	"claude-code-go/internal/types"
)

func TestTaskManagerLifecycleAndNotices(t *testing.T) {
	t.Parallel()

	manager := task.EmptyManager()
	agentTask, err := manager.CreateAgentTask("demo task", "general-purpose", "", "test-model", "hello", true)
	if err != nil {
		t.Fatalf("create agent task: %v", err)
	}

	manager.SetRunning(agentTask.ID)
	manager.SetSessionID(agentTask.ID, "session-1")
	manager.SetInput(agentTask.ID, "follow-up")
	manager.UpdateMessages(agentTask.ID, []types.Message{
		{Role: types.RoleUser, Content: "follow-up"},
	})
	manager.Complete(agentTask.ID, "done", "summary", []types.Message{
		{Role: types.RoleAssistant, Content: "done"},
	})

	got, ok := manager.Get(agentTask.ID)
	if !ok {
		t.Fatalf("expected task to exist")
	}
	if got.Status != task.StatusCompleted {
		t.Fatalf("unexpected status: %s", got.Status)
	}
	if got.SessionID != "session-1" {
		t.Fatalf("unexpected session id: %s", got.SessionID)
	}
	if got.LastUserPrompt != "follow-up" || got.LastAssistantReply != "done" {
		t.Fatalf("unexpected prompts/reply: %#v", got)
	}
	if got.TurnCount != 1 {
		t.Fatalf("expected turn_count=1, got %d", got.TurnCount)
	}

	notices := manager.DrainNotices()
	if len(notices) != 1 || notices[0].Kind != "completed" {
		t.Fatalf("unexpected notices: %#v", notices)
	}
}

func TestTaskManagerFailAndKill(t *testing.T) {
	t.Parallel()

	manager := task.EmptyManager()
	failing, err := manager.CreateAgentTask("failing", "general-purpose", "", "test-model", "hello", true)
	if err != nil {
		t.Fatalf("create failing task: %v", err)
	}
	manager.Fail(failing.ID, errors.New("boom"), "partial summary", []types.Message{
		{Role: types.RoleAssistant, Content: "partial summary"},
	})

	got, ok := manager.Get(failing.ID)
	if !ok || got.Status != task.StatusFailed {
		t.Fatalf("unexpected failed task: %#v", got)
	}
	if got.Error != "boom" || got.Summary != "partial summary" {
		t.Fatalf("unexpected failure state: %#v", got)
	}

	killed, err := manager.CreateAgentTask("killed", "general-purpose", "", "test-model", "hello", true)
	if err != nil {
		t.Fatalf("create killed task: %v", err)
	}
	if ok := manager.Kill(killed.ID, "stop now"); !ok {
		t.Fatalf("expected kill to succeed")
	}
	got, ok = manager.Get(killed.ID)
	if !ok || got.Status != task.StatusKilled {
		t.Fatalf("unexpected killed task: %#v", got)
	}
	if got.Error != "stop now" {
		t.Fatalf("unexpected kill reason: %#v", got)
	}
}

