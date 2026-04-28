package tests

import (
	"testing"
	"time"

	"claude-go/internal/tool"
	"claude-go/internal/tool/repl"
	"claude-go/internal/ui"
	"claude-go/internal/ui/collapse"
)

func TestCollapseREPLWrapperIsAbsorbedWithoutBreakingReadGroups(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(repl.REPLTool{})
	reg.Register(collapsibleReadTool{})

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  repl.REPLToolName,
			ToolUseID: "repl-1",
			ToolInput: `{"script":"Read('README.md')"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "repl-1",
			UUID:      "e2",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read-1",
			ToolInput: `{"file_path":"README.md"}`,
			UUID:      "e3",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "read-1",
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
	if out[0].Meta.ReadCount != 1 {
		t.Fatalf("expected read count 1, got %d", out[0].Meta.ReadCount)
	}
	if out[0].Meta.SearchCount != 0 || out[0].Meta.ListCount != 0 {
		t.Fatalf("expected only read count to be incremented, got search=%d list=%d", out[0].Meta.SearchCount, out[0].Meta.ListCount)
	}
	if len(out[0].Meta.GroupMessages) != 4 {
		t.Fatalf("expected REPL wrapper messages to be preserved inside group, got %d messages", len(out[0].Meta.GroupMessages))
	}
}

func TestCollapseUsesREPLPrimitiveFallbackWhenRegistryHidesPrimitive(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(repl.REPLTool{})

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read-1",
			ToolInput: `{"file_path":"README.md"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "read-1",
			UUID:      "e2",
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
	if out[0].Meta.ReadCount != 1 {
		t.Fatalf("expected read count 1, got %d", out[0].Meta.ReadCount)
	}
}

func TestCollapseAbsorbsReplProgressHint(t *testing.T) {
	t.Parallel()

	reg := tool.EmptyRegistry()
	reg.Register(repl.REPLTool{})
	reg.Register(collapsibleReadTool{})

	entries := []ui.TranscriptEntry{
		{
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read-1",
			ToolInput: `{"file_path":"README.md"}`,
			UUID:      "e1",
			Timestamp: time.Now(),
		},
		{
			Kind:      "progress",
			ToolUseID: "read-1",
			Data:      `{"type":"repl_tool_call","phase":"start","toolName":"Read","toolInput":{"file_path":"src/main.go"}}`,
			UUID:      "e2",
			Timestamp: time.Now(),
		},
		{
			Kind:      "tool_result",
			ToolUseID: "read-1",
			UUID:      "e3",
			Timestamp: time.Now(),
		},
	}

	out := collapse.ReadSearchGroups(entries, reg, false)
	if len(out) != 1 || out[0].Kind != "collapsed" {
		t.Fatalf("expected one collapsed entry, got %#v", out)
	}
	if out[0].Meta.DisplayHint != "src/main.go" {
		t.Fatalf("expected progress hint to be absorbed into display hint, got %q", out[0].Meta.DisplayHint)
	}
}
