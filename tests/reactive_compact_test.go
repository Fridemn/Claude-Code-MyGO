package tests

import (
	"testing"

	"claude-go/internal/services"
)

func TestReactiveCompactConfig(t *testing.T) {
	t.Parallel()

	config := services.DefaultReactiveCompactConfig()

	if config.Enabled != false {
		t.Errorf("Enabled should be false by default, got %v", config.Enabled)
	}
	if config.GapThresholdMinutes != 60 {
		t.Errorf("GapThresholdMinutes should be 60, got %d", config.GapThresholdMinutes)
	}
	if config.KeepRecent != 5 {
		t.Errorf("KeepRecent should be 5, got %d", config.KeepRecent)
	}
}

func TestReactiveCompactServiceEnabled(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Initially disabled
	if svc.IsReactiveCompactEnabled() {
		t.Error("Reactive compact should be disabled by default")
	}

	// Enable
	svc.SetEnabled(true)
	if !svc.IsReactiveCompactEnabled() {
		t.Error("Reactive compact should be enabled after SetEnabled(true)")
	}

	// Disable
	svc.SetEnabled(false)
	if svc.IsReactiveCompactEnabled() {
		t.Error("Reactive compact should be disabled after SetEnabled(false)")
	}
}

func TestReactiveOnlyMode(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Initially disabled
	if svc.IsReactiveOnlyMode() {
		t.Error("Reactive-only mode should be disabled by default")
	}

	// Enable
	svc.SetReactiveOnlyMode(true)
	if !svc.IsReactiveOnlyMode() {
		t.Error("Reactive-only mode should be enabled")
	}
}

func TestCompactThreshold(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Default threshold
	if svc.GetCompactThreshold() != 150000 {
		t.Errorf("Default threshold should be 150000, got %d", svc.GetCompactThreshold())
	}

	// Set new threshold
	svc.SetCompactThreshold(100000)
	if svc.GetCompactThreshold() != 100000 {
		t.Errorf("Threshold should be 100000, got %d", svc.GetCompactThreshold())
	}
}

func TestHasAttempted(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Initially false
	if svc.GetHasAttempted() {
		t.Error("HasAttempted should be false initially")
	}

	// Set to true
	svc.SetHasAttempted(true)
	if !svc.GetHasAttempted() {
		t.Error("HasAttempted should be true")
	}

	// Reset
	svc.ResetReactiveCompactState()
	if svc.GetHasAttempted() {
		t.Error("HasAttempted should be false after reset")
	}
}

func TestIsPromptTooLongMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg      *services.CompactMessage
		expected bool
	}{
		{nil, false},
		{&services.CompactMessage{Content: "normal message"}, false},
		{&services.CompactMessage{Content: "Prompt is too long: 1000 tokens"}, true},
		{&services.CompactMessage{Content: "prompt is too long"}, true},
		{&services.CompactMessage{Content: "Error: prompt is too long for context"}, true},
	}

	for _, tc := range tests {
		result := services.IsPromptTooLongMessage(tc.msg)
		if result != tc.expected {
			t.Errorf("IsPromptTooLongMessage(%v) = %v, want %v", tc.msg, result, tc.expected)
		}
	}
}

func TestIsMediaSizeErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg      *services.CompactMessage
		expected bool
	}{
		{nil, false},
		{&services.CompactMessage{Content: "normal message"}, false},
		{&services.CompactMessage{Content: "image exceeds maximum size"}, true},
		{&services.CompactMessage{Content: "Image dimensions exceed many-image limit"}, true},
		{&services.CompactMessage{Content: "maximum of 10 PDF pages exceeded"}, true},
		{&services.CompactMessage{Content: "file too large"}, false},
	}

	for _, tc := range tests {
		result := services.IsMediaSizeErrorMessage(tc.msg)
		if result != tc.expected {
			t.Errorf("IsMediaSizeErrorMessage(%v) = %v, want %v", tc.msg, result, tc.expected)
		}
	}
}

func TestGetPromptTooLongTokenGap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		message  string
		expected int
	}{
		{"prompt is too long: 5000 tokens over the limit", 5000},
		{"Prompt is too long: 10000 tokens", 10000},
		{"no token info here", 0},
		{"", 0},
	}

	for _, tc := range tests {
		result := services.GetPromptTooLongTokenGap(tc.message)
		if result != tc.expected {
			t.Errorf("GetPromptTooLongTokenGap(%q) = %d, want %d", tc.message, result, tc.expected)
		}
	}
}

func TestIsWithheldPromptTooLong(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Nil message
	if svc.IsWithheldPromptTooLong(nil) {
		t.Error("nil message should return false")
	}

	// Normal message
	msg := &services.CompactMessage{Content: "normal"}
	if svc.IsWithheldPromptTooLong(msg) {
		t.Error("normal message should return false")
	}

	// PTL message
	ptlMsg := &services.CompactMessage{Content: "prompt is too long"}
	if !svc.IsWithheldPromptTooLong(ptlMsg) {
		t.Error("PTL message should return true")
	}
}

func TestIsWithheldMediaSizeError(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Nil message
	if svc.IsWithheldMediaSizeError(nil) {
		t.Error("nil message should return false")
	}

	// Normal message
	msg := &services.CompactMessage{Content: "normal"}
	if svc.IsWithheldMediaSizeError(msg) {
		t.Error("normal message should return false")
	}

	// Media error message
	mediaMsg := &services.CompactMessage{Content: "image exceeds maximum size"}
	if !svc.IsWithheldMediaSizeError(mediaMsg) {
		t.Error("media error message should return true")
	}
}

func TestTryReactiveCompactDisabled(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()
	svc.SetEnabled(false)

	params := &services.TryReactiveCompactParams{
		Messages: []services.CompactMessage{
			{Type: "user", Role: "user", Content: "test"},
		},
	}

	result := svc.TryReactiveCompact(params)
	if result.OK || result.Reason != "disabled" {
		t.Errorf("expected disabled result, got OK=%v Reason=%s", result.OK, result.Reason)
	}
}

func TestTryReactiveCompactAlreadyAttempted(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()
	svc.SetEnabled(true)

	params := &services.TryReactiveCompactParams{
		HasAttempted: true,
		Messages: []services.CompactMessage{
			{Type: "user", Role: "user", Content: "test"},
		},
	}

	result := svc.TryReactiveCompact(params)
	if result.OK || result.Reason != "exhausted" {
		t.Errorf("expected exhausted result, got OK=%v Reason=%s", result.OK, result.Reason)
	}
}

func TestTryReactiveCompactAborted(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()
	svc.SetEnabled(true)

	params := &services.TryReactiveCompactParams{
		Aborted: true,
		Messages: []services.CompactMessage{
			{Type: "user", Role: "user", Content: "test"},
		},
	}

	result := svc.TryReactiveCompact(params)
	if result.OK || result.Reason != "aborted" {
		t.Errorf("expected aborted result, got OK=%v Reason=%s", result.OK, result.Reason)
	}
}

func TestGlobalReactiveCompactService(t *testing.T) {
	t.Parallel()

	svc := services.GetGlobalReactiveCompactService()
	if svc == nil {
		t.Fatal("global service should not be nil")
	}

	// Test that we can modify it
	svc.SetEnabled(true)
	if !svc.IsReactiveCompactEnabled() {
		t.Error("global service should be enabled")
	}

	// Reset
	svc.SetEnabled(false)
}

func TestReactiveCompactConfigSetGet(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	newConfig := services.ReactiveCompactConfig{
		Enabled:           true,
		GapThresholdMinutes: 30,
		KeepRecent:        3,
	}

	svc.SetReactiveCompactConfig(newConfig)
	got := svc.GetReactiveCompactConfig()

	if got.Enabled != newConfig.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", got.Enabled, newConfig.Enabled)
	}
	if got.GapThresholdMinutes != newConfig.GapThresholdMinutes {
		t.Errorf("GapThresholdMinutes mismatch: got %d, want %d", got.GapThresholdMinutes, newConfig.GapThresholdMinutes)
	}
	if got.KeepRecent != newConfig.KeepRecent {
		t.Errorf("KeepRecent mismatch: got %d, want %d", got.KeepRecent, newConfig.KeepRecent)
	}
}

func TestLastAssistantTime(t *testing.T) {
	t.Parallel()

	svc := services.NewReactiveCompactService()

	// Initially 0
	if svc.GetLastAssistantTime() != 0 {
		t.Error("LastAssistantTime should be 0 initially")
	}

	// Set
	svc.UpdateLastAssistantTime(12345)
	if svc.GetLastAssistantTime() != 12345 {
		t.Errorf("LastAssistantTime should be 12345, got %d", svc.GetLastAssistantTime())
	}
}