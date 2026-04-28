package tests

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"claude-go/internal/ui"
	"claude-go/internal/ui/collapse"
)

func TestCollapseTeammateShutdownsBatchesConsecutive(t *testing.T) {
	t.Parallel()

	makeTaskStatus := func(uuid string) ui.TranscriptEntry {
		return ui.TranscriptEntry{
			Kind:      "attachment",
			Title:     "Attachment",
			Content:   "teammate done",
			UUID:      uuid,
			Timestamp: time.Now(),
			Subtype:   "task_status",
			Data:      `{"type":"task_status","taskType":"in_process_teammate","status":"completed"}`,
		}
	}

	entries := []ui.TranscriptEntry{
		makeTaskStatus("a1"),
		makeTaskStatus("a2"),
		makeTaskStatus("a3"),
		{
			Kind:      "assistant",
			Content:   "separator",
			UUID:      "s1",
			Timestamp: time.Now(),
		},
		makeTaskStatus("a4"),
	}

	out := collapse.TeammateShutdowns(entries)
	if len(out) != 3 {
		t.Fatalf("expected 3 entries after collapse, got %d", len(out))
	}

	if out[0].Kind != "attachment" || out[0].Subtype != "teammate_shutdown_batch" {
		t.Fatalf("expected first entry to be teammate_shutdown_batch, got kind=%q subtype=%q", out[0].Kind, out[0].Subtype)
	}
	if out[0].UUID != "a1" {
		t.Fatalf("expected batch to keep first UUID a1, got %q", out[0].UUID)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out[0].Data), &payload); err != nil {
		t.Fatalf("expected valid batch payload JSON, got error: %v", err)
	}
	if got := payload["type"]; got != "teammate_shutdown_batch" {
		t.Fatalf("expected payload type teammate_shutdown_batch, got %#v", got)
	}
	if got := int(payload["count"].(float64)); got != 3 {
		t.Fatalf("expected payload count 3, got %d", got)
	}

	if out[2].UUID != "a4" || out[2].Subtype != "task_status" {
		t.Fatalf("expected trailing single task_status to remain unchanged, got %#v", out[2])
	}
}

func TestCollapseHookSummariesMergesSameHookLabel(t *testing.T) {
	t.Parallel()

	entries := []ui.TranscriptEntry{
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			UUID:      "h1",
			Timestamp: time.Now(),
			Data:      `{"type":"system","subtype":"stop_hook_summary","hookLabel":"PostToolUse","hookCount":1,"hookInfos":[{"hookName":"a"}],"hookErrors":["e1"],"preventedContinuation":false,"hasOutput":false,"totalDurationMs":120,"content":"first"}`,
		},
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			UUID:      "h2",
			Timestamp: time.Now(),
			Data:      `{"type":"system","subtype":"stop_hook_summary","hookLabel":"PostToolUse","hookCount":2,"hookInfos":[{"hookName":"b"},{"hookName":"c"}],"hookErrors":["e2"],"preventedContinuation":true,"hasOutput":true,"totalDurationMs":240,"content":"second"}`,
		},
		{
			Kind:      "system",
			Subtype:   "stop_hook_summary",
			UUID:      "h3",
			Timestamp: time.Now(),
			Data:      `{"type":"system","subtype":"stop_hook_summary","hookLabel":"SubagentStop","hookCount":1,"hookInfos":[],"hookErrors":[],"preventedContinuation":false,"hasOutput":false,"totalDurationMs":30,"content":"third"}`,
		},
	}

	out := collapse.HookSummaries(entries)
	if len(out) != 2 {
		t.Fatalf("expected 2 entries after hook collapse, got %d", len(out))
	}
	if out[0].UUID != "h1" {
		t.Fatalf("expected merged entry to preserve first UUID h1, got %q", out[0].UUID)
	}
	if out[1].UUID != "h3" {
		t.Fatalf("expected different hookLabel entry to remain separate, got %q", out[1].UUID)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out[0].Data), &payload); err != nil {
		t.Fatalf("expected valid merged payload JSON, got error: %v", err)
	}

	if got := int(payload["hookCount"].(float64)); got != 3 {
		t.Fatalf("expected merged hookCount=3, got %d", got)
	}
	if got := len(payload["hookInfos"].([]any)); got != 3 {
		t.Fatalf("expected merged hookInfos length=3, got %d", got)
	}
	if got := len(payload["hookErrors"].([]any)); got != 2 {
		t.Fatalf("expected merged hookErrors length=2, got %d", got)
	}
	if got := payload["preventedContinuation"].(bool); !got {
		t.Fatalf("expected preventedContinuation=true, got %v", got)
	}
	if got := payload["hasOutput"].(bool); !got {
		t.Fatalf("expected hasOutput=true, got %v", got)
	}
	if got := int(payload["totalDurationMs"].(float64)); got != 240 {
		t.Fatalf("expected merged totalDurationMs=max(120,240)=240, got %d", got)
	}
}

func TestCollapseBackgroundBashNotificationsMergesCompletedOnly(t *testing.T) {
	t.Parallel()

	notification := func(status, summary string) string {
		return "<task-notification>" +
			"<status>" + status + "</status>" +
			"<summary>" + summary + "</summary>" +
			"</task-notification>"
	}

	entries := []ui.TranscriptEntry{
		{
			Kind:      "user",
			UUID:      "n1",
			Timestamp: time.Now(),
			Content:   notification("completed", `Background command "npm test" completed`),
		},
		{
			Kind:      "user",
			UUID:      "n2",
			Timestamp: time.Now(),
			Content:   notification("completed", `Background command "go test ./..." completed`),
		},
		{
			Kind:      "user",
			UUID:      "n3",
			Timestamp: time.Now(),
			Content:   notification("completed", `Monitor "build" stream ended`),
		},
		{
			Kind:      "user",
			UUID:      "n4",
			Timestamp: time.Now(),
			Content:   notification("failed", `Background command "npm test" failed`),
		},
	}

	out := collapse.BackgroundBashNotifications(entries)
	if len(out) != 3 {
		t.Fatalf("expected 3 entries after collapse, got %d", len(out))
	}
	if out[0].UUID != "n1" {
		t.Fatalf("expected collapsed notification to preserve first UUID n1, got %q", out[0].UUID)
	}
	if !strings.Contains(out[0].Content, "<summary>2 background commands completed</summary>") {
		t.Fatalf("expected merged summary for 2 commands, got %q", out[0].Content)
	}
	if out[1].UUID != "n3" || out[2].UUID != "n4" {
		t.Fatalf("expected non-matching notifications to remain unchanged, got UUIDs %q and %q", out[1].UUID, out[2].UUID)
	}
}
