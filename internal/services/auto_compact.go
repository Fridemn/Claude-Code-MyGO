package services

import (
	"context"
	"fmt"
)

// Auto-compact constants
// Ported from src/services/compact/autoCompact.ts
const (
	MaxOutputTokensForSummary    = 20000
	AutocompactBufferTokens      = 13000
	WarningThresholdBufferTokens = 20000
	ErrorThresholdBufferTokens   = 20000
	ManualCompactBufferTokens    = 3000
	MaxConsecutiveAutocompactFailures = 3
)

// AutoCompactTrackingState tracks auto-compact state across turns.
// Ported from src/services/compact/autoCompact.ts:AutoCompactTrackingState
type AutoCompactTrackingState struct {
	Compacted            bool
	TurnCounter          int
	TurnID               string
	ConsecutiveFailures  int
}

// AutoCompactConfig contains configuration for auto-compaction.
type AutoCompactConfig struct {
	Enabled   bool
	Threshold int
}

// DefaultAutoCompactConfig returns default auto-compact configuration.
func DefaultAutoCompactConfig(contextWindow int) AutoCompactConfig {
	return AutoCompactConfig{
		Enabled:   true,
		Threshold: contextWindow - AutocompactBufferTokens,
	}
}

// GetEffectiveContextWindowSize calculates the effective context window.
// Ported from src/services/compact/autoCompact.ts:getEffectiveContextWindowSize
func GetEffectiveContextWindowSize(model string, maxOutputTokens int, contextWindowOverride int) int {
	// Use default if not specified
	if maxOutputTokens <= 0 {
		maxOutputTokens = MaxOutputTokensForSummary
	}

	contextWindow := contextWindowOverride
	if contextWindow <= 0 {
		// Default context window based on model
		contextWindow = 200000
	}

	return contextWindow - maxOutputTokens
}

// GetAutoCompactThreshold calculates the auto-compact threshold.
// Ported from src/services/compact/autoCompact.ts:getAutoCompactThreshold
func GetAutoCompactThreshold(model string, contextWindow int) int {
	effectiveWindow := GetEffectiveContextWindowSize(model, MaxOutputTokensForSummary, contextWindow)
	return effectiveWindow - AutocompactBufferTokens
}

// CalculateTokenWarningState calculates warning states based on token usage.
// Ported from src/services/compact/autoCompact.ts:calculateTokenWarningState
func CalculateTokenWarningState(tokenUsage int, model string, autoCompactEnabled bool, contextWindow int) TokenWarningState {
	threshold := contextWindow
	if autoCompactEnabled {
		threshold = GetAutoCompactThreshold(model, contextWindow)
	}

	percentLeft := max(0, ((threshold-tokenUsage)*100)/threshold)

	warningThreshold := threshold - WarningThresholdBufferTokens
	errorThreshold := threshold - ErrorThresholdBufferTokens

	isAboveWarningThreshold := tokenUsage >= warningThreshold
	isAboveErrorThreshold := tokenUsage >= errorThreshold
	isAboveAutoCompactThreshold := autoCompactEnabled && tokenUsage >= GetAutoCompactThreshold(model, contextWindow)

	defaultBlockingLimit := contextWindow - ManualCompactBufferTokens
	isAtBlockingLimit := tokenUsage >= defaultBlockingLimit

	return TokenWarningState{
		PercentLeft:                percentLeft,
		IsAboveWarningThreshold:    isAboveWarningThreshold,
		IsAboveErrorThreshold:      isAboveErrorThreshold,
		IsAboveAutoCompactThreshold: isAboveAutoCompactThreshold,
		IsAtBlockingLimit:          isAtBlockingLimit,
	}
}

// TokenWarningState contains warning state for token usage.
type TokenWarningState struct {
	PercentLeft                 int
	IsAboveWarningThreshold     bool
	IsAboveErrorThreshold       bool
	IsAboveAutoCompactThreshold bool
	IsAtBlockingLimit           bool
}

// ShouldAutoCompact determines if auto-compaction should trigger.
// Ported from src/services/compact/autoCompact.ts:shouldAutoCompact
func ShouldAutoCompact(messages []CompactMessage, model string, querySource string, contextWindow int, autoCompactEnabled bool) bool {
	// Recursion guards
	if querySource == "session_memory" || querySource == "compact" {
		return false
	}

	if !autoCompactEnabled {
		return false
	}

	tokenCount := EstimateMessagesTokenCount(messages)
	threshold := GetAutoCompactThreshold(model, contextWindow)

	return tokenCount >= threshold
}

// AutoCompactIfNeeded performs auto-compaction if needed.
// Ported from src/services/compact/autoCompact.ts:autoCompactIfNeeded
func AutoCompactIfNeeded(
	ctx context.Context,
	messages []CompactMessage,
	compactService *CompactService,
	model string,
	querySource string,
	tracking *AutoCompactTrackingState,
	contextWindow int,
	autoCompactEnabled bool,
) (*CompactionResult, error) {
	if !autoCompactEnabled {
		return nil, nil
	}

	// Circuit breaker: stop retrying after N consecutive failures
	if tracking != nil && tracking.ConsecutiveFailures >= MaxConsecutiveAutocompactFailures {
		return nil, nil
	}

	shouldCompact := ShouldAutoCompact(messages, model, querySource, contextWindow, autoCompactEnabled)
	if !shouldCompact {
		return nil, nil
	}

	recompactionInfo := RecompactionInfo{
		IsRecompactionInChain:     tracking != nil && tracking.Compacted,
		TurnsSincePreviousCompact: -1,
		PreviousCompactTurnID:     "",
		AutoCompactThreshold:      GetAutoCompactThreshold(model, contextWindow),
		QuerySource:               querySource,
	}

	if tracking != nil {
		recompactionInfo.TurnsSincePreviousCompact = tracking.TurnCounter
		recompactionInfo.PreviousCompactTurnID = tracking.TurnID
	}

	// Perform compaction
	result, err := compactService.Compact(ctx, messages, "", true)
	if err != nil {
		// Increment consecutive failure count
		return nil, fmt.Errorf("auto-compact failed: %w", err)
	}

	return result, nil
}

// RunPostCompactCleanup runs cleanup after compaction.
// Ported from src/services/compact/postCompactCleanup.ts:runPostCompactCleanup
func RunPostCompactCleanup(querySource string) {
	// Reset microcompact state
	ResetMicrocompactState()

	// Clear classifier approvals, speculative checks, etc.
	// In full implementation, this would clear various caches
}

// ResetMicrocompactState resets the microcompact state.
func ResetMicrocompactState() {
	// In full implementation, this would reset cached MC state
}
