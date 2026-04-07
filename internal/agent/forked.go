// Package agent provides forked agent functionality for background tasks.
// This implements the pattern from src/utils/forkedAgent.ts for running
// background agents that share the parent's prompt cache.
package agent

import (
	"context"
	"strings"
	"sync"

	"claude-code-go/internal/types"
)

// ForkedAgentResult contains the result of a forked agent execution.
type ForkedAgentResult struct {
	Messages   []types.Message
	TotalUsage Usage
}

// Usage tracks token usage across API calls.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// ForkedAgentParams contains parameters for running a forked agent.
type ForkedAgentParams struct {
	// PromptMessages are the initial messages for the forked query.
	PromptMessages []types.Message
	// SystemPrompt is the system prompt (must match parent for cache hits).
	SystemPrompt string
	// UserContext is prepended to messages, affects cache.
	UserContext map[string]string
	// SystemContext is appended to system prompt, affects cache.
	SystemContext map[string]string
	// MaxOutputTokens is an optional cap on output tokens.
	MaxOutputTokens int
	// MaxTurns is an optional cap on number of turns.
	MaxTurns int
	// QuerySource is the source identifier for tracking.
	QuerySource string
	// ForkLabel is the label for analytics.
	ForkLabel string
	// CanUseTool is the permission check function.
	CanUseTool func(toolName string, input map[string]any) (allowed bool, message string)
}

// ForkedAgent runs background agents with isolated context.
type ForkedAgent struct {
	// QueryFunc is the function that performs the actual query.
	// This is injected to avoid circular dependencies.
	QueryFunc func(ctx context.Context, params ForkedAgentParams) (ForkedAgentResult, error)
}

// ForkedAgent creates a new forked agent runner.
func CreateForkedAgent(queryFunc func(ctx context.Context, params ForkedAgentParams) (ForkedAgentResult, error)) *ForkedAgent {
	return &ForkedAgent{QueryFunc: queryFunc}
}

// Run executes a forked agent query loop and tracks cache hit metrics.
func (a *ForkedAgent) Run(ctx context.Context, params ForkedAgentParams) (ForkedAgentResult, error) {
	if a.QueryFunc == nil {
		return ForkedAgentResult{}, nil
	}
	return a.QueryFunc(ctx, params)
}

// SubagentContext creates an isolated context for subagents.
// By default, ALL mutable state is isolated to prevent interference.
type SubagentContext struct {
	// ReadFileState tracks files that have been read.
	ReadFileState map[string]FileState
	// LoadedNestedMemoryPaths tracks loaded memory paths.
	LoadedNestedMemoryPaths map[string]bool
	// AgentID is the unique identifier for this subagent.
	AgentID string
	// AgentType is the type of subagent.
	AgentType string
	// Messages is the message history.
	Messages []types.Message
	// ParentAbortController is linked to parent for abort propagation.
	ParentAbortController context.Context
}

// FileState represents the state of a read file.
type FileState struct {
	Content   string
	Timestamp int64
}

// SubagentContextOverrides contains options for creating subagent context.
type SubagentContextOverrides struct {
	// Options overrides the options object.
	Options interface{}
	// AgentID overrides the agentId.
	AgentID string
	// AgentType overrides the agentType.
	AgentType string
	// Messages overrides the messages array.
	Messages []types.Message
	// ReadFileState overrides the readFileState.
	ReadFileState map[string]FileState
}

// CreateSubagentContext creates an isolated context for subagents.
func CreateSubagentContext(parentCtx context.Context, overrides SubagentContextOverrides) *SubagentContext {
	ctx := &SubagentContext{
		ReadFileState:           make(map[string]FileState),
		LoadedNestedMemoryPaths: make(map[string]bool),
		AgentID:                 overrides.AgentID,
		AgentType:               overrides.AgentType,
		Messages:                overrides.Messages,
		ParentAbortController:   parentCtx,
	}

	// Clone readFileState if provided
	if overrides.ReadFileState != nil {
		for k, v := range overrides.ReadFileState {
			ctx.ReadFileState[k] = v
		}
	}

	return ctx
}

// ForkedAgentManager manages multiple forked agents.
type ForkedAgentManager struct {
	mu     sync.Mutex
	agents map[string]*runningAgent
}

type runningAgent struct {
	ctx    context.CancelFunc
	result chan ForkedAgentResult
	err    chan error
}

// ForkedAgentManager creates a new forked agent manager.
func CreateForkedAgentManager() *ForkedAgentManager {
	return &ForkedAgentManager{
		agents: make(map[string]*runningAgent),
	}
}

// Spawn starts a forked agent in the background.
func (m *ForkedAgentManager) Spawn(ctx context.Context, id string, params ForkedAgentParams, runner *ForkedAgent) <-chan ForkedAgentResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	resultCh := make(chan ForkedAgentResult, 1)
	errCh := make(chan error, 1)

	agentCtx, cancel := context.WithCancel(ctx)
	m.agents[id] = &runningAgent{
		ctx:    cancel,
		result: resultCh,
		err:    errCh,
	}

	go func() {
		defer cancel()
		result, err := runner.Run(agentCtx, params)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	return resultCh
}

// Cancel cancels a running forked agent.
func (m *ForkedAgentManager) Cancel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, ok := m.agents[id]; ok {
		agent.ctx()
		delete(m.agents, id)
	}
}

// CancelAll cancels all running forked agents.
func (m *ForkedAgentManager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, agent := range m.agents {
		agent.ctx()
		delete(m.agents, id)
	}
}

// CanUseToolForMemoryExtraction creates a canUseTool function for memory extraction.
// This allows only Read, Grep, Glob, read-only Bash, and Edit/Write within the memory directory.
func CanUseToolForMemoryExtraction(memoryDir string) func(toolName string, input map[string]any) (bool, string) {
	return func(toolName string, input map[string]any) (bool, string) {
		switch toolName {
		case "Read", "Grep", "Glob":
			return true, ""

		case "Bash":
			// Check if command is read-only
			if cmd, ok := input["command"].(string); ok {
				if isReadOnlyBashCommand(cmd) {
					return true, ""
				}
			}
			return false, "Only read-only shell commands are permitted in this context"

		case "Edit", "Write":
			// Check if file path is within memory directory
			if filePath, ok := input["file_path"].(string); ok {
				if isPathWithinDir(filePath, memoryDir) {
					return true, ""
				}
			}
			return false, "Only edits within the memory directory are permitted"

		default:
			return false, "Tool not permitted in this context"
		}
	}
}

func isReadOnlyBashCommand(cmd string) bool {
	// Check for common read-only commands
	readOnlyPrefixes := []string{
		"ls", "cat", "head", "tail", "grep", "find", "stat", "wc",
		"git status", "git log", "git diff", "git branch",
		"echo", "pwd", "which", "type",
	}

	for _, prefix := range readOnlyPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

func isPathWithinDir(path, dir string) bool {
	// Simple check - in production would resolve symlinks and use filepath.Rel
	return strings.HasPrefix(path, dir)
}
