package session

import (
	"encoding/json"
	"testing"

	"claude-go/internal/types"
)

func TestManagerSaveNormalizesInvalidRawMessages(t *testing.T) {
	t.Parallel()

	mgr, err := CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	sess := mgr.CreateWithID("rawmessage-normalize")
	sess.AddMessage(types.Message{
		Role: types.RoleAssistant,
		ToolCalls: []types.ToolCall{
			{ID: "tool-1", Name: "Write", Arguments: json.RawMessage("")},
			{ID: "tool-2", Name: "Read", Arguments: json.RawMessage("{")},
		},
		Blocks: []types.ContentBlock{
			{Type: types.ContentTypeToolUse, ID: "tool-1", Name: "Write", Input: json.RawMessage("")},
			{Type: types.ContentTypeToolUse, ID: "tool-2", Name: "Read", Input: json.RawMessage("{")},
		},
	})

	if err := mgr.Save(sess); err != nil {
		t.Fatalf("Save should normalize invalid RawMessage values: %v", err)
	}

	loaded, err := mgr.Load("rawmessage-normalize")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := string(loaded.Messages[0].ToolCalls[0].Arguments); got != "{}" {
		t.Fatalf("expected empty tool args normalized to {}, got %q", got)
	}
	if got := string(loaded.Messages[0].ToolCalls[1].Arguments); got != "{}" {
		t.Fatalf("expected invalid tool args normalized to {}, got %q", got)
	}
	if got := string(loaded.Messages[0].Blocks[0].Input); got != "{}" {
		t.Fatalf("expected empty tool_use input normalized to {}, got %q", got)
	}
	if got := string(loaded.Messages[0].Blocks[1].Input); got != "{}" {
		t.Fatalf("expected invalid tool_use input normalized to {}, got %q", got)
	}
}
