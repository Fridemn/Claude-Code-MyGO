package tests

import (
	"context"
	"testing"
	"time"

	"claude-code-go/internal/api"
)

func TestStreamWatchdogConfig(t *testing.T) {
	cfg := api.DefaultStreamWatchdogConfig()

	// Default should be enabled
	if !cfg.Enabled {
		t.Error("Default watchdog should be enabled")
	}

	// Default timeout should be 90 seconds
	if cfg.IdleTimeout != 90*time.Second {
		t.Errorf("Default idle timeout should be 90s, got %v", cfg.IdleTimeout)
	}

	// Warning timeout should be half of idle timeout
	if cfg.WarningTimeout != 45*time.Second {
		t.Errorf("Warning timeout should be 45s, got %v", cfg.WarningTimeout)
	}
}

func TestStreamWatchdogStartStop(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := api.StreamWatchdogConfig{
		IdleTimeout:    5 * time.Second,
		WarningTimeout: 2 * time.Second,
		Enabled:        true,
	}

	watchdog := api.CreateStreamWatchdog(cfg, cancel)

	// Start should begin timers
	watchdog.Start()

	// Stop should cancel timers
	watchdog.Stop()

	// Should not be aborted
	if watchdog.WasAborted() {
		t.Error("Watchdog should not be aborted after Stop()")
	}
}

func TestStreamWatchdogReset(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := api.StreamWatchdogConfig{
		IdleTimeout:    100 * time.Millisecond,
		WarningTimeout: 50 * time.Millisecond,
		Enabled:        true,
	}

	watchdog := api.CreateStreamWatchdog(cfg, cancel)
	watchdog.Start()

	// Reset multiple times quickly to prevent timeout
	for i := 0; i < 10; i++ {
		watchdog.Reset()
		time.Sleep(10 * time.Millisecond)
	}

	watchdog.Stop()

	// Should not be aborted because we kept resetting
	if watchdog.WasAborted() {
		t.Error("Watchdog should not be aborted with continuous resets")
	}
}

func TestStreamWatchdogAbort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cfg := api.StreamWatchdogConfig{
		IdleTimeout:    50 * time.Millisecond,
		WarningTimeout: 25 * time.Millisecond,
		Enabled:        true,
	}

	watchdog := api.CreateStreamWatchdog(cfg, cancel)

	timeoutCalled := false
	watchdog.SetTimeoutCallback(func(d time.Duration) {
		timeoutCalled = true
	})

	watchdog.Start()

	// Wait for timeout to fire
	time.Sleep(100 * time.Millisecond)

	// Should be aborted
	if !watchdog.WasAborted() {
		t.Error("Watchdog should be aborted after idle timeout")
	}

	// Timeout callback should have been called
	if !timeoutCalled {
		t.Error("Timeout callback should have been called")
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled after watchdog abort")
	}
}

func TestStreamWatchdogDisabled(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := api.StreamWatchdogConfig{
		IdleTimeout:    50 * time.Millisecond,
		WarningTimeout: 25 * time.Millisecond,
		Enabled:        false,
	}

	watchdog := api.CreateStreamWatchdog(cfg, cancel)
	watchdog.Start()

	// Wait longer than timeout
	time.Sleep(100 * time.Millisecond)

	// Should not be aborted when disabled
	if watchdog.WasAborted() {
		t.Error("Disabled watchdog should never abort")
	}
}

func TestNonStreamingFallbackConfig(t *testing.T) {
	cfg := api.DefaultNonStreamingFallbackConfig()

	// Default should be enabled
	if !cfg.Enabled {
		t.Error("Default fallback should be enabled")
	}

	// Default timeout should be 300 seconds for local
	if cfg.Timeout != 300*time.Second {
		t.Errorf("Default timeout should be 300s, got %v", cfg.Timeout)
	}

	// Default max tokens should be reasonable
	if cfg.MaxTokens <= 0 {
		t.Error("Default max tokens should be positive")
	}
}

func TestStreamIdleTimeoutError(t *testing.T) {
	err := &api.StreamIdleTimeoutError{Timeout: 90 * time.Second}
	if !api.IsStreamIdleTimeout(err) {
		t.Error("IsStreamIdleTimeout should return true for StreamIdleTimeoutError")
	}

	// Should be recognized as timeout error
	if !api.IsTimeoutError(err) {
		t.Error("StreamIdleTimeoutError should be a timeout error")
	}
}

func TestStreamNoEventsError(t *testing.T) {
	err := &api.StreamNoEventsError{Reason: "test reason"}

	if !api.IsStreamNoEvents(err) {
		t.Error("IsStreamNoEvents should return true for StreamNoEventsError")
	}
}

func TestShouldFallbackToNonStreaming(t *testing.T) {
	cfg := api.DefaultNonStreamingFallbackConfig()

	// Should fallback for idle timeout
	idleErr := &api.StreamIdleTimeoutError{Timeout: 90 * time.Second}
	if !api.ShouldFallbackToNonStreaming(idleErr, cfg) {
		t.Error("Should fallback for stream idle timeout")
	}

	// Should fallback for no events
	noEventsErr := &api.StreamNoEventsError{Reason: "test"}
	if !api.ShouldFallbackToNonStreaming(noEventsErr, cfg) {
		t.Error("Should fallback for stream no events")
	}

	// Should NOT fallback for user cancel
	cancelErr := context.Canceled
	if api.ShouldFallbackToNonStreaming(cancelErr, cfg) {
		t.Error("Should NOT fallback for user cancellation")
	}

	// Should NOT fallback when disabled
	disabledCfg := api.NonStreamingFallbackConfig{Enabled: false}
	if api.ShouldFallbackToNonStreaming(idleErr, disabledCfg) {
		t.Error("Should NOT fallback when disabled")
	}
}

func TestStreamingResult(t *testing.T) {
	// Test structure fields
	result := &api.StreamingResult{
		Response:     nil,
		UsedFallback: false,
	}

	if result.UsedFallback {
		t.Error("UsedFallback should default to false")
	}
}