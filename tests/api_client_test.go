package tests

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"claude-go/internal/api"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/types"
)

func TestOpenAICompatibleClient_DecodeStandardChoices(t *testing.T) {
	t.Parallel()

	client := api.CreateOpenAICompatibleClientWithHTTPClient(config.Config{
		APIKey:  "test-key",
		BaseURL: "http://unit.test/v1/chat/completions",
		Model:   "test-model",
	}, stubHTTPClient(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"model":"test-model"`) {
			t.Fatalf("unexpected request body: %s", string(body))
		}
		return jsonResponse(http.StatusOK, `{"choices":[{"message":{"content":"hello from choices"}}]}`), nil
	}))
	resp, err := client.Complete(context.Background(), engine.Request{
		Messages: []types.Message{{Role: types.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resp.Text != "hello from choices" {
		t.Fatalf("unexpected response: %q", resp.Text)
	}
}

func TestOpenAICompatibleClient_DecodeFallbackPaths(t *testing.T) {
	t.Parallel()

	// Test that we handle a response with no message content gracefully
	client := api.CreateOpenAICompatibleClientWithHTTPClient(config.Config{
		APIKey:  "test-key",
		BaseURL: "http://unit.test/v1/chat/completions",
		Model:   "test-model",
	}, stubHTTPClient(func(r *http.Request) (*http.Response, error) {
		// Response with empty choices (edge case)
		return jsonResponse(http.StatusOK, `{"choices":[]}`), nil
	}))
	resp, err := client.Complete(context.Background(), engine.Request{})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	// Empty response should return empty string
	if resp.Text != "" {
		t.Fatalf("unexpected response: %q", resp.Text)
	}
}

func TestOpenAICompatibleClient_ProviderErrorIncludesBody(t *testing.T) {
	t.Parallel()

	client := api.CreateOpenAICompatibleClientWithHTTPClient(config.Config{
		APIKey:  "test-key",
		BaseURL: "http://unit.test/v1/chat/completions",
		Model:   "test-model",
	}, stubHTTPClient(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadGateway, "bad gateway"), nil
	}))
	client.SetMaxRetries(0)
	_, err := client.Complete(context.Background(), engine.Request{})
	if err == nil {
		t.Fatalf("expected provider error")
	}
	// The error message format is "API error 502: <message>"
	if !strings.Contains(err.Error(), "API error 502") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
