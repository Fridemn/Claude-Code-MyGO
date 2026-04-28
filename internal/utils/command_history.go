package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CommandHistoryConfig holds configuration for the command history manager
type CommandHistoryConfig struct {
	ConfigDir   string // Directory to store history file
	MaxEntries  int    // Maximum number of entries to keep
	ProjectRoot string // Project root path (for future multi-project support)
	SessionID   string // Session ID (for future per-session history)
}

// CommandHistoryEntry represents a single history entry
type CommandHistoryEntry struct {
	Command   string `json:"command"`
	Timestamp string `json:"timestamp"`
}

// CommandHistoryManager manages persistent command history
type CommandHistoryManager struct {
	mu             sync.RWMutex
	entries        []CommandHistoryEntry
	filePath       string
	maxEntries     int
	projectRoot    string
	sessionID      string
	pendingEntries []CommandHistoryEntry // Entries waiting to be flushed to file
}

// CreateCommandHistoryManager creates a new command history manager
func CreateCommandHistoryManager(config CommandHistoryConfig) *CommandHistoryManager {
	// Set defaults
	if config.MaxEntries <= 0 {
		config.MaxEntries = 100
	}

	mgr := &CommandHistoryManager{
		entries:        make([]CommandHistoryEntry, 0),
		maxEntries:     config.MaxEntries,
		projectRoot:    config.ProjectRoot,
		sessionID:      config.SessionID,
		pendingEntries: make([]CommandHistoryEntry, 0),
	}

	// Determine config directory
	configDir := config.ConfigDir
	if configDir == "" {
		// Use Go CLI specific location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return mgr // Return manager without persistence
		}
		configDir = filepath.Join(homeDir, ".claude-go")
	}

	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return mgr // Return manager without persistence
	}

	mgr.filePath = filepath.Join(configDir, "history.jsonl")

	// Load existing history
	mgr.loadFromFile()

	return mgr
}

// Add adds a command to the history
func (m *CommandHistoryManager) Add(command string) {
	if command == "" {
		return
	}

	// Trim whitespace
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove duplicates (move to front if exists)
	m.entries = m.removeDuplicates(m.entries, command)

	// Add new entry at the beginning
	entry := CommandHistoryEntry{
		Command:   command,
		Timestamp: "", // Will be set when flushed
	}
	m.entries = append([]CommandHistoryEntry{entry}, m.entries...)

	// Trim to max entries
	if len(m.entries) > m.maxEntries {
		m.entries = m.entries[:m.maxEntries]
	}

	// Add to pending for persistence
	m.pendingEntries = append(m.pendingEntries, entry)

	// Flush to file asynchronously (simplified: flush immediately)
	m.flushToFile()
}

// GetAll returns all history entries as strings (newest first)
func (m *CommandHistoryManager) GetAll() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.entries))
	for i, entry := range m.entries {
		result[i] = entry.Command
	}
	return result
}

// Search searches history for commands containing the query
func (m *CommandHistoryManager) Search(query string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	var results []string
	for _, entry := range m.entries {
		if strings.Contains(strings.ToLower(entry.Command), query) {
			results = append(results, entry.Command)
		}
	}
	return results
}

// removeDuplicates removes duplicate commands from the slice
func (m *CommandHistoryManager) removeDuplicates(entries []CommandHistoryEntry, command string) []CommandHistoryEntry {
	for i, entry := range entries {
		if entry.Command == command {
			// Remove the duplicate
			return append(entries[:i], entries[i+1:]...)
		}
	}
	return entries
}

// loadFromFile loads history from the JSONL file
func (m *CommandHistoryManager) loadFromFile() {
	if m.filePath == "" {
		return
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		// File doesn't exist or error reading
		return
	}

	// Parse JSONL format (one JSON object per line)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry CommandHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Skip duplicates while loading
		if !m.hasEntry(m.entries, entry.Command) {
			m.entries = append(m.entries, entry)
		}
	}

	// Reverse to get newest first
	m.reverseEntries()
}

// flushToFile saves history to the JSONL file
func (m *CommandHistoryManager) flushToFile() {
	if m.filePath == "" {
		return
	}

	// Build JSONL content
	var lines []string
	for _, entry := range m.entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		lines = append(lines, string(data))
	}

	// Write to file
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(m.filePath, []byte(content), 0644); err != nil {
		// Log error but don't fail
	}
}

// hasEntry checks if the entry already exists in the slice
func (m *CommandHistoryManager) hasEntry(entries []CommandHistoryEntry, command string) bool {
	for _, entry := range entries {
		if entry.Command == command {
			return true
		}
	}
	return false
}

// reverseEntries reverses the entries slice (newest first)
func (m *CommandHistoryManager) reverseEntries() {
	for i, j := 0, len(m.entries)-1; i < j; i, j = i+1, j-1 {
		m.entries[i], m.entries[j] = m.entries[j], m.entries[i]
	}
}
