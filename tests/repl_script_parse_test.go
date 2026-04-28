package tests

import (
	"testing"

	"claude-go/internal/tool"
	"claude-go/internal/tool/repl"
)

func TestExtractFirstPrimitiveCall_JSONShape(t *testing.T) {
	t.Parallel()

	name, input, ok := repl.ExtractFirstPrimitiveCall(`{"tool":"Read","input":{"file_path":"/tmp/a.txt","offset":5}}`)
	if !ok {
		t.Fatal("expected parser to extract JSON call")
	}
	if name != "Read" {
		t.Fatalf("expected tool Read, got %q", name)
	}
	if got, _ := input["file_path"].(string); got != "/tmp/a.txt" {
		t.Fatalf("unexpected file_path: %#v", input["file_path"])
	}
}

func TestExtractFirstPrimitiveCall_JSONFlatShape(t *testing.T) {
	t.Parallel()

	name, input, ok := repl.ExtractFirstPrimitiveCall(`{"tool":"Read","file_path":"/tmp/flat.txt","offset":9}`)
	if !ok {
		t.Fatal("expected parser to extract flat JSON call")
	}
	if name != "Read" {
		t.Fatalf("expected tool Read, got %q", name)
	}
	if got, _ := input["file_path"].(string); got != "/tmp/flat.txt" {
		t.Fatalf("unexpected file_path: %#v", input["file_path"])
	}
}

func TestExtractFirstPrimitiveCall_FunctionStyle(t *testing.T) {
	t.Parallel()

	script := `const x = await Read({ file_path: "/tmp/a.txt", offset: 12 });`
	name, input, ok := repl.ExtractFirstPrimitiveCall(script)
	if !ok {
		t.Fatal("expected parser to extract function-style call")
	}
	if name != "Read" {
		t.Fatalf("expected tool Read, got %q", name)
	}
	if got, _ := input["file_path"].(string); got != "/tmp/a.txt" {
		t.Fatalf("unexpected file_path: %#v", input["file_path"])
	}
}

func TestExtractFirstPrimitiveCall_FunctionStyleStringShorthand(t *testing.T) {
	t.Parallel()

	name, input, ok := repl.ExtractFirstPrimitiveCall(`await Read("/tmp/simple.txt")`)
	if !ok {
		t.Fatal("expected parser to extract shorthand call")
	}
	if name != "Read" {
		t.Fatalf("expected tool Read, got %q", name)
	}
	if got, _ := input["file_path"].(string); got != "/tmp/simple.txt" {
		t.Fatalf("unexpected file_path: %#v", input["file_path"])
	}
}

func TestExtractPrimitiveCalls_JSONPlan(t *testing.T) {
	t.Parallel()

	calls, ok := repl.ExtractPrimitiveCalls(`{"calls":[{"tool":"Read","input":{"file_path":"a.md"}},{"tool":"Grep","input":{"pattern":"TODO","path":"."}}]}`)
	if !ok {
		t.Fatal("expected parser to extract JSON plan calls")
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "Read" || calls[1].Name != "Grep" {
		t.Fatalf("unexpected call order: %#v", calls)
	}
}

func TestExtractPrimitiveCalls_FunctionStyleRejectsMultiStepHeuristics(t *testing.T) {
	t.Parallel()

	_, ok := repl.ExtractPrimitiveCalls(`Read({"file_path":"a.md"}); Grep({"pattern":"TODO","path":"."});`)
	if ok {
		t.Fatal("expected function-style multi-step script to be rejected")
	}
}

func TestClassifyPrimitiveTool_Fallback(t *testing.T) {
	t.Parallel()

	classification, ok := repl.ClassifyPrimitiveTool("Grep", tool.Input{"pattern": "TODO"})
	if !ok {
		t.Fatal("expected Grep primitive classification fallback")
	}
	if !classification.IsCollapsible || !classification.IsSearch {
		t.Fatalf("unexpected classification: %#v", classification)
	}
}

func TestClassifyPrimitiveTool_MemoryWriteFallback(t *testing.T) {
	t.Parallel()

	classification, ok := repl.ClassifyPrimitiveTool("Write", tool.Input{
		"file_path": "/tmp/.claude/memory/session-memory/current.md",
	})
	if !ok {
		t.Fatal("expected Write primitive classification fallback")
	}
	if !classification.IsCollapsible || !classification.IsMemoryWrite {
		t.Fatalf("unexpected classification: %#v", classification)
	}
}

func TestClassifyPrimitiveTool_AgentFallbackRecognized(t *testing.T) {
	t.Parallel()

	classification, ok := repl.ClassifyPrimitiveTool("Agent", tool.Input{"prompt": "hello"})
	if !ok {
		t.Fatal("expected Agent primitive classification fallback")
	}
	if classification.IsCollapsible || classification.IsSearch || classification.IsRead || classification.IsList {
		t.Fatalf("unexpected classification flags: %#v", classification)
	}
}
