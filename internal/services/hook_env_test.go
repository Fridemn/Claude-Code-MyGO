package services

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/engine"
	"claude-go/internal/types"
)

func TestBuildHookEnvVarStripsNUL(t *testing.T) {
	got := buildHookEnvVar("CLAUDE_CODE_HOOK_TARGET", "继续\x00")
	if strings.ContainsRune(got, '\x00') {
		t.Fatalf("expected NUL to be removed, got %q", got)
	}
	if got != "CLAUDE_CODE_HOOK_TARGET=继续" {
		t.Fatalf("unexpected env var value: %q", got)
	}
}

func TestRunHookCommandHandlesNULTarget(t *testing.T) {
	out, err := runHookCommand(context.Background(), Hook{
		Command: "echo ok",
		Shell:   "bash",
	}, engine.HookEvent{
		Name:   "Stop",
		Target: "继续\x00",
	})
	if err != nil {
		t.Fatalf("expected hook command to run, got error: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected hook output, got %q", out)
	}
}

func TestHookExecutorRunHookCommandHandlesNULInput(t *testing.T) {
	exec := &HookExecutor{
		cwd:            ".",
		sessionID:      "s\x00ession",
		transcriptPath: "/tmp/t\x00ranscript.jsonl",
	}
	out, err := exec.runHookCommand(context.Background(), Hook{
		Command: "echo ok",
		Shell:   "bash",
		Event:   "UserPromptSubmit",
	}, string([]byte(`{"prompt":"继续\x00"}`)))
	if err != nil {
		t.Fatalf("expected hook command to run, got error: %v", err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected hook output, got %q", out)
	}
}

func TestExecuteUserPromptSubmitHooksHandlesNULPrompt(t *testing.T) {
	service := CreateHooksService(filepath.Join(t.TempDir(), "hooks.json"))
	service.Register(Hook{
		Event:   "UserPromptSubmit",
		Source:  "test",
		Command: "echo ok",
		Enabled: true,
		Matcher: "*",
	})
	executor := &HookExecutor{
		service:        service,
		cwd:            ".",
		sessionID:      "session",
		transcriptPath: "/tmp/transcript.jsonl",
	}
	results, err := executor.ExecuteHooks(context.Background(), types.UserPromptSubmitHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      "session",
			TranscriptPath: "/tmp/transcript.jsonl",
			CWD:            ".",
		},
		HookEventName: "UserPromptSubmit",
		Prompt:        "继续\x00",
	}, "", 1000)
	if err != nil {
		t.Fatalf("expected hook execution to succeed, got error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected hook result")
	}
	if results[0].Error != "" {
		t.Fatalf("expected no hook error, got %q", results[0].Error)
	}
}
