package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claude-go/internal/session"
	"claude-go/internal/types"
)

func TestEnhancedManagerCreation(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := session.CreateEnhancedManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Check transcript directory was created
	transcriptDir := filepath.Join(tmpDir, "transcripts")
	if _, err := os.Stat(transcriptDir); os.IsNotExist(err) {
		t.Error("transcript directory not created")
	}

	// Check GetProjectDir returns something
	projectDir := mgr.GetProjectDir()
	if projectDir == "" {
		t.Error("project dir should not be empty")
	}
}

func TestSessionIDGeneration(t *testing.T) {
	id := session.GenerateSessionID()

	if id == "" {
		t.Error("generated session ID is empty")
	}

	if len(id) < 10 {
		t.Errorf("session ID too short: %s", id)
	}

	// Generate another and check they're different
	id2 := session.GenerateSessionID()

	if id == id2 {
		t.Error("generated session IDs should be unique")
	}
}

func TestTranscriptPath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	path := mgr.GetTranscriptPath(sessionID)
	expected := filepath.Join(tmpDir, "transcripts", sessionID+".jsonl")

	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}
}

func TestSessionIdExists(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"

	// Should not exist initially
	if mgr.SessionIdExists(sessionID) {
		t.Error("session should not exist initially")
	}

	// Create a transcript file
	path := mgr.GetTranscriptPath(sessionID)
	os.WriteFile(path, []byte("{}\n"), 0644)

	// Now it should exist
	if !mgr.SessionIdExists(sessionID) {
		t.Error("session should exist after creating file")
	}
}

func TestSerializeMessage(t *testing.T) {
	msg := types.Message{
		UUID:      "test-uuid-123",
		Type:      types.MessageTypeUser,
		Role:      types.RoleUser,
		Content:   "Hello, world!",
		Timestamp: time.Now(),
	}

	serialized := session.SerializeMessage(msg, "session-123", "/home/user", "cli")

	if serialized.UUID != "test-uuid-123" {
		t.Errorf("expected UUID test-uuid-123, got %s", serialized.UUID)
	}
	if serialized.SessionID != "session-123" {
		t.Errorf("expected session ID session-123, got %s", serialized.SessionID)
	}
	if serialized.CWD != "/home/user" {
		t.Errorf("expected CWD /home/user, got %s", serialized.CWD)
	}
	if serialized.UserType != "cli" {
		t.Errorf("expected user type cli, got %s", serialized.UserType)
	}
	if serialized.Version != session.CurrentVersion {
		t.Errorf("expected version %s, got %s", session.CurrentVersion, serialized.Version)
	}
}

func TestRecordTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeUser,
			Role:      types.RoleUser,
			Content:   "Hello",
			Timestamp: time.Now(),
		},
		{
			UUID:      "msg-2",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			Content:   "Hi there!",
			Timestamp: time.Now(),
		},
	}

	err := mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")
	if err != nil {
		t.Fatalf("failed to record transcript: %v", err)
	}

	// Check file was created
	path := mgr.GetTranscriptPath(sessionID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("transcript file was not created")
	}

	// Read back the transcript
	readMessages, err := mgr.ReadTranscript(sessionID)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}

	if len(readMessages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(readMessages))
	}

	if readMessages[0].Content != "Hello" {
		t.Errorf("expected first message content 'Hello', got %s", readMessages[0].Content)
	}
}

func TestRecordCompactBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"

	// Record some messages first
	messages := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeUser,
			Role:      types.RoleUser,
			Content:   "Hello",
			Timestamp: time.Now(),
		},
	}
	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	// Record compact boundary
	err := mgr.RecordCompactBoundary(sessionID, "Compacted summary", 10000, 5000)
	if err != nil {
		t.Fatalf("failed to record compact boundary: %v", err)
	}

	// File should still exist
	if !mgr.SessionIdExists(sessionID) {
		t.Error("session should still exist after compact boundary")
	}
}

func TestReadTranscriptHead(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "First", Timestamp: time.Now()},
		{UUID: "msg-2", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Second", Timestamp: time.Now()},
		{UUID: "msg-3", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Third", Timestamp: time.Now()},
	}
	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	// Read head with larger limit to ensure we get content
	headMessages, err := mgr.ReadTranscriptHead(sessionID, 5000)
	if err != nil {
		t.Fatalf("failed to read transcript head: %v", err)
	}

	// Should get all 3 messages
	if len(headMessages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(headMessages))
	}
}

func TestReadTranscriptTail(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "First", Timestamp: time.Now()},
		{UUID: "msg-2", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Second", Timestamp: time.Now()},
		{UUID: "msg-3", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Third", Timestamp: time.Now()},
	}
	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	// Read tail with small limit
	tailMessages, err := mgr.ReadTranscriptTail(sessionID, 100)
	if err != nil {
		t.Fatalf("failed to read transcript tail: %v", err)
	}

	// Should get at least one message
	if len(tailMessages) == 0 {
		t.Error("expected at least one message from tail")
	}
}

func TestDeleteTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Hello", Timestamp: time.Now()},
	}
	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	if !mgr.SessionIdExists(sessionID) {
		t.Fatal("session should exist before delete")
	}

	err := mgr.DeleteTranscript(sessionID)
	if err != nil {
		t.Fatalf("failed to delete transcript: %v", err)
	}

	if mgr.SessionIdExists(sessionID) {
		t.Error("session should not exist after delete")
	}
}

func TestGetTranscriptStats(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Hello", Timestamp: time.Now()},
		{UUID: "msg-2", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "World", Timestamp: time.Now()},
	}
	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	count, size, err := mgr.GetTranscriptStats(sessionID)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 messages, got %d", count)
	}
	if size <= 0 {
		t.Errorf("expected positive size, got %d", size)
	}
}

func TestListSessions(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	// Create some sessions
	mgr.RecordTranscript("session-1", []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Hello", Timestamp: time.Now()},
	}, "/home/user", "cli")
	mgr.RecordTranscript("session-2", []types.Message{
		{UUID: "msg-2", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "World", Timestamp: time.Now()},
	}, "/home/user", "cli")

	logs, err := mgr.ListSessions()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(logs))
	}
}

func TestCreateAndSaveSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	ctx := context.Background()
	sess, err := mgr.CreateSession(ctx, "/home/user", "cli")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}

	// Add a message
	sess.Messages = append(sess.Messages, types.Message{
		UUID:      "msg-1",
		Type:      types.MessageTypeUser,
		Role:      types.RoleUser,
		Content:   "Test message",
		Timestamp: time.Now(),
	})

	// Save session
	err = mgr.SaveSession(sess, "/home/user", "cli")
	if err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	// Load session back
	loaded, err := mgr.LoadSession(sess.ID)
	if err != nil {
		t.Fatalf("failed to load session: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("expected session ID %s, got %s", sess.ID, loaded.ID)
	}

	if len(loaded.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(loaded.Messages))
	}
}

func TestEmptyTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	// Read non-existent transcript
	messages, err := mgr.ReadTranscript("non-existent")
	if err != nil {
		t.Fatalf("failed to read empty transcript: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestProgressMessagesSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-123"
	messages := []types.Message{
		{UUID: "msg-1", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "Hello", Timestamp: time.Now()},
		{UUID: "msg-2", Type: types.MessageTypeProgress, Role: "progress", Content: "Loading...", Timestamp: time.Now()},
		{UUID: "msg-3", Type: types.MessageTypeUser, Role: types.RoleUser, Content: "World", Timestamp: time.Now()},
	}

	mgr.RecordTranscript(sessionID, messages, "/home/user", "cli")

	readMessages, _ := mgr.ReadTranscript(sessionID)

	// Progress message should be skipped
	if len(readMessages) != 2 {
		t.Errorf("expected 2 messages (progress skipped), got %d", len(readMessages))
	}
}

func TestRecordTranscript_ExternalStripsREPLWrapperAndPromotesVirtual(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-external-repl"
	messages := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			IsVirtual: true,
			ToolCalls: []types.ToolCall{
				{ID: "repl-1", Name: "REPL", Arguments: json.RawMessage(`{"script":"Read({\"file_path\":\"README.md\"})"}`)},
			},
			Timestamp: time.Now(),
		},
		{
			UUID:       "msg-2",
			Type:       types.MessageTypeTool,
			Role:       types.RoleTool,
			ToolCallID: "repl-1",
			Content:    "tool=REPL\nstatus=ok",
			Timestamp:  time.Now(),
		},
		{
			UUID:      "msg-3",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			IsVirtual: true,
			ToolCalls: []types.ToolCall{
				{ID: "read-1", Name: "Read", Arguments: json.RawMessage(`{"file_path":"README.md"}`)},
			},
			Timestamp: time.Now(),
		},
		{
			UUID:       "msg-4",
			Type:       types.MessageTypeTool,
			Role:       types.RoleTool,
			ToolCallID: "read-1",
			Content:    "tool=Read\nstatus=ok",
			IsVirtual:  true,
			Timestamp:  time.Now(),
		},
	}

	if err := mgr.RecordTranscript(sessionID, messages, "/home/user", "cli"); err != nil {
		t.Fatalf("record transcript failed: %v", err)
	}
	readMessages, err := mgr.ReadTranscript(sessionID)
	if err != nil {
		t.Fatalf("read transcript failed: %v", err)
	}

	if len(readMessages) != 2 {
		t.Fatalf("expected 2 messages after stripping REPL wrapper, got %d", len(readMessages))
	}
	for _, msg := range readMessages {
		if msg.IsVirtual {
			t.Fatalf("expected virtual flag to be stripped for external transcript: %#v", msg)
		}
		if msg.ToolCallID == "repl-1" {
			t.Fatalf("expected REPL tool_result to be stripped: %#v", msg)
		}
		for _, call := range msg.ToolCalls {
			if call.Name == "REPL" {
				t.Fatalf("expected REPL tool_use to be stripped: %#v", msg)
			}
		}
	}
}

func TestRecordTranscript_AntKeepsREPLWrapperAndVirtual(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-ant-repl"
	messages := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			IsVirtual: true,
			ToolCalls: []types.ToolCall{
				{ID: "repl-1", Name: "REPL", Arguments: json.RawMessage(`{"script":"Read({\"file_path\":\"README.md\"})"}`)},
			},
			Timestamp: time.Now(),
		},
	}

	if err := mgr.RecordTranscript(sessionID, messages, "/home/user", "ant"); err != nil {
		t.Fatalf("record transcript failed: %v", err)
	}
	readMessages, err := mgr.ReadTranscript(sessionID)
	if err != nil {
		t.Fatalf("read transcript failed: %v", err)
	}
	if len(readMessages) != 1 {
		t.Fatalf("expected ant transcript to keep REPL wrapper, got %d", len(readMessages))
	}
	if !readMessages[0].IsVirtual {
		t.Fatalf("expected ant transcript to preserve virtual flag, got %#v", readMessages[0])
	}
	if len(readMessages[0].ToolCalls) != 1 || readMessages[0].ToolCalls[0].Name != "REPL" {
		t.Fatalf("expected ant transcript to keep REPL tool_call, got %#v", readMessages[0].ToolCalls)
	}
}

func TestRecordTranscript_ExternalTracksREPLIDsAcrossSeparateWrites(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, _ := session.CreateEnhancedManager(tmpDir)

	sessionID := "test-session-external-repl-split"
	firstBatch := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			IsVirtual: true,
			ToolCalls: []types.ToolCall{
				{ID: "repl-split-1", Name: "REPL", Arguments: json.RawMessage(`{"script":"Read({\"file_path\":\"README.md\"})"}`)},
			},
			Timestamp: time.Now(),
		},
	}
	if err := mgr.RecordTranscript(sessionID, firstBatch, "/home/user", "cli"); err != nil {
		t.Fatalf("record first batch failed: %v", err)
	}

	secondBatch := []types.Message{
		{
			UUID:       "msg-2",
			Type:       types.MessageTypeTool,
			Role:       types.RoleTool,
			ToolCallID: "repl-split-1",
			Content:    "tool=REPL\nstatus=ok",
			Timestamp:  time.Now(),
		},
		{
			UUID:       "msg-3",
			Type:       types.MessageTypeTool,
			Role:       types.RoleTool,
			ToolCallID: "read-2",
			Content:    "tool=Read\nstatus=ok",
			Timestamp:  time.Now(),
		},
	}
	if err := mgr.RecordTranscript(sessionID, secondBatch, "/home/user", "cli"); err != nil {
		t.Fatalf("record second batch failed: %v", err)
	}

	readMessages, err := mgr.ReadTranscript(sessionID)
	if err != nil {
		t.Fatalf("read transcript failed: %v", err)
	}
	for _, msg := range readMessages {
		if msg.ToolCallID == "repl-split-1" {
			t.Fatalf("expected split-write REPL tool_result to be stripped, got %#v", msg)
		}
	}
	foundRead := false
	for _, msg := range readMessages {
		if msg.ToolCallID == "read-2" {
			foundRead = true
			break
		}
	}
	if !foundRead {
		t.Fatalf("expected non-REPL tool_result to remain, got %#v", readMessages)
	}
}

func TestRecordTranscript_ExternalTracksREPLIDsAcrossManagerRestart(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-external-repl-restart"

	firstMgr, _ := session.CreateEnhancedManager(tmpDir)
	firstBatch := []types.Message{
		{
			UUID:      "msg-1",
			Type:      types.MessageTypeAssistant,
			Role:      types.RoleAssistant,
			IsVirtual: true,
			ToolCalls: []types.ToolCall{
				{ID: "repl-restart-1", Name: "REPL", Arguments: json.RawMessage(`{"script":"Read({\"file_path\":\"README.md\"})"}`)},
			},
			Timestamp: time.Now(),
		},
	}
	if err := firstMgr.RecordTranscript(sessionID, firstBatch, "/home/user", "cli"); err != nil {
		t.Fatalf("record first batch failed: %v", err)
	}

	// Simulate a process restart: new manager instance, no in-memory REPL ID cache.
	secondMgr, _ := session.CreateEnhancedManager(tmpDir)
	secondBatch := []types.Message{
		{
			UUID:       "msg-2",
			Type:       types.MessageTypeTool,
			Role:       types.RoleTool,
			ToolCallID: "repl-restart-1",
			Content:    "tool=REPL\nstatus=ok",
			Timestamp:  time.Now(),
		},
	}
	if err := secondMgr.RecordTranscript(sessionID, secondBatch, "/home/user", "cli"); err != nil {
		t.Fatalf("record second batch failed: %v", err)
	}

	readMessages, err := secondMgr.ReadTranscript(sessionID)
	if err != nil {
		t.Fatalf("read transcript failed: %v", err)
	}
	for _, msg := range readMessages {
		if msg.ToolCallID == "repl-restart-1" {
			t.Fatalf("expected REPL tool_result to be stripped across manager restart, got %#v", msg)
		}
	}
}
