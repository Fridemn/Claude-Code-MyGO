package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/tool"
	"claude-go/internal/tool/search"
)

func TestGlobTool_ExactAbsolutePathPattern(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(target, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := (search.GlobTool{}).Call(context.Background(), tool.Input{
		"pattern": target,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("glob call failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	content, ok := result.Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", result.Content)
	}
	if !strings.Contains(content, target) {
		t.Fatalf("expected content to include absolute file path %q, got %q", target, content)
	}
}

func TestGlobTool_AbsoluteGlobPatternWithoutPath(t *testing.T) {
	tmpDir := t.TempDir()
	matchFile := filepath.Join(tmpDir, "a.json")
	otherFile := filepath.Join(tmpDir, "b.txt")

	if err := os.WriteFile(matchFile, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("write match fixture: %v", err)
	}
	if err := os.WriteFile(otherFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-match fixture: %v", err)
	}

	result, err := (search.GlobTool{}).Call(context.Background(), tool.Input{
		"pattern": filepath.ToSlash(filepath.Join(tmpDir, "*.json")),
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("glob call failed: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected tool error: %s", result.Error)
	}

	content, ok := result.Content.(string)
	if !ok {
		t.Fatalf("expected string content, got %T", result.Content)
	}
	if !strings.Contains(content, "a.json") {
		t.Fatalf("expected content to include matched file name, got %q", content)
	}
	if strings.Contains(content, "b.txt") {
		t.Fatalf("did not expect non-matching file in result, got %q", content)
	}
}
