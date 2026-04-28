package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/task"
	"claude-go/internal/tool"
	"claude-go/internal/tool/bash"
	"claude-go/internal/tool/repl"
)

type stubTaskStore struct{}

func (stubTaskStore) List() []*task.AgentTask            { return nil }
func (stubTaskStore) Get(string) (*task.AgentTask, bool) { return nil, false }

func TestIsModeEnabled(t *testing.T) {
	tests := []struct {
		name       string
		codeRepl   string
		replMode   string
		userType   string
		entrypoint string
		want       bool
	}{
		{
			name:       "explicit opt-out wins",
			codeRepl:   "0",
			replMode:   "1",
			userType:   "ant",
			entrypoint: "cli",
			want:       false,
		},
		{
			name:     "legacy env forces mode on",
			replMode: "1",
			want:     true,
		},
		{
			name:       "ant cli defaults to enabled",
			userType:   "ant",
			entrypoint: "cli",
			want:       true,
		},
		{
			name:       "ant non-cli stays disabled",
			userType:   "ant",
			entrypoint: "sdk-cli",
			want:       false,
		},
		{
			name: "external defaults to disabled",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_REPL", tc.codeRepl)
			t.Setenv("CLAUDE_REPL_MODE", tc.replMode)
			t.Setenv("USER_TYPE", tc.userType)
			t.Setenv("CLAUDE_CODE_ENTRYPOINT", tc.entrypoint)

			got := repl.IsModeEnabled()
			if got != tc.want {
				t.Fatalf("IsModeEnabled() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestREPLToolParametersSchema_RequiresScript(t *testing.T) {
	t.Parallel()

	schema := (repl.REPLTool{}).ParametersSchema()
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("expected required field list, got %T", schema["required"])
	}
	if len(required) != 1 || required[0] != "script" {
		t.Fatalf("expected required=[script], got %#v", required)
	}
}

func TestREPLToolCall_ModeDisabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "0")
	t.Setenv("CLAUDE_REPL_MODE", "")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	out, err := (repl.REPLTool{}).Call(context.Background(), tool.Input{
		"script": "console.log('ok')",
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if out.Error == "" || !strings.Contains(out.Error, "disabled") {
		t.Fatalf("expected disabled error, got %#v", out)
	}
	if got := out.Meta["code"]; got != "mode_disabled" {
		t.Fatalf("expected meta code mode_disabled, got %#v", got)
	}
}

func TestREPLToolCall_Validation(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "")
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	cases := []struct {
		name  string
		input tool.Input
		want  string
	}{
		{
			name:  "missing script",
			input: tool.Input{},
			want:  "missing required field: script",
		},
		{
			name: "non-string script",
			input: tool.Input{
				"script": 123,
			},
			want: "invalid field type for script",
		},
		{
			name: "empty script",
			input: tool.Input{
				"script": "   ",
			},
			want: "script must be a non-empty string",
		},
		{
			name: "invalid background type",
			input: tool.Input{
				"script":     "console.log('ok')",
				"background": "true",
			},
			want: "invalid field type for background",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := (repl.REPLTool{}).Call(context.Background(), tc.input, tool.Runtime{})
			if err != nil {
				t.Fatalf("Call returned error: %v", err)
			}
			if !strings.Contains(out.Error, tc.want) {
				t.Fatalf("expected error to contain %q, got %#v", tc.want, out)
			}
		})
	}
}

func TestREPLToolCall_UnsupportedScriptClassified(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "")
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	script := "console.log('hello')"
	out, err := (repl.REPLTool{}).Call(context.Background(), tool.Input{
		"script":     script,
		"background": true,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if out.Error == "" || !strings.Contains(out.Error, "supported primitive") {
		t.Fatalf("expected unsupported script error, got %#v", out)
	}
	if got := out.Meta["code"]; got != "unsupported_script" {
		t.Fatalf("expected meta code unsupported_script, got %#v", got)
	}
	if got := out.Meta["background"]; got != true {
		t.Fatalf("expected meta background true, got %#v", got)
	}
	if got := out.Meta["script_length"]; got != len(script) {
		t.Fatalf("expected meta script_length=%d, got %#v", len(script), got)
	}
}

func TestREPLToolCall_ExecutesReadPrimitive(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "")
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	dir := t.TempDir()
	target := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(target, []byte("hello repl\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out, err := (repl.REPLTool{}).Call(context.Background(), tool.Input{
		"script": `Read({"file_path":"` + target + `"})`,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if strings.TrimSpace(out.Error) != "" {
		t.Fatalf("expected success, got %#v", out)
	}
	if out.Meta == nil {
		t.Fatalf("expected meta for wrapped tool, got %#v", out)
	}
	if got := out.Meta["wrapped_tool_name"]; got != "Read" {
		t.Fatalf("expected wrapped tool Read, got %#v", got)
	}
	wrappedInput, ok := out.Meta["wrapped_tool_input"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapped_tool_input map, got %#v", out.Meta["wrapped_tool_input"])
	}
	if got, _ := wrappedInput["file_path"].(string); got != target {
		t.Fatalf("expected wrapped file_path %q, got %#v", target, wrappedInput["file_path"])
	}
}

func TestREPLToolCall_ExecutesReadPrimitive_StringShorthand(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "")
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	dir := t.TempDir()
	target := filepath.Join(dir, "simple.txt")
	if err := os.WriteFile(target, []byte("hello shorthand\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	out, err := (repl.REPLTool{}).Call(context.Background(), tool.Input{
		"script": `Read("` + target + `")`,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if strings.TrimSpace(out.Error) != "" {
		t.Fatalf("expected success, got %#v", out)
	}
	if got := out.Meta["wrapped_tool_name"]; got != "Read" {
		t.Fatalf("expected wrapped tool Read, got %#v", got)
	}
}

func TestREPLToolCall_BackgroundBashRunsAsync(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REPL", "")
	t.Setenv("CLAUDE_REPL_MODE", "1")
	t.Setenv("USER_TYPE", "")
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "")

	previousSandbox := bash.IsSandboxingEnabled()
	bash.SetSandboxingEnabled(false)
	defer bash.SetSandboxingEnabled(previousSandbox)

	out, err := (repl.REPLTool{}).Call(context.Background(), tool.Input{
		"script":     `{"tool":"Bash","input":{"command":"echo repl"}}`,
		"background": true,
	}, tool.Runtime{Tasks: stubTaskStore{}})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	if strings.TrimSpace(out.Error) != "" {
		t.Fatalf("expected success, got %#v", out)
	}
	wrappedInput, ok := out.Meta["wrapped_tool_input"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapped_tool_input map, got %#v", out.Meta["wrapped_tool_input"])
	}
	if runInBackground, _ := wrappedInput["run_in_background"].(bool); !runInBackground {
		t.Fatalf("expected wrapped run_in_background=true, got %#v", wrappedInput["run_in_background"])
	}

	content, ok := out.Content.(bash.BashOutput)
	if !ok {
		t.Fatalf("expected bash output content, got %#v", out.Content)
	}
	if strings.TrimSpace(content.BackgroundTaskID) == "" {
		t.Fatalf("expected background task id, got %#v", content)
	}
}

func TestREPLToolSearchReadClassification_Absorbed(t *testing.T) {
	t.Parallel()

	classification := (repl.REPLTool{}).IsSearchOrReadCommand(tool.Input{
		"script": "Read('README.md')",
	})
	if !classification.IsCollapsible {
		t.Fatalf("expected REPL wrapper to be collapsible for group continuity")
	}
	if !classification.IsAbsorbedSilently {
		t.Fatalf("expected REPL wrapper to be absorbed silently")
	}
	if classification.IsSearch || classification.IsRead || classification.IsList || classification.IsMemoryWrite {
		t.Fatalf("unexpected non-empty classification flags: %#v", classification)
	}
}
