package tests

import (
	"context"
	"testing"

	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/session"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

func TestEngineSubmit_NativeToolLoopDoesNotInsertHookSystemMessagesBetweenToolCallAndResult(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	provider := &scriptedProvider{
		responses: []engine.Response{
			{
				ToolCalls: []types.ToolCall{
					{
						ID:        "call_1",
						Name:      "echo_tool",
						Arguments: `{"value":"hello"}`,
					},
				},
			},
			{Text: `final answer`},
		},
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider: provider,
		Tools:    registry,
		Hooks: hookRunnerFunc(func(_ context.Context, event engine.HookEvent) ([]engine.HookExecution, error) {
			if event.Name == "pre_tool" || event.Name == "post_tool" {
				return []engine.HookExecution{
					{
						Event:  event.Name,
						Hook:   "demo",
						Result: "ok",
					},
				}, nil
			}
			return nil, nil
		}),
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	if _, err := eng.Submit(context.Background(), "hi"); err != nil {
		t.Fatalf("submit: %v", err)
	}

	messages := eng.Messages()
	var assistantIdx, toolIdx int = -1, -1
	for i, msg := range messages {
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) == 1 && msg.ToolCalls[0].ID == "call_1" {
			assistantIdx = i
		}
		if msg.Role == types.RoleTool && msg.ToolCallID == "call_1" {
			toolIdx = i
		}
	}
	if assistantIdx == -1 || toolIdx == -1 {
		t.Fatalf("expected assistant tool_call and tool result messages, got %#v", messages)
	}
	if toolIdx != assistantIdx+1 {
		t.Fatalf("expected tool result to immediately follow assistant tool_call, got assistant=%d tool=%d", assistantIdx, toolIdx)
	}
}

type hookRunnerFunc func(context.Context, engine.HookEvent) ([]engine.HookExecution, error)

func (f hookRunnerFunc) Trigger(ctx context.Context, event engine.HookEvent) ([]engine.HookExecution, error) {
	return f(ctx, event)
}
