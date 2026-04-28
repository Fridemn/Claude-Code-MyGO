package tests

import (
	"testing"
	"time"

	"claude-go/internal/api"
)

func TestClassifyAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		expectClass   api.APIErrorClassification
	}{
		{
			name:        "nil error",
			err:         nil,
			expectClass: api.ClassificationUnknown,
		},
		{
			name:        "rate limit 429",
			err:         &api.APIError{StatusCode: 429, Message: "rate limited"},
			expectClass: api.ClassificationRateLimit,
		},
		{
			name:        "529 overload",
			err:         &api.APIError{StatusCode: 529, Message: "overloaded"},
			expectClass: api.ClassificationServerOverload,
		},
		{
			name:        "529 with overloaded_error in message",
			err:         &api.APIError{StatusCode: 529, Message: `{"type":"overloaded_error"}`},
			expectClass: api.ClassificationServerOverload,
		},
		{
			name:        "prompt too long",
			err:         &api.APIError{StatusCode: 400, Message: "prompt is too long: 100000 tokens > 90000 maximum"},
			expectClass: api.ClassificationPromptTooLong,
		},
		{
			name:        "authentication error 401",
			err:         &api.APIError{StatusCode: 401, Message: "invalid x-api-key"},
			expectClass: api.ClassificationInvalidAPIKey,
		},
		{
			name:        "OAuth token revoked",
			err:         &api.APIError{StatusCode: 403, Message: "OAuth token has been revoked"},
			expectClass: api.ClassificationTokenRevoked,
		},
		{
			name:        "server error 500",
			err:         &api.APIError{StatusCode: 500, Message: "internal server error"},
			expectClass: api.ClassificationServerError,
		},
		{
			name:        "client error 400",
			err:         &api.APIError{StatusCode: 400, Message: "bad request"},
			expectClass: api.ClassificationClientError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			class := api.ClassifyAPIError(tc.err)
			if class != tc.expectClass {
				t.Errorf("expected classification %q, got %q", tc.expectClass, class)
			}
		})
	}
}

func TestIs529Error(t *testing.T) {
	t.Parallel()

	if api.Is529Error(nil) {
		t.Fatal("nil should not be 529 error")
	}

	// 529 status code
	err := &api.APIError{StatusCode: 529, Message: "overloaded"}
	if !api.Is529Error(err) {
		t.Fatal("529 status should be 529 error")
	}

	// Overloaded error in message (SDK sometimes fails to pass status)
	err = &api.APIError{StatusCode: 500, Message: `{"type":"overloaded_error"}`}
	if !api.Is529Error(err) {
		t.Fatal("overloaded_error in message should be 529 error")
	}

	// Not 529
	err = &api.APIError{StatusCode: 500, Message: "internal error"}
	if api.Is529Error(err) {
		t.Fatal("500 without overloaded should not be 529 error")
	}
}

func TestIsOAuthTokenRevokedError(t *testing.T) {
	t.Parallel()

	// Token revoked
	err := &api.APIError{StatusCode: 403, Message: "OAuth token has been revoked"}
	if !api.IsOAuthTokenRevokedError(err) {
		t.Fatal("should detect revoked token error")
	}

	// Not revoked
	err = &api.APIError{StatusCode: 403, Message: "forbidden"}
	if api.IsOAuthTokenRevokedError(err) {
		t.Fatal("should not detect non-revoked error")
	}
}

func TestIsMediaSizeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw      string
		expected bool
	}{
		{"image exceeds 5 MB maximum: 5316852 bytes > 5242880 bytes", true},
		{"image dimensions exceed 2000px limit for many-image requests", true},
		{"maximum of 100 PDF pages", true},
		{"some other error", false},
		{"prompt is too long", false},
	}

	for _, tc := range tests {
		result := api.IsMediaSizeError(tc.raw)
		if result != tc.expected {
			t.Errorf("IsMediaSizeError(%q) = %v, want %v", tc.raw, result, tc.expected)
		}
	}
}

func TestParsePromptTooLongTokenCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw          string
		expectActual int
		expectLimit  int
		expectValues bool
	}{
		{
			raw:          "prompt is too long: 137500 tokens > 135000 maximum",
			expectActual: 137500,
			expectLimit:  135000,
			expectValues: true,
		},
		{
			raw:          "Prompt is too long: 100000 tokens > 90000 maximum",
			expectActual: 100000,
			expectLimit:  90000,
			expectValues: true,
		},
		{
			raw:          "some other error",
			expectValues: false,
		},
	}

	for _, tc := range tests {
		counts := api.ParsePromptTooLongTokenCounts(tc.raw)
		if counts.HasValues != tc.expectValues {
			t.Errorf("ParsePromptTooLongTokenCounts(%q).HasValues = %v, want %v", tc.raw, counts.HasValues, tc.expectValues)
			continue
		}
		if tc.expectValues {
			if counts.ActualTokens != tc.expectActual {
				t.Errorf("ParsePromptTooLongTokenCounts(%q).ActualTokens = %d, want %d", tc.raw, counts.ActualTokens, tc.expectActual)
			}
			if counts.LimitTokens != tc.expectLimit {
				t.Errorf("ParsePromptTooLongTokenCounts(%q).LimitTokens = %d, want %d", tc.raw, counts.LimitTokens, tc.expectLimit)
			}
		}
	}
}

func TestShouldRetry529(t *testing.T) {
	t.Parallel()

	// Foreground sources should retry
	foregroundSources := []api.QuerySource{
		api.QuerySourceREPLMainThread,
		api.QuerySourceSDK,
		api.QuerySourceAgentCustom,
		api.QuerySourceAgentDefault,
		api.QuerySourceCompact,
		api.QuerySourceAutoMode,
	}

	for _, src := range foregroundSources {
		if !api.ShouldRetry529(src) {
			t.Errorf("foreground source %q should retry on 529", src)
		}
	}

	// Empty source should retry (conservative default)
	if !api.ShouldRetry529("") {
		t.Fatal("empty source should retry on 529")
	}

	// Background sources should not retry
	if api.ShouldRetry529(api.QuerySourceBackground) {
		t.Fatal("background source should not retry on 529")
	}
}

func TestRetryConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg := api.DefaultRetryConfig()
	if cfg.MaxRetries != api.DefaultMaxRetries {
		t.Errorf("expected MaxRetries %d, got %d", api.DefaultMaxRetries, cfg.MaxRetries)
	}
	if cfg.BaseDelay != api.DefaultBaseDelay {
		t.Errorf("expected BaseDelay %v, got %v", api.DefaultBaseDelay, cfg.BaseDelay)
	}
	if cfg.JitterFactor != api.DefaultJitterFactor {
		t.Errorf("expected JitterFactor %v, got %v", api.DefaultJitterFactor, cfg.JitterFactor)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	t.Parallel()

	cfg := api.DefaultRetryConfig()

	// First attempt: BaseDelay * 2^1 = 1000ms (with jitter ±25%)
	delay1 := api.CalculateRetryDelay(1, nil, cfg)
	// With jitter, it should be between ~750ms and ~1250ms
	if delay1 < 700*time.Millisecond || delay1 > 1300*time.Millisecond {
		t.Errorf("first attempt delay %v should be around 1s (BaseDelay * 2, with jitter)", delay1)
	}

	// Second attempt: BaseDelay * 2^2 = 2000ms
	delay2 := api.CalculateRetryDelay(2, nil, cfg)
	if delay2 <= delay1 {
		t.Errorf("delay should increase with attempts: delay1=%v, delay2=%v", delay1, delay2)
	}

	// High attempt number: should cap at MaxDelay
	delayLarge := api.CalculateRetryDelay(20, nil, cfg)
	// With jitter, max is MaxDelay * 1.25 = 40s
	if delayLarge > cfg.MaxDelay*2 {
		t.Errorf("delay %v should be capped around max delay %v", delayLarge, cfg.MaxDelay)
	}
}

func TestCalculateRetryDelayWithRetryAfter(t *testing.T) {
	t.Parallel()

	cfg := api.DefaultRetryConfig()

	// Error with RetryAfter set (simulating what happens after parsing headers)
	err := &api.APIError{
		StatusCode: 429,
		Message:    "rate limited",
		RetryAfter: 30 * time.Second,
	}

	delay := api.CalculateRetryDelay(1, err, cfg)
	// Should return RetryAfter value as duration
	if delay < 28*time.Second || delay > 32*time.Second {
		t.Errorf("expected delay around 30s for RetryAfter=30, got %v", delay)
	}
}

func TestCannotRetryError(t *testing.T) {
	t.Parallel()

	originalErr := &api.APIError{StatusCode: 500, Message: "server error"}
	retryErr := &api.CannotRetryError{
		OriginalError: originalErr,
		Model:         "test-model",
	}

	if retryErr.Error() != originalErr.Error() {
		t.Errorf("CannotRetryError.Error() should return original error message")
	}

	if retryErr.Unwrap() != originalErr {
		t.Errorf("CannotRetryError.Unwrap() should return original error")
	}
}

func TestFallbackTriggeredError(t *testing.T) {
	t.Parallel()

	err := &api.FallbackTriggeredError{
		OriginalModel: "opus",
		FallbackModel: "sonnet",
	}

	expected := "model fallback triggered: opus -> sonnet"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestCategorizeRetryableAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err       error
		expectCat string
	}{
		{&api.APIError{StatusCode: 529}, "rate_limit"},
		{&api.APIError{StatusCode: 429}, "rate_limit"},
		{&api.APIError{StatusCode: 401}, "authentication_failed"},
		{&api.APIError{StatusCode: 403}, "authentication_failed"},
		{&api.APIError{StatusCode: 500}, "server_error"},
		{&api.APIError{StatusCode: 502}, "server_error"},
		{&api.APIError{StatusCode: 400}, "unknown"},
	}

	for _, tc := range tests {
		cat := api.CategorizeRetryableAPIError(tc.err)
		if cat != tc.expectCat {
			t.Errorf("CategorizeRetryableAPIError(%v) = %q, want %q", tc.err, cat, tc.expectCat)
		}
	}
}
