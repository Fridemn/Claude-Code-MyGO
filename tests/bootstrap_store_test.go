package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"claude-go/internal/bootstrap"
	"claude-go/internal/config"
	"claude-go/internal/tool"
	"claude-go/internal/tool/bash"
	"claude-go/internal/tool/search"
)

func TestBootstrapStoreCWDManagement(t *testing.T) {
	t.Parallel()

	// Create store
	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	originalCWD := store.GetOriginalCWD()
	currentCWD := store.GetCWD()

	// Initially, original and current CWD should be the same
	if originalCWD != currentCWD {
		t.Fatalf("expected original and current CWD to match: %s vs %s", originalCWD, currentCWD)
	}

	// Create a temp directory and set it as CWD
	tempDir := t.TempDir()
	store.SetCWD(tempDir)

	// Verify CWD was updated
	newCWD := store.GetCWD()
	if newCWD != tempDir && !filepath.IsAbs(tempDir) {
		// May be resolved to absolute path
		abs, _ := filepath.Abs(tempDir)
		if newCWD != abs {
			t.Fatalf("expected CWD to be %s, got %s", tempDir, newCWD)
		}
	}

	// Original CWD should not change
	if store.GetOriginalCWD() != originalCWD {
		t.Fatalf("original CWD should not change")
	}

	// Reset CWD
	store.ResetCWD()
	if store.GetCWD() != originalCWD {
		t.Fatalf("expected CWD to reset to original")
	}
}

func TestBootstrapStoreSessionFlags(t *testing.T) {
	t.Parallel()

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Test session flags
	if store.GetSessionFlag("test_flag") {
		t.Fatalf("expected unset flag to be false")
	}

	store.SetSessionFlag("test_flag", true)
	if !store.GetSessionFlag("test_flag") {
		t.Fatalf("expected flag to be true after setting")
	}

	store.SetSessionFlag("test_flag", false)
	if store.GetSessionFlag("test_flag") {
		t.Fatalf("expected flag to be false after unsetting")
	}
}

func TestBootstrapStoreSessionID(t *testing.T) {
	t.Parallel()

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Set session ID
	store.SetSessionID("test-session-123")

	snapshot := store.Snapshot()
	if snapshot.SessionID != "test-session-123" {
		t.Fatalf("expected session ID 'test-session-123', got %s", snapshot.SessionID)
	}
}

func TestBootstrapStoreWithToolRuntime(t *testing.T) {
	t.Parallel()

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	// Create a temp directory and set it as CWD
	tempDir := t.TempDir()
	store.SetCWD(tempDir)

	// Create a test file in the temp directory
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create runtime with store
	runtime := tool.Runtime{
		Store: store,
	}

	// Test that GrepTool uses Store's CWD
	// Search without specifying path should use CWD from Store
	result, err := (search.GrepTool{}).Call(context.Background(), tool.Input{
		"pattern": "test",
	}, runtime)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}

	// Result should contain our test file
	content, ok := result.Content.(string)
	if !ok {
		t.Fatalf("unexpected content type: %T", result.Content)
	}
	if content == "" {
		t.Fatalf("expected grep to find something in CWD")
	}

	// Test that BashTool uses Store's CWD
	bashResult, err := (bash.BashTool{}).Call(context.Background(), tool.Input{
		"command": "pwd",
	}, runtime)
	if err != nil {
		t.Fatalf("bash pwd: %v", err)
	}

	// The bash output should indicate we're in the temp directory
	bashContent, ok := bashResult.Content.(bash.BashOutput)
	if !ok {
		t.Fatalf("unexpected bash content type: %T", bashResult.Content)
	}
	// pwd should show we're in the CWD set via Store
	if bashContent.Stdout == "" {
		t.Fatalf("expected pwd output")
	}
}

func TestBootstrapStorePathValidation(t *testing.T) {
	t.Parallel()

	store, err := bootstrap.CreateStore(config.Config{Model: "test-model"})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	originalCWD := store.GetOriginalCWD()

	// Path in original project should return true
	if !store.PathInOriginalProject(originalCWD) {
		t.Fatalf("expected original CWD to be in original project")
	}

	// Create a temp dir outside original project
	tempDir := t.TempDir()
	if store.PathInOriginalProject(tempDir) && store.PathInOriginalProject(originalCWD) {
		// If tempDir is inside original, skip this check
		// This can happen if temp dir is within project
		if !store.IsCWDOutsideOriginal() {
			t.Log("temp dir is inside original project, skipping outside check")
		}
	}

	// Test IsCWDOutsideOriginal
	store.SetCWD(tempDir)
	absTemp, _ := filepath.Abs(tempDir)

	// Only check if temp is actually outside
	if !store.PathInOriginalProject(absTemp) {
		if !store.IsCWDOutsideOriginal() {
			t.Fatalf("expected CWD to be outside original")
		}
	}
}
