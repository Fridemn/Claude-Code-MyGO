package tests

import (
	"testing"

	"claude-code-go/internal/tool/bash"
)

func TestDetectBlockedSleepPattern(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"sleep 5", "standalone sleep 5"},
		{"sleep 10 && echo done", "sleep 10 followed by: && echo done"},
		{"sleep 2", "standalone sleep 2"}, // 2 seconds is blocked (only < 2s allowed)
		{"sleep 1", ""}, // 1 second is allowed (< 2s)
		{"sleep 1.5", ""}, // < 2 seconds is allowed
		{"sleep 0.5", ""}, // < 2 seconds is allowed
		{"echo hello; sleep 5", ""}, // sleep not first command
		{"sleep 5 | cat", ""}, // sleep in pipeline is fine
		{"(sleep 5)", ""}, // sleep in subshell is fine
		{"sleep 5; echo done", "sleep 5 followed by: ; echo done"},
		{"sleep 100", "standalone sleep 100"},
		{"sleep 3 && ls -la", "sleep 3 followed by: && ls -la"},
		{"ls", ""}, // not a sleep command
		{"", ""}, // empty command
	}

	for _, tt := range tests {
		result := bash.DetectBlockedSleepPattern(tt.command)
		if result != tt.expected {
			t.Errorf("detectBlockedSleepPattern(%q) = %q, want %q", tt.command, result, tt.expected)
		}
	}
}

func TestStripEmptyLines(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"\n\nhello\n\n", "hello"},
		{"\n\nhello\nworld\n\n", "hello\nworld"},
		{"hello\nworld", "hello\nworld"},
		{"\n\n\n", ""},
		{"", ""},
		{"  \n  hello  \n  ", "  hello  "},
		{"hello", "hello"},
		{"\nhello\n", "hello"},
		{"\t\nhello\n\t", "hello"},
	}

	for _, tt := range tests {
		result := bash.StripEmptyLines(tt.input)
		if result != tt.expected {
			t.Errorf("stripEmptyLines(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsImageOutput(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEA", true},
		{"data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD", true},
		{"data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP", true},
		{"data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3", true},
		{"hello world", false},
		{"data:text/plain;base64,hello", false},
		{"", false},
		{"not a data URI", false},
	}

	for _, tt := range tests {
		result := bash.IsImageOutput(tt.content)
		if result != tt.expected {
			t.Errorf("isImageOutput(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}

func TestParseDataUri(t *testing.T) {
	tests := []struct {
		content     string
		expectMedia string
		expectData  string
		expectNil   bool
	}{
		{"data:image/png;base64,ABC123", "image/png", "ABC123", false},
		{"data:image/jpeg;base64,/9j/4AAQ", "image/jpeg", "/9j/4AAQ", false},
		{"hello world", "", "", true},
		{"data:text/plain;charset=utf-8;base64,hello", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		result := bash.ParseDataUri(tt.content)
		if tt.expectNil {
			if result != nil {
				t.Errorf("parseDataUri(%q) expected nil, got %+v", tt.content, result)
			}
		} else {
			if result == nil {
				t.Errorf("parseDataUri(%q) expected non-nil result", tt.content)
			} else {
				if result.MediaType != tt.expectMedia {
					t.Errorf("parseDataUri(%q).MediaType = %q, want %q", tt.content, result.MediaType, tt.expectMedia)
				}
				if result.Data != tt.expectData {
					t.Errorf("parseDataUri(%q).Data = %q, want %q", tt.content, result.Data, tt.expectData)
				}
			}
		}
	}
}

func TestFormatOutput(t *testing.T) {
	// Test short output
	shortContent := "hello world"
	result := bash.FormatOutput(shortContent)
	if result.IsImage {
		t.Error("short text should not be image")
	}
	if result.TruncatedContent != shortContent {
		t.Errorf("short content should not be truncated")
	}
	if result.TotalLines != 1 {
		t.Errorf("short content should have 1 line")
	}

	// Test multi-line output
	multiLine := "line1\nline2\nline3"
	result = bash.FormatOutput(multiLine)
	if result.TotalLines != 3 {
		t.Errorf("multiLine should have 3 lines, got %d", result.TotalLines)
	}

	// Test image output
	imageContent := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEA"
	result = bash.FormatOutput(imageContent)
	if !result.IsImage {
		t.Error("image data URI should be detected as image")
	}
	if result.TotalLines != 1 {
		t.Errorf("image output should count as 1 line")
	}
}

func TestExtractClaudeCodeHints(t *testing.T) {
	tests := []struct {
		output       string
		command      string
		expectHints  int
		expectStrip  string
	}{
		{
			output:      "hello world",
			command:     "echo hello",
			expectHints: 0,
			expectStrip: "hello world",
		},
		{
			output:      `<claude-code-hint v="1" type="plugin" value="test@marketplace" />`,
			command:     "test-cli",
			expectHints: 1,
			expectStrip: "",
		},
		{
			output:      "output\n<claude-code-hint v=\"1\" type=\"plugin\" value=\"test@npm\" />\nmore output",
			command:     "cli",
			expectHints: 1,
			expectStrip: "output\n\nmore output",
		},
		{
			output:      `<claude-code-hint v="2" type="plugin" value="test" />`,
			command:     "test",
			expectHints: 0, // unsupported version
			expectStrip: "",
		},
		{
			output:      `<claude-code-hint v="1" type="unknown" value="test" />`,
			command:     "test",
			expectHints: 0, // unsupported type
			expectStrip: "",
		},
		{
			output:      `<claude-code-hint v="1" type="plugin" value="" />`,
			command:     "test",
			expectHints: 0, // empty value
			expectStrip: "",
		},
	}

	for _, tt := range tests {
		result := bash.ExtractClaudeCodeHints(tt.output, tt.command)
		if len(result.Hints) != tt.expectHints {
			t.Errorf("extractClaudeCodeHints(%q) got %d hints, want %d", tt.output, len(result.Hints), tt.expectHints)
		}
		if result.Stripped != tt.expectStrip {
			t.Errorf("extractClaudeCodeHints(%q).Stripped = %q, want %q", tt.output, result.Stripped, tt.expectStrip)
		}
		if tt.expectHints > 0 && len(result.Hints) > 0 {
			hint := result.Hints[0]
			if hint.V != 1 {
				t.Errorf("hint version should be 1, got %d", hint.V)
			}
			if hint.Type != "plugin" {
				t.Errorf("hint type should be plugin, got %q", hint.Type)
			}
		}
	}
}

func TestFirstCommandToken(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"echo hello", "echo"},
		{"ls -la /tmp", "ls"},
		{"git status", "git"},
		{"npm install package", "npm"},
		{"singlecommand", "singlecommand"},
		{"  spaced  command  ", "spaced"},
		{"", ""},
	}

	for _, tt := range tests {
		result := bash.FirstCommandToken(tt.command)
		if result != tt.expected {
			t.Errorf("firstCommandToken(%q) = %q, want %q", tt.command, result, tt.expected)
		}
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		content  string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello\nworld", 2},
		{"hello\nworld\nfoo", 3},
		{"a\nb\nc\n", 4}, // trailing newline creates empty line
	}

	for _, tt := range tests {
		result := bash.CountLines(tt.content)
		if result != tt.expected {
			t.Errorf("countLines(%q) = %d, want %d", tt.content, result, tt.expected)
		}
	}
}