package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StreamWatchdogConfig holds configuration for stream idle timeout watchdog
type StreamWatchdogConfig struct {
	// IdleTimeout is the maximum time to wait for a chunk before aborting
	IdleTimeout time.Duration
	// WarningTimeout is when to emit a warning (typically half of IdleTimeout)
	WarningTimeout time.Duration
	// Enabled controls whether the watchdog is active
	Enabled bool
}

// DefaultStreamWatchdogConfig returns default watchdog configuration
// Matches TypeScript: STREAM_IDLE_TIMEOUT_MS = 90_000 (90 seconds)
func DefaultStreamWatchdogConfig() StreamWatchdogConfig {
	// Allow override via environment variable (same as TS)
	timeoutMs := 90000 // default 90 seconds
	if envVal := os.Getenv("CLAUDE_STREAM_IDLE_TIMEOUT_MS"); envVal != "" {
		if parsed, err := strconv.Atoi(envVal); err == nil && parsed > 0 {
			timeoutMs = parsed
		}
	}

	// Enable by default, can disable via CLAUDE_ENABLE_STREAM_WATCHDOG=false
	enabled := true
	if envVal := os.Getenv("CLAUDE_ENABLE_STREAM_WATCHDOG"); envVal != "" {
		enabled = strings.ToLower(envVal) != "false" && envVal != "0"
	}

	return StreamWatchdogConfig{
		IdleTimeout:    time.Duration(timeoutMs) * time.Millisecond,
		WarningTimeout: time.Duration(timeoutMs/2) * time.Millisecond,
		Enabled:        enabled,
	}
}

// StreamWatchdog monitors streaming for idle timeouts
type StreamWatchdog struct {
	config        StreamWatchdogConfig
	lastChunkTime time.Time
	timer         *time.Timer
	warningTimer  *time.Timer
	aborted       bool
	firedAt       time.Time
	cancelFunc    context.CancelFunc
	mu            sync.Mutex
	onWarning     func(duration time.Duration)
	onTimeout     func(duration time.Duration)
}

// CreateStreamWatchdog creates a new stream watchdog
func CreateStreamWatchdog(config StreamWatchdogConfig, cancelFunc context.CancelFunc) *StreamWatchdog {
	return &StreamWatchdog{
		config:     config,
		cancelFunc: cancelFunc,
	}
}

// Start begins the watchdog monitoring
func (w *StreamWatchdog) Start() {
	if !w.config.Enabled {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastChunkTime = time.Now()
	w.aborted = false

	// Set up warning timer
	if w.onWarning != nil && w.config.WarningTimeout > 0 {
		w.warningTimer = time.AfterFunc(w.config.WarningTimeout, func() {
			w.onWarning(w.config.WarningTimeout)
		})
	}

	// Set up idle timeout timer
	w.timer = time.AfterFunc(w.config.IdleTimeout, func() {
		w.mu.Lock()
		w.aborted = true
		w.firedAt = time.Now()
		w.mu.Unlock()

		if w.onTimeout != nil {
			w.onTimeout(w.config.IdleTimeout)
		}

		// Abort the stream by canceling context
		if w.cancelFunc != nil {
			w.cancelFunc()
		}
	})
}

// Reset resets the timers after receiving a chunk
func (w *StreamWatchdog) Reset() {
	if !w.config.Enabled {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.lastChunkTime = time.Now()

	// Reset warning timer
	if w.warningTimer != nil {
		w.warningTimer.Reset(w.config.WarningTimeout)
	}

	// Reset idle timeout timer
	if w.timer != nil {
		w.timer.Reset(w.config.IdleTimeout)
	}
}

// Stop stops the watchdog timers
func (w *StreamWatchdog) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.warningTimer != nil {
		w.warningTimer.Stop()
		w.warningTimer = nil
	}

	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
}

// WasAborted returns true if the watchdog aborted the stream
func (w *StreamWatchdog) WasAborted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.aborted
}

// FiredAt returns the time when the watchdog fired (for measuring abort propagation delay)
func (w *StreamWatchdog) FiredAt() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.firedAt
}

// SetWarningCallback sets the warning callback
func (w *StreamWatchdog) SetWarningCallback(fn func(duration time.Duration)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onWarning = fn
}

// SetTimeoutCallback sets the timeout callback
func (w *StreamWatchdog) SetTimeoutCallback(fn func(duration time.Duration)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onTimeout = fn
}

// StreamIdleTimeoutError represents a stream idle timeout error
type StreamIdleTimeoutError struct {
	Timeout time.Duration
}

func (e *StreamIdleTimeoutError) Error() string {
	return fmt.Sprintf("stream idle timeout: no chunks received for %d seconds", int(e.Timeout.Seconds()))
}

// IsTimeoutError implementation
func (e *StreamIdleTimeoutError) IsTimeoutError() bool {
	return true
}

// StreamNoEventsError represents a stream that completed without events
type StreamNoEventsError struct {
	Reason string
}

func (e *StreamNoEventsError) Error() string {
	return fmt.Sprintf("stream ended without receiving events: %s", e.Reason)
}

// NonStreamingFallbackConfig holds configuration for non-streaming fallback
type NonStreamingFallbackConfig struct {
	// Timeout for non-streaming requests (matches TS getNonstreamingFallbackTimeoutMs)
	Timeout time.Duration
	// Max tokens for non-streaming (typically capped)
	MaxTokens int
	// Enabled controls whether fallback is allowed
	Enabled bool
}

// DefaultNonStreamingFallbackConfig returns default fallback configuration
// Matches TypeScript: 120s for remote, 300s for local
func DefaultNonStreamingFallbackConfig() NonStreamingFallbackConfig {
	timeoutMs := 300000 // default 300 seconds for local
	if os.Getenv("CLAUDE_CODE_REMOTE") != "" {
		timeoutMs = 120000 // 120 seconds for remote (CCR container idle-kill ~5min)
	}

	// Allow override via API_TIMEOUT_MS
	if envVal := os.Getenv("API_TIMEOUT_MS"); envVal != "" {
		if parsed, err := strconv.Atoi(envVal); err == nil && parsed > 0 {
			timeoutMs = parsed
		}
	}

	// Check if disabled via env var
	enabled := true
	if envVal := os.Getenv("CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK"); envVal != "" {
		enabled = strings.ToLower(envVal) != "false" && envVal != "0"
	}

	return NonStreamingFallbackConfig{
		Timeout:   time.Duration(timeoutMs) * time.Millisecond,
		MaxTokens: 4096, // Default cap for non-streaming
		Enabled:   enabled,
	}
}

// StreamingResult holds the result of a streaming request with fallback support
type StreamingResult struct {
	Response     *ChatCompletionResponse
	UsedFallback bool
	FallbackError error
}

// CompleteStreamWithWatchdog performs a streaming request with idle timeout watchdog and fallback
func (c *OpenAICompatibleClient) CompleteStreamWithWatchdog(
	ctx context.Context,
	req ChatCompletionRequest,
	onChunk func(chunk *StreamChunk) error,
	fallbackConfig NonStreamingFallbackConfig,
) (*StreamingResult, error) {
	// Create a cancellable context for the watchdog
	streamCtx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	// Set up watchdog
	watchdogConfig := DefaultStreamWatchdogConfig()
	watchdog := CreateStreamWatchdog(watchdogConfig, cancelFunc)

	// Set up callbacks
	watchdog.SetWarningCallback(func(d time.Duration) {
		// Log warning (could be integrated with logging system)
		// Matches TS: logForDebugging('Streaming idle warning...')
	})

	watchdog.SetTimeoutCallback(func(d time.Duration) {
		// Log timeout (could be integrated with logging system)
		// Matches TS: logForDebugging('Streaming idle timeout...')
	})

	// Start watchdog
	watchdog.Start()

	// Attempt streaming
	resp, streamErr := c.tryCompleteStreamWithWatchdog(streamCtx, req, onChunk, watchdog)

	// Stop watchdog after stream completes
	watchdog.Stop()

	// Check if watchdog aborted the stream
	if watchdog.WasAborted() {
		streamErr = &StreamIdleTimeoutError{Timeout: watchdogConfig.IdleTimeout}
	}

	// Handle streaming errors with fallback
	if streamErr != nil {
		// Check if fallback is enabled
		if !fallbackConfig.Enabled {
			return &StreamingResult{
				Response:     nil,
				UsedFallback: false,
				FallbackError: streamErr,
			}, streamErr
		}

		// Don't fallback for user aborts
		if streamErr == context.Canceled {
			return &StreamingResult{
				Response:     nil,
				UsedFallback: false,
				FallbackError: streamErr,
			}, streamErr
		}

		// Perform non-streaming fallback
		fallbackResp, fallbackErr := c.doNonStreamingFallback(ctx, req, fallbackConfig)

		return &StreamingResult{
			Response:     fallbackResp,
			UsedFallback: true,
			FallbackError: streamErr,
		}, fallbackErr
	}

	return &StreamingResult{
		Response:     resp,
		UsedFallback: false,
		FallbackError: nil,
	}, nil
}

// tryCompleteStreamWithWatchdog performs streaming with watchdog monitoring
func (c *OpenAICompatibleClient) tryCompleteStreamWithWatchdog(
	ctx context.Context,
	req ChatCompletionRequest,
	onChunk func(chunk *StreamChunk) error,
	watchdog *StreamWatchdog,
) (*ChatCompletionResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := createHTTPRequest(ctx, c.baseURL, body, c.apiKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, wrapConnectionError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := readAll(resp.Body)
		headers := ExtractHeadersFromResponse(resp)
		return nil, ParseErrorWithHeaders(resp.StatusCode, respBody, headers)
	}

	aggregated := &ChatCompletionResponse{
		Choices: []Choice{
			{Message: &ChatMessage{Role: "assistant", Content: ""}},
		},
	}

	// Track stream state for no-events detection
	receivedContentBlocks := false
	stopReason := ""

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Reset watchdog on each chunk
		watchdog.Reset()

		line := scanner.Text()

		if line == "" || hasPrefix(line, ":") {
			continue
		}
		if !hasPrefix(line, "data: ") {
			continue
		}

		data := trimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Track message state (matches TS partialMessage tracking)
		// Note: OpenAI format doesn't have message_start event like Anthropic
		// We track by checking for actual content
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok && content != "" {
				receivedContentBlocks = true
			}
			if chunk.Choices[0].FinishReason != nil {
				stopReason = *chunk.Choices[0].FinishReason
			}
		}

		if onChunk != nil {
			if err := onChunk(&chunk); err != nil {
				return nil, err
			}
		}

		// Aggregate chunks
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				aggregated.Choices[0].Message.Content = aggregated.Choices[0].Message.Content.(string) + content
			}
			if chunk.Choices[0].Delta.ToolCalls != nil {
				aggregated.Choices[0].Message.ToolCalls = mergeToolCalls(
					aggregated.Choices[0].Message.ToolCalls,
					chunk.Choices[0].Delta.ToolCalls,
				)
			}
			if chunk.Choices[0].FinishReason != nil {
				aggregated.Choices[0].FinishReason = *chunk.Choices[0].FinishReason
			}
		}

		aggregated.ID = chunk.ID
		aggregated.Model = chunk.Model
	}

	if err := scanner.Err(); err != nil {
		return nil, wrapConnectionError(err)
	}

	// Check for incomplete stream (matches TS: !partialMessage || newMessages.length === 0)
	// In OpenAI format, we check if we received any content or a valid finish reason
	if !receivedContentBlocks && stopReason == "" {
		return nil, &StreamNoEventsError{
			Reason: "no content blocks completed and no stop_reason received",
		}
	}

	return aggregated, nil
}

// doNonStreamingFallback performs a non-streaming request as fallback
func (c *OpenAICompatibleClient) doNonStreamingFallback(
	ctx context.Context,
	req ChatCompletionRequest,
	config NonStreamingFallbackConfig,
) (*ChatCompletionResponse, error) {
	// Create a context with timeout for non-streaming
	timeoutCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Adjust request for non-streaming
	req.Stream = false

	// Cap max tokens if needed
	if req.MaxTokens == nil || *req.MaxTokens > config.MaxTokens {
		capped := config.MaxTokens
		req.MaxTokens = &capped
	}

	// Perform non-streaming request with retry
	cfg := DefaultRetryConfig()
	cfg.MaxRetries = 2 // Fewer retries for fallback

	return RetryWithResult(timeoutCtx, cfg, func() (*ChatCompletionResponse, error) {
		return c.executeRequest(timeoutCtx, mustMarshal(req))
	})
}

// mustMarshal marshals to JSON, panicking on error (for internal use)
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return b
}

// DoCompleteStreamWithFallback is the public API for streaming with fallback support
func (c *OpenAICompatibleClient) DoCompleteStreamWithFallback(
	ctx context.Context,
	req ChatCompletionRequest,
	onChunk func(chunk *StreamChunk) error,
) (*ChatCompletionResponse, error) {
	fallbackConfig := DefaultNonStreamingFallbackConfig()
	result, err := c.CompleteStreamWithWatchdog(ctx, req, onChunk, fallbackConfig)
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

// IsStreamIdleTimeout checks if an error is a stream idle timeout
func IsStreamIdleTimeout(err error) bool {
	if _, ok := err.(*StreamIdleTimeoutError); ok {
		return true
	}
	return false
}

// IsStreamNoEvents checks if an error is a stream no-events error
func IsStreamNoEvents(err error) bool {
	if _, ok := err.(*StreamNoEventsError); ok {
		return true
	}
	return false
}

// ShouldFallbackToNonStreaming determines if a streaming error should trigger fallback
func ShouldFallbackToNonStreaming(err error, config NonStreamingFallbackConfig) bool {
	if !config.Enabled {
		return false
	}

	// Don't fallback for user aborts
	if err == context.Canceled {
		return false
	}

	// Fallback for idle timeout
	if IsStreamIdleTimeout(err) {
		return true
	}

	// Fallback for connection errors
	if IsConnectionError(err) {
		return true
	}

	// Fallback for timeout errors
	if IsTimeoutError(err) {
		return true
	}

	// Fallback for stream no-events
	if IsStreamNoEvents(err) {
		return true
	}

	return false
}