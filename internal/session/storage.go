package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"claude-go/internal/types"
	"claude-go/internal/utils"
)

// SerializedMessage represents a message stored in the transcript.
type SerializedMessage struct {
	types.Message
	CWD        string    `json:"cwd"`
	UserType   string    `json:"userType"`
	Entrypoint string    `json:"entrypoint,omitempty"`
	SessionID  string    `json:"sessionId"`
	Version    string    `json:"version"`
	GitBranch  string    `json:"gitBranch,omitempty"`
	Slug       string    `json:"slug,omitempty"`
	StoredAt   time.Time `json:"storedAt"`
}

// LogOption represents session log metadata.
type LogOption struct {
	Date         string              `json:"date"`
	Messages     []SerializedMessage `json:"messages"`
	FullPath     string              `json:"fullPath,omitempty"`
	Value        int64               `json:"value"`
	Created      time.Time           `json:"created"`
	Modified     time.Time           `json:"modified"`
	FirstPrompt  string              `json:"firstPrompt"`
	MessageCount int                 `json:"messageCount"`
	FileSize     int64               `json:"fileSize,omitempty"`
	IsSidechain  bool                `json:"isSidechain"`
	IsLite       bool                `json:"isLite,omitempty"`
	SessionID    string              `json:"sessionId,omitempty"`
	TeamName     string              `json:"teamName,omitempty"`
	AgentName    string              `json:"agentName,omitempty"`
	AgentColor   string              `json:"agentColor,omitempty"`
	AgentSetting string              `json:"agentSetting,omitempty"`
	IsTeammate   bool                `json:"isTeammate,omitempty"`
	LeafUUID     string              `json:"leafUuid,omitempty"`
	Summary      string              `json:"summary,omitempty"`
	CustomTitle  string              `json:"customTitle,omitempty"`
	Tag          string              `json:"tag,omitempty"`
	GitBranch    string              `json:"gitBranch,omitempty"`
	ProjectPath  string              `json:"projectPath,omitempty"`
	ProjectName  string              `json:"projectName,omitempty"` // Display name for picker
	PRNumber     int                 `json:"prNumber,omitempty"`
	PRURL        string              `json:"prUrl,omitempty"`
	PRRepository string              `json:"prRepository,omitempty"`
	Mode         string              `json:"mode,omitempty"`
}

// TranscriptEntry represents a single entry in the JSONL transcript.
type TranscriptEntry struct {
	Type      string             `json:"type"`
	Message   *SerializedMessage `json:"message,omitempty"`
	Timestamp string             `json:"timestamp"`
}

// CompactBoundary represents a compact boundary marker.
type CompactBoundary struct {
	Type         string `json:"type"`
	Subtype      string `json:"subtype"`
	Summary      string `json:"summary"`
	Timestamp    string `json:"timestamp"`
	TokensBefore int    `json:"tokensBefore,omitempty"`
	TokensAfter  int    `json:"tokensAfter,omitempty"`
}

// Version string for transcripts
const CurrentVersion = "2.0"

// Manager handles session storage and transcript operations.
type EnhancedManager struct {
	dir            string
	transcriptDir  string
	version        string
	mu             sync.Mutex
	replToolUseIDs map[string]map[string]bool
}

// CreateEnhancedManager creates a new enhanced session manager.
// Sessions are stored directly in the project directory (matches TS format).
func CreateEnhancedManager(dir string) (*EnhancedManager, error) {
	// Check if the directory already exists with TS format (direct .jsonl files)
	// or if we need to create it
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	// Use the directory directly as transcriptDir (matches TS format)
	// TS stores sessions in ~/.claude/projects/<project>/ directly, not in transcripts subdir
	// Go CLI uses ~/.claude-go/projects/<project>/ for independent storage
	return &EnhancedManager{
		dir:            dir,
		transcriptDir:  dir,
		version:        CurrentVersion,
		replToolUseIDs: make(map[string]map[string]bool),
	}, nil
}

// GetTranscriptPath returns the path to a session's transcript file.
func (m *EnhancedManager) GetTranscriptPath(sessionID string) string {
	return filepath.Join(m.transcriptDir, sessionID+".jsonl")
}

// GetTranscriptPathForSession is an alias for GetTranscriptPath.
func (m *EnhancedManager) GetTranscriptPathForSession(sessionID string) string {
	return m.GetTranscriptPath(sessionID)
}

// GetProjectDir returns the projects directory.
func (m *EnhancedManager) GetProjectDir() string {
	return filepath.Dir(m.dir)
}

// SessionIdExists checks if a session transcript exists.
func (m *EnhancedManager) SessionIdExists(sessionID string) bool {
	path := m.GetTranscriptPath(sessionID)
	_, err := os.Stat(path)
	return err == nil
}

// SerializeMessage converts a Message to SerializedMessage for storage.
func SerializeMessage(msg types.Message, sessionID, cwd, userType string) SerializedMessage {
	return SerializedMessage{
		Message:   msg,
		CWD:       cwd,
		UserType:  userType,
		SessionID: sessionID,
		Version:   CurrentVersion,
		StoredAt:  time.Now(),
	}
}

// DeserializeMessage converts SerializedMessage back to Message.
func DeserializeMessage(sm SerializedMessage) (types.Message, error) {
	msg := sm.Message
	// Use stored timestamp if available
	if !sm.StoredAt.IsZero() {
		msg.Timestamp = sm.StoredAt
	}
	return msg, nil
}

// RecordTranscript appends messages to the JSONL transcript file.
func (m *EnhancedManager) RecordTranscript(sessionID string, messages []types.Message, cwd, userType string) error {
	if len(messages) == 0 {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.GetTranscriptPath(sessionID)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	replIDs := m.replToolUseIDs[sessionID]
	if replIDs == nil {
		replIDs = m.loadREPLToolUseIDsFromTranscript(sessionID)
		m.replToolUseIDs[sessionID] = replIDs
	}
	for id := range collectREPLToolUseIDs(messages) {
		replIDs[id] = true
	}
	if err := m.persistREPLToolUseIDs(sessionID, replIDs); err != nil {
		return fmt.Errorf("persist repl id cache: %w", err)
	}

	for _, msg := range cleanMessagesForTranscript(messages, userType, replIDs) {
		entry := TranscriptEntry{
			Type:      "message",
			Message:   ptrToSerialize(&msg, sessionID, cwd, userType),
			Timestamp: time.Now().Format(time.RFC3339),
		}

		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}

		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write entry: %w", err)
		}
	}

	return nil
}

func (m *EnhancedManager) loadREPLToolUseIDsFromTranscript(sessionID string) map[string]bool {
	ids := map[string]bool{}

	for id := range m.loadPersistedREPLToolUseIDs(sessionID) {
		ids[id] = true
	}

	previous, err := m.ReadTranscript(sessionID)
	if err != nil || len(previous) == 0 {
		return ids
	}
	for id := range collectREPLToolUseIDs(previous) {
		ids[id] = true
	}
	return ids
}

func (m *EnhancedManager) replToolUseIDsPath(sessionID string) string {
	return filepath.Join(m.transcriptDir, sessionID+".repl_ids.json")
}

func (m *EnhancedManager) loadPersistedREPLToolUseIDs(sessionID string) map[string]bool {
	ids := map[string]bool{}
	path := m.replToolUseIDsPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return ids
	}

	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return ids
	}
	for _, id := range values {
		id = strings.TrimSpace(id)
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func (m *EnhancedManager) persistREPLToolUseIDs(sessionID string, ids map[string]bool) error {
	values := make([]string, 0, len(ids))
	for id := range ids {
		if strings.TrimSpace(id) != "" {
			values = append(values, id)
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return os.WriteFile(m.replToolUseIDsPath(sessionID), data, 0644)
}

func cleanMessagesForTranscript(messages []types.Message, userType string, replToolUseIDs map[string]bool) []types.Message {
	filtered := make([]types.Message, 0, len(messages))
	if len(messages) == 0 {
		return filtered
	}

	isAnt := strings.EqualFold(strings.TrimSpace(userType), "ant")

	for _, msg := range messages {
		if !shouldIncludeTranscriptMessage(msg) {
			continue
		}

		current := msg
		if !isAnt {
			if stripREPLWrapperBlocks(&current, replToolUseIDs) {
				continue
			}
			if current.Role == types.RoleTool && current.ToolCallID != "" && replToolUseIDs[current.ToolCallID] {
				continue
			}
			current.IsVirtual = false
		}

		filtered = append(filtered, current)
	}
	return filtered
}

func shouldIncludeTranscriptMessage(msg types.Message) bool {
	if msg.Type == types.MessageTypeProgress {
		return false
	}
	if types.IsNotEmptyMessage(msg) {
		return true
	}
	return len(msg.ToolCalls) > 0 || len(msg.Blocks) > 0
}

func collectREPLToolUseIDs(messages []types.Message) map[string]bool {
	ids := map[string]bool{}
	for _, msg := range messages {
		for _, call := range msg.ToolCalls {
			if strings.EqualFold(strings.TrimSpace(call.Name), "REPL") && strings.TrimSpace(call.ID) != "" {
				ids[call.ID] = true
			}
		}
		for _, block := range msg.Blocks {
			if block.Type == types.ContentTypeToolUse &&
				strings.EqualFold(strings.TrimSpace(block.Name), "REPL") &&
				strings.TrimSpace(block.ID) != "" {
				ids[block.ID] = true
			}
		}
	}
	return ids
}

func stripREPLWrapperBlocks(msg *types.Message, replToolUseIDs map[string]bool) bool {
	if msg == nil {
		return true
	}

	if len(msg.ToolCalls) > 0 {
		filteredCalls := make([]types.ToolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			if strings.EqualFold(strings.TrimSpace(call.Name), "REPL") {
				continue
			}
			filteredCalls = append(filteredCalls, call)
		}
		msg.ToolCalls = filteredCalls
	}

	if len(msg.Blocks) > 0 {
		filteredBlocks := make([]types.ContentBlock, 0, len(msg.Blocks))
		for _, block := range msg.Blocks {
			if block.Type == types.ContentTypeToolUse &&
				strings.EqualFold(strings.TrimSpace(block.Name), "REPL") {
				continue
			}
			if block.Type == types.ContentTypeToolResult && replToolUseIDs[block.ToolUseID] {
				continue
			}
			filteredBlocks = append(filteredBlocks, block)
		}
		msg.Blocks = filteredBlocks
	}

	if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 && len(msg.Blocks) == 0 {
		return true
	}
	return false
}

func ptrToSerialize(msg *types.Message, sessionID, cwd, userType string) *SerializedMessage {
	sm := SerializeMessage(*msg, sessionID, cwd, userType)
	return &sm
}

// RecordCompactBoundary records a compact boundary in the transcript.
func (m *EnhancedManager) RecordCompactBoundary(sessionID string, summary string, tokensBefore, tokensAfter int) error {
	path := m.GetTranscriptPath(sessionID)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	boundary := CompactBoundary{
		Type:         "system",
		Subtype:      "compact_boundary",
		Summary:      summary,
		Timestamp:    time.Now().Format(time.RFC3339),
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
	}

	entry := TranscriptEntry{
		Type:      "compact",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Store boundary in message field for JSONL compatibility
	msgData, _ := json.Marshal(map[string]interface{}{
		"type":         "system",
		"subtype":      "compact_boundary",
		"summary":      summary,
		"tokensBefore": tokensBefore,
		"tokensAfter":  tokensAfter,
		"boundary":     boundary,
	})
	entry.Message = &SerializedMessage{
		Message: types.Message{
			Type:    types.MessageTypeSystem,
			Content: string(msgData),
		},
		SessionID: sessionID,
		Version:   CurrentVersion,
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	_, err = file.Write(append(entryData, '\n'))
	return err
}

// ReadTranscript reads all messages from a session transcript.
func (m *EnhancedManager) ReadTranscript(sessionID string) ([]types.Message, error) {
	path := m.GetTranscriptPath(sessionID)
	return m.readTranscriptFile(path)
}

// ReadTranscriptFile reads messages from a specific transcript file.
func (m *EnhancedManager) ReadTranscriptFile(path string) ([]types.Message, error) {
	return m.readTranscriptFile(path)
}

func (m *EnhancedManager) readTranscriptFile(path string) ([]types.Message, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	var messages []types.Message
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Message != nil && entry.Message.Type != "compact" {
			msg, err := DeserializeMessage(*entry.Message)
			if err != nil {
				continue
			}
			if shouldIncludeTranscriptMessage(msg) {
				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

// ReadTranscriptHead reads the first N bytes of a transcript.
func (m *EnhancedManager) ReadTranscriptHead(sessionID string, maxBytes int) ([]types.Message, error) {
	path := m.GetTranscriptPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Truncate to maxBytes
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}

	var messages []types.Message
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Message != nil {
			msg, err := DeserializeMessage(*entry.Message)
			if err != nil {
				continue
			}
			if shouldIncludeTranscriptMessage(msg) {
				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

// ReadTranscriptTail reads the last N bytes of a transcript.
func (m *EnhancedManager) ReadTranscriptTail(sessionID string, maxBytes int) ([]types.Message, error) {
	path := m.GetTranscriptPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Find last newline and read from there
	truncated := false
	if len(data) > maxBytes {
		// Find first complete line after maxBytes
		for i := maxBytes; i < len(data) && i < maxBytes+1000; i++ {
			if data[i] == '\n' {
				data = data[i+1:]
				truncated = true
				break
			}
		}
		if !truncated {
			data = data[len(data)-maxBytes:]
		}
	}

	var messages []types.Message
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry TranscriptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Message != nil {
			msg, err := DeserializeMessage(*entry.Message)
			if err != nil {
				continue
			}
			if shouldIncludeTranscriptMessage(msg) {
				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

// ListSessions returns all session logs in the directory.
// Extracts first prompt and custom title from session files.
func (m *EnhancedManager) ListSessions() ([]LogOption, error) {
	entries, err := os.ReadDir(m.transcriptDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	var logs []LogOption
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(m.transcriptDir, entry.Name())

		// Extract metadata from session file
		firstPrompt, customTitle, messageCount := m.extractSessionMetadata(fullPath)

		log := LogOption{
			SessionID:    sessionID,
			FullPath:     fullPath,
			Modified:     info.ModTime(),
			Value:        info.Size(),
			FileSize:     info.Size(),
			MessageCount: messageCount,
			IsSidechain:  false,
			FirstPrompt:  firstPrompt,
			CustomTitle:  customTitle,
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// extractSessionMetadata reads session file to extract first prompt and custom title
func (m *EnhancedManager) extractSessionMetadata(path string) (string, string, int) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", 0
	}
	defer file.Close()

	var firstPrompt string
	var customTitle string
	var messageCount int

	// Read file line by line to find first user message and custom title
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Check for custom title entry
		if entry["type"] == "custom-title" {
			if title, ok := entry["customTitle"].(string); ok {
				customTitle = title
			}
			continue
		}

		// JSONL format: {"type": "message", "message": {"role": "user", "content": "..."}}
		if entry["type"] == "message" {
			msg, ok := entry["message"].(map[string]interface{})
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			if role == "user" && firstPrompt == "" {
				if content, ok := msg["content"].(string); ok {
					if len(content) > 100 {
						content = content[:100]
					}
					firstPrompt = content
				}
			}
			if role == "user" || role == "assistant" {
				messageCount++
			}
		}
	}

	return firstPrompt, customTitle, messageCount
}

// DeleteTranscript deletes a session transcript file.
func (m *EnhancedManager) DeleteTranscript(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.replToolUseIDs, sessionID)

	path := m.GetTranscriptPath(sessionID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	cachePath := m.replToolUseIDsPath(sessionID)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SaveCustomTitle saves a custom title to the transcript as a distinct entry.
// Mirrors TS saveCustomTitle: appends a custom-title entry to the JSONL file.
func (m *EnhancedManager) SaveCustomTitle(sessionID, customTitle string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.GetTranscriptPath(sessionID)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	entry := map[string]interface{}{
		"type":        "custom-title",
		"customTitle": customTitle,
		"sessionId":   sessionID,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

// SaveAgentName saves an agent name to the transcript.
func (m *EnhancedManager) SaveAgentName(sessionID, agentName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.GetTranscriptPath(sessionID)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	entry := map[string]interface{}{
		"type":       "agent-name",
		"agentName":  agentName,
		"sessionId":  sessionID,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

// LoadCustomTitle loads the custom title from a transcript.
// Returns the most recent custom-title entry.
func (m *EnhancedManager) LoadCustomTitle(sessionID string) (string, error) {
	path := m.GetTranscriptPath(sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file: %w", err)
	}

	var customTitle string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entryType, ok := entry["type"].(string); ok && entryType == "custom-title" {
			if title, ok := entry["customTitle"].(string); ok {
				customTitle = title // Keep most recent
			}
		}
	}

	return customTitle, nil
}

// GetTranscriptStats returns statistics about a transcript.
func (m *EnhancedManager) GetTranscriptStats(sessionID string) (int, int64, error) {
	path := m.GetTranscriptPath(sessionID)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	messages, err := m.ReadTranscript(sessionID)
	if err != nil {
		return 0, info.Size(), err
	}

	return len(messages), info.Size(), nil
}

// AppendJSONL appends a JSON object as a line to a file.
func AppendJSONL(path string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer file.Close()

	_, err = file.Write(append(jsonData, '\n'))
	return err
}

// ReadJSONL reads all JSON objects from a JSONL file.
func ReadJSONL[T any](path string) ([]T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read: %w", err)
	}

	var result []T
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var obj T
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		result = append(result, obj)
	}

	return result, nil
}

// GenerateSessionID generates a new session ID.
func GenerateSessionID() string {
	id, err := utils.GenerateID("sess_", 8)
	if err != nil {
		// Fallback to a simple ID if generation fails
		return fmt.Sprintf("sess_%d", time.Now().UnixNano()%1000000)
	}
	return id
}

// CreateSession creates a new session with transcript.
func (m *EnhancedManager) CreateSession(ctx context.Context, cwd, userType string) (*Session, error) {
	id := GenerateSessionID()
	session := &Session{
		ID:       id,
		Messages: []types.Message{},
	}

	// Create empty transcript file
	path := m.GetTranscriptPath(id)
	if err := AppendJSONL(path, map[string]string{
		"type":      "session_start",
		"sessionId": id,
		"timestamp": time.Now().Format(time.RFC3339),
	}); err != nil {
		return nil, fmt.Errorf("create transcript: %w", err)
	}

	return session, nil
}

// SaveSession saves a session and records its transcript.
func (m *EnhancedManager) SaveSession(session *Session, cwd, userType string) error {
	// Save session metadata
	sessionPath := filepath.Join(m.dir, session.ID+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}

	// Record transcript
	if len(session.Messages) > 0 {
		if err := m.RecordTranscript(session.ID, session.Messages, cwd, userType); err != nil {
			return fmt.Errorf("record transcript: %w", err)
		}
	}

	return nil
}

// LoadSession loads a session and its transcript.
func (m *EnhancedManager) LoadSession(sessionID string) (*Session, error) {
	// Load session metadata
	sessionPath := filepath.Join(m.dir, sessionID+".json")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}

	// Load transcript
	messages, err := m.ReadTranscript(sessionID)
	if err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	// Merge with existing messages (transcript has latest)
	if len(messages) > 0 {
		session.Messages = messages
	}

	return &session, nil
}

// ---------------------------------------------------------------------------
// Lite session file reading (TS sessionStoragePortable.ts)
// ---------------------------------------------------------------------------

// LiteReadBufSize is the size of the head/tail buffer for lite metadata reads.
// TS: LITE_READ_BUF_SIZE = 65536 (64KB)
const LiteReadBufSize = 65536

// LiteSessionFile represents a lightweight session file read.
// TS: LiteSessionFile type
type LiteSessionFile struct {
	MTime   time.Time `json:"mtime"`
	Size    int64     `json:"size"`
	Head    string    `json:"head"`
	Tail    string    `json:"tail"`
}

// ReadHeadAndTail reads the first and last LiteReadBufSize bytes of a file.
// TS sessionStoragePortable.ts:215-242 - readHeadAndTail
// For small files where head covers tail, tail === head.
// Returns { head: '', tail: '' } on any error.
func ReadHeadAndTail(filePath string, fileSize int64) (head string, tail string) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", ""
	}
	defer file.Close()

	// Read head
	headBuf := make([]byte, LiteReadBufSize)
	headN, err := file.ReadAt(headBuf, 0)
	if err != nil && err != io.EOF {
		return "", ""
	}
	if headN == 0 {
		return "", ""
	}
	head = string(headBuf[:headN])

	// Calculate tail offset
	tailOffset := fileSize - LiteReadBufSize
	if tailOffset <= 0 {
		// File is smaller than buffer, tail = head
		return head, head
	}

	// Read tail
	tailBuf := make([]byte, LiteReadBufSize)
	tailN, err := file.ReadAt(tailBuf, tailOffset)
	if err != nil && err != io.EOF {
		return head, head
	}
	tail = string(tailBuf[:tailN])

	return head, tail
}

// ReadSessionLite opens a session file, stats it, and reads head + tail.
// TS sessionStoragePortable.ts:256-282 - readSessionLite
// Returns nil on any error.
func ReadSessionLite(filePath string) *LiteSessionFile {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil
	}

	head, tail := ReadHeadAndTail(filePath, info.Size())
	if head == "" && tail == "" {
		return nil
	}

	return &LiteSessionFile{
		MTime: info.ModTime(),
		Size:  info.Size(),
		Head:  head,
		Tail:  tail,
	}
}

// ExtractFirstPromptFromHead extracts the first meaningful user prompt from a JSONL head chunk.
// TS sessionStoragePortable.ts:135-202 - extractFirstPromptFromHead
// Skips tool_result, isMeta, isCompactSummary, command-name messages, and auto-generated patterns.
func ExtractFirstPromptFromHead(head string) string {
	lines := strings.Split(head, "\n")
	commandFallback := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// JSONL format: {"type": "message", "message": {"role": "user", ...}}
		// Must contain "message" as type and "user" as role
		if !strings.Contains(line, `"type":"message"`) && !strings.Contains(line, `"type": "message"`) {
			continue
		}
		if !strings.Contains(line, `"role":"user"`) && !strings.Contains(line, `"role": "user"`) {
			continue
		}

		// Skip tool_result
		if strings.Contains(line, `"tool_result"`) {
			continue
		}

		// Skip isMeta
		if strings.Contains(line, `"isMeta":true`) || strings.Contains(line, `"isMeta": true`) {
			continue
		}

		// Skip isCompactSummary
		if strings.Contains(line, `"isCompactSummary":true`) || strings.Contains(line, `"isCompactSummary": true`) {
			continue
		}

		// Extract content
		content := extractJsonStringField(line, "content")
		if content == "" {
			continue
		}

		// Skip auto-generated patterns (TS: SKIP_FIRST_PROMPT_PATTERN)
		// Matches: <ide...>, <hook...>, <tick...>, <channel...>, [Request interrupted...]
		if strings.HasPrefix(strings.TrimSpace(content), "<") &&
			!strings.HasPrefix(strings.TrimSpace(content), "<command-name>") {
			// Check for command-name fallback
			if matches := commandNameRegex.FindStringSubmatch(content); len(matches) > 1 {
				commandFallback = matches[1]
			}
			continue
		}
		if strings.HasPrefix(content, "[Request interrupted") {
			continue
		}

		// Found first prompt - truncate to 200 chars
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		return content
	}

	// Return command fallback if found
	if commandFallback != "" {
		return commandFallback
	}
	return ""
}

// extractJsonStringField extracts a JSON string field value from raw text.
// TS sessionStoragePortable.ts:53-76 - extractJsonStringField
func extractJsonStringField(text string, key string) string {
	patterns := []string{
		`"` + key + `":"`,
		`"` + key + `": "`,
	}

	for _, pattern := range patterns {
		idx := strings.Index(text, pattern)
		if idx < 0 {
			continue
		}

		valueStart := idx + len(pattern)
		i := valueStart
		for i < len(text) {
			if text[i] == '\\' {
				i += 2
				continue
			}
			if text[i] == '"' {
				raw := text[valueStart:i]
				return unescapeJsonString(raw)
			}
			i++
		}
	}
	return ""
}

// unescapeJsonString unescapes a JSON string value extracted as raw text.
// TS sessionStoragePortable.ts:39-46 - unescapeJsonString
func unescapeJsonString(raw string) string {
	if !strings.Contains(raw, "\\") {
		return raw
	}
	// Parse as JSON string
	var result string
	if err := json.Unmarshal([]byte(`"` + raw + `"`), &result); err != nil {
		return raw
	}
	return result
}

// commandNameRegex matches <command-name>...</command-name>
var commandNameRegex = regexp.MustCompile(`<command-name>(.*?)</command-name>`)
