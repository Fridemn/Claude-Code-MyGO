package services

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Agent summary service for background agent progress tracking.
// Ported from src/services/AgentSummary/agentSummary.ts

const (
	// SummaryIntervalMs is the interval between summary generations
	SummaryIntervalMs = 30000 // 30 seconds
)

// AgentSummaryService generates periodic summaries for sub-agents.
type AgentSummaryService struct {
	mu       sync.Mutex
	running  map[string]*agentSummaryRunner
	provider SummaryProvider
}

// agentSummaryRunner tracks a running summary task
type agentSummaryRunner struct {
	taskID      string
	agentID     string
	stopCh      chan struct{}
	stopped     bool
	lastSummary string
}

// EmptyAgentSummaryService creates an empty agent summary service.
func EmptyAgentSummaryService() *AgentSummaryService {
	return &AgentSummaryService{
		running: make(map[string]*agentSummaryRunner),
	}
}

// CreateAgentSummaryService creates a new agent summary service with a provider.
func CreateAgentSummaryService(provider SummaryProvider) *AgentSummaryService {
	return &AgentSummaryService{
		running:  make(map[string]*agentSummaryRunner),
		provider: provider,
	}
}

// SetProvider sets the LLM provider for summary generation.
func (s *AgentSummaryService) SetProvider(provider SummaryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// StartAgentSummarization starts periodic summarization for an agent.
// Ported from src/services/AgentSummary/agentSummary.ts:startAgentSummarization
func (s *AgentSummaryService) StartAgentSummarization(taskID, agentID string) func() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already running
	if _, exists := s.running[taskID]; exists {
		return func() {}
	}

	runner := &agentSummaryRunner{
		taskID:  taskID,
		agentID: agentID,
		stopCh:  make(chan struct{}),
		stopped: false,
	}
	s.running[taskID] = runner

	// Start background ticker
	go s.runSummarizationLoop(runner)

	return func() {
		s.StopAgentSummarization(taskID)
	}
}

// StopAgentSummarization stops summarization for a task.
func (s *AgentSummaryService) StopAgentSummarization(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runner, exists := s.running[taskID]; exists {
		runner.stopped = true
		close(runner.stopCh)
		delete(s.running, taskID)
	}
}

// runSummarizationLoop runs the periodic summarization loop.
func (s *AgentSummaryService) runSummarizationLoop(runner *agentSummaryRunner) {
	ticker := time.NewTicker(SummaryIntervalMs)
	defer ticker.Stop()

	for {
		select {
		case <-runner.stopCh:
			return
		case <-ticker.C:
			if runner.stopped {
				return
			}
			summary := s.generateSummary(runner)
			if summary != "" {
				runner.lastSummary = summary
			}
		}
	}
}

// generateSummary generates a progress summary for an agent.
func (s *AgentSummaryService) generateSummary(runner *agentSummaryRunner) string {
	// Use LLM provider if available
	if s.provider != nil {
		ctx := context.Background()
		prompt := BuildSummaryPrompt(runner.lastSummary)
		summary, err := s.provider.GenerateSummary(ctx, prompt)
		if err == nil && summary != "" {
			return summary
		}
	}

	// Fallback: return a placeholder
	return "Processing..."
}

// BuildSummaryPrompt builds the summary prompt.
// Ported from src/services/AgentSummary/agentSummary.ts:buildSummaryPrompt
func BuildSummaryPrompt(previousSummary string) string {
	prevLine := ""
	if previousSummary != "" {
		prevLine = fmt.Sprintf("\nPrevious: \"%s\" — say something NEW.\n", previousSummary)
	}

	return fmt.Sprintf(`Describe your most recent action in 3-5 words using present tense (-ing). Name the file or function, not the branch. Do not use tools.
%s
Good: "Reading runAgent.ts"
Good: "Fixing null check in validate.ts"
Good: "Running auth module tests"
Good: "Adding retry logic to fetchUser"

Bad (past tense): "Analyzed the branch diff"
Bad (too vague): "Investigating the issue"
Bad (too long): "Reviewing full branch diff and AgentTool.tsx integration"
Bad (branch name): "Analyzed adam/background-summary branch diff"`, prevLine)
}

// GetLastSummary returns the last generated summary for a task.
func (s *AgentSummaryService) GetLastSummary(taskID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runner, exists := s.running[taskID]; exists {
		return runner.lastSummary
	}
	return ""
}

// SummarizeMessages generates a simple summary from messages (synchronous version).
func (s *AgentSummaryService) SummarizeMessages(ctx context.Context, messages []CompactMessage, fallback string) string {
	// Find the last assistant message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Type == MessageTypeAssistant && messages[i].Content != "" {
			content := messages[i].Content
			if len(content) > 100 {
				return content[:100] + "..."
			}
			return content
		}
	}
	return fallback
}
