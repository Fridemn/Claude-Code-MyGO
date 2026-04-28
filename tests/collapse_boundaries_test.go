package tests

import (
	"context"
	"testing"
	"time"

	"claude-go/internal/tool"
	"claude-go/internal/ui"
	"claude-go/internal/ui/collapse"
)

type collapsibleReadTool struct{}

func (collapsibleReadTool) Name() string               { return "Read" }
func (collapsibleReadTool) Description() string        { return "read file" }
func (collapsibleReadTool) IsReadOnly(tool.Input) bool { return true }
func (collapsibleReadTool) Call(context.Context, tool.Input, tool.Runtime) (tool.Result, error) {
	return tool.Result{}, nil
}
func (collapsibleReadTool) IsSearchOrReadCommand(input tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible: true,
		IsRead:        true,
	}
}

func TestCollapseAbsorbsRelevantMemoriesAndHookTiming(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(collapsibleReadTool{})

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "tool-1",
			ToolInput: `{"file_path":"src/main.ts"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "tool-1",
			UUID:      "e2",
			Timestamp: time.Now(),
		},
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			Data:      `{"totalDurationMs":420,"hookInfos":[{"durationMs":111}]}`,
			UUID:      "e3",
			Timestamp: time.Now(),
		},
		{
			Kind:      "attachment",
			Subtype:   "relevant_memories",
			Data:      `{"type":"relevant_memories","memories":[{"path":"m1"},{"path":"m2"}]}`,
			UUID:      "e4",
			Timestamp: time.Now(),
		},
	}

	out := collapse.ReadSearchGroups(entries, reg, false)
	if len(out) != 1 {
		t.Fatalf("expected one collapsed entry, got %d", len(out))
	}
	if out[0].Kind != "collapsed" {
		t.Fatalf("expected collapsed entry, got %s", out[0].Kind)
	}
	if out[0].Meta.HookCount != 1 {
		t.Fatalf("expected hook count 1, got %d", out[0].Meta.HookCount)
	}
	if out[0].Meta.HookTotalMs != 420 {
		t.Fatalf("expected hook total ms 420, got %d", out[0].Meta.HookTotalMs)
	}
	if out[0].Meta.MemoryReadCount != 2 {
		t.Fatalf("expected memory read count 2 (absorbed relevant memories), got %d", out[0].Meta.MemoryReadCount)
	}
}

func TestCollapseAccumulatesMultipleHookSummariesAndAttachmentMemories(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(collapsibleReadTool{})

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "tool-1",
			ToolInput: `{"file_path":"src/main.ts"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "tool-1",
			UUID:      "e2",
			Timestamp: time.Now(),
		},
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			Data:      `{"hookInfos":[{"durationMs":75},{"durationMs":25}]}`,
			UUID:      "e3",
			Timestamp: time.Now(),
		},
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			Data:      `{"totalDurationMs":200}`,
			UUID:      "e4",
			Timestamp: time.Now(),
		},
		{
			Kind:      "attachment",
			Subtype:   "relevant_memories",
			Data:      `{"attachments":[{"type":"relevant_memories","memories":[{"path":"m1"},{"path":"m2"},{"path":"m3"}]}]}`,
			UUID:      "e5",
			Timestamp: time.Now(),
		},
	}

	out := collapse.ReadSearchGroups(entries, reg, false)
	if len(out) != 1 {
		t.Fatalf("expected one collapsed entry, got %d", len(out))
	}
	if out[0].Kind != "collapsed" {
		t.Fatalf("expected collapsed entry, got %s", out[0].Kind)
	}
	if out[0].Meta.HookCount != 2 {
		t.Fatalf("expected hook count 2, got %d", out[0].Meta.HookCount)
	}
	if out[0].Meta.HookTotalMs != 300 {
		t.Fatalf("expected total hook ms 300, got %d", out[0].Meta.HookTotalMs)
	}
	if out[0].Meta.MemoryReadCount != 3 {
		t.Fatalf("expected memory read count 3, got %d", out[0].Meta.MemoryReadCount)
	}
}

func TestCollapseKeepsNestedMemoryAttachmentBeforeCollapsedSummary(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(collapsibleReadTool{})

	nested := ui.TranscriptEntry{
		Kind:      "attachment",
		Subtype:   "nested_memory",
		Content:   "loaded memory lines",
		UUID:      "nested",
		Timestamp: time.Now(),
	}

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "tool-1",
			ToolInput: `{"file_path":"src/main.ts"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		nested,
		{
			Kind:      "tool_result",
			ToolUseID: "tool-1",
			UUID:      "e2",
			Timestamp: time.Now(),
		},
		{
			Kind:      "assistant",
			Content:   "done",
			UUID:      "e3",
			Timestamp: time.Now(),
		},
	}

	out := collapse.ReadSearchGroups(entries, reg, false)
	if len(out) < 2 {
		t.Fatalf("expected nested attachment and collapsed entry, got %d entries", len(out))
	}
	if out[0].Kind != "attachment" || out[0].Subtype != "nested_memory" {
		t.Fatalf("expected nested_memory attachment first, got kind=%s subtype=%s", out[0].Kind, out[0].Subtype)
	}
	if out[1].Kind != "collapsed" {
		t.Fatalf("expected collapsed entry after nested memory, got %s", out[1].Kind)
	}
}
