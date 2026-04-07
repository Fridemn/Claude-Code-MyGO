package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"claude-code-go/internal/config"
)

type State struct {
	OriginalCWD         string          `json:"original_cwd"`
	ProjectRoot         string          `json:"project_root"`
	CWD                 string          `json:"cwd"`
	SessionID           string          `json:"session_id"`
	ParentSessionID     string          `json:"parent_session_id,omitempty"`
	CurrentModel        string          `json:"current_model"`
	MainLoopModel       string          `json:"main_loop_model"`
	TotalCostUSD        float64         `json:"total_cost_usd"`
	TotalAPIDuration    time.Duration   `json:"total_api_duration"`
	TotalToolDuration   time.Duration   `json:"total_tool_duration"`
	TurnCount           int             `json:"turn_count"`
	ToolCallCount       int             `json:"tool_call_count"`
	StartTime           time.Time       `json:"start_time"`
	LastInteractionTime time.Time       `json:"last_interaction_time"`
	IsInteractive       bool            `json:"is_interactive"`
	ClientType          string          `json:"client_type"`
	AllowedChannels     []string        `json:"allowed_channels,omitempty"`
	ModelUsage          map[string]int  `json:"model_usage,omitempty"`
	SessionFlags        map[string]bool `json:"session_flags,omitempty"`
	InMemoryErrorLog    []InMemoryError `json:"in_memory_error_log,omitempty"`
	LastError           string          `json:"last_error,omitempty"`
	Todos               map[string][]TodoItem `json:"todos,omitempty"`
}

type InMemoryError struct {
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

// TodoItemStatus represents the status of a todo item
type TodoItemStatus string

const (
	TodoStatusPending    TodoItemStatus = "pending"
	TodoStatusInProgress TodoItemStatus = "in_progress"
	TodoStatusCompleted  TodoItemStatus = "completed"
)

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string         `json:"content"`
	Status     TodoItemStatus `json:"status"`
	ActiveForm string         `json:"activeForm"`
}

type Store struct {
	mu    sync.RWMutex
	state State
}

func CreateStore(cfg config.Config) (*Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	root := cwd
	if abs, err := filepath.Abs(cwd); err == nil {
		root = abs
	}

	now := time.Now()
	return &Store{
		state: State{
			OriginalCWD:         root,
			ProjectRoot:         root,
			CWD:                 root,
			CurrentModel:        cfg.Model,
			MainLoopModel:       cfg.Model,
			StartTime:           now,
			LastInteractionTime: now,
			IsInteractive:       true,
			ClientType:          "terminal",
			ModelUsage:          map[string]int{},
			SessionFlags:        map[string]bool{},
		},
	}, nil
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := s.state
	snapshot.AllowedChannels = append([]string(nil), s.state.AllowedChannels...)
	snapshot.InMemoryErrorLog = append([]InMemoryError(nil), s.state.InMemoryErrorLog...)
	snapshot.ModelUsage = cloneIntMap(s.state.ModelUsage)
	snapshot.SessionFlags = cloneBoolMap(s.state.SessionFlags)
	return snapshot
}

func (s *Store) SetSessionID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.SessionID = id
	s.state.LastInteractionTime = time.Now()
}

func (s *Store) SetCurrentModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CurrentModel = model
	s.state.MainLoopModel = model
}

func (s *Store) RecordTurn(model string, apiDuration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.TurnCount++
	s.state.LastInteractionTime = time.Now()
	s.state.TotalAPIDuration += apiDuration
	if strings.TrimSpace(model) != "" {
		s.state.ModelUsage[model]++
	}
}

func (s *Store) RecordToolCall(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.ToolCallCount++
	s.state.TotalToolDuration += duration
	s.state.LastInteractionTime = time.Now()
}

func (s *Store) RecordError(err error) {
	if err == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry := InMemoryError{
		Error:     err.Error(),
		Timestamp: time.Now(),
	}
	s.state.LastError = entry.Error
	s.state.InMemoryErrorLog = append(s.state.InMemoryErrorLog, entry)
	s.state.LastInteractionTime = entry.Timestamp
}

func cloneIntMap(src map[string]int) map[string]int {
	if src == nil {
		return nil
	}
	dst := make(map[string]int, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneBoolMap(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	dst := make(map[string]bool, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// GetOriginalCWD returns the original working directory at session start
func (s *Store) GetOriginalCWD() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.OriginalCWD
}

// GetCWD returns the current working directory
func (s *Store) GetCWD() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.CWD
}

// SetCWD updates the current working directory
func (s *Store) SetCWD(cwd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Resolve symlinks for consistency
	if abs, err := filepath.Abs(cwd); err == nil {
		s.state.CWD = abs
	} else {
		s.state.CWD = cwd
	}
	s.state.LastInteractionTime = time.Now()
}

// ResetCWD resets current working directory to original
func (s *Store) ResetCWD() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CWD = s.state.OriginalCWD
	s.state.LastInteractionTime = time.Now()
}

// IsCWDOutsideOriginal checks if current CWD is outside original directory
func (s *Store) IsCWDOutsideOriginal() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.CWD != s.state.OriginalCWD
}

// PathInOriginalProject checks if path is within original project directory
func (s *Store) PathInOriginalProject(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Resolve both paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absOriginal, err := filepath.Abs(s.state.OriginalCWD)
	if err != nil {
		return false
	}
	// Check if path is under original directory
	rel, err := filepath.Rel(absOriginal, absPath)
	if err != nil {
		return false
	}
	// If relative path starts with "..", it's outside
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// SetSessionFlag sets a session flag value
func (s *Store) SetSessionFlag(key string, value bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.SessionFlags == nil {
		s.state.SessionFlags = map[string]bool{}
	}
	s.state.SessionFlags[key] = value
	s.state.LastInteractionTime = time.Now()
}

// GetSessionFlag gets a session flag value
func (s *Store) GetSessionFlag(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.SessionFlags == nil {
		return false
	}
	return s.state.SessionFlags[key]
}

// GetTodos returns todos for a specific agent/session key
func (s *Store) GetTodos(key string) []TodoItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state.Todos == nil {
		return nil
	}
	return s.state.Todos[key]
}

// SetTodos sets todos for a specific agent/session key
func (s *Store) SetTodos(key string, todos []TodoItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Todos == nil {
		s.state.Todos = map[string][]TodoItem{}
	}
	// If all todos are completed or the list is empty, clear the entry
	if len(todos) == 0 {
		delete(s.state.Todos, key)
	} else {
		s.state.Todos[key] = todos
	}
	s.state.LastInteractionTime = time.Now()
}

// GetSessionID returns the current session ID
func (s *Store) GetSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.SessionID
}
