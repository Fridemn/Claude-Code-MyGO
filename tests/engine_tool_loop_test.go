package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/tool"
	"claude-go/internal/tool/repl"
	"claude-go/internal/types"
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
						Arguments: json.RawMessage(`{"value":"hello"}`),
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
						Arguments: json.RawMessage(`{"value":"hello"}`),
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

func TestEngineSubmit_RetriesEmptyAssistantAfterToolResult(t *testing.T) {
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
						Arguments: json.RawMessage(`{"value":"hello"}`),
					},
				},
			},
			{},
			{Text: "final answer"},
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
	if provider.calls != 3 {
		t.Fatalf("expected 3 provider calls, got %d", provider.calls)
	}

	for _, msg := range eng.Messages() {
		if msg.Role == types.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			t.Fatalf("unexpected empty assistant message persisted: %#v", msg)
		}
	}
}

func TestEngineSubmit_ErrorsAfterConsecutiveEmptyAssistantPostToolResponses(t *testing.T) {
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
						Arguments: json.RawMessage(`{"value":"hello"}`),
					},
				},
			},
			{},
			{},
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

	_, err = eng.Submit(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error for repeated empty assistant responses after tool execution")
	}
	if !strings.Contains(err.Error(), "empty assistant response after tool execution") {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != 3 {
		t.Fatalf("expected 3 provider calls, got %d", provider.calls)
	}

	for _, msg := range eng.Messages() {
		if msg.Role == types.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			t.Fatalf("unexpected empty assistant message persisted: %#v", msg)
		}
	}
}

func TestEngineSubmitStream_RetriesEmptyAssistantAfterToolResult(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{
				ToolCalls: []types.ToolCall{
					{
						ID:        "call_1",
						Name:      "echo_tool",
						Arguments: json.RawMessage(`{"value":"hello"}`),
					},
				},
			},
			{},
			{Text: "final answer"},
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

	resp, err := eng.SubmitStream(context.Background(), "hi", nil)
	if err != nil {
		t.Fatalf("submit stream: %v", err)
	}
	if resp.Text != "final answer" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if provider.calls != 3 {
		t.Fatalf("expected 3 provider calls, got %d", provider.calls)
	}

	for _, msg := range eng.Messages() {
		if msg.Role == types.RoleAssistant && strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			t.Fatalf("unexpected empty assistant message persisted: %#v", msg)
		}
	}
}

func TestEngineSubmitStream_EmitsToolLifecycleChunks(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{
				ToolCalls: []types.ToolCall{
					{
						ID:        "call_1",
						Name:      "echo_tool",
						Arguments: json.RawMessage(`{"value":"hello"}`),
					},
				},
			},
			{Text: "final answer"},
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

	var seenToolStart, seenToolDone bool
	resp, err := eng.SubmitStream(context.Background(), "hi", func(chunk engine.StreamChunk) error {
		if chunk.Status == "Running tool: echo_tool" && len(chunk.ToolCalls) > 0 {
			seenToolStart = true
		}
		if chunk.Status == "Tool completed: echo_tool" && chunk.ToolCallID == "call_1" && chunk.ToolResult == "ok" {
			seenToolDone = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("submit stream: %v", err)
	}
	if resp.Text != "final answer" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if !seenToolStart {
		t.Fatalf("expected tool start lifecycle chunk")
	}
	if !seenToolDone {
		t.Fatalf("expected tool completion lifecycle chunk")
	}
}

func TestEngineSubmitStream_REPLProgressUsesInnerPrimitiveMetadata(t *testing.T) {
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("CLAUDE_CODE_REPL", "")

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	registry := tool.EmptyRegistry()
	registry.Register(repl.REPLTool{})
	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{
				ToolCalls: []types.ToolCall{
					{
						ID:        "repl_call_1",
						Name:      "REPL",
						Arguments: json.RawMessage(`{"script":"Read({\"file_path\":\"src/main.go\"})"}`),
					},
				},
			},
			{Text: "done"},
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

	resp, err := eng.SubmitStream(context.Background(), "hi", nil)
	if err != nil {
		t.Fatalf("submit stream: %v", err)
	}
	if resp.Text != "done" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}

	var startProgressFound bool
	for _, msg := range eng.Messages() {
		if msg.Type != types.MessageTypeProgress || msg.ToolCallID != "repl_call_1" {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
			t.Fatalf("invalid progress payload: %v", err)
		}
		phase, _ := payload["phase"].(string)
		if phase != "start" {
			continue
		}
		startProgressFound = true

		if toolName, _ := payload["toolName"].(string); toolName != "Read" {
			t.Fatalf("expected progress toolName Read, got %q", toolName)
		}
		toolInput, _ := payload["toolInput"].(map[string]any)
		if filePath, _ := toolInput["file_path"].(string); filePath != "src/main.go" {
			t.Fatalf("expected progress toolInput.file_path=src/main.go, got %#v", toolInput["file_path"])
		}
	}

	if !startProgressFound {
		t.Fatal("expected REPL start progress message")
	}
}

func TestEngineContinueStream_UsesExistingMessagesWithoutAppendingUserInput(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{Text: "acknowledged"},
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	seed := []types.Message{
		{
			Role:      types.RoleSystem,
			Content:   "system prompt",
			Timestamp: timeNowForTest(),
		},
		{
			Role:                      types.RoleSystem,
			Type:                      types.SystemSubtypeLocalCommand,
			Content:                   "<local-command-stdout>hidden transcript-only</local-command-stdout>",
			IsVisibleInTranscriptOnly: true,
			Timestamp:                 timeNowForTest(),
		},
		{
			Role:      types.RoleUser,
			Content:   "meta follow-up context",
			IsMeta:    true,
			Timestamp: timeNowForTest(),
		},
	}
	eng.ReplaceMessages(seed)

	resp, err := eng.ContinueStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("continue stream: %v", err)
	}
	if resp.Text != "acknowledged" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if provider.calls != 1 {
		t.Fatalf("expected 1 provider call, got %d", provider.calls)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one captured request, got %d", len(provider.requests))
	}
	reqMessages := provider.requests[0].Messages
	if len(reqMessages) != 2 {
		t.Fatalf("expected transcript-only system local command to be excluded, got %d messages", len(reqMessages))
	}
	if reqMessages[len(reqMessages)-1].Role != types.RoleUser || reqMessages[len(reqMessages)-1].Content != "meta follow-up context" || !reqMessages[len(reqMessages)-1].IsMeta {
		t.Fatalf("unexpected tail request message: %#v", reqMessages[len(reqMessages)-1])
	}
	for _, msg := range reqMessages {
		if msg.Type == types.SystemSubtypeLocalCommand {
			t.Fatalf("expected local_command transcript-only message to be excluded from model request: %#v", reqMessages)
		}
	}

	all := eng.Messages()
	if len(all) != len(seed)+1 {
		t.Fatalf("expected one assistant message appended, got %d total", len(all))
	}
	if all[len(all)-1].Role != types.RoleAssistant || all[len(all)-1].Content != "acknowledged" {
		t.Fatalf("unexpected final assistant message: %#v", all[len(all)-1])
	}
}

func TestEngineContinueStream_FiltersProgressMessagesFromModelRequest(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{Text: "acknowledged"},
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	seed := []types.Message{
		{
			Role:      types.RoleSystem,
			Content:   "system prompt",
			Timestamp: timeNowForTest(),
		},
		{
			Type:       types.MessageTypeProgress,
			Role:       types.RoleSystem,
			ToolCallID: "repl-call-1",
			Content:    `{"type":"repl_tool_call","phase":"start","toolName":"REPL"}`,
			Timestamp:  timeNowForTest(),
		},
		{
			Role:      types.RoleUser,
			Content:   "meta follow-up context",
			IsMeta:    true,
			Timestamp: timeNowForTest(),
		},
	}
	eng.ReplaceMessages(seed)

	resp, err := eng.ContinueStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("continue stream: %v", err)
	}
	if resp.Text != "acknowledged" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one captured request, got %d", len(provider.requests))
	}
	reqMessages := provider.requests[0].Messages
	if len(reqMessages) != 2 {
		t.Fatalf("expected progress message to be excluded, got %d messages", len(reqMessages))
	}
	for _, msg := range reqMessages {
		if msg.Type == types.MessageTypeProgress || msg.Role == types.MessageTypeProgress {
			t.Fatalf("expected progress message to be excluded from model request: %#v", reqMessages)
		}
	}
}

func TestEngineContinueStream_FiltersVirtualMessagesFromModelRequest(t *testing.T) {
	t.Parallel()

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}

	provider := &scriptedStreamingProvider{
		responses: []engine.Response{
			{Text: "acknowledged"},
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
		Sessions: sessions,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	seed := []types.Message{
		{
			Role:      types.RoleSystem,
			Content:   "system prompt",
			Timestamp: timeNowForTest(),
		},
		{
			Role:      types.RoleAssistant,
			Content:   "<virtual tool_use>",
			IsVirtual: true,
			Timestamp: timeNowForTest(),
		},
		{
			Role:      types.RoleUser,
			Content:   "follow-up",
			Timestamp: timeNowForTest(),
		},
	}
	eng.ReplaceMessages(seed)

	resp, err := eng.ContinueStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("continue stream: %v", err)
	}
	if resp.Text != "acknowledged" {
		t.Fatalf("unexpected final response: %q", resp.Text)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("expected one captured request, got %d", len(provider.requests))
	}
	reqMessages := provider.requests[0].Messages
	if len(reqMessages) != 2 {
		t.Fatalf("expected virtual message to be excluded, got %d", len(reqMessages))
	}
	for _, msg := range reqMessages {
		if msg.IsVirtual {
			t.Fatalf("expected virtual message to be excluded from model request: %#v", reqMessages)
		}
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

type scriptedStreamingProvider struct {
	responses []engine.Response
	calls     int
	requests  []engine.Request
}

func (p *scriptedStreamingProvider) Complete(_ context.Context, req engine.Request) (engine.Response, error) {
	p.requests = append(p.requests, req)
	if p.calls >= len(p.responses) {
		return engine.Response{Text: "unexpected extra call"}, nil
	}
	resp := p.responses[p.calls]
	p.calls++
	return resp, nil
}

func (p *scriptedStreamingProvider) CompleteStream(_ context.Context, req engine.Request, onChunk func(engine.StreamChunk) error) (engine.Response, error) {
	p.requests = append(p.requests, req)
	if p.calls >= len(p.responses) {
		return engine.Response{Text: "unexpected extra call"}, nil
	}
	resp := p.responses[p.calls]
	p.calls++

	if onChunk != nil {
		if strings.TrimSpace(resp.Text) != "" {
			if err := onChunk(engine.StreamChunk{Text: resp.Text}); err != nil {
				return engine.Response{}, err
			}
		}
		if len(resp.ToolCalls) > 0 {
			specs, err := tool.ParseNativeCalls(resp.ToolCalls)
			if err != nil {
				return engine.Response{}, err
			}
			if err := onChunk(engine.StreamChunk{ToolCalls: specs}); err != nil {
				return engine.Response{}, err
			}
		}
	}

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
					Arguments: json.RawMessage(`{"value":"old"}`),
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
