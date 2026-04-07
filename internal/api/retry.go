package api

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

// Default retry configuration constants
const (
	DefaultMaxRetries     = 10
	DefaultBaseDelay      = 500 * time.Millisecond
	DefaultMaxDelay       = 32 * time.Second
	DefaultJitterFactor   = 0.25
	Max529Retries         = 3
	StreamIdleTimeout     = 60 * time.Second
	PersistentMaxBackoff  = 5 * time.Minute
	PersistentResetCap    = 6 * time.Hour
	HeartbeatInterval     = 30 * time.Second
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxRetries       int
	BaseDelay        time.Duration
	MaxDelay         time.Duration
	MaxBackoff       time.Duration // For persistent mode
	JitterFactor     float64
	RetryableChecker func(error) bool
	PersistentMode   bool // For unattended sessions
	QuerySource      string
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       DefaultMaxRetries,
		BaseDelay:        DefaultBaseDelay,
		MaxDelay:         DefaultMaxDelay,
		JitterFactor:     DefaultJitterFactor,
		RetryableChecker: DefaultRetryableChecker,
	}
}

// StreamingRetryConfig returns retry configuration for streaming requests
func StreamingRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       DefaultMaxRetries,
		BaseDelay:        DefaultBaseDelay,
		MaxDelay:         DefaultMaxDelay,
		JitterFactor:     DefaultJitterFactor,
		RetryableChecker: StreamingRetryableChecker,
	}
}

// BackgroundRetryConfig returns retry configuration for background tasks
func BackgroundRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       3,
		BaseDelay:        DefaultBaseDelay,
		MaxDelay:         10 * time.Second,
		JitterFactor:     DefaultJitterFactor,
		RetryableChecker: DefaultRetryableChecker,
	}
}

// PersistentRetryConfig returns retry configuration for unattended sessions
func PersistentRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:       DefaultMaxRetries,
		BaseDelay:        DefaultBaseDelay,
		MaxDelay:         PersistentMaxBackoff,
		MaxBackoff:       PersistentMaxBackoff,
		JitterFactor:     DefaultJitterFactor,
		RetryableChecker: PersistentRetryableChecker,
		PersistentMode:   true,
	}
}

// RetryResult holds the result of a retry operation
type RetryResult struct {
	Attempts    int
	TotalDelay  time.Duration
	LastDelay   time.Duration
	LastAttempt time.Time
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	_, err := RetryWithResult(ctx, cfg, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// RetryWithResult executes a function with retry logic and returns a result
func RetryWithResult[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error
	persistentAttempt := 0

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if error is retryable
		if cfg.RetryableChecker != nil && !cfg.RetryableChecker(err) {
			return result, err
		}

		// Don't sleep after last attempt (unless in persistent mode)
		if !cfg.PersistentMode && attempt == cfg.MaxRetries {
			break
		}

		// Calculate delay
		delay := CalculateRetryDelay(attempt+1, err, cfg)

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}

		if cfg.PersistentMode {
			persistentAttempt++
			// In persistent mode, clamp the attempt counter so loop never terminates
			if attempt >= cfg.MaxRetries {
				attempt = cfg.MaxRetries - 1
			}
		}
	}

	return result, lastErr
}

// RetryWithCallback executes a function with retry logic and calls onRetry before each retry
func RetryWithCallback[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error), onRetry func(attempt int, err error, delay time.Duration)) (T, error) {
	var result T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if error is retryable
		if cfg.RetryableChecker != nil && !cfg.RetryableChecker(err) {
			return result, err
		}

		// Don't sleep after last attempt
		if attempt == cfg.MaxRetries {
			break
		}

		// Calculate delay
		delay := CalculateRetryDelay(attempt+1, err, cfg)

		// Call retry callback
		if onRetry != nil {
			onRetry(attempt+1, err, delay)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(delay):
		}
	}

	return result, lastErr
}

// CalculateRetryDelay calculates the delay with exponential backoff and jitter
func CalculateRetryDelay(attempt int, err error, cfg RetryConfig) time.Duration {
	// Check for Retry-After header
	if retryAfter := GetRetryAfter(err); retryAfter > 0 {
		return retryAfter
	}

	// Calculate base delay with exponential backoff
	delay := float64(cfg.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= 2
	}

	// Cap at max delay
	maxDelay := float64(cfg.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter
	if cfg.JitterFactor > 0 {
		jitter := delay * cfg.JitterFactor * (2*rand.Float64() - 1)
		delay += jitter
	}

	// Ensure minimum delay
	if delay < float64(cfg.BaseDelay) {
		delay = float64(cfg.BaseDelay)
	}

	return time.Duration(delay)
}

// DefaultRetryableChecker is the default function to determine if an error is retryable
func DefaultRetryableChecker(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors (not retryable)
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Check for rate limit errors
	if IsRateLimitError(err) {
		return true
	}

	// Check for overloaded errors
	if IsOverloadedError(err) {
		return true
	}

	// Check for server errors (5xx)
	if IsServerError(err) {
		return true
	}

	// Check for connection errors
	if IsConnectionError(err) {
		return true
	}

	// Check for timeout errors
	if IsTimeoutError(err) {
		return true
	}

	// Check for stale connection errors
	if IsStaleConnectionError(err) {
		return true
	}

	return false
}

// StreamingRetryableChecker determines if a streaming error is retryable
func StreamingRetryableChecker(err error) bool {
	// For streaming, we're more conservative
	if err == nil {
		return false
	}

	// Check for context errors (not retryable)
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Rate limits are retryable for streaming
	if IsRateLimitError(err) {
		return true
	}

	// Server overload is retryable
	if IsOverloadedError(err) {
		return true
	}

	// Server errors are retryable
	if IsServerError(err) {
		return true
	}

	// Connection errors during streaming are retryable
	if IsConnectionError(err) || IsStaleConnectionError(err) {
		return true
	}

	return false
}

// PersistentRetryableChecker determines if an error is retryable in persistent mode
func PersistentRetryableChecker(err error) bool {
	if err == nil {
		return false
	}

	// In persistent mode, 429 and 529 are always retryable
	if IsRateLimitError(err) || IsOverloadedError(err) {
		return true
	}

	// Server errors are retryable
	if IsServerError(err) {
		return true
	}

	// Connection errors are retryable
	if IsConnectionError(err) || IsTimeoutError(err) {
		return true
	}

	return false
}

// Backoff implements a simple exponential backoff
type Backoff struct {
	attempt    int
	base       time.Duration
	max        time.Duration
	multiplier float64
	jitter     float64
}

// CreateBackoff creates a new backoff instance
func CreateBackoff(base, max time.Duration, multiplier, jitter float64) *Backoff {
	return &Backoff{
		base:       base,
		max:        max,
		multiplier: multiplier,
		jitter:     jitter,
	}
}

// Next returns the next delay and increments the attempt counter
func (b *Backoff) Next() time.Duration {
	delay := float64(b.base)
	for i := 0; i < b.attempt; i++ {
		delay *= b.multiplier
	}
	if delay > float64(b.max) {
		delay = float64(b.max)
	}

	// Add jitter
	if b.jitter > 0 {
		jitterAmount := delay * b.jitter * (2*rand.Float64() - 1)
		delay += jitterAmount
	}

	b.attempt++
	return time.Duration(delay)
}

// Reset resets the backoff counter
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt number
func (b *Backoff) Attempt() int {
	return b.attempt
}

// IsRetryableError checks if an error should trigger a retry (for backward compatibility)
func IsRetryableError(err error) bool {
	return DefaultRetryableChecker(err)
}

// IsRetryableStatus checks if an HTTP status code indicates a retryable error
func IsRetryableStatus(statusCode int) bool {
	switch statusCode {
	case 408, 409, 429, 500, 502, 503, 504, 529:
		return true
	default:
		return statusCode >= 500
	}
}

// ShouldRetryHeader checks the x-should-retry header
func ShouldRetryHeader(headers http.Header) (retryable bool, hasHeader bool) {
	value := headers.Get("X-Should-Retry")
	if value == "" {
		return false, false
	}
	return value == "true", true
}

// ExtractHeadersFromResponse extracts relevant headers from an HTTP response
func ExtractHeadersFromResponse(resp *http.Response) map[string]string {
	headers := make(map[string]string)
	if resp == nil {
		return headers
	}

	// Extract relevant headers
	headerNames := []string{
		"Retry-After",
		"X-Request-Id",
		"Request-Id",
		"X-Should-Retry",
		"Anthropic-Ratelimit-Unified-Reset",
		"Anthropic-Ratelimit-Unified-Representative-Claim",
	}

	for _, name := range headerNames {
		if value := resp.Header.Get(name); value != "" {
			headers[name] = value
		}
	}

	return headers
}
