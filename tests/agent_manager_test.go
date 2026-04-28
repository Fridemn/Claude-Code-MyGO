package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"claude-go/internal/agent"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/task"
	"claude-go/internal/tool"
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

// TestAgentManagerRetriesOnEmptyResponse verifies that when the engine returns
// "empty assistant response after tool execution", the agent manager retries
// and eventually succeeds when the provider recovers.
func TestAgentManagerRetriesOnEmptyResponse(t *testing.T) {
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
		&flakyEmptyResponseProvider{
			failCount: 2, // Fail first 2 attempts, succeed on 3rd
			successResponse: engine.Response{Text: "recovered response"},
		},
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	result, err := manager.Spawn(context.Background(), agent.SpawnInput{
		Description:  "retry test",
		Prompt:       "do something",
		SubagentType: "general-purpose",
		Background:   false,
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if result.Task == nil {
		t.Fatalf("expected task")
	}
	if result.Task.Status != task.StatusCompleted {
		t.Fatalf("expected completed status, got %s, error: %s", result.Task.Status, result.Task.Error)
	}
	if result.Task.Output != "recovered response" {
		t.Fatalf("expected 'recovered response', got %q", result.Task.Output)
	}
}

// TestAgentManagerFailsAfterMaxRetries verifies that when the engine consistently
// returns "empty assistant response after tool execution", the agent eventually
// fails with an error indicating all retries were exhausted.
func TestAgentManagerFailsAfterMaxRetries(t *testing.T) {
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
		&flakyEmptyResponseProvider{
			failCount:       100, // Always fail
			successResponse: engine.Response{Text: "should not reach"},
		},
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	result, err := manager.Spawn(context.Background(), agent.SpawnInput{
		Description:  "exhausted retries test",
		Prompt:       "do something",
		SubagentType: "general-purpose",
		Background:   false,
	})
	if err != nil {
		t.Fatalf("spawn should not return error directly: %v", err)
	}
	if result.Task == nil {
		t.Fatalf("expected task")
	}
	if result.Task.Status != task.StatusFailed {
		t.Fatalf("expected failed status, got %s", result.Task.Status)
	}
	if result.Task.Error == "" {
		t.Fatalf("expected error message on task")
	}
	// Verify the error mentions retries and the original empty response error
	errMsg := result.Task.Error
	if !containsSubstring(errMsg, "retries") || !containsSubstring(errMsg, "empty assistant response") {
		t.Fatalf("expected error to mention retries and empty response, got: %s", errMsg)
	}
}

// TestAgentManagerNonRetryableErrorNotRetried verifies that non-retryable errors
// (like context cancellation) are not retried.
func TestAgentManagerNonRetryableErrorNotRetried(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &nonRetryableErrorProvider{callCount: 0}
	manager := agent.CreateManager(
		config.Config{
			Model:      "test-model",
			MaxTurns:   8,
			SessionDir: t.TempDir(),
		},
		provider,
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	result, err := manager.Spawn(context.Background(), agent.SpawnInput{
		Description:  "non-retryable test",
		Prompt:       "do something",
		SubagentType: "general-purpose",
		Background:   false,
	})
	if err != nil {
		t.Fatalf("spawn should not return error directly: %v", err)
	}
	if result.Task == nil {
		t.Fatalf("expected task")
	}
	if result.Task.Status != task.StatusFailed {
		t.Fatalf("expected failed status, got %s", result.Task.Status)
	}
	// Non-retryable errors should only be called once
	if provider.callCount != 1 {
		t.Fatalf("expected exactly 1 call for non-retryable error, got %d", provider.callCount)
	}
}

func containsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

// flakyEmptyResponseProvider simulates a provider that returns "empty assistant response"
// for the first failCount calls, then returns successResponse.
type flakyEmptyResponseProvider struct {
	failCount       int
	callCount       int
	successResponse engine.Response
}

func (p *flakyEmptyResponseProvider) Complete(_ context.Context, _ engine.Request) (engine.Response, error) {
	p.callCount++
	if p.callCount <= p.failCount {
		return engine.Response{}, fmt.Errorf("model returned empty assistant response after tool execution")
	}
	return p.successResponse, nil
}

// nonRetryableErrorProvider always returns a non-retryable error.
type nonRetryableErrorProvider struct {
	callCount int
}

func (p *nonRetryableErrorProvider) Complete(_ context.Context, _ engine.Request) (engine.Response, error) {
	p.callCount++
	return engine.Response{}, fmt.Errorf("some non-retryable error")
}

