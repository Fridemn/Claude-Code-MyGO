package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/session"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

func TestEngineSubmit_ToolLoop(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: `<tool_call name="echo_tool">{"value":"hello"}</tool_call>`},
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	resp, err := eng.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Text != "final answer" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", provider.calls)
	}

	messages := eng.Messages()
	if len(messages) < 4 {
		t.Fatalf("expected session messages, got %d", len(messages))
	}
	foundToolResult := false
	for _, msg := range messages {
		if msg.Role == types.RoleSystem && strings.Contains(msg.Content, "tool=echo_tool") {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Fatalf("expected tool result to be recorded in session messages")
	}
}

func TestEngineSubmit_DynamicMCPToolLoop(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: `<tool_call name="mcp__demo__workspace_echo">{"value":"ping"}</tool_call>`},
			{Text: `done`},
		},
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider: provider,
		Tools:    tool.EmptyRegistry(),
		ToolRuntime: tool.Runtime{
			MCP: stubMCPRuntime{
				dynamic: []tool.MCPDynamicToolInfo{
					{
						Name:        "mcp__demo__workspace_echo",
						Server:      "demo",
						Tool:        "workspace.echo",
						Description: "dynamic test tool",
						ReadOnly:    true,
					},
				},
			},
		},
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	resp, err := eng.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Text != "done" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
}

func TestEngineSubmit_NativeToolLoop(t *testing.T) {
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	resp, err := eng.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Text != "final answer" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if provider.calls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", provider.calls)
	}
	if len(provider.requests) == 0 || len(provider.requests[0].Tools) == 0 {
		t.Fatalf("expected tool definitions to be passed to provider")
	}

	messages := eng.Messages()
	foundAssistantToolCall := false
	foundToolResult := false
	for _, msg := range messages {
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) == 1 && msg.ToolCalls[0].ID == "call_1" {
			foundAssistantToolCall = true
		}
		if msg.Role == types.RoleTool && msg.ToolCallID == "call_1" && strings.Contains(msg.Content, "tool=echo_tool") {
			foundToolResult = true
		}
	}
	if !foundAssistantToolCall {
		t.Fatalf("expected assistant tool call to be recorded in session messages")
	}
	if !foundToolResult {
		t.Fatalf("expected native tool result to be recorded in session messages")
	}
}

func TestEngineSubmit_NativeToolLoopWithoutProviderIDs(t *testing.T) {
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	resp, err := eng.Submit(context.Background(), "hi")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Text != "final answer" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}

	messages := eng.Messages()
	var assistantCallID string
	for _, msg := range messages {
		if msg.Role == types.RoleAssistant && len(msg.ToolCalls) == 1 {
			assistantCallID = msg.ToolCalls[0].ID
		}
	}
	if assistantCallID == "" {
		t.Fatalf("expected synthesized tool call ID in assistant message")
	}
	for _, msg := range messages {
		if msg.Role == types.RoleTool && msg.ToolCallID == assistantCallID {
			return
		}
	}
	t.Fatalf("expected tool result message to reuse synthesized tool call ID")
}

func TestEngineSubmit_ToolLoopUsesConfiguredMaxTurns(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: `<tool_call name="echo_tool">{"value":"step-1"}</tool_call>`},
			{Text: `<tool_call name="echo_tool">{"value":"step-2"}</tool_call>`},
			{Text: `<tool_call name="echo_tool">{"value":"step-3"}</tool_call>`},
			{Text: `<tool_call name="echo_tool">{"value":"step-4"}</tool_call>`},
			{Text: `final answer after exploration`},
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	resp, err := eng.Submit(context.Background(), "analyze the migration")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if resp.Text != "final answer after exploration" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if provider.calls != 5 {
		t.Fatalf("expected 5 provider calls, got %d", provider.calls)
	}
}

type scriptedProvider struct {
	responses []engine.Response
	calls     int
	requests  []engine.Request
}

func (p *scriptedProvider) Complete(_ context.Context, req engine.Request) (engine.Response, error) {
	p.requests = append(p.requests, req)
	if p.calls >= len(p.responses) {
		return engine.Response{Text: "unexpected extra call"}, nil
	}
	resp := p.responses[p.calls]
	p.calls++
	return resp, nil
}

type testEchoTool struct{}

func (testEchoTool) Name() string               { return "echo_tool" }
func (testEchoTool) Description() string        { return "echo input value" }
func (testEchoTool) IsReadOnly(tool.Input) bool { return true }
func (testEchoTool) Call(_ context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	value, _ := in["value"].(string)
	return tool.Result{Content: "echo:" + value}, nil
}

func TestEngineNew_PersistsSessionFileImmediately(t *testing.T) {
	t.Parallel()

	sessionDir := t.TempDir()
	sessions, err := session.CreateManager(sessionDir)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     8,
		},
		Provider: &scriptedProvider{},
		Tools:    tool.EmptyRegistry(),
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	path := filepath.Join(sessionDir, eng.SessionID()+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected persisted session file at %s: %v", path, err)
	}
}

func TestEngineSubmit_TrimHistoryKeepsWholeUserTurns(t *testing.T) {
	t.Parallel()

	sessionDir := t.TempDir()
	sessions, err := session.CreateManager(sessionDir)
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedProvider{
		responses: []engine.Response{
			{Text: "final answer"},
		},
	}

	eng, err := engine.Create(context.Background(), engine.Options{
		Config: config.Config{
			Model:        "test-model",
			SystemPrompt: "system prompt",
			MaxTurns:     1,
		},
		Provider: provider,
		Tools:    tool.EmptyRegistry(),
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	eng.ReplaceMessages([]types.Message{
		{
			Role:      types.RoleSystem,
			Content:   "system prompt",
			Timestamp: timeNowForTest(),
		},
		{
			Role:      types.RoleUser,
			Content:   "old turn",
			Timestamp: timeNowForTest(),
		},
		{
			Role:    types.RoleAssistant,
			Content: "",
			ToolCalls: []types.ToolCall{
				{
					ID:        "call_old",
					Name:      "echo_tool",
					Arguments: `{"value":"old"}`,
				},
			},
			Timestamp: timeNowForTest(),
		},
		{
			Role:       types.RoleTool,
			Content:    "tool=echo_tool\nstatus=ok\n\nold",
			ToolCallID: "call_old",
			Timestamp:  timeNowForTest(),
		},
	})

	if _, err := eng.Submit(context.Background(), "current turn"); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if len(provider.requests) == 0 {
		t.Fatalf("expected provider to receive at least one request")
	}

	reqMessages := provider.requests[0].Messages
	for _, msg := range reqMessages {
		if msg.Role == types.RoleTool {
			t.Fatalf("expected trimmed request history to exclude orphan tool messages: %#v", reqMessages)
		}
	}
	if len(reqMessages) < 2 || reqMessages[len(reqMessages)-1].Role != types.RoleUser || reqMessages[len(reqMessages)-1].Content != "current turn" {
		t.Fatalf("unexpected trimmed request history: %#v", reqMessages)
	}
}

func timeNowForTest() time.Time {
	return time.Unix(0, 0)
}
