package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"claude-go/internal/api"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/types"
)

func TestOpenAICompatibleClient_Complete(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Expected Bearer token")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected JSON content type")
		}

		// Send response
		resp := api.ChatCompletionResponse{
			ID:      "test-id",
			Object:  "chat.completion",
			Model:   "test-model",
			Created: 1234567890,
			Choices: []api.Choice{
				{
					Index: 0,
					Message: &api.ChatMessage{
						Role:    "assistant",
						Content: "Hello, world!",
					},
					FinishReason: "stop",
				},
			},
			Usage: api.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)

	// Make request using engine.Provider interface
	req := engine.Request{
		Model: "test-model",
		Messages: []types.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify response
	if resp.Text != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", resp.Text)
	}
}

func TestBuildMessagesFromTypes_FiltersProgressAndTranscriptOnlyMessages(t *testing.T) {
	t.Parallel()

	converted := api.BuildMessagesFromTypes([]types.Message{
		{
			Role:    types.RoleUser,
			Content: "visible",
		},
		{
			Type:       types.MessageTypeProgress,
			Role:       types.RoleSystem,
			Content:    `{"type":"repl_tool_call","phase":"start","toolName":"REPL"}`,
			ToolCallID: "repl-1",
		},
		{
			Role:                      types.RoleSystem,
			Type:                      types.SystemSubtypeLocalCommand,
			Content:                   "<local-command-stdout>hidden</local-command-stdout>",
			IsVisibleInTranscriptOnly: true,
		},
	})

	if len(converted) != 1 {
		t.Fatalf("expected only one API message, got %d", len(converted))
	}
	if converted[0].Role != types.RoleUser || converted[0].Content != "visible" {
		t.Fatalf("unexpected converted message: %#v", converted[0])
	}
}

func TestOpenAICompatibleClient_Chat(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := api.ChatCompletionResponse{
			ID:    "test-id",
			Model: "test-model",
			Choices: []api.Choice{
				{
					Index: 0,
					Message: &api.ChatMessage{
						Role:    "assistant",
						Content: "Test response",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)

	req := api.ChatCompletionRequest{
		Model: "test-model",
		Messages: []api.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok || content != "Test response" {
		t.Errorf("Expected 'Test response', got %v", resp.Choices[0].Message.Content)
	}
}

func TestOpenAICompatibleClient_ChatStream(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Get flusher for streaming
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Streaming not supported")
			return
		}

		// Send chunks
		chunks := []string{
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	// Create client
	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)

	// Track received content
	var receivedContent string

	// Make request
	req := api.ChatCompletionRequest{
		Model: "test-model",
		Messages: []api.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := client.ChatStream(context.Background(), req, func(chunk *api.StreamChunk) error {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				receivedContent += content
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStream failed: %v", err)
	}

	// Verify content was received via callback
	if receivedContent != "Hello world!" {
		t.Errorf("Expected 'Hello world!', got '%s'", receivedContent)
	}

	// Verify aggregated response
	if len(resp.Choices) == 0 {
		t.Fatal("Expected at least one choice")
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok || content != "Hello world!" {
		t.Errorf("Expected 'Hello world!', got %v", resp.Choices[0].Message.Content)
	}
}

func TestOpenAICompatibleClient_ErrorHandling(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "bad-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)

	req := engine.Request{
		Model: "test-model",
		Messages: []types.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	_, err := client.Complete(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if !api.IsAuthenticationError(err) {
		t.Errorf("Expected authentication error, got %v", err)
	}
}

func TestOpenAICompatibleClient_CompleteRetries429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":{"message":"The current model is experiencing high demand","type":"rate_limit_error"}}`))
			return
		}

		resp := api.ChatCompletionResponse{
			ID:    "test-id",
			Model: "test-model",
			Choices: []api.Choice{
				{
					Index: 0,
					Message: &api.ChatMessage{
						Role:    "assistant",
						Content: "Recovered after retry",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)
	client.SetRetryDelay(1 * time.Millisecond)

	resp, err := client.Complete(context.Background(), engine.Request{
		Model: "test-model",
		Messages: []types.Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("Complete failed after retry: %v", err)
	}
	if resp.Text != "Recovered after retry" {
		t.Fatalf("unexpected response text: %q", resp.Text)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestOpenAICompatibleClient_ChatStreamRetries429ThenSucceeds(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := attempts.Add(1)
		if current <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":{"message":"The current model is experiencing high demand","type":"rate_limit_error"}}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("streaming not supported")
			return
		}
		chunks := []string{
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"test","object":"chat.completion.chunk","created":1234567890,"model":"test-model","choices":[{"index":0,"delta":{"content":" retry"},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)
	client.SetRetryDelay(1 * time.Millisecond)

	var received string
	resp, err := client.ChatStream(context.Background(), api.ChatCompletionRequest{
		Model: "test-model",
		Messages: []api.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}, func(chunk *api.StreamChunk) error {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				received += content
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream failed after retry: %v", err)
	}
	if received != "Hello retry" {
		t.Fatalf("unexpected streamed text: %q", received)
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	if content != "Hello retry" {
		t.Fatalf("unexpected aggregated content: %q", content)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestOpenAICompatibleClient_CompleteFailsAfterMax429Retries(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":{"message":"The current model is experiencing high demand","type":"rate_limit_error"}}`))
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)
	client.SetMaxRetries(2)
	client.SetRetryDelay(1 * time.Millisecond)

	_, err := client.Complete(context.Background(), engine.Request{
		Model: "test-model",
		Messages: []types.Message{
			{Role: "user", Content: "Hello"},
		},
	})
	if err == nil {
		t.Fatal("expected 429 failure after retry exhaustion")
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 API error, got %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 total attempts, got %d", got)
	}
}

func TestOpenAICompatibleClient_ChatStreamRetryWaitIsCancelable(t *testing.T) {
	var attempts atomic.Int32
	firstAttemptSeen := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			firstAttemptSeen <- struct{}{}
		}
		w.WriteHeader(http.StatusTooManyRequests)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":{"message":"The current model is experiencing high demand","type":"rate_limit_error"}}`))
	}))
	defer server.Close()

	cfg := config.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	}
	client := api.CreateOpenAICompatibleClient(cfg)
	client.SetMaxRetries(10)
	client.SetRetryDelay(2 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		_, err := client.ChatStream(ctx, api.ChatCompletionRequest{
			Model: "test-model",
			Messages: []api.ChatMessage{
				{Role: "user", Content: "Hello"},
			},
		}, func(chunk *api.StreamChunk) error {
			return nil
		})
		resultCh <- err
	}()

	select {
	case <-firstAttemptSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first retryable attempt was not observed")
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stream retry did not stop promptly after cancel")
	}

	if got := attempts.Load(); got != 1 {
		t.Fatalf("expected cancellation during first backoff, got %d attempts", got)
	}
}

func TestTokenEstimator(t *testing.T) {
	estimator := api.CreateTokenEstimator()

	tests := []struct {
		text     string
		minToken int
		maxToken int
	}{
		{"Hello, world!", 2, 10},
		{"", 0, 0},
		{"a", 1, 2},
		{"The quick brown fox jumps over the lazy dog.", 5, 15},
	}

	for _, tt := range tests {
		est := estimator.Estimate(tt.text)
		if est < tt.minToken || est > tt.maxToken {
			t.Errorf("Estimate(%q) = %d, want between %d and %d", tt.text, est, tt.minToken, tt.maxToken)
		}
	}
}

func TestIsRateLimitError(t *testing.T) {
	err := &api.APIError{StatusCode: 429, Message: "Rate limit exceeded"}
	if !api.IsRateLimitError(err) {
		t.Error("Expected rate limit error to be detected")
	}

	err = &api.APIError{StatusCode: 500, Message: "Server error"}
	if api.IsRateLimitError(err) {
		t.Error("Expected server error not to be rate limit")
	}
}

func TestIsServerError(t *testing.T) {
	codes := []int{500, 502, 503, 504}
	for _, code := range codes {
		err := &api.APIError{StatusCode: code, Message: "Server error"}
		if !api.IsServerError(err) {
			t.Errorf("Expected %d to be server error", code)
		}
	}
}
