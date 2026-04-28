package tests

import (
	"os"
	"path/filepath"
	"testing"

	"claude-go/internal/utils"
)

func TestCommandHistoryManager_AddAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:   tmpDir,
		MaxEntries:  10,
		ProjectRoot: "/test/project",
		SessionID:   "test-session",
	})

	mgr.Add("first command")
	mgr.Add("second command")
	mgr.Add("third command")

	history := mgr.GetAll()
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}

	if history[0] != "third command" {
		t.Errorf("expected 'third command', got %s", history[0])
	}
	if history[1] != "second command" {
		t.Errorf("expected 'second command', got %s", history[1])
	}
	if history[2] != "first command" {
		t.Errorf("expected 'first command', got %s", history[2])
	}
}

func TestCommandHistoryManager_Deduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 10,
	})

	mgr.Add("duplicate")
	mgr.Add("duplicate")
	mgr.Add("duplicate")

	history := mgr.GetAll()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry (deduplicated), got %d", len(history))
	}
}

func TestCommandHistoryManager_Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 10,
	})

	mgr.Add("git status")
	mgr.Add("git commit")
	mgr.Add("npm install")
	mgr.Add("npm test")

	results := mgr.Search("git")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'git', got %d", len(results))
	}

	results = mgr.Search("npm")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'npm', got %d", len(results))
	}

	results = mgr.Search("test")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'test', got %d", len(results))
	}
}

func TestCommandHistoryManager_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr1 := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 10,
	})
	mgr1.Add("persistent command 1")
	mgr1.Add("persistent command 2")

	mgr2 := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 10,
	})

	history := mgr2.GetAll()
	if len(history) != 2 {
		t.Errorf("expected 2 history entries from persistence, got %d", len(history))
	}

	if history[0] != "persistent command 2" {
		t.Errorf("expected 'persistent command 2', got %s", history[0])
	}
}

func TestCommandHistoryManager_MaxEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 5,
	})

	for i := 0; i < 10; i++ {
		mgr.Add("command")
	}

	history := mgr.GetAll()
	if len(history) > 5 {
		t.Errorf("expected max 5 history entries, got %d", len(history))
	}
}

func TestCommandHistoryManager_FileFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "claude-code-test-*")
	if err != err {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ConfigDir:  tmpDir,
		MaxEntries: 10,
	})

	mgr.Add("test command")

	historyFile := filepath.Join(tmpDir, "history.jsonl")
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Error("history file was not created")
	}
}
