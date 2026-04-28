package session

import (
	"time"

	"claude-go/internal/types"
)

// Session represents a conversation session
type Session struct {
	ID        string          `json:"id"`
	Messages  []types.Message `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`

	// Metadata
	Model       string            `json:"model,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	TurnCount   int               `json:"turn_count"`
	TotalTokens int               `json:"total_tokens"`

	// Custom title for the session (user-specified name)
	CustomTitle string `json:"custom_title,omitempty"`
	// Agent name for display
	AgentName string `json:"agent_name,omitempty"`

	// Cost tracking
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalAPICalls int    `json:"total_api_calls"`

	// Session state
	IsCompacted  bool   `json:"is_compacted"`
	CompactedAt  *time.Time `json:"compacted_at,omitempty"`
	OriginalSize int    `json:"original_size,omitempty"`

	// Parent session for subagent tracking
	ParentSessionID string `json:"parent_session_id,omitempty"`
}

// NewSession creates a new session with the given ID
func NewSession(id string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		Messages:  make([]types.Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
		EnvVars:   make(map[string]string),
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(msg types.Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
	s.TurnCount++

	// Update token count if available
	if msg.Usage != nil && (msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0) {
		s.TotalTokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
	}
}

// GetMessages returns all messages in the session
func (s *Session) GetMessages() []types.Message {
	return s.Messages
}

// GetLastMessage returns the last message in the session
func (s *Session) GetLastMessage() *types.Message {
	if len(s.Messages) == 0 {
		return nil
	}
	return &s.Messages[len(s.Messages)-1]
}

// GetMessageCount returns the number of messages
func (s *Session) GetMessageCount() int {
	return len(s.Messages)
}

// TrimToLastNMessages keeps only the last N messages
func (s *Session) TrimToLastNMessages(n int) []types.Message {
	if len(s.Messages) <= n {
		return nil
	}

	// Return the trimmed messages (for potential summarization)
	trimmed := make([]types.Message, len(s.Messages)-n)
	copy(trimmed, s.Messages[:len(s.Messages)-n])

	// Keep only the last N messages
	s.Messages = s.Messages[len(s.Messages)-n:]
	s.UpdatedAt = time.Now()
	s.IsCompacted = true
	now := time.Now()
	s.CompactedAt = &now

	return trimmed
}

// TrimToTokenLimit trims messages to fit within a token limit
// Returns trimmed messages that were removed
func (s *Session) TrimToTokenLimit(maxTokens int) []types.Message {
	if s.TotalTokens <= maxTokens {
		return nil
	}

	var trimmed []types.Message
	var currentTokens int

	// Find the index where we should start keeping messages
	startIdx := 0
	for i := len(s.Messages) - 1; i >= 0; i-- {
		msgTokens := estimateTokens(s.Messages[i])
		if currentTokens+msgTokens > maxTokens {
			startIdx = i + 1
			break
		}
		currentTokens += msgTokens
	}

	if startIdx > 0 {
		trimmed = make([]types.Message, startIdx)
		copy(trimmed, s.Messages[:startIdx])
		s.Messages = s.Messages[startIdx:]
		s.TotalTokens = currentTokens
		s.UpdatedAt = time.Now()
		s.IsCompacted = true
		now := time.Now()
		s.CompactedAt = &now
	}

	return trimmed
}

// estimateTokens provides a rough estimate of tokens in a message
func estimateTokens(msg types.Message) int {
	// Rough estimate: ~4 characters per token
	contentLen := len(msg.Content)
	if contentLen == 0 {
		return 0
	}
	return contentLen / 4
}

// UpdateCost updates the session cost tracking
func (s *Session) UpdateCost(costUSD float64, apiCalls int) {
	s.TotalCostUSD += costUSD
	s.TotalAPICalls += apiCalls
	s.UpdatedAt = time.Now()
}

// SetModel sets the model used in this session
func (s *Session) SetModel(model string) {
	s.Model = model
	s.UpdatedAt = time.Now()
}

// SetWorkingDir sets the working directory for this session
func (s *Session) SetWorkingDir(cwd string) {
	s.Cwd = cwd
	s.UpdatedAt = time.Now()
}

// SetEnvVar sets an environment variable for this session
func (s *Session) SetEnvVar(key, value string) {
	if s.EnvVars == nil {
		s.EnvVars = make(map[string]string)
	}
	s.EnvVars[key] = value
	s.UpdatedAt = time.Now()
}

// SetParentSession sets the parent session ID
func (s *Session) SetParentSession(parentID string) {
	s.ParentSessionID = parentID
	s.UpdatedAt = time.Now()
}

// SetCustomTitle sets the custom title for the session
func (s *Session) SetCustomTitle(title string) {
	s.CustomTitle = title
	s.UpdatedAt = time.Now()
}

// SetAgentName sets the agent name for display
func (s *Session) SetAgentName(name string) {
	s.AgentName = name
	s.UpdatedAt = time.Now()
}

// GetDuration returns how long the session has been active
func (s *Session) GetDuration() time.Duration {
	return s.UpdatedAt.Sub(s.CreatedAt)
}

// IsActive returns true if the session has had activity in the last hour
func (s *Session) IsActive() bool {
	return time.Since(s.UpdatedAt) < time.Hour
}

// Clone creates a deep copy of the session
func (s *Session) Clone() *Session {
	clone := &Session{
		ID:              s.ID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
		Model:           s.Model,
		Cwd:             s.Cwd,
		TurnCount:       s.TurnCount,
		TotalTokens:     s.TotalTokens,
		TotalCostUSD:    s.TotalCostUSD,
		TotalAPICalls:   s.TotalAPICalls,
		IsCompacted:     s.IsCompacted,
		OriginalSize:    s.OriginalSize,
		ParentSessionID: s.ParentSessionID,
		CustomTitle:     s.CustomTitle,
		AgentName:       s.AgentName,
	}

	// Deep copy messages
	clone.Messages = make([]types.Message, len(s.Messages))
	copy(clone.Messages, s.Messages)

	// Deep copy env vars
	if s.EnvVars != nil {
		clone.EnvVars = make(map[string]string, len(s.EnvVars))
		for k, v := range s.EnvVars {
			clone.EnvVars[k] = v
		}
	}

	// Copy compaction time
	if s.CompactedAt != nil {
		t := *s.CompactedAt
		clone.CompactedAt = &t
	}

	return clone
}
