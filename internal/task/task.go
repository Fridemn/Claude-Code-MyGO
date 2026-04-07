package task

import (
	"sort"
	"sync"
	"time"

	"claude-code-go/internal/types"
	"claude-code-go/internal/utils"
)

type Type string

const (
	TypeLocalAgent Type = "local_agent"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusKilled    Status = "killed"
)

type AgentTask struct {
	ID                 string          `json:"id"`
	Type               Type            `json:"type"`
	Status             Status          `json:"status"`
	Description        string          `json:"description"`
	AgentType          string          `json:"agent_type"`
	SessionID          string          `json:"session_id,omitempty"`
	Model              string          `json:"model,omitempty"`
	Prompt             string          `json:"prompt"`
	Summary            string          `json:"summary,omitempty"`
	Output             string          `json:"output,omitempty"`
	Error              string          `json:"error,omitempty"`
	TurnCount          int             `json:"turn_count"`
	LastUserPrompt     string          `json:"last_user_prompt,omitempty"`
	LastAssistantReply string          `json:"last_assistant_reply,omitempty"`
	StartTime          time.Time       `json:"start_time"`
	EndTime            time.Time       `json:"end_time,omitempty"`
	UpdatedAt          time.Time       `json:"updated_at,omitempty"`
	Background         bool            `json:"background"`
	Messages           []types.Message `json:"messages,omitempty"`
	NotificationSent   bool            `json:"notification_sent"`
	CompletionNotified bool            `json:"completion_notified"`
}

type Notice struct {
	TaskID       string
	AgentType    string
	Status       Status
	Description  string
	Output       string
	Error        string
	IsBackground bool
	Kind         string
}

type Manager struct {
	mu      sync.RWMutex
	tasks   map[string]*AgentTask
	notices []Notice
}

func EmptyManager() *Manager {
	return &Manager{tasks: map[string]*AgentTask{}}
}

func (m *Manager) CreateAgentTask(description, agentType, sessionID, model, prompt string, background bool) (*AgentTask, error) {
	id, err := generateID("a")
	if err != nil {
		return nil, err
	}
	task := &AgentTask{
		ID:             id,
		Type:           TypeLocalAgent,
		Status:         StatusPending,
		Description:    description,
		AgentType:      agentType,
		SessionID:      sessionID,
		Model:          model,
		Prompt:         prompt,
		LastUserPrompt: prompt,
		StartTime:      time.Now(),
		UpdatedAt:      time.Now(),
		Background:     background,
	}
	m.mu.Lock()
	m.tasks[id] = task
	m.mu.Unlock()
	return cloneTask(task), nil
}

func (m *Manager) UpdateMessages(id string, messages []types.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Messages = append([]types.Message(nil), messages...)
		task.UpdatedAt = time.Now()
	}
}

func (m *Manager) SetInput(id, prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Prompt = prompt
		task.LastUserPrompt = prompt
		task.UpdatedAt = time.Now()
	}
}

func (m *Manager) SetSessionID(id, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.SessionID = sessionID
		task.UpdatedAt = time.Now()
	}
}

func (m *Manager) SetRunning(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Status = StatusRunning
		task.UpdatedAt = time.Now()
	}
}

func (m *Manager) Complete(id, output, summary string, messages []types.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Status = StatusCompleted
		task.Output = output
		task.Summary = summary
		task.TurnCount++
		task.LastAssistantReply = output
		task.Messages = append([]types.Message(nil), messages...)
		task.EndTime = time.Now()
		task.UpdatedAt = task.EndTime
		if task.Background && !task.NotificationSent {
			m.notices = append(m.notices, Notice{
				TaskID:       task.ID,
				AgentType:    task.AgentType,
				Status:       task.Status,
				Description:  task.Description,
				Output:       task.Summary,
				IsBackground: true,
				Kind:         "completed",
			})
			task.NotificationSent = true
		}
	}
}

func (m *Manager) Fail(id string, err error, summary string, messages []types.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Status = StatusFailed
		task.Error = err.Error()
		task.Summary = summary
		task.TurnCount++
		task.LastAssistantReply = summary
		task.Messages = append([]types.Message(nil), messages...)
		task.EndTime = time.Now()
		task.UpdatedAt = task.EndTime
		if task.Background && !task.NotificationSent {
			m.notices = append(m.notices, Notice{
				TaskID:       task.ID,
				AgentType:    task.AgentType,
				Status:       task.Status,
				Description:  task.Description,
				Error:        task.Error,
				IsBackground: true,
				Kind:         "failed",
			})
			task.NotificationSent = true
		}
	}
}

func (m *Manager) Kill(id, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return false
	}
	if task.Status == StatusCompleted || task.Status == StatusFailed || task.Status == StatusKilled {
		return true
	}
	task.Status = StatusKilled
	task.Error = reason
	task.Summary = reason
	task.EndTime = time.Now()
	task.UpdatedAt = task.EndTime
	if task.Background && !task.NotificationSent {
		m.notices = append(m.notices, Notice{
			TaskID:       task.ID,
			AgentType:    task.AgentType,
			Status:       task.Status,
			Description:  task.Description,
			Error:        task.Error,
			IsBackground: true,
			Kind:         "killed",
		})
		task.NotificationSent = true
	}
	return true
}

func (m *Manager) Get(id string) (*AgentTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, false
	}
	return cloneTask(task), true
}

func (m *Manager) List() []*AgentTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*AgentTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		out = append(out, cloneTask(task))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.Before(out[j].StartTime)
	})
	return out
}

func cloneTask(task *AgentTask) *AgentTask {
	copy := *task
	copy.Messages = append([]types.Message(nil), task.Messages...)
	return &copy
}

func (m *Manager) DrainNotices() []Notice {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]Notice(nil), m.notices...)
	m.notices = nil
	return out
}

func (m *Manager) PushNotice(n Notice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notices = append(m.notices, n)
}

func generateID(prefix string) (string, error) {
	return utils.GenerateID(prefix, 6)
}
