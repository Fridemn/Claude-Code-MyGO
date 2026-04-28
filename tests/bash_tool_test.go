package tests

import (
	bashtool "claude-go/internal/tool/bash"

	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claude-go/internal/tool"
)

func TestBashToolName(t *testing.T) {
	bash := bashtool.BashTool{}
	if bash.Name() != "Bash" {
		t.Errorf("expected name 'Bash', got '%s'", bash.Name())
	}
}

func TestBashToolDescription(t *testing.T) {
	bash := bashtool.BashTool{}
	desc := bash.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestBashToolIsReadOnly(t *testing.T) {
	bash := bashtool.BashTool{}

	tests := []struct {
		command  string
		readOnly bool
	}{
		{"ls -la", true},
		{"cat file.txt", true},
		{"grep pattern file", true},
		{"find . -name '*.go'", true},
		{"echo hello", false},
		{"rm file.txt", false},
		{"mv a b", false},
		{"npm install", false},
	}

	for _, tt := range tests {
		in := tool.Input{"command": tt.command}
		if got := bash.IsReadOnly(in); got != tt.readOnly {
			t.Errorf("IsReadOnly(%q) = %v, want %v", tt.command, got, tt.readOnly)
		}
	}
}

func TestBashToolReadOnlyAndSyncExecution(t *testing.T) {
	bashtool.SetGlobalPermissionMode(bashtool.PermissionModeBypassPermissions)
	bashtool.SetSandboxingEnabled(false)
	t.Cleanup(func() {
		bashtool.SetGlobalPermissionMode(bashtool.PermissionModeAsk)
		bashtool.SetSandboxingEnabled(true)
	})

	bash := bashtool.BashTool{}
	if !bash.IsReadOnly(tool.Input{"command": "cat README.md"}) {
		t.Fatalf("expected cat to be read-only")
	}
	if bash.IsReadOnly(tool.Input{"command": "rm -rf /tmp/demo"}) {
		t.Fatalf("expected rm not to be read-only")
	}

	result, err := bash.Call(context.Background(), tool.Input{
		"command": "printf hello",
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("bash call: %v", err)
	}
	output, ok := result.Content.(bashtool.BashOutput)
	if !ok {
		t.Fatalf("unexpected bash output: %#v", result.Content)
	}
	if output.Stdout != "hello" {
		t.Fatalf("unexpected stdout: %q", output.Stdout)
	}
	if output.ReturnCode != 0 {
		t.Fatalf("unexpected return code: %d", output.ReturnCode)
	}
}

func TestBashToolCallWithTimeout(t *testing.T) {
	bashtool.SetGlobalPermissionMode(bashtool.PermissionModeBypassPermissions)
	bashtool.SetSandboxingEnabled(false)
	t.Cleanup(func() {
		bashtool.SetGlobalPermissionMode(bashtool.PermissionModeAsk)
		bashtool.SetSandboxingEnabled(true)
	})

	bash := bashtool.BashTool{}
	result, err := bash.Call(context.Background(), tool.Input{
		"command": "sleep 1",
		"timeout": float64(100),
	}, tool.Runtime{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output, ok := result.Content.(bashtool.BashOutput)
	if !ok {
		t.Fatalf("expected BashOutput, got %T", result.Content)
	}

	if !output.Interrupted {
		t.Error("expected command to be interrupted")
	}
}

func TestBashToolSecurityWarningPreventsExecution(t *testing.T) {
	bashtool.SetGlobalPermissionMode(bashtool.PermissionModeBypassPermissions)
	bashtool.SetSandboxingEnabled(false)
	t.Cleanup(func() {
		bashtool.SetGlobalPermissionMode(bashtool.PermissionModeAsk)
		bashtool.SetSandboxingEnabled(true)
	})

	result, err := (bashtool.BashTool{}).Call(context.Background(), tool.Input{
		"command": "rm -rf /tmp/demo",
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("bash call: %v", err)
	}
	output := result.Content.(bashtool.BashOutput)
	if !strings.Contains(output.Stderr, "security warning") {
		t.Fatalf("expected security warning, got %#v", output)
	}
}

func TestBashToolCallEcho(t *testing.T) {
	bashtool.SetGlobalPermissionMode(bashtool.PermissionModeBypassPermissions)
	bashtool.SetSandboxingEnabled(false)
	t.Cleanup(func() {
		bashtool.SetGlobalPermissionMode(bashtool.PermissionModeAsk)
		bashtool.SetSandboxingEnabled(true)
	})

	bash := bashtool.BashTool{}
	result, err := bash.Call(context.Background(), tool.Input{
		"command": "echo hello",
	}, tool.Runtime{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output, ok := result.Content.(bashtool.BashOutput)
	if !ok {
		t.Fatalf("expected BashOutput, got %T", result.Content)
	}

	if !strings.Contains(output.Stdout, "hello") {
		t.Errorf("expected 'hello' in stdout, got %q", output.Stdout)
	}
}

func TestPermissionChecker(t *testing.T) {
	checker := bashtool.CreatePermissionChecker(bashtool.PermissionModeAsk)

	result := checker.Check("ls -la", "list files")
	if !result.Allowed {
		t.Fatalf("expected read-only ls to be auto-allowed in ask mode, got %#v", result)
	}

	// Test bypass mode
	checker.SetMode(bashtool.PermissionModeBypassPermissions)
	result = checker.Check("rm -rf /", "test")
	if !result.Allowed {
		t.Error("expected command to be allowed in bypass mode")
	}

	// Test limit tools mode
	checker.SetMode(bashtool.PermissionModeLimitTools)
	result = checker.Check("ls", "list files")
	if !result.Allowed {
		t.Error("expected ls to be allowed in limitTools mode")
	}

	// Test ask mode
	checker.SetMode(bashtool.PermissionModeAsk)
	result = checker.Check("rm file", "remove file")
	if result.Allowed {
		t.Error("expected rm to require permission in ask mode")
	}
}

func TestSandboxManager(t *testing.T) {
	manager := bashtool.CreateSandboxManager()

	if !manager.IsEnabled() {
		t.Error("expected sandbox to be enabled by default")
	}

	manager.SetEnabled(false)
	if manager.IsEnabled() {
		t.Error("expected sandbox to be disabled")
	}

	manager.SetEnabled(true)
	manager.SetDeniedPaths([]string{"/etc/shadow", "/.ssh/"})

	result := manager.Execute(context.Background(), "cat /etc/shadow", "/")
	if result.Allowed {
		t.Error("expected /etc/shadow access to be denied")
	}
	if result.Violation == nil {
		t.Error("expected violation for /etc/shadow access")
	}
}

func TestBashHelpersPipelineStreamAndFileTracking(t *testing.T) {
	bashtool.SetGlobalPermissionMode(bashtool.PermissionModeBypassPermissions)
	bashtool.SetSandboxingEnabled(false)
	t.Cleanup(func() {
		bashtool.SetGlobalPermissionMode(bashtool.PermissionModeAsk)
		bashtool.SetSandboxingEnabled(true)
	})

	pipeline := bashtool.CreatePipeline().Add("printf hello").Add("wc -c").Build()
	if pipeline != "printf hello | wc -c" {
		t.Fatalf("unexpected pipeline: %q", pipeline)
	}

	var stdoutLines []string
	var stderrLines []string
	var completeStdout string
	var completeStderr string
	streamErr := bashtool.StreamCommand(context.Background(), bashtool.StreamConfig{
		Command: `printf "one\ntwo\n"; printf "warn\n" 1>&2`,
		Cwd:     ".",
		Timeout: time.Second,
		OnStdout: func(line string) {
			stdoutLines = append(stdoutLines, line)
		},
		OnStderr: func(line string) {
			stderrLines = append(stderrLines, line)
		},
		OnComplete: func(stdout, stderr string, err error) {
			completeStdout = stdout
			completeStderr = stderr
		},
	})
	if streamErr != nil {
		t.Fatalf("stream command: %v", streamErr)
	}
	if len(stdoutLines) != 2 || stdoutLines[0] != "one" || stdoutLines[1] != "two" {
		t.Fatalf("unexpected streamed stdout: %#v", stdoutLines)
	}
	if len(stderrLines) != 1 || stderrLines[0] != "warn" {
		t.Fatalf("unexpected streamed stderr: %#v", stderrLines)
	}
	if !strings.Contains(completeStdout, "one\ntwo\n") || !strings.Contains(completeStderr, "warn\n") {
		t.Fatalf("unexpected completion output: stdout=%q stderr=%q", completeStdout, completeStderr)
	}

	root := t.TempDir()
	filePath := filepath.Join(root, "tracked.txt")
	output, states, err := bashtool.RunWithFileTracking(context.Background(), `printf changed > tracked.txt`, root, []string{filePath})
	if err != nil {
		t.Fatalf("run with file tracking: %v", err)
	}
	if output.Interrupted {
		t.Fatalf("unexpected interrupted output: %#v", output)
	}
	if _, ok := states[filePath]; !ok {
		t.Fatalf("expected tracked file state, got %#v", states)
	}
}
