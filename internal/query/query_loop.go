package query

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"claude-go/internal/provider"
	"claude-go/internal/services"
)

// QueryLoop is the main query processing loop
type QueryLoop struct {
	mu           sync.RWMutex
	provider     provider.Provider
	messages     []provider.Message
	maxTurns     int
	currentTurn  int
	tools        []Tool
	sessionID    string
}

// Tool represents a callable tool
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
}

// QueryConfig contains configuration for the query loop
type QueryConfig struct {
	MaxTurns  int
	SessionID string
	Tools     []Tool
}

// NewQueryLoop creates a new query loop instance
func NewQueryLoop(cfg QueryConfig) *QueryLoop {
	return &QueryLoop{
		messages:    make([]provider.Message, 0),
		maxTurns:    cfg.MaxTurns,
		tools:       cfg.Tools,
		sessionID:   cfg.SessionID,
	}
}

// SetProvider sets the LLM provider
func (q *QueryLoop) SetProvider(p provider.Provider) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.provider = p
}

// GetProvider returns the current LLM provider
func (q *QueryLoop) GetProvider() provider.Provider {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.provider
}

// AddSystemMessage adds a system message to the conversation
func (q *QueryLoop) AddSystemMessage(content string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, provider.Message{
		Role:    "system",
		Content: content,
	})
}

// AddUserMessage adds a user message to the conversation
func (q *QueryLoop) AddUserMessage(content string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, provider.Message{
		Role:    "user",
		Content: content,
	})
}

// Query processes a user query and returns the response
func (q *QueryLoop) Query(ctx context.Context, userInput string) (*QueryResult, error) {
	q.mu.Lock()
	q.currentTurn++
	turn := q.currentTurn
	q.mu.Unlock()

	// Check max turns
	if q.maxTurns > 0 && turn > q.maxTurns {
		return &QueryResult{
			Response: "Maximum turns reached.",
			StopReason: "max_turns",
		}, nil
	}

	// Add user message
	q.AddUserMessage(userInput)

	// Get messages to send
	q.mu.RLock()
	messages := make([]provider.Message, len(q.messages))
	copy(messages, q.messages)
	q.mu.RUnlock()

	// Check if provider is set
	q.mu.RLock()
	p := q.provider
	q.mu.RUnlock()

	if p == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	// Call LLM
	resp, err := p.Complete(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM error: %w", err)
	}

	// Add assistant response
	q.AddAssistantMessage(resp.Content)

	// Process stop reason
	result := &QueryResult{
		Response:   resp.Content,
		StopReason: resp.StopReason,
		Usage:      resp.Usage,
		Turn:       turn,
	}

	// Handle different stop reasons
	switch resp.StopReason {
	case "tool_use":
		// Parse tool calls from response (simplified)
		toolCalls := q.parseToolCalls(resp.Content)
		if len(toolCalls) > 0 {
			result.ToolCalls = toolCalls
		}
	case "max_tokens":
		result.StopReason = "length"
	}

	return result, nil
}

// QueryStream processes a query with streaming response
func (q *QueryLoop) QueryStream(ctx context.Context, userInput string, handler func(string) error) (*QueryResult, error) {
	q.mu.Lock()
	q.currentTurn++
	turn := q.currentTurn
	q.mu.Unlock()

	// Check max turns
	if q.maxTurns > 0 && turn > q.maxTurns {
		return &QueryResult{
			Response: "Maximum turns reached.",
			StopReason: "max_turns",
		}, nil
	}

	// Add user message
	q.AddUserMessage(userInput)

	// Get messages to send
	q.mu.RLock()
	messages := make([]provider.Message, len(q.messages))
	copy(messages, q.messages)
	q.mu.RUnlock()

	// Check if provider is set
	q.mu.RLock()
	p := q.provider
	q.mu.RUnlock()

	if p == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	// Stream from LLM
	stream, err := p.CompleteStream(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM stream error: %w", err)
	}

	// Collect streaming response
	var fullResponse strings.Builder
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case chunk, ok := <-stream:
			if !ok {
				goto done
			}
			if chunk.Content != "" {
				fullResponse.WriteString(chunk.Content)
				if handler != nil {
					if err := handler(chunk.Content); err != nil {
						return nil, err
					}
				}
			}
			if chunk.Done {
				goto done
			}
		}
	}

done:
	response := fullResponse.String()

	// Add assistant response
	q.AddAssistantMessage(response)

	result := &QueryResult{
		Response:   response,
		StopReason: "end_turn",
		Turn:       turn,
	}

	return result, nil
}

// AddAssistantMessage adds an assistant message to the conversation
func (q *QueryLoop) AddAssistantMessage(content string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = append(q.messages, provider.Message{
		Role:    "assistant",
		Content: content,
	})
}

// GetMessages returns all messages in the conversation
func (q *QueryLoop) GetMessages() []provider.Message {
	q.mu.RLock()
	defer q.mu.RUnlock()
	messages := make([]provider.Message, len(q.messages))
	copy(messages, q.messages)
	return messages
}

// GetMessageCount returns the number of messages
func (q *QueryLoop) GetMessageCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

// GetTurnCount returns the current turn count
func (q *QueryLoop) GetTurnCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.currentTurn
}

// Reset clears the conversation history
func (q *QueryLoop) Reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages = make([]provider.Message, 0)
	q.currentTurn = 0
}

// ResetExceptSystem clears all messages except system messages
func (q *QueryLoop) ResetExceptSystem() {
	q.mu.Lock()
	defer q.mu.Unlock()
	var systemMsgs []provider.Message
	for _, msg := range q.messages {
		if msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
		}
	}
	q.messages = systemMsgs
	q.currentTurn = 0
}

// SessionID returns the session ID
func (q *QueryLoop) SessionID() string {
	return q.sessionID
}

// parseToolCalls extracts tool calls from the response (simplified)
func (q *QueryLoop) parseToolCalls(response string) []ToolCall {
	// This is a simplified parser - full implementation would use proper parsing
	var calls []ToolCall

	// Look for tool_use blocks
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Tool:") {
			name := strings.TrimPrefix(line, "Tool:")
			name = strings.TrimSpace(name)
			calls = append(calls, ToolCall{
				Name:      name,
				Arguments: make(map[string]interface{}),
			})
		}
	}

	return calls
}

// QueryResult contains the result of a query
type QueryResult struct {
	Response   string          `json:"response"`
	StopReason string          `json:"stop_reason"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	Usage      provider.Usage `json:"usage,omitempty"`
	Turn       int            `json:"turn"`
	Error      string         `json:"error,omitempty"`
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// ToolExecutor is a function that executes a tool
type ToolExecutor func(ctx context.Context, tool ToolCall) (*ToolResult, error)

// ExecuteTool executes a tool and returns the result
func (q *QueryLoop) ExecuteTool(ctx context.Context, executor ToolExecutor, call ToolCall) (*ToolResult, error) {
	if executor == nil {
		return nil, fmt.Errorf("no tool executor configured")
	}
	return executor(ctx, call)
}

// CompactTriggerReason represents why compaction was triggered
type CompactTriggerReason string

const (
	CompactTriggerAuto    CompactTriggerReason = "auto"
	CompactTriggerManual  CompactTriggerReason = "manual"
	CompactTriggerPTL     CompactTriggerReason = "prompt_too_long"
)

// ShouldCompact checks if compaction should be triggered
func (q *QueryLoop) ShouldCompact(tokenCount int, threshold int) bool {
	return tokenCount > threshold
}

// RunCompact triggers compaction on the conversation
func (q *QueryLoop) RunCompact(ctx context.Context, compactService *services.CompactService, customInstructions string) (*services.CompactionResult, error) {
	q.mu.RLock()
	messages := q.copyMessages()
	q.mu.RUnlock()

	// Convert to CompactMessage format
	compactMsgs := convertToCompactMessages(messages)

	// Run compaction
	result, err := compactService.Compact(ctx, compactMsgs, customInstructions, true)
	if err != nil {
		return nil, err
	}

	// Update messages with compacted version
	q.mu.Lock()
	q.messages = convertFromCompactionResult(result, q.messages)
	q.mu.Unlock()

	return result, nil
}

// copyMessages creates a copy of messages
func (q *QueryLoop) copyMessages() []provider.Message {
	messages := make([]provider.Message, len(q.messages))
	copy(messages, q.messages)
	return messages
}

// convertToCompactMessages converts provider.Message to services.CompactMessage
func convertToCompactMessages(messages []provider.Message) []services.CompactMessage {
	var result []services.CompactMessage
	for _, msg := range messages {
		compactMsg := services.CompactMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
		switch msg.Role {
		case "user":
			compactMsg.Type = services.MessageTypeUser
		case "assistant":
			compactMsg.Type = services.MessageTypeAssistant
		case "system":
			compactMsg.Type = services.MessageTypeSystem
		}
		result = append(result, compactMsg)
	}
	return result
}

// convertFromCompactionResult creates provider.Message from compaction result
func convertFromCompactionResult(result *services.CompactionResult, currentMessages []provider.Message) []provider.Message {
	// Keep system messages
	var messages []provider.Message
	for _, msg := range currentMessages {
		if msg.Role == "system" {
			messages = append(messages, msg)
		}
	}

	// Add summary as a system message
	if len(result.SummaryMessages) > 0 {
		var summary strings.Builder
		for _, msg := range result.SummaryMessages {
			summary.WriteString(msg.Content)
			summary.WriteString("\n")
		}
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: "[Summary of earlier conversation]\n" + summary.String(),
		})
	}

	return messages
}

// EstimateTokenCount estimates total tokens in messages
func (q *QueryLoop) EstimateTokenCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	total := 0
	for _, msg := range q.messages {
		total += countTokens(msg.Content)
	}
	return total
}

// countTokens estimates token count
func countTokens(s string) int {
	return (len(s) + 3) / 4
}

// History returns conversation history for display
func (q *QueryLoop) History() []HistoryEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var history []HistoryEntry
	for i, msg := range q.messages {
		entry := HistoryEntry{
			Turn:    i + 1,
			Role:    msg.Role,
			Content: msg.Content,
			Time:    time.Now(), // Simplified - would track actual time
		}
		history = append(history, entry)
	}

	return history
}

// HistoryEntry represents a single message in history
type HistoryEntry struct {
	Turn    int       `json:"turn"`
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}
