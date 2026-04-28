package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/session"
	"claude-go/internal/task"
	"claude-go/internal/tool"
	"claude-go/internal/types"
)

// Constants for retry behavior
const (
	// maxAgentEmptyResponseRetries is the maximum number of retries when
	// the engine returns "empty assistant response after tool execution" error.
	// This allows the agent to recover from transient proxy/provider issues.
	maxAgentEmptyResponseRetries = 3

	// recoveryHintDelayMs is a delay between retries to allow the model/provider
	// to stabilize.
	recoveryHintDelayMs = 500
)

type SpawnInput struct {
	Description  string
	Prompt       string
	SubagentType string
	Model        string
	Background   bool
	TaskID       string
}

type Result struct {
	Task  *task.AgentTask `json:"task"`
	Agent Definition      `json:"agent"`
}

type Manager struct {
	cfg      config.Config
	provider engine.Provider
	tools    *tool.Registry
	mcp      tool.MCPRuntime
	hooks    engine.HookRunner
	sessions *session.Manager
	registry *Registry
	tasks    *task.Manager
	wg       sync.WaitGroup
	cancels  map[string]context.CancelFunc
	mu       sync.Mutex
}

func CreateManager(cfg config.Config, provider engine.Provider, tools *tool.Registry, sessions *session.Manager, hooks engine.HookRunner, mcp tool.MCPRuntime) *Manager {
	return &Manager{
		cfg:      cfg,
		provider: provider,
		tools:    tools,
		mcp:      mcp,
		hooks:    hooks,
		sessions: sessions,
		registry: EmptyRegistry(),
		tasks:    task.EmptyManager(),
		cancels:  map[string]context.CancelFunc{},
	}
}

func (m *Manager) Registry() *Registry { return m.registry }

func (m *Manager) Tasks() *task.Manager { return m.tasks }

func (m *Manager) ReloadDefinitions(defs []Definition) {
	m.registry.Reset()
	for _, def := range defs {
		m.registry.Register(def)
	}
}

func (m *Manager) Spawn(ctx context.Context, input SpawnInput) (*Result, error) {
	agentType := input.SubagentType
	if agentType == "" {
		agentType = "general-purpose"
	}
	definition, err := m.registry.Get(agentType)
	if err != nil {
		return nil, err
	}

	model := input.Model
	if model == "" || model == "inherit" {
		if definition.Model != "" && definition.Model != "inherit" {
			model = definition.Model
		} else {
			model = m.cfg.Model
		}
	}

	background := input.Background || definition.Background
	var taskState *task.AgentTask
	if input.TaskID != "" {
		existing, ok := m.tasks.Get(input.TaskID)
		if !ok {
			return nil, fmt.Errorf("task not found: %s", input.TaskID)
		}
		taskState = existing
		m.tasks.SetInput(taskState.ID, input.Prompt)
	} else {
		taskState, err = m.tasks.CreateAgentTask(input.Description, definition.AgentType, "", model, input.Prompt, background)
		if err != nil {
			return nil, err
		}
		if background {
			m.tasks.PushNotice(task.Notice{
				TaskID:       taskState.ID,
				AgentType:    definition.AgentType,
				Status:       task.StatusPending,
				Description:  taskState.Description,
				Output:       "background agent launched",
				IsBackground: true,
				Kind:         "launched",
			})
		}
	}

	if input.TaskID != "" && background {
		m.tasks.PushNotice(task.Notice{
			TaskID:       taskState.ID,
			AgentType:    definition.AgentType,
			Status:       task.StatusPending,
			Description:  taskState.Description,
			Output:       "background continuation launched",
			IsBackground: true,
			Kind:         "continued",
		})
	}

	run := func() {
		defer m.wg.Done()
		runCtx, cancel := context.WithCancel(ctx)
		m.setCancel(taskState.ID, cancel)
		defer m.clearCancel(taskState.ID)
		defer cancel()
		m.tasks.SetRunning(taskState.ID)
		output, messages, sessionID, runErr := m.runAgent(runCtx, definition, model, input.Prompt, taskState.SessionID, taskState.Messages)
		m.tasks.SetSessionID(taskState.ID, sessionID)
		summary := summarizeAgentMessages(messages, output)
		if runErr != nil {
			if errors.Is(runErr, context.Canceled) {
				m.tasks.Kill(taskState.ID, "agent stopped by user")
				return
			}
			m.tasks.Fail(taskState.ID, runErr, summary, messages)
			return
		}
		m.tasks.Complete(taskState.ID, output, summary, messages)
	}

	m.wg.Add(1)
	if background {
		go run()
	} else {
		run()
	}

	finalTask, _ := m.tasks.Get(taskState.ID)
	return &Result{Task: finalTask, Agent: definition}, nil
}

func (m *Manager) runAgent(ctx context.Context, definition Definition, model, prompt, sessionID string, initialMessages []types.Message) (string, []types.Message, string, error) {
	eng, err := engine.Create(ctx, engine.Options{
		Config: config.Config{
			APIKey:       m.cfg.APIKey,
			BaseURL:      m.cfg.BaseURL,
			Model:        model,
			AppName:      m.cfg.AppName,
			MaxTurns:     maxTurns(definition, m.cfg.MaxTurns),
			SessionDir:   m.cfg.SessionDir,
			SystemPrompt: composeSystemPrompt(definition, m.cfg.SystemPrompt),
		},
		Provider: m.provider,
		Tools:    m.tools,
		Hooks:    m.hooks,
		ToolRuntime: tool.Runtime{
			Tasks: m.Tasks(),
			Stop:  m.Stop,
			MCP:   m.mcp,
			SpawnAgent: func(ctx context.Context, req tool.AgentSpawnRequest) (*task.AgentTask, error) {
				result, err := m.Spawn(ctx, SpawnInput{
					Description:  req.Description,
					Prompt:       req.Prompt,
					SubagentType: req.Type,
					Background:   req.Background,
				})
				if err != nil {
					return nil, err
				}
				return result.Task, nil
			},
			ContinueAgent: func(ctx context.Context, taskID, prompt string, background bool) (*task.AgentTask, error) {
				result, err := m.Continue(ctx, taskID, prompt, background)
				if err != nil {
					return nil, err
				}
				return result.Task, nil
			},
		},
		Sessions:        m.sessions,
		InitialMessages: initialMessages,
		SessionID:       sessionID,
	})
	if err != nil {
		return "", nil, "", err
	}

	input := prompt
	if definition.InitialPrompt != "" {
		input = definition.InitialPrompt + "\n\n" + prompt
	}

	// Retry loop for recoverable errors like "empty assistant response after tool execution"
	var lastErr error
	var retryCount int
	
	for retryCount = 0; retryCount <= maxAgentEmptyResponseRetries; retryCount++ {
		response, err := eng.Submit(ctx, input)
		if err == nil {
			if definition.ReadOnly {
				return response.Text, eng.Messages(), eng.SessionID(), nil
			}
			if m.tools == nil {
				return response.Text, eng.Messages(), eng.SessionID(), nil
			}
			return response.Text, eng.Messages(), eng.SessionID(), nil
		}

		// Check if this is a retryable error
		if !isRetryableEmptyResponseError(err) {
			// Non-retryable error, return immediately
			return "", eng.Messages(), eng.SessionID(), err
		}

		// Record the error for potential final return
		lastErr = err

		// If we have more retries available, prepare recovery input
		if retryCount < maxAgentEmptyResponseRetries {
			// Add a small delay to allow the provider to stabilize
			time.Sleep(recoveryHintDelayMs * time.Millisecond)
			
			// Inject a recovery hint to help the model continue
			input = prompt + fmt.Sprintf("\n\n[System: The previous attempt returned an empty response (attempt %d of %d). Please continue with the task.]", retryCount+1, maxAgentEmptyResponseRetries+1)
		}
	}

	// All retries exhausted, return the last error with context
	if lastErr != nil {
		return "", eng.Messages(), eng.SessionID(), fmt.Errorf("agent failed after %d retries: %w", maxAgentEmptyResponseRetries+1, lastErr)
	}
	return "", eng.Messages(), eng.SessionID(), nil
}

// isRetryableEmptyResponseError checks if the error is the "empty assistant response"
// error that can be recovered by retrying with a recovery hint.
func isRetryableEmptyResponseError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "empty assistant response") ||
		strings.Contains(errStr, "model returned empty assistant response")
}

func composeSystemPrompt(definition Definition, base string) string {
	if base == "" {
		return definition.SystemPrompt
	}
	return definition.SystemPrompt + "\n\n---\n\nParent session baseline:\n" + base
}

func maxTurns(definition Definition, fallback int) int {
	if definition.MaxTurns > 0 {
		return definition.MaxTurns
	}
	return fallback
}

func (m *Manager) WaitForTask(id string) (*task.AgentTask, error) {
	taskState, ok := m.tasks.Get(id)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	for taskState.Status == task.StatusPending || taskState.Status == task.StatusRunning {
		time.Sleep(100 * time.Millisecond)
		taskState, _ = m.tasks.Get(id)
	}
	return taskState, nil
}

func BuildAgentPromptMessages(definition Definition, prompt string) []types.Message {
	return []types.Message{
		{Role: types.RoleSystem, Content: definition.SystemPrompt},
		{Role: types.RoleUser, Content: prompt},
	}
}

func (m *Manager) Continue(ctx context.Context, taskID, prompt string, background bool) (*Result, error) {
	existing, ok := m.tasks.Get(taskID)
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return m.Spawn(ctx, SpawnInput{
		TaskID:       taskID,
		Description:  existing.Description,
		Prompt:       prompt,
		SubagentType: existing.AgentType,
		Model:        existing.Model,
		Background:   background,
	})
}

func (m *Manager) Stop(taskID string) error {
	m.mu.Lock()
	cancel, ok := m.cancels[taskID]
	m.mu.Unlock()
	if !ok {
		if _, exists := m.tasks.Get(taskID); !exists {
			return fmt.Errorf("task not found: %s", taskID)
		}
		m.tasks.Kill(taskID, "agent stopped by user")
		return nil
	}
	cancel()
	return nil
}

func (m *Manager) setCancel(taskID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancels[taskID] = cancel
}

func (m *Manager) clearCancel(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, taskID)
}

func summarizeAgentMessages(messages []types.Message, fallback string) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != types.RoleAssistant {
			continue
		}
		if trimmed := trimSummary(msg.Content); trimmed != "" {
			return trimmed
		}
	}
	return trimSummary(fallback)
}

func trimSummary(v string) string {
	const max = 280
	v = string([]rune(v))
	v = strings.TrimSpace(v)
	if len([]rune(v)) <= max {
		return v
	}
	return string([]rune(v)[:max]) + "..."
}
