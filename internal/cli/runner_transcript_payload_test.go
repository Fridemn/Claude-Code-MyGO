package cli

import (
	"testing"
	"time"

	"claude-go/internal/types"
)

func TestBuildTranscriptEntriesUsesStructuredSystemContent(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Type:      types.MessageTypeSystem,
			Role:      types.RoleSystem,
			Content:   `{"type":"system","subtype":"stop_hook_summary","content":"2 stop hooks \u00b7 420ms","totalDurationMs":420}`,
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Subtype != "stop_hook_summary" {
		t.Fatalf("expected subtype stop_hook_summary, got %q", entries[0].Subtype)
	}
	if entries[0].Content != "2 stop hooks \u00b7 420ms" {
		t.Fatalf("expected structured content text, got %q", entries[0].Content)
	}
	if entries[0].Data == "" {
		t.Fatal("expected raw structured payload in Data")
	}
}

func TestBuildTranscriptEntriesSuppressesNoisyPostTurnAttachment(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Type:      types.MessageTypeAttachment,
			Role:      types.RoleSystem,
			Content:   `{"type":"hook_success","hookEvent":"post_turn","command":"echo post_turn","content":"post_turn"}`,
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 0 {
		t.Fatalf("expected noisy post_turn attachment to be suppressed, got %d entries", len(entries))
	}
}

func TestBuildTranscriptEntriesKeepsNonPostTurnAttachment(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Type:      types.MessageTypeAttachment,
			Role:      types.RoleSystem,
			Content:   `{"type":"hook_success","hookEvent":"post_tool","content":"tool output"}`,
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected non-post_turn attachment to remain, got %d entries", len(entries))
	}
	if entries[0].Kind != "attachment" || entries[0].Content != "tool output" {
		t.Fatalf("unexpected entry: %#v", entries[0])
	}
}

func TestBuildTranscriptEntriesSkipsMetaMessages(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Role:      types.RoleUser,
			Content:   "invisible meta",
			IsMeta:    true,
			Timestamp: time.Now(),
		},
		{
			Role:      types.RoleUser,
			Content:   "visible user",
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected only non-meta message to render, got %d entries", len(entries))
	}
	if entries[0].Kind != "user" || entries[0].Content != "visible user" {
		t.Fatalf("unexpected entry: %#v", entries[0])
	}
}

func TestBuildTranscriptEntriesRendersSystemLocalCommandAsCommand(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		buildLocalJSXSystemLocalCommandMessage("help", "--brief", "Done"),
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one local_command entry, got %d", len(entries))
	}
	if entries[0].Kind != "command" {
		t.Fatalf("expected command entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Title != "Command /help" {
		t.Fatalf("unexpected command title: %q", entries[0].Title)
	}
	if entries[0].Content != "Done" {
		t.Fatalf("unexpected command content: %q", entries[0].Content)
	}
}

func TestBuildTranscriptEntriesRendersSystemLocalCommandStderrAsCommand(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Role:      types.RoleSystem,
			Type:      types.SystemSubtypeLocalCommand,
			Content:   "<command-name>/help</command-name>\n<local-command-stderr>Failed</local-command-stderr>",
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one local_command entry, got %d", len(entries))
	}
	if entries[0].Kind != "command" {
		t.Fatalf("expected command entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Title != "Command /help" {
		t.Fatalf("unexpected command title: %q", entries[0].Title)
	}
	if entries[0].Content != "Failed" {
		t.Fatalf("unexpected command content: %q", entries[0].Content)
	}
}

func TestBuildTranscriptEntriesRendersUserLocalCommandStdoutAsCommand(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Role:      types.RoleUser,
			Content:   types.FormatCommandInputTags("help", ""),
			Timestamp: time.Now(),
		},
		{
			Role:      types.RoleUser,
			Content:   "<local-command-stdout>Done</local-command-stdout>",
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one combined user local-command entry, got %d", len(entries))
	}
	if entries[0].Kind != "command" {
		t.Fatalf("expected command entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Title != "Command /help" {
		t.Fatalf("unexpected command title: %q", entries[0].Title)
	}
	if entries[0].Content != "Done" {
		t.Fatalf("unexpected command content: %q", entries[0].Content)
	}
}

func TestBuildTranscriptEntriesRendersUserLocalCommandStderrAsCommand(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Role:      types.RoleUser,
			Content:   types.FormatCommandInputTags("help", ""),
			Timestamp: time.Now(),
		},
		{
			Role:      types.RoleUser,
			Content:   "<local-command-stderr>Failed</local-command-stderr>",
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one combined user local-command entry, got %d", len(entries))
	}
	if entries[0].Kind != "command" {
		t.Fatalf("expected command entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Title != "Command /help" {
		t.Fatalf("unexpected command title: %q", entries[0].Title)
	}
	if entries[0].Content != "Failed" {
		t.Fatalf("unexpected command content: %q", entries[0].Content)
	}
}

func TestBuildTranscriptEntriesRendersProgressMessage(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Type:       types.MessageTypeProgress,
			Role:       types.RoleSystem,
			ToolCallID: "repl-tool-1",
			Content:    `{"type":"repl_tool_call","phase":"start","toolName":"REPL"}`,
			Timestamp:  time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one progress entry, got %d", len(entries))
	}
	if entries[0].Kind != "progress" {
		t.Fatalf("expected progress entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Subtype != "repl_tool_call" {
		t.Fatalf("expected repl_tool_call subtype, got %q", entries[0].Subtype)
	}
	if entries[0].Content != "REPL started" {
		t.Fatalf("unexpected progress content: %q", entries[0].Content)
	}
	if entries[0].ToolUseID != "repl-tool-1" {
		t.Fatalf("unexpected progress tool_use_id: %q", entries[0].ToolUseID)
	}
}

func TestBuildTranscriptEntriesRendersProgressMessageFromToolUseResultData(t *testing.T) {
	t.Parallel()

	messages := []types.Message{
		{
			Type:       types.MessageTypeProgress,
			Role:       types.RoleSystem,
			ToolCallID: "repl-tool-2",
			ToolUseResult: map[string]any{
				"toolUseID": "repl-tool-2",
				"data": map[string]any{
					"type":     "repl_tool_call",
					"phase":    "end",
					"toolName": "REPL",
					"status":   "error",
					"error":    "not implemented",
				},
			},
			Timestamp: time.Now(),
		},
	}

	entries := buildTranscriptEntries(messages, "")
	if len(entries) != 1 {
		t.Fatalf("expected one progress entry, got %d", len(entries))
	}
	if entries[0].Kind != "progress" {
		t.Fatalf("expected progress entry kind, got %q", entries[0].Kind)
	}
	if entries[0].Content != "REPL failed: not implemented" {
		t.Fatalf("unexpected progress content: %q", entries[0].Content)
	}
	if entries[0].Data == "" {
		t.Fatal("expected structured progress payload in Data")
	}
}
