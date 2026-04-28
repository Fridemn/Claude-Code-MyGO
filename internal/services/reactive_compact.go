package services

import (
	"fmt"
	"strings"
	"sync"
)

// Reactive Compact configuration and state
// Ported from src/services/compact/reactiveCompact.ts

// ReactiveCompactConfig for reactive compact (uses TimeBasedMCConfig from micro_compact.go)
type ReactiveCompactConfig struct {
	Enabled             bool
	GapThresholdMinutes int
	KeepRecent          int
}

// DefaultReactiveCompactConfig returns default reactive compact configuration
func DefaultReactiveCompactConfig() ReactiveCompactConfig {
	return ReactiveCompactConfig{
		Enabled:             false,
		GapThresholdMinutes: 60,
		KeepRecent:          5,
	}
}

// ReactiveCompactService handles reactive compaction on prompt-too-long errors
// Ported from src/services/compact/reactiveCompact.ts
type ReactiveCompactService struct {
	mu                sync.RWMutex
	enabled           bool
	reactiveOnlyMode  bool
	lastAssistantTime int64
	hasAttempted      bool
	compactThreshold  int
	config            ReactiveCompactConfig
}

// Global reactive compact service instance
var reactiveCompactService = &ReactiveCompactService{
	enabled:          false,
	reactiveOnlyMode: false,
	compactThreshold: 150000, // Default threshold
	config:           DefaultReactiveCompactConfig(),
}

// NewReactiveCompactService creates a new reactive compact service
func NewReactiveCompactService() *ReactiveCompactService {
	return &ReactiveCompactService{
		enabled:          false,
		reactiveOnlyMode: false,
		compactThreshold: 150000,
		config:           DefaultReactiveCompactConfig(),
	}
}

// IsReactiveCompactEnabled returns whether reactive compact is enabled
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) IsReactiveCompactEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// SetEnabled sets whether reactive compact is enabled
func (s *ReactiveCompactService) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsReactiveOnlyMode returns whether we're in reactive-only mode
// In this mode, proactive auto-compact is suppressed
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) IsReactiveOnlyMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reactiveOnlyMode
}

// SetReactiveOnlyMode sets reactive-only mode
func (s *ReactiveCompactService) SetReactiveOnlyMode(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reactiveOnlyMode = enabled
}

// SetCompactThreshold sets the compact threshold
func (s *ReactiveCompactService) SetCompactThreshold(threshold int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactThreshold = threshold
}

// GetCompactThreshold returns the compact threshold
func (s *ReactiveCompactService) GetCompactThreshold() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compactThreshold
}

// SetReactiveCompactConfig sets the reactive compact configuration
func (s *ReactiveCompactService) SetReactiveCompactConfig(config ReactiveCompactConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = config
}

// GetReactiveCompactConfig returns the reactive compact configuration
func (s *ReactiveCompactService) GetReactiveCompactConfig() ReactiveCompactConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateLastAssistantTime updates the last assistant message timestamp
func (s *ReactiveCompactService) UpdateLastAssistantTime(timestamp int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAssistantTime = timestamp
}

// GetLastAssistantTime returns the last assistant message timestamp
func (s *ReactiveCompactService) GetLastAssistantTime() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastAssistantTime
}

// SetHasAttempted sets whether reactive compact has been attempted
func (s *ReactiveCompactService) SetHasAttempted(attempted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasAttempted = attempted
}

// GetHasAttempted returns whether reactive compact has been attempted
func (s *ReactiveCompactService) GetHasAttempted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasAttempted
}

// IsWithheldPromptTooLong checks if a message is a withheld prompt-too-long error
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) IsWithheldPromptTooLong(msg *CompactMessage) bool {
	if msg == nil {
		return false
	}
	// A withheld PTL error message has isApiErrorMessage set and contains PTL text
	// This is checked via the isApiErrorMessage flag and error message detection
	return IsPromptTooLongMessage(msg)
}

// IsWithheldMediaSizeError checks if a message is a withheld media size error
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) IsWithheldMediaSizeError(msg *CompactMessage) bool {
	if msg == nil {
		return false
	}
	return IsMediaSizeErrorMessage(msg)
}

// ReactiveCompactOutcome represents the outcome of a reactive compact attempt
type ReactiveCompactOutcome struct {
	OK     bool
	Reason string // "too_few_groups", "aborted", "exhausted", "error", "media_unstrippable", ""
	Result *CompactionResult
}

// TryReactiveCompact attempts reactive compaction
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) TryReactiveCompact(params *TryReactiveCompactParams) *ReactiveCompactOutcome {
	if !s.IsReactiveCompactEnabled() {
		return &ReactiveCompactOutcome{OK: false, Reason: "disabled"}
	}

	if params.HasAttempted {
		// Already attempted - don't retry
		return &ReactiveCompactOutcome{OK: false, Reason: "exhausted"}
	}

	if params.Aborted {
		return &ReactiveCompactOutcome{OK: false, Reason: "aborted"}
	}

	// Perform compact on the messages
	// This is a simplified implementation - full version would integrate with LLM
	result, err := s.PerformReactiveCompact(params.Messages, params.CustomInstructions)
	if err != nil {
		return &ReactiveCompactOutcome{OK: false, Reason: "error"}
	}

	if result == nil {
		return &ReactiveCompactOutcome{OK: false, Reason: "too_few_groups"}
	}

	s.SetHasAttempted(true)
	return &ReactiveCompactOutcome{OK: true, Result: result}
}

// TryReactiveCompactParams contains parameters for reactive compact attempt
type TryReactiveCompactParams struct {
	HasAttempted       bool
	Messages           []CompactMessage
	CustomInstructions string
	Aborted            bool
}

// PerformReactiveCompact performs the actual reactive compaction
func (s *ReactiveCompactService) PerformReactiveCompact(messages []CompactMessage, customInstructions string) (*CompactionResult, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	// Check if there are enough groups to compact
	groups := GroupMessagesByApiRound(messages)
	if len(groups) < 2 {
		return nil, nil
	}

	// Find last compact boundary to determine what to summarize
	startIndex := FindLastCompactBoundaryIndex(messages)
	if startIndex < 0 {
		startIndex = 0
	}

	// Calculate messages to summarize
	messagesToSummarize := messages[startIndex:]
	if len(messagesToSummarize) == 0 {
		return nil, nil
	}

	// Create compact service
	compactService := EmptyCompactService()

	// Perform compaction
	result, err := compactService.Compact(nil, messagesToSummarize, customInstructions, false)
	if err != nil {
		return nil, err
	}

	// Return a result that includes the post-compact messages
	return &CompactionResult{
		BoundaryMarker:            result.BoundaryMarker,
		SummaryMessages:           result.SummaryMessages,
		MessagesToKeep:            messages,
		PreCompactTokenCount:      result.PreCompactTokenCount,
		PostCompactTokenCount:     result.PostCompactTokenCount,
		TruePostCompactTokenCount: result.TruePostCompactTokenCount,
	}, nil
}

// ReactiveCompactOnPromptTooLong performs reactive compact specifically for PTL errors
// Ported from reactiveCompact.ts
func (s *ReactiveCompactService) ReactiveCompactOnPromptTooLong(
	messages []CompactMessage,
	params *ReactiveCompactParams,
) *ReactiveCompactOutcome {
	if params.HasAttempted {
		return &ReactiveCompactOutcome{OK: false, Reason: "exhausted"}
	}

	if params.Aborted {
		return &ReactiveCompactOutcome{OK: false, Reason: "aborted"}
	}

	// Find the PTL error message and extract token gap
	var tokenGap int
	for _, msg := range messages {
		if IsPromptTooLongMessage(&msg) && msg.Content != "" {
			tokenGap = GetPromptTooLongTokenGap(msg.Content)
			break
		}
	}

	// Perform reactive compact with token gap awareness
	result := s.tryCompactWithTokenGap(messages, params.CustomInstructions, tokenGap)
	if result == nil {
		return &ReactiveCompactOutcome{OK: false, Reason: "too_few_groups"}
	}

	s.SetHasAttempted(true)
	return &ReactiveCompactOutcome{OK: true, Result: result}
}

// ReactiveCompactParams contains parameters for PTL reactive compact
type ReactiveCompactParams struct {
	CustomInstructions string
	Trigger            string // "manual" or "auto"
	HasAttempted       bool
	Aborted            bool
}

// tryCompactWithTokenGap attempts compact with awareness of PTL token gap
func (s *ReactiveCompactService) tryCompactWithTokenGap(messages []CompactMessage, customInstructions string, tokenGap int) *CompactionResult {
	groups := GroupMessagesByApiRound(messages)

	if len(groups) < 2 {
		return nil
	}

	// If we have a token gap, try to calculate how many groups to drop
	dropCount := 0
	if tokenGap > 0 {
		acc := 0
		for i, group := range groups {
			acc += EstimateMessagesTokenCount(group)
			if acc >= tokenGap {
				dropCount = i
				break
			}
		}
	}

	// Ensure we keep at least one group
	if dropCount >= len(groups)-1 {
		dropCount = len(groups) - 2
	}
	if dropCount < 1 {
		dropCount = 1
	}

	// Drop oldest groups and compact
	messagesToSummarize := flattenGroups(groups[dropCount:])
	if len(messagesToSummarize) == 0 {
		return nil
	}

	compactService := EmptyCompactService()
	result, err := compactService.Compact(nil, messagesToSummarize, customInstructions, false)
	if err != nil {
		return nil
	}

	return result
}

// ResetReactiveCompactState resets the reactive compact state
// Useful for testing or when starting a new session
func (s *ReactiveCompactService) ResetReactiveCompactState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasAttempted = false
	s.lastAssistantTime = 0
}

// GetGlobalReactiveCompactService returns the global reactive compact service instance
func GetGlobalReactiveCompactService() *ReactiveCompactService {
	return reactiveCompactService
}

// IsPromptTooLongMessage checks if a message is a prompt-too-long error
func IsPromptTooLongMessage(msg *CompactMessage) bool {
	if msg == nil {
		return false
	}
	content := strings.ToLower(msg.Content)
	return strings.Contains(content, "prompt is too long") ||
		strings.Contains(content, "prompt exceeds max length") ||
		strings.Contains(content, "input characters limit") ||
		strings.Contains(content, "input character limit")
}

// IsMediaSizeErrorMessage checks if a message is a media size error
func IsMediaSizeErrorMessage(msg *CompactMessage) bool {
	if msg == nil {
		return false
	}
	content := strings.ToLower(msg.Content)
	return (strings.Contains(content, "image exceeds") && strings.Contains(content, "maximum")) ||
		(strings.Contains(content, "image dimensions exceed") && strings.Contains(content, "many-image")) ||
		strings.Contains(content, "maximum of") && strings.Contains(content, "pdf pages")
}

// GetPromptTooLongTokenGap extracts the token gap from a PTL error message
func GetPromptTooLongTokenGap(errorMessage string) int {
	// Try to find "N tokens over" pattern
	idx := strings.Index(strings.ToLower(errorMessage), "tokens")
	if idx > 0 {
		// Find number before "tokens"
		before := errorMessage[:idx]
		for i := len(before) - 1; i >= 0; i-- {
			if before[i] >= '0' && before[i] <= '9' {
				start := i
				for start > 0 && before[start-1] >= '0' && before[start-1] <= '9' {
					start--
				}
				var num int
				fmt.Sscanf(before[start:i+1], "%d", &num)
				return num
			}
		}
	}
	return 0
}
