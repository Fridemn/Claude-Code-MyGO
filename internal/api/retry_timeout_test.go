package api

import (
	"fmt"
	"net/url"
	"testing"
)

func TestStreamingRetryableCheckerRetriesTimeouts(t *testing.T) {
	t.Parallel()

	err := &APIError{
		StatusCode: 408,
		Type:       ErrorTypeTimeout,
		Message:    `Post "https://code.coolyeah.net/v1/chat/completions": net/http: TLS handshake timeout`,
		Cause: &url.Error{
			Op:  "Post",
			URL: "https://code.coolyeah.net/v1/chat/completions",
			Err: fmt.Errorf("net/http: TLS handshake timeout"),
		},
	}

	if !IsTimeoutError(err) {
		t.Fatal("408 TLS handshake timeout should be recognized as timeout")
	}
	if !StreamingRetryableChecker(err) {
		t.Fatal("streaming requests should retry 408 TLS handshake timeouts")
	}
}

func TestRetryableCheckersUnwrapAPIError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("stream read failed before first chunk: %w", WrapAsAPIError(&url.Error{
		Op:  "Post",
		URL: "https://code.coolyeah.net/v1/chat/completions",
		Err: timeoutError{},
	}, 0, ErrorTypeSSL))

	if !IsTimeoutError(err) {
		t.Fatal("wrapped API timeout cause should be recognized")
	}
	if !StreamingRetryableChecker(err) {
		t.Fatal("wrapped API timeout cause should be retryable for streaming")
	}
}

func TestRetryableCheckersUnwrapAPIErrorStatus(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("stream read failed before first chunk: %w", &APIError{
		StatusCode: 408,
		Type:       ErrorTypeTimeout,
		Message:    "request timed out",
	})

	if !IsTimeoutError(err) {
		t.Fatal("wrapped API timeout should be recognized")
	}
	if !StreamingRetryableChecker(err) {
		t.Fatal("wrapped API timeout should be retryable for streaming")
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "net/http: TLS handshake timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
