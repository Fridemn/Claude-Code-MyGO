package paste

import (
	"testing"
)

func TestGetPastedTextRefNumLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"single line", 0},
		{"line1\nline2", 1},
		{"line1\nline2\nline3", 2},
		{"line1\r\nline2", 1},
		{"line1\rline2", 1},
		{"line1\nline2\nline3\nline4", 3},
	}

	for _, test := range tests {
		result := GetPastedTextRefNumLines(test.input)
		if result != test.expected {
			t.Errorf("GetPastedTextRefNumLines(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestFormatPastedTextRef(t *testing.T) {
	tests := []struct {
		id       int
		numLines int
		expected string
	}{
		{1, 0, "[Pasted text #1]"},
		{1, 5, "[Pasted text #1 +5 lines]"},
		{2, 10, "[Pasted text #2 +10 lines]"},
		{3, 1, "[Pasted text #3 +1 lines]"},
	}

	for _, test := range tests {
		result := FormatPastedTextRef(test.id, test.numLines)
		if result != test.expected {
			t.Errorf("FormatPastedTextRef(%d, %d) = %q, expected %q", test.id, test.numLines, result, test.expected)
		}
	}
}

func TestFormatImageRef(t *testing.T) {
	result := FormatImageRef(1)
	expected := "[Image #1]"
	if result != expected {
		t.Errorf("FormatImageRef(1) = %q, expected %q", result, expected)
	}
}

func TestFormatTruncatedTextRef(t *testing.T) {
	tests := []struct {
		id       int
		numLines int
		expected string
	}{
		{1, 0, "[...Truncated text #1...]"},
		{1, 5, "[...Truncated text #1 +5 lines...]"},
	}

	for _, test := range tests {
		result := FormatTruncatedTextRef(test.id, test.numLines)
		if result != test.expected {
			t.Errorf("FormatTruncatedTextRef(%d, %d) = %q, expected %q", test.id, test.numLines, result, test.expected)
		}
	}
}

func TestParseReferences(t *testing.T) {
	tests := []struct {
		input    string
		expected []Reference
	}{
		{
			"no references here",
			nil,
		},
		{
			"[Pasted text #1]",
			[]Reference{{ID: 1, Match: "[Pasted text #1]", Index: 0}},
		},
		{
			"[Pasted text #1 +10 lines]",
			[]Reference{{ID: 1, Match: "[Pasted text #1 +10 lines]", Index: 0}},
		},
		{
			"some text [Pasted text #2 +5 lines] more text",
			[]Reference{{ID: 2, Match: "[Pasted text #2 +5 lines]", Index: 10}},
		},
		{
			"[Image #3]",
			[]Reference{{ID: 3, Match: "[Image #3]", Index: 0}},
		},
		{
			"[...Truncated text #4 +2 lines...]",
			[]Reference{{ID: 4, Match: "[...Truncated text #4 +2 lines...]", Index: 0}},
		},
		{
			"multiple [Pasted text #1] and [Pasted text #2 +3 lines]",
			[]Reference{
				{ID: 1, Match: "[Pasted text #1]", Index: 9},
				{ID: 2, Match: "[Pasted text #2 +3 lines]", Index: 30},
			},
		},
	}

	for _, test := range tests {
		result := ParseReferences(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("ParseReferences(%q) returned %d refs, expected %d", test.input, len(result), len(test.expected))
			continue
		}
		for i, ref := range result {
			if ref.ID != test.expected[i].ID || ref.Match != test.expected[i].Match || ref.Index != test.expected[i].Index {
				t.Errorf("ParseReferences(%q)[%d] = %+v, expected %+v", test.input, i, ref, test.expected[i])
			}
		}
	}
}

func TestExpandPastedTextRefs(t *testing.T) {
	contents := map[int]*PastedContent{
		1: {ID: 1, Type: "text", Content: "This is the pasted content"},
		2: {ID: 2, Type: "text", Content: "line1\nline2\nline3"},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			"no references",
			"no references",
		},
		{
			"[Pasted text #1]",
			"This is the pasted content",
		},
		{
			"prefix [Pasted text #1] suffix",
			"prefix This is the pasted content suffix",
		},
		{
			"[Pasted text #1] and [Pasted text #2 +2 lines]",
			"This is the pasted content and line1\nline2\nline3",
		},
		{
			"[Image #3]", // No content for ID 3, stays as-is
			"[Image #3]",
		},
	}

	for _, test := range tests {
		result := ExpandPastedTextRefs(test.input, contents)
		if result != test.expected {
			t.Errorf("ExpandPastedTextRefs(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestManager(t *testing.T) {
	m := NewManager()

	// Short text should be returned as-is
	shortText := "hello world"
	result := m.AddPaste(shortText)
	if result != shortText {
		t.Errorf("AddPaste(short text) = %q, expected %q", result, shortText)
	}

	// Long text should be collapsed (>40 chars)
	longText := "This is a longer text that exceeds forty characters total"
	result = m.AddPaste(longText)
	if result == longText {
		t.Errorf("AddPaste(long text) should return collapsed reference, got %q", result)
	}
	if result != "[Pasted text #1]" {
		t.Errorf("AddPaste(long text) = %q, expected [Pasted text #1]", result)
	}

	// Content should be stored
	content := m.GetContent(1)
	if content == nil {
		t.Error("GetContent(1) returned nil")
	} else if content.Content != longText {
		t.Error("GetContent(1).Content doesn't match original text")
	}

	// Expand should work
	expanded := m.ExpandInput("[Pasted text #1]")
	if expanded != longText {
		t.Errorf("ExpandInput() = %q, expected original long text", expanded)
	}

	// Clear should reset
	m.Clear()
	if m.GetContent(1) != nil {
		t.Error("Clear() should remove all content")
	}
}

func TestManagerWithLines(t *testing.T) {
	m := NewManager()

	// Long text with multiple lines
	longText := "line1\nline2\nline3\nline4\nline5"
	for i := 0; i < 900; i++ {
		longText += "x"
	}
	result := m.AddPaste(longText)
	if result != "[Pasted text #1 +4 lines]" {
		t.Errorf("AddPaste(multiline long text) = %q, expected [Pasted text #1 +4 lines]", result)
	}
}

func TestManagerMultiplePastes(t *testing.T) {
	m := NewManager()

	text1 := ""
	for i := 0; i < 1000; i++ {
		text1 += "a"
	}
	text2 := ""
	for i := 0; i < 900; i++ {
		text2 += "b"
	}

	ref1 := m.AddPaste(text1)
	ref2 := m.AddPaste(text2)

	if ref1 != "[Pasted text #1]" {
		t.Errorf("First paste = %q, expected [Pasted text #1]", ref1)
	}
	if ref2 != "[Pasted text #2]" {
		t.Errorf("Second paste = %q, expected [Pasted text #2]", ref2)
	}

	// Both should expand correctly
	input := ref1 + " and " + ref2
	expanded := m.ExpandInput(input)
	expected := text1 + " and " + text2
	if expanded != expected {
		t.Errorf("ExpandInput() = %q (len %d), expected len %d", expanded, len(expanded), len(expected))
	}
}
