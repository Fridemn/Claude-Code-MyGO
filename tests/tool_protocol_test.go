package tests

import (
	"strings"
	"testing"

	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

func TestToolProtocolParseStripAndRender(t *testing.T) {
	t.Parallel()

	text := `before
<tool_call name="grep">{"pattern":"TODO","path":"."}</tool_call>
after`

	calls, err := tool.ParseCalls(text)
	if err != nil {
		t.Fatalf("parse calls: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one call, got %d", len(calls))
	}
	if calls[0].Name != "grep" {
		t.Fatalf("unexpected call name: %s", calls[0].Name)
	}
	if calls[0].Input["pattern"] != "TODO" {
		t.Fatalf("unexpected call input: %#v", calls[0].Input)
	}

	stripped := tool.StripCalls(text)
	if strings.Contains(stripped, "<tool_call") {
		t.Fatalf("expected tool calls to be stripped, got %q", stripped)
	}
	if !strings.Contains(stripped, "before") || !strings.Contains(stripped, "after") {
		t.Fatalf("unexpected stripped text: %q", stripped)
	}

	rendered := tool.RenderResult("grep", tool.Result{Content: "hit"})
	if !strings.Contains(rendered, "tool=grep") || !strings.Contains(rendered, "status=ok") || !strings.Contains(rendered, "hit") {
		t.Fatalf("unexpected rendered result: %s", rendered)
	}
}

func TestToolProtocolParseNativeCalls(t *testing.T) {
	t.Parallel()

	calls, err := tool.ParseNativeCalls([]types.ToolCall{
		{
			ID:        "call_123",
			Name:      "grep",
			Arguments: `{"pattern":"TODO","path":"."}`,
		},
	})
	if err != nil {
		t.Fatalf("parse native calls: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one native call, got %d", len(calls))
	}
	if calls[0].ID != "call_123" || calls[0].Name != "grep" {
		t.Fatalf("unexpected native call: %#v", calls[0])
	}
	if calls[0].Input["pattern"] != "TODO" {
		t.Fatalf("unexpected native call input: %#v", calls[0].Input)
	}
}

func TestToolProtocolRenderAgentTaskAndErrors(t *testing.T) {
	t.Parallel()

	rendered := tool.RenderResult("agent_run", tool.Result{
		Content: &task.AgentTask{
			ID:        "a_123",
			AgentType: "general-purpose",
			Status:    task.StatusCompleted,
		},
	})
	for _, want := range []string{"tool=agent_run", "task_id=a_123", "agent=general-purpose", "status=completed"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected rendered task result to contain %q, got %s", want, rendered)
		}
	}

	renderedErr := tool.RenderResult("grep", tool.Result{Error: "failed"})
	if !strings.Contains(renderedErr, "status=error") || !strings.Contains(renderedErr, "error=failed") {
		t.Fatalf("unexpected error result: %s", renderedErr)
	}
}

func TestToolSystemPromptFragmentIncludesDynamicMCPHint(t *testing.T) {
	t.Parallel()

	registry := tool.EmptyRegistry()
	registry.Register(testEchoTool{})
	fragment := tool.SystemPromptFragment(registry.List())
	if !strings.Contains(fragment, "Available tools:") {
		t.Fatalf("missing available tools section")
	}
	if !strings.Contains(fragment, "echo_tool") {
		t.Fatalf("missing tool name in fragment: %s", fragment)
	}
	if !strings.Contains(fragment, "dynamic MCP tools prefixed with mcp__") {
		t.Fatalf("missing dynamic MCP hint: %s", fragment)
	}
	for _, want := range []string{
		"File search: use Glob or list_files",
		"Read files: use Read",
		"Current repository root should be referenced as '.'",
	} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("missing tool preference guidance %q in fragment: %s", want, fragment)
		}
	}
}
