package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-go/internal/engine"
	"claude-go/internal/services"
	"claude-go/internal/types"
)

func TestHookEventConstants(t *testing.T) {
	events := types.AllHookEvents()
	if len(events) < 26 {
		t.Errorf("expected at least 26 hook events, got %d", len(events))
	}

	// Test specific events
	if !types.IsHookEvent("PreToolUse") {
		t.Error("PreToolUse should be a valid hook event")
	}
	if !types.IsHookEvent("PostToolUse") {
		t.Error("PostToolUse should be a valid hook event")
	}
	if types.IsHookEvent("InvalidEvent") {
		t.Error("InvalidEvent should not be a valid hook event")
	}
}

func TestHookInputTypes(t *testing.T) {
	t.Run("PreToolUseHookInput", func(t *testing.T) {
		input := types.PreToolUseHookInput{
			BaseHookInput: types.BaseHookInput{
				SessionID:      "test-session",
				TranscriptPath: "/path/to/transcript",
				CWD:            "/home/user",
			},
			HookEventName: "PreToolUse",
			ToolName:      "Bash",
			ToolInput:     map[string]interface{}{"command": "ls"},
			ToolUseID:     "tool-123",
		}

		if input.GetHookEventName() != "PreToolUse" {
			t.Errorf("expected PreToolUse, got %s", input.GetHookEventName())
		}
		if input.GetSessionID() != "test-session" {
			t.Errorf("expected test-session, got %s", input.GetSessionID())
		}
	})

	t.Run("PostToolUseHookInput", func(t *testing.T) {
		input := types.PostToolUseHookInput{
			BaseHookInput: types.BaseHookInput{
				SessionID: "test-session",
			},
			HookEventName: "PostToolUse",
			ToolName:      "Read",
			ToolInput:     map[string]interface{}{"file_path": "/test.txt"},
			ToolResponse:  "file contents",
			ToolUseID:     "tool-456",
		}

		if input.GetHookEventName() != "PostToolUse" {
			t.Errorf("expected PostToolUse, got %s", input.GetHookEventName())
		}
	})

	t.Run("SessionEndHookInput", func(t *testing.T) {
		input := types.SessionEndHookInput{
			BaseHookInput: types.BaseHookInput{
				SessionID: "test-session",
			},
			HookEventName: "SessionEnd",
			Reason:        "user_exit",
		}

		if input.GetHookEventName() != "SessionEnd" {
			t.Errorf("expected SessionEnd, got %s", input.GetHookEventName())
		}
		if input.Reason != "user_exit" {
			t.Errorf("expected user_exit, got %s", input.Reason)
		}
	})
}

func TestHookJSONParsing(t *testing.T) {
	t.Run("SyncHookOutput", func(t *testing.T) {
		jsonStr := `{"continue": true, "suppressOutput": false}`
		var output types.SyncHookOutput
		if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if !output.Continue {
			t.Error("expected continue to be true")
		}
		if output.SuppressOutput {
			t.Error("expected suppressOutput to be false")
		}
	})

	t.Run("BlockingOutput", func(t *testing.T) {
		jsonStr := `{"continue": false, "stopReason": "Hook blocked execution", "decision": "block"}`
		var output types.SyncHookOutput
		if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if output.Continue {
			t.Error("expected continue to be false")
		}
		if output.Decision != "block" {
			t.Errorf("expected decision block, got %s", output.Decision)
		}
	})

	t.Run("AsyncHookOutput", func(t *testing.T) {
		jsonStr := `{"async": true}`
		var output types.AsyncHookOutput
		if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		if !output.Async {
			t.Error("expected async to be true")
		}
	})
}

func TestHookResultBlocking(t *testing.T) {
	tests := []struct {
		name     string
		results  []types.HookResult
		expected bool
	}{
		{
			name:     "empty results",
			results:  nil,
			expected: false,
		},
		{
			name: "continue true",
			results: []types.HookResult{
				{Continue: true},
			},
			expected: false,
		},
		{
			name: "continue false",
			results: []types.HookResult{
				{Continue: false},
			},
			expected: true,
		},
		{
			name: "blocking flag",
			results: []types.HookResult{
				{IsBlocking: true, Continue: true},
			},
			expected: true,
		},
		{
			name: "decision block",
			results: []types.HookResult{
				{Decision: "block", Continue: true},
			},
			expected: true,
		},
		{
			name: "mixed results with block",
			results: []types.HookResult{
				{Continue: true},
				{Decision: "block", Continue: true},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := services.HasBlockingResult(tt.results)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHookMatching(t *testing.T) {
	tests := []struct {
		pattern string
		query   string
		match   bool
	}{
		{"", "", true},
		{"*", "anything", true},
		{"Bash", "Bash", true},
		{"Bash", "Read", false},
		{"Bash*", "Bash", true},
		{"Bash*", "BashCommand", true},
		{"*Tool", "WriteTool", true},
		{"*Tool", "Bash", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.query, func(t *testing.T) {
			result := services.MatchesPattern(tt.pattern, tt.query)
			if result != tt.match {
				t.Errorf("pattern %q query %q: expected %v, got %v", tt.pattern, tt.query, tt.match, result)
			}
		})
	}
}

func TestHooksServiceBasics(t *testing.T) {
	// Create temp file for hooks
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)

	// Check initial state
	if !service.IsEnabled() {
		t.Error("service should be enabled by default")
	}

	// Test adding a hook
	hook := services.Hook{
		Event:       "PreToolUse",
		Source:      "test",
		Command:     "echo test",
		Enabled:     true,
		Matcher:     "Bash",
		TimeoutMs:   1000,
		Description: "Test hook",
	}

	service.Register(hook)

	hooks := service.List()
	if len(hooks) == 0 {
		t.Error("expected hooks to be registered")
	}

	// Test matching hooks
	matched := service.GetMatchingHooks("PreToolUse", "Bash")
	if len(matched) == 0 {
		t.Error("expected PreToolUse Bash hook to match")
	}

	// Test non-matching
	notMatched := service.GetMatchingHooks("PreToolUse", "Read")
	if len(notMatched) > 0 {
		t.Error("expected no match for Read tool")
	}
}

func TestHooksServiceTrigger(t *testing.T) {
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)

	// Add a hook that will run quickly
	hook := services.Hook{
		Event:     "test_event",
		Source:    "test",
		Command:   "echo 'hook output'",
		Enabled:   true,
		Matcher:   "*",
		TimeoutMs: 5000,
	}
	service.Register(hook)

	ctx := context.Background()
	event := engine.HookEvent{
		Name:   "test_event",
		Target: "test_target",
	}

	reports, err := service.Trigger(ctx, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(reports) == 0 {
		t.Error("expected hook to be triggered")
	}

	if reports[0].Result != "ok" {
		t.Errorf("expected ok result, got %s", reports[0].Result)
	}
}

func TestHookExecutorBasics(t *testing.T) {
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)

	// Add a hook
	hook := services.Hook{
		Event:     "PreToolUse",
		Source:    "test",
		Command:   "echo 'pre-tool'",
		Enabled:   true,
		Matcher:   "*",
		TimeoutMs: 5000,
	}
	service.Register(hook)

	// Create executor
	executor := service.CreateExecutor("/tmp", "test-session", "/tmp/transcript.jsonl")

	ctx := context.Background()
	results, err := executor.ExecutePreToolHooks(ctx, "Bash", "tool-123", map[string]string{"command": "ls"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Error("expected hook results")
	}
}

func TestHookSettingsManager(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "settings.json")

		mgr := services.NewHooksSettingsManager(configPath)
		if err := mgr.Load(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		hooks := mgr.GetHooksForEvent(types.HookEventPreToolUse)
		if len(hooks) != 0 {
			t.Error("expected no hooks from empty config")
		}
	})

	t.Run("config with hooks", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "settings.json")

		// Write config file
		config := map[string]interface{}{
			"hooks": map[string]interface{}{
				"PreToolUse": []interface{}{
					map[string]interface{}{
						"matcher": "Bash",
						"hooks": []interface{}{
							map[string]interface{}{
								"type":    "command",
								"command": "echo 'pre-bash'",
							},
						},
					},
				},
			},
		}

		data, _ := json.MarshalIndent(config, "", "  ")
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		mgr := services.NewHooksSettingsManager(configPath)
		if err := mgr.Load(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		hooks := mgr.GetHooksForEvent(types.HookEventPreToolUse)
		if len(hooks) != 1 {
			t.Errorf("expected 1 hook matcher, got %d", len(hooks))
		}
	})

	t.Run("add and save hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "settings.json")

		mgr := services.NewHooksSettingsManager(configPath)
		if err := mgr.Load(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		matcher := services.HookMatcherConfig{
			Matcher: "Read",
			Hooks: []services.HookDefinition{
				{Type: "command", Command: "echo 'pre-read'"},
			},
		}

		if err := mgr.AddHook(types.HookEventPreToolUse, matcher); err != nil {
			t.Errorf("unexpected error adding hook: %v", err)
		}

		// Verify it was saved
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Errorf("unexpected error reading config: %v", err)
		}

		var loaded services.HooksConfig
		if err := json.Unmarshal(data, &loaded); err != nil {
			t.Errorf("unexpected error parsing config: %v", err)
		}

		if len(loaded.Hooks["PreToolUse"]) != 1 {
			t.Error("expected hook to be saved")
		}
	})
}

func TestHookAttachmentCreation(t *testing.T) {
	t.Run("success attachment", func(t *testing.T) {
		result := types.HookResult{
			HookName:  "test-hook",
			HookEvent: "PreToolUse",
			ToolUseID: "tool-123",
			Output:    "hook output",
			Continue:  true,
		}

		att := types.HookResultToAttachment(result)
		if att.Type != types.AttachmentTypeHookSuccess {
			t.Errorf("expected success type, got %s", att.Type)
		}
	})

	t.Run("error attachment", func(t *testing.T) {
		result := types.HookResult{
			HookName:   "test-hook",
			HookEvent:  "PreToolUse",
			ToolUseID:  "tool-123",
			Error:      "hook failed",
			IsBlocking: true,
			Continue:   false,
		}

		att := types.HookResultToAttachment(result)
		if att.Type != types.AttachmentTypeHookBlockingError {
			t.Errorf("expected blocking error type, got %s", att.Type)
		}
		if att.HookError != "hook failed" {
			t.Errorf("expected hook error, got %s", att.HookError)
		}
	})

	t.Run("attachment message", func(t *testing.T) {
		results := []types.HookResult{
			{HookName: "hook1", HookEvent: "PreToolUse", Output: "output1", Continue: true},
			{HookName: "hook2", HookEvent: "PreToolUse", Output: "output2", Continue: true},
		}

		msg := types.HookResultsToAttachmentMessage("PreToolUse", results)
		if msg == nil {
			t.Fatal("expected attachment message")
		}

		if msg.Type != types.MessageTypeAttachment {
			t.Errorf("expected attachment type, got %s", msg.Type)
		}
	})
}

func TestHookTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)

	// Add a hook that will timeout (sleep for 5 seconds with 100ms timeout)
	hook := services.Hook{
		Event:     "timeout_test",
		Source:    "test",
		Command:   "sleep 5",
		Enabled:   true,
		Matcher:   "*",
		TimeoutMs: 100, // 100ms timeout
	}
	service.Register(hook)

	ctx := context.Background()
	event := engine.HookEvent{
		Name:   "timeout_test",
		Target: "test",
	}

	start := time.Now()
	reports, _ := service.Trigger(ctx, event)
	elapsed := time.Since(start)

	// Should timeout before 1 second
	if elapsed > time.Second {
		t.Errorf("hook should have timed out quickly, took %v", elapsed)
	}

	if len(reports) > 0 && reports[0].Result != "error" {
		t.Error("expected error result due to timeout")
	}
}

func TestHooksServiceTriggerStripsNULFromEnvValues(t *testing.T) {
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)
	service.Register(services.Hook{
		Event:     "nul_target_test",
		Source:    "test",
		Command:   "echo 'ok'",
		Enabled:   true,
		Matcher:   "*",
		TimeoutMs: 5000,
	})

	ctx := context.Background()
	reports, err := service.Trigger(ctx, engine.HookEvent{
		Name:   "nul_target_test",
		Target: "继续\x00",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) == 0 {
		t.Fatalf("expected reports")
	}
	if reports[0].Result != "ok" {
		t.Fatalf("expected hook to succeed, got result=%s error=%s", reports[0].Result, reports[0].Error)
	}
}

func TestHookExecutorStripsNULFromEnvValues(t *testing.T) {
	tmpDir := t.TempDir()
	hooksPath := filepath.Join(tmpDir, "hooks.json")

	service := services.CreateHooksService(hooksPath)
	service.Register(services.Hook{
		Event:     "UserPromptSubmit",
		Source:    "test",
		Command:   "echo 'ok'",
		Enabled:   true,
		Matcher:   "*",
		TimeoutMs: 5000,
	})

	executor := service.CreateExecutor("/tmp", "test-session", "/tmp/transcript.jsonl")
	ctx := context.Background()
	results, err := executor.ExecuteUserPromptSubmitHooks(ctx, "继续\x00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected hook results")
	}
	if results[0].Error != "" {
		t.Fatalf("expected no hook error, got %s", results[0].Error)
	}
}
