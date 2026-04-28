package services

import "sync"

// Compact Warning State
// Ported from src/services/compact/compactWarningState.ts

// CompactWarningState tracks whether the compact warning should be suppressed.
// We suppress immediately after successful compaction since we don't have accurate
// token counts until the next API response.
type CompactWarningState struct {
	suppressed bool
	mu         sync.RWMutex
}

// Global compact warning state
var compactWarningState = &CompactWarningState{}

// SuppressCompactWarning suppresses the compact warning.
// Call after successful compaction.
func SuppressCompactWarning() {
	compactWarningState.mu.Lock()
	defer compactWarningState.mu.Unlock()
	compactWarningState.suppressed = true
}

// ClearCompactWarningSuppression clears the compact warning suppression.
// Called at start of new compact attempt.
func ClearCompactWarningSuppression() {
	compactWarningState.mu.Lock()
	defer compactWarningState.mu.Unlock()
	compactWarningState.suppressed = false
}

// IsCompactWarningSuppressed returns whether the compact warning is currently suppressed.
func IsCompactWarningSuppressed() bool {
	compactWarningState.mu.RLock()
	defer compactWarningState.mu.RUnlock()
	return compactWarningState.suppressed
}

// ResetCompactWarningState resets the compact warning state to initial values.
// Useful for testing.
func ResetCompactWarningState() {
	compactWarningState.mu.Lock()
	defer compactWarningState.mu.Unlock()
	compactWarningState.suppressed = false
}
