package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"claude-go/internal/config"
)

func TestCompleteStreamWithWatchdogRetriesRateLimitBeforeFallback(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt <= 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":{"type":"rate_limit_error","message":"该模型当前访问量过大，请您稍后再试"}}`)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"id\":\"ok\",\"model\":\"glm-5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := CreateOpenAICompatibleClientWithHTTPClient(config.Config{
		APIKey:  "key",
		BaseURL: server.URL,
		Model:   "glm-5",
	}, server.Client())
	client.SetMaxRetries(3)
	client.SetRetryDelay(time.Millisecond)

	result, err := client.CompleteStreamWithWatchdog(context.Background(), ChatCompletionRequest{
		Model: "glm-5",
		Messages: []ChatMessage{
			{Role: "user", Content: "hello"},
		},
	}, nil, NonStreamingFallbackConfig{Enabled: false})
	if err != nil {
		t.Fatalf("expected retry to recover from 429, got %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 streaming attempts, got %d", attempts.Load())
	}
	if result == nil || result.Response == nil || result.UsedFallback {
		t.Fatalf("expected streaming result without fallback, got %#v", result)
	}
}
