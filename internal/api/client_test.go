package api

import (
	"testing"

	"claude-go/internal/config"
)

func TestClientSanitizesEndpointConfig(t *testing.T) {
	client := CreateOpenAICompatibleClient(config.Config{
		APIKey:  "\x00secret\r",
		BaseURL: "\x00https://api.example.com/v1/chat/completions/\r",
		Model:   "\x00glm-5\n",
	})

	if client.baseURL != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("expected sanitized base URL, got %q", client.baseURL)
	}
	if client.apiKey != "secret" {
		t.Fatalf("expected sanitized api key, got %q", client.apiKey)
	}
	if client.model != "glm-5" {
		t.Fatalf("expected sanitized model, got %q", client.model)
	}
}
