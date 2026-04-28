package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claude-go/internal/types"
	"claude-go/internal/utils"
)

// Manager manages session persistence
type Manager struct {
	dir         string
	autoSave    bool
	maxSessions int
}

// ManagerOptions holds options for creating a Manager
type ManagerOptions struct {
	AutoSave    bool
	MaxSessions int // Maximum number of sessions to keep (0 = unlimited)
}

// CreateManager creates a new session manager
func CreateManager(dir string) (*Manager, error) {
	return CreateManagerWithOptions(dir, ManagerOptions{})
}

// CreateManagerWithOptions creates a session manager with options
func CreateManagerWithOptions(dir string, opts ManagerOptions) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	m := &Manager{
		dir:         dir,
		autoSave:    opts.AutoSave,
		maxSessions: opts.MaxSessions,
	}

	// Clean up old sessions if maxSessions is set
	if m.maxSessions > 0 {
		if err := m.cleanupOldSessions(); err != nil {
			// Log but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to cleanup old sessions: %v\n", err)
		}
	}

	return m, nil
}

// Create creates a new session
func (m *Manager) Create(_ context.Context) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}
	return NewSession(id), nil
}

// CreateWithID creates a session with a specific ID
func (m *Manager) CreateWithID(id string) *Session {
	return NewSession(id)
}

// CreateFromParent creates a new session that's a child of an existing session
func (m *Manager) CreateFromParent(parentID string) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, err
	}

	sess := NewSession(id)
	sess.SetParentSession(parentID)
	return sess, nil
}

// Load loads a session by ID
func (m *Manager) Load(id string) (*Session, error) {
	path := m.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Initialize nil maps
	if sess.EnvVars == nil {
		sess.EnvVars = make(map[string]string)
	}
	if sess.Messages == nil {
		sess.Messages = make([]types.Message, 0)
	}

	return &sess, nil
}

// LoadOrCreate loads a session or creates a new one if it doesn't exist
func (m *Manager) LoadOrCreate(id string) (*Session, error) {
	sess, err := m.Load(id)
	if err == nil {
		return sess, nil
	}

	// Session doesn't exist, create new
	return m.CreateWithID(id), nil
}

// Save persists a session to disk
func (m *Manager) Save(s *Session) error {
	s.UpdatedAt = time.Now()
	return m.saveInternal(s)
}

// SaveWithoutTouch persists a session to disk without updating UpdatedAt
func (m *Manager) SaveWithoutTouch(s *Session) error {
	return m.saveInternal(s)
}

// saveInternal is the internal save implementation
func (m *Manager) saveInternal(s *Session) error {
	normalizeSessionRawMessages(s)

	path := m.sessionPath(s.ID)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, path)
}

func normalizeSessionRawMessages(s *Session) {
	if s == nil {
		return
	}

	for i := range s.Messages {
		msg := &s.Messages[i]
		for j := range msg.ToolCalls {
			msg.ToolCalls[j].Arguments = types.NormalizeObjectRawMessage(msg.ToolCalls[j].Arguments)
		}
		for j := range msg.Blocks {
			if msg.Blocks[j].Type != types.ContentTypeToolUse {
				continue
			}
			msg.Blocks[j].Input = types.NormalizeObjectRawMessage(msg.Blocks[j].Input)
		}
	}
}

// Delete removes a session file
func (m *Manager) Delete(id string) error {
	path := m.sessionPath(id)
	return os.Remove(path)
}

// Exists checks if a session exists
func (m *Manager) Exists(id string) bool {
	path := m.sessionPath(id)
	_, err := os.Stat(path)
	return err == nil
}

// List lists all session IDs
func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// ListSorted lists all sessions sorted by update time (most recent first)
func (m *Manager) ListSorted() ([]string, error) {
	ids, err := m.List()
	if err != nil {
		return nil, err
	}

	// Load metadata for each session
	type sessionMeta struct {
		id        string
		updatedAt time.Time
	}

	var metas []sessionMeta
	for _, id := range ids {
		sess, err := m.Load(id)
		if err != nil {
			continue
		}
		metas = append(metas, sessionMeta{
			id:        id,
			updatedAt: sess.UpdatedAt,
		})
	}

	// Sort by update time (most recent first)
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].updatedAt.After(metas[j].updatedAt)
	})

	// Extract sorted IDs
	sorted := make([]string, len(metas))
	for i, m := range metas {
		sorted[i] = m.id
	}

	return sorted, nil
}

// GetRecent returns the N most recent sessions
func (m *Manager) GetRecent(n int) ([]*Session, error) {
	ids, err := m.ListSorted()
	if err != nil {
		return nil, err
	}

	if len(ids) > n {
		ids = ids[:n]
	}

	sessions := make([]*Session, 0, len(ids))
	for _, id := range ids {
		sess, err := m.Load(id)
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// GetDirectory returns the session directory
func (m *Manager) GetDirectory() string {
	return m.dir
}

// SetAutoSave sets the auto-save flag
func (m *Manager) SetAutoSave(enabled bool) {
	m.autoSave = enabled
}

// AutoSaveIfEnabled saves the session if auto-save is enabled
func (m *Manager) AutoSaveIfEnabled(s *Session) error {
	if !m.autoSave {
		return nil
	}
	return m.Save(s)
}

// Cleanup removes sessions older than the specified duration
func (m *Manager) Cleanup(olderThan time.Duration) (int, error) {
	ids, err := m.List()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for _, id := range ids {
		sess, err := m.Load(id)
		if err != nil {
			continue
		}

		if sess.UpdatedAt.Before(cutoff) {
			if err := m.Delete(id); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// cleanupOldSessions removes old sessions if maxSessions is exceeded
func (m *Manager) cleanupOldSessions() error {
	if m.maxSessions <= 0 {
		return nil
	}

	ids, err := m.ListSorted()
	if err != nil {
		return err
	}

	if len(ids) <= m.maxSessions {
		return nil
	}

	// Remove oldest sessions
	for i := m.maxSessions; i < len(ids); i++ {
		if err := m.Delete(ids[i]); err != nil {
			// Log but continue
			fmt.Fprintf(os.Stderr, "warning: failed to delete session %s: %v\n", ids[i], err)
		}
	}

	return nil
}

// sessionPath returns the file path for a session
func (m *Manager) sessionPath(id string) string {
	return filepath.Join(m.dir, id+".json")
}

// generateID generates a random session ID
func generateID() (string, error) {
	return utils.GenerateID("", 8)
}

// Snapshot creates a snapshot of the current messages (deep copy)
func Snapshot(messages []types.Message) []types.Message {
	if messages == nil {
		return nil
	}
	snap := make([]types.Message, len(messages))
	copy(snap, messages)
	return snap
}
