package api

import (
	"encoding/json"
	"testing"

	"claude-go/internal/types"
)

func TestBuildTypesToolCallsNormalizesArguments(t *testing.T) {
	t.Parallel()

	resp := &ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: &ChatMessage{
					ToolCalls: []ToolCall{
						{
							ID:   "empty",
							Type: "function",
							Function: FunctionCallData{
								Name:      "Write",
								Arguments: "",
							},
						},
						{
							ID:   "invalid",
							Type: "function",
							Function: FunctionCallData{
								Name:      "Read",
								Arguments: "{",
							},
						},
						{
							ID:   "valid",
							Type: "function",
							Function: FunctionCallData{
								Name:      "Glob",
								Arguments: `{"pattern":"*.go"}`,
							},
						},
					},
				},
			},
		},
	}

	calls := buildTypesToolCalls(resp)
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}

	if got := string(calls[0].Arguments); got != "{}" {
		t.Fatalf("expected empty args normalized to {}, got %q", got)
	}
	if got := string(calls[1].Arguments); got != "{}" {
		t.Fatalf("expected invalid args normalized to {}, got %q", got)
	}
	if got := string(calls[2].Arguments); got != `{"pattern":"*.go"}` {
		t.Fatalf("expected valid args unchanged, got %q", got)
	}
}

func TestBuildTypesToolCallsRepairsSplitMalformedCall(t *testing.T) {
	t.Parallel()

	resp := &ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: &ChatMessage{
					ToolCalls: []ToolCall{
						{
							ID:   "call-1",
							Type: "function",
							Function: FunctionCallData{
								Name:      "Write",
								Arguments: "",
							},
						},
						{
							ID:   "call-2",
							Type: "function",
							Function: FunctionCallData{
								Name:      "",
								Arguments: `{"file_path":"test.json","content":"{}"}`,
							},
						},
					},
				},
			},
		},
	}

	calls := buildTypesToolCalls(resp)
	if len(calls) != 1 {
		t.Fatalf("expected malformed split call to be repaired into one call, got %d", len(calls))
	}
	if calls[0].Name != "Write" {
		t.Fatalf("expected repaired call name Write, got %q", calls[0].Name)
	}
	if got := string(calls[0].Arguments); got != `{"file_path":"test.json","content":"{}"}` {
		t.Fatalf("unexpected repaired call arguments: %q", got)
	}
}

func TestBuildMessagesFromTypesSkipsInvalidToolCalls(t *testing.T) {
	t.Parallel()

	msgs := []types.Message{
		{
			Role: types.RoleAssistant,
			ToolCalls: []types.ToolCall{
				{ID: "valid", Name: "Write", Arguments: json.RawMessage(`{"file_path":"a.json","content":"{}"}`)},
				{ID: "invalid-name", Name: "", Arguments: json.RawMessage(`{"file_path":"b.json"}`)},
			},
		},
	}

	chat := BuildMessagesFromTypes(msgs)
	if len(chat) != 1 {
		t.Fatalf("expected 1 message, got %d", len(chat))
	}
	if len(chat[0].ToolCalls) != 1 {
		t.Fatalf("expected invalid tool call to be filtered, got %d calls", len(chat[0].ToolCalls))
	}
	if chat[0].ToolCalls[0].Function.Name != "Write" {
		t.Fatalf("unexpected tool call name: %q", chat[0].ToolCalls[0].Function.Name)
	}
}

func TestBuildMessagesFromTypesSkipsOrphanToolResult(t *testing.T) {
	t.Parallel()

	msgs := []types.Message{
		{
			Role: types.RoleAssistant,
			ToolCalls: []types.ToolCall{
				{ID: "call-valid", Name: "Write", Arguments: json.RawMessage(`{"file_path":"a.json","content":"{}"}`)},
				{ID: "call-bad", Name: "", Arguments: json.RawMessage(`{"file_path":"b.json","content":"{}"}`)},
			},
		},
		{Role: types.RoleTool, ToolCallID: "call-valid", Content: "ok"},
		{Role: types.RoleTool, ToolCallID: "call-bad", Content: "unknown tool"},
	}

	chat := BuildMessagesFromTypes(msgs)
	if len(chat) != 2 {
		t.Fatalf("expected assistant + one valid tool result, got %d messages", len(chat))
	}
	if len(chat[0].ToolCalls) != 1 {
		t.Fatalf("expected only one valid tool call, got %d", len(chat[0].ToolCalls))
	}
	if chat[1].Role != types.RoleTool || chat[1].ToolCallID != "call-valid" {
		t.Fatalf("unexpected second message: role=%q tool_call_id=%q", chat[1].Role, chat[1].ToolCallID)
	}
}
