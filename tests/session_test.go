package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-go/internal/session"
	"claude-go/internal/types"
)

func TestNewSession(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")
	if sess.ID != "test-id" {
		t.Fatalf("expected ID test-id, got %s", sess.ID)
	}
	if sess.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if sess.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
	if len(sess.Messages) != 0 {
		t.Fatalf("expected empty messages, got %d", len(sess.Messages))
	}
}

func TestSessionAddMessage(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")

	msg := types.Message{
		Role:    types.RoleUser,
		Content: "hello",
		Usage: &types.Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	sess.AddMessage(msg)

	if len(sess.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sess.Messages))
	}
	if sess.GetMessageCount() != 1 {
		t.Fatalf("expected GetMessageCount=1, got %d", sess.GetMessageCount())
	}
	if sess.TurnCount != 1 {
		t.Fatalf("expected TurnCount=1, got %d", sess.TurnCount)
	}
	if sess.TotalTokens != 150 {
		t.Fatalf("expected TotalTokens=150, got %d", sess.TotalTokens)
	}
}

func TestSessionTrimToLastNMessages(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")
	for i := 0; i < 10; i++ {
		sess.AddMessage(types.Message{
			Role:    types.RoleUser,
			Content: string(rune('0' + i)),
		})
	}

	trimmed := sess.TrimToLastNMessages(5)
	if len(trimmed) != 5 {
		t.Fatalf("expected 5 trimmed messages, got %d", len(trimmed))
	}
	if len(sess.Messages) != 5 {
		t.Fatalf("expected 5 remaining messages, got %d", len(sess.Messages))
	}
	if !sess.IsCompacted {
		t.Fatal("expected IsCompacted to be true")
	}
	if sess.CompactedAt == nil {
		t.Fatal("expected CompactedAt to be set")
	}
}

func TestSessionClone(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")
	sess.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: "hello",
	})
	sess.SetModel("claude-sonnet")

	clone := sess.Clone()
	if clone.ID != sess.ID {
		t.Fatalf("expected ID %s, got %s", sess.ID, clone.ID)
	}
	if len(clone.Messages) != len(sess.Messages) {
		t.Fatalf("expected %d messages, got %d", len(sess.Messages), len(clone.Messages))
	}

	// Modify clone, verify original unchanged
	clone.AddMessage(types.Message{
		Role:    types.RoleAssistant,
		Content: "hi",
	})
	if len(sess.Messages) != 1 {
		t.Fatalf("expected original to still have 1 message, got %d", len(sess.Messages))
	}
}

func TestManagerCreateAndSave(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	sess := mgr.CreateWithID("test-session")
	sess.AddMessage(types.Message{
		Role:    types.RoleUser,
		Content: "hello",
	})

	if err := mgr.Save(sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "test-session.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected session file at %s: %v", path, err)
	}

	// Load and verify
	loaded, err := mgr.Load("test-session")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "test-session" {
		t.Fatalf("expected ID test-session, got %s", loaded.ID)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded.Messages))
	}
}

func TestManagerListSorted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	// Create sessions with different IDs and specific UpdatedAt
	// Use SaveWithoutTouch to preserve the UpdatedAt values
	sess1 := mgr.CreateWithID("old-session")
	sess1.UpdatedAt = time.Now().Add(-24 * time.Hour)
	mgr.SaveWithoutTouch(sess1)

	sess2 := mgr.CreateWithID("new-session")
	sess2.UpdatedAt = time.Now()
	mgr.SaveWithoutTouch(sess2)

	sess3 := mgr.CreateWithID("middle-session")
	sess3.UpdatedAt = time.Now().Add(-12 * time.Hour)
	mgr.SaveWithoutTouch(sess3)

	ids, err := mgr.ListSorted()
	if err != nil {
		t.Fatalf("ListSorted: %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(ids))
	}

	// Should be sorted newest first
	if ids[0] != "new-session" {
		t.Fatalf("expected first to be new-session, got %s", ids[0])
	}
	if ids[1] != "middle-session" {
		t.Fatalf("expected second to be middle-session, got %s", ids[1])
	}
	if ids[2] != "old-session" {
		t.Fatalf("expected third to be old-session, got %s", ids[2])
	}
}

func TestManagerCleanup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	// Create old session with specific UpdatedAt
	sess1 := mgr.CreateWithID("old-session")
	mgr.Save(sess1) // Save first to get file
	loaded1, err := mgr.Load("old-session") // Reload
	if err != nil {
		t.Fatalf("Load old-session: %v", err)
	}
	loaded1.UpdatedAt = time.Now().Add(-48 * time.Hour)
	mgr.SaveWithoutTouch(loaded1) // Save again with old UpdatedAt

	// Create recent session
	sess2 := mgr.CreateWithID("recent-session")
	mgr.Save(sess2)

	// Cleanup sessions older than 24 hours
	removed, err := mgr.Cleanup(24 * time.Hour)
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}

	// Verify old session is gone
	if mgr.Exists("old-session") {
		t.Fatal("expected old-session to be deleted")
	}

	// Verify recent session still exists
	if !mgr.Exists("recent-session") {
		t.Fatal("expected recent-session to still exist")
	}
}

func TestManagerLoadOrCreate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	// First call should create
	sess1, err := mgr.LoadOrCreate("new-session")
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	// Second call should load
	sess2, err := mgr.LoadOrCreate("new-session")
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	// Should be the same ID
	if sess1.ID != sess2.ID {
		t.Fatalf("expected same ID, got %s and %s", sess1.ID, sess2.ID)
	}

	// First should have CreatedAt, second should have loaded
	if sess1.CreatedAt.Equal(sess2.CreatedAt) {
		t.Fatal("expected CreatedAt to be updated after load")
	}
}

func TestSessionSetEnvVar(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")
	sess.SetEnvVar("FOO", "bar")

	if val := sess.EnvVars["FOO"]; val != "bar" {
		t.Fatalf("expected FOO=bar, got %s", val)
	}
}

func TestSessionGetDuration(t *testing.T) {
	t.Parallel()

	sess := session.NewSession("test-id")
	duration := sess.GetDuration()

	// Should be a very short duration (just created)
	if duration > time.Second {
		t.Fatalf("expected duration < 1s, got %v", duration)
	}
}

func TestSessionSnapshot(t *testing.T) {
	t.Parallel()

	original := []types.Message{
		{Role: types.RoleUser, Content: "hello"},
	}

	snapshot := session.Snapshot(original)
	if len(snapshot) != 1 {
		t.Fatalf("expected 1 message, got %d", len(snapshot))
	}

	// Verify deep copy
	snapshot[0].Content = "changed"
	if original[0].Content != "hello" {
		t.Fatal("expected snapshot to be independent of original")
	}
}

func TestManagerDelete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	sess := mgr.CreateWithID("to-delete")
	mgr.Save(sess)

	if err := mgr.Delete("to-delete"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if mgr.Exists("to-delete") {
		t.Fatal("expected session to be deleted")
	}
}

func TestManagerGetRecent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr, err := session.CreateManager(dir)
	if err != nil {
		t.Fatalf("CreateManager: %v", err)
	}

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		sess := mgr.CreateWithID("session-" + string(rune('a'+i)))
		mgr.Save(sess)
	}

	recent, err := mgr.GetRecent(3)
	if err != nil {
		t.Fatalf("GetRecent: %v", err)
	}

	if len(recent) != 3 {
		t.Fatalf("expected 3 recent sessions, got %d", len(recent))
	}
}
