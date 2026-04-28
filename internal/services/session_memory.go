package services

import (
	"context"
	"strings"
	"sync"

	"claude-go/internal/types"
)

// SessionMemoryConfig contains configuration for session memory extraction.
type SessionMemoryConfig struct {
	// MinimumMessageTokensToInit is the minimum token count to initialize.
	MinimumMessageTokensToInit int
	// MinimumTokensBetweenUpdate is the minimum tokens between updates.
	MinimumTokensBetweenUpdate int
	// ToolCallsBetweenUpdates is the minimum tool calls between updates.
	ToolCallsBetweenUpdates int
}

// DefaultSessionMemoryConfig returns default configuration.
func DefaultSessionMemoryConfig() SessionMemoryConfig {
	return SessionMemoryConfig{
		MinimumMessageTokensToInit:   10000,
		MinimumTokensBetweenUpdate:   5000,
		ToolCallsBetweenUpdates:      5,
	}
}

// SessionMemoryService maintains a markdown file with notes about the current conversation.
// It runs periodically in the background to extract key information.
type SessionMemoryService struct {
	config SessionMemoryConfig

	// State tracking
	initialized         bool
	lastExtractedUuid   string
	lastExtractionToken int
	mu                  sync.Mutex
}

// CreateSessionMemoryService creates a new session memory service.
func CreateSessionMemoryService() *SessionMemoryService {
	return &SessionMemoryService{
		config: DefaultSessionMemoryConfig(),
	}
}

// ShouldExtractMemory determines if session memory should be extracted.
func (s *SessionMemoryService) ShouldExtractMemory(messages []types.Message) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current token count
	currentTokenCount := estimateTokenCount(messages)

	// Check initialization threshold
	if !s.initialized {
		if currentTokenCount < s.config.MinimumMessageTokensToInit {
			return false
		}
		s.initialized = true
	}

	// Check update thresholds
	tokenGrowth := currentTokenCount - s.lastExtractionToken
	hasMetTokenThreshold := tokenGrowth >= s.config.MinimumTokensBetweenUpdate

	// Count tool calls since last extraction
	toolCallsSinceLast := countToolCallsSince(messages, s.lastExtractedUuid)
	hasMetToolCallThreshold := toolCallsSinceLast >= s.config.ToolCallsBetweenUpdates

	// Check if last assistant turn has tool calls
	hasToolCallsInLastTurn := hasToolCallsInLastAssistantTurn(messages)

	// Trigger extraction when:
	// 1. Both thresholds are met (tokens AND tool calls), OR
	// 2. No tool calls in last turn AND token threshold is met
	shouldExtract := (hasMetTokenThreshold && hasMetToolCallThreshold) ||
		(hasMetTokenThreshold && !hasToolCallsInLastTurn)

	if shouldExtract {
		// Update last extraction token count
		s.lastExtractionToken = currentTokenCount
	}

	return shouldExtract
}

// Extract extracts session memory from messages.
// This is a simplified implementation - the full version uses a forked agent.
func (s *SessionMemoryService) Extract(ctx context.Context, messages []types.Message) (string, error) {
	// Build extraction prompt
	prompt := s.buildExtractionPrompt(messages)

	// In a full implementation, this would spawn a forked agent
	// to extract structured notes from the conversation.

	// For now, return a placeholder
	return prompt, nil
}

// Snapshot returns a snapshot of the current session memory state.
func (s *SessionMemoryService) Snapshot(messages []types.Message) []types.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]types.Message(nil), messages...)
}

// buildExtractionPrompt builds the prompt for memory extraction.
func (s *SessionMemoryService) buildExtractionPrompt(messages []types.Message) string {
	// Session memory template structure
	var sections []string

	sections = append(sections, "# Session Notes")
	sections = append(sections, "")
	sections = append(sections, "## Session Title")
	sections = append(sections, "")
	sections = append(sections, "## Current State")
	sections = append(sections, "")
	sections = append(sections, "## Task Specification")
	sections = append(sections, "")
	sections = append(sections, "## Files and Functions")
	sections = append(sections, "")
	sections = append(sections, "## Workflow")
	sections = append(sections, "")
	sections = append(sections, "## Errors & Corrections")
	sections = append(sections, "")
	sections = append(sections, "## Learnings")
	sections = append(sections, "")
	sections = append(sections, "## Key Results")
	sections = append(sections, "")
	sections = append(sections, "## Worklog")

	return joinSections(sections)
}

func joinSections(sections []string) string {
	return strings.Join(sections, "\n")
}

// Helper functions

func estimateTokenCount(messages []types.Message) int {
	// Rough estimation: ~4 chars per token
	totalChars := 0
	for _, msg := range messages {
		// Count characters in message content
		totalChars += len(msg.Content)
		// Also count tool call arguments
		for _, tc := range msg.ToolCalls {
			totalChars += len(tc.Arguments)
		}
	}
	return totalChars / 4
}

func countToolCallsSince(messages []types.Message, sinceUuid string) int {
	if sinceUuid == "" {
		return 0
	}

	count := 0
	found := false

	for _, msg := range messages {
		if !found {
			if msg.UUID == sinceUuid {
				found = true
			}
			continue
		}

		// Count tool uses in assistant messages
		if msg.Role == "assistant" {
			count += len(msg.ToolCalls)
		}
	}

	return count
}

func hasToolCallsInLastAssistantTurn(messages []types.Message) bool {
	// Find the last assistant message and check for tool calls
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].HasToolCalls()
		}
	}
	return false
}

// GetSessionMemoryPath returns the path to the session memory file.
func GetSessionMemoryPath(sessionDir string) string {
	return sessionDir + "/session-notes.md"
}

// GetSessionMemoryDir returns the directory for session memory.
func GetSessionMemoryDir(sessionDir string) string {
	return sessionDir + "/.session"
}

// SessionMemoryPrompts contains prompt templates for session memory.
type SessionMemoryPrompts struct{}

// GetUpdatePrompt returns the prompt for updating session memory.
func (p *SessionMemoryPrompts) GetUpdatePrompt(currentMemory, memoryPath string) string {
	return `Update the session memory file with the latest conversation context.

Current memory content:
` + currentMemory + `

Memory file path: ` + memoryPath + `

Instructions:
- Update the session notes to reflect the current state of the conversation
- Add new files, functions, and key learnings
- Update the workflow and worklog
- Keep the notes concise and focused on what's relevant for continuing the session`
}

// GetTemplate returns the default session memory template.
func (p *SessionMemoryPrompts) GetTemplate() string {
	return `# Session Notes

## Session Title
<!-- Brief description of the session's main focus -->

## Current State
<!-- What are we currently working on? What's the next step? -->

## Task Specification
<!-- What did the user ask us to do? -->

## Files and Functions
<!-- Key files and functions involved in this session -->

## Workflow
<!-- What approach are we taking? -->

## Errors & Corrections
<!-- Any errors encountered and how they were resolved -->

## Codebase and System Documentation
<!-- Notes about the codebase or system being worked on -->

## Learnings
<!-- What did we learn during this session? -->

## Key Results
<!-- What were the main outcomes? -->

## Worklog
<!-- Chronological list of what was done -->
`
}