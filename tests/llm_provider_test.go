package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"claude-go/internal/provider"
)

func TestOpenAIProviderCreation(t *testing.T) {
	t.Parallel()

	p := provider.NewOpenAIProvider(provider.OpenAIConfig{
		APIKey: "test-key",
		Model:  "gpt-4",
	})

	if p == nil {
		t.Fatal("NewOpenAIProvider returned nil")
	}

	if p.Name() != "openai" {
		t.Errorf("Name should be 'openai', got %s", p.Name())
	}

	if p.Model() != "gpt-4" {
		t.Errorf("Model should be 'gpt-4', got %s", p.Model())
	}
}

func TestOpenAIProviderCustomBaseURL(t *testing.T) {
	t.Parallel()

	p := provider.NewOpenAIProvider(provider.OpenAIConfig{
		APIKey:  "test-key",
		Model:   "gpt-4",
		BaseURL: "https://custom.api.com/v1",
	})

	if p.Model() != "gpt-4" {
		t.Errorf("Model should be 'gpt-4', got %s", p.Model())
	}

	// Test SetModel and SetBaseURL
	p.SetModel("gpt-3.5-turbo")
	if p.Model() != "gpt-3.5-turbo" {
		t.Errorf("Model should be 'gpt-3.5-turbo', got %s", p.Model())
	}

	p.SetBaseURL("https://another.api.com/v1")
}

func TestOpenAIProviderWithClient(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 60 * time.Second}
	p := provider.NewOpenAIProviderWithClient(provider.OpenAIConfig{
		APIKey: "test-key",
		Model:  "gpt-4",
	}, client)

	if p == nil {
		t.Fatal("NewOpenAIProviderWithClient returned nil")
	}
}

func TestProviderFactory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		providerType provider.ProviderType
		name         string
		model        string
	}{
		{provider.ProviderOpenAI, "openai", "gpt-4"},
		{provider.ProviderSimple, "simple", "simple-model"},
	}

	for _, tc := range tests {
		p := provider.CreateProvider(tc.providerType, "test-key", tc.model, "https://api.openai.com/v1")

		if p == nil {
			t.Errorf("CreateProvider(%s) returned nil", tc.name)
		}

		if p.Model() != tc.model {
			t.Errorf("Model mismatch for %s: got %s, want %s", tc.name, p.Model(), tc.model)
		}
	}
}

func TestSimpleProviderWithResponses(t *testing.T) {
	t.Parallel()

	responses := map[string]string{
		"hello": "Hello! How can I help you today?",
		"help":  "I can help you with various tasks.",
	}

	p := provider.NewSimpleProviderWithResponses("test", "model", responses)

	result, err := p.Complete(context.Background(), []provider.Message{
		{Role: "user", Content: "hello there"},
	})

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if result.Content != "Hello! How can I help you today?" {
		t.Errorf("Wrong response: %s", result.Content)
	}

	// Test help keyword
	result, _ = p.Complete(context.Background(), []provider.Message{
		{Role: "user", Content: "can you help me?"},
	})

	if result.Content != "I can help you with various tasks." {
		t.Errorf("Wrong help response: %s", result.Content)
	}
}

func TestSimpleProviderStream(t *testing.T) {
	t.Parallel()

	p := provider.NewSimpleProvider("test", "model")

	ch, err := p.CompleteStream(context.Background(), []provider.Message{
		{Role: "user", Content: "Hello"},
	})

	if err != nil {
		t.Fatalf("CompleteStream failed: %v", err)
	}

	var content string
	for resp := range ch {
		content += resp.Content
		if resp.Done {
			break
		}
	}

	if content == "" {
		t.Error("Streamed content should not be empty")
	}
}

func TestMapFinishReason(t *testing.T) {
	t.Parallel()

	// Test through SimpleProvider that responses are correctly formed
	p := provider.NewSimpleProvider("test", "model")
	result, _ := p.Complete(context.Background(), []provider.Message{
		{Role: "user", Content: "test"},
	})

	if result.StopReason != "end_turn" {
		t.Errorf("StopReason should be 'end_turn', got %s", result.StopReason)
	}
}

func TestProviderInterface(t *testing.T) {
	t.Parallel()

	var p provider.Provider
	p = provider.NewSimpleProvider("test", "model")

	// Verify interface compliance
	if p.Name() != "test" {
		t.Errorf("Name mismatch: got %s", p.Name())
	}

	if p.Model() != "model" {
		t.Errorf("Model mismatch: got %s", p.Model())
	}

	_, err := p.Complete(context.Background(), nil)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	_, err = p.CompleteStream(context.Background(), nil)
	if err != nil {
		t.Fatalf("CompleteStream failed: %v", err)
	}
}

func TestUsageStruct(t *testing.T) {
	t.Parallel()

	usage := provider.Usage{
		InputTokens:  100,
		OutputTokens: 50,
		TotalTokens:  150,
	}

	if usage.InputTokens != 100 {
		t.Errorf("InputTokens mismatch: got %d", usage.InputTokens)
	}

	if usage.OutputTokens != 50 {
		t.Errorf("OutputTokens mismatch: got %d", usage.OutputTokens)
	}

	if usage.TotalTokens != 150 {
		t.Errorf("TotalTokens mismatch: got %d", usage.TotalTokens)
	}
}

func TestProviderTypeConstants(t *testing.T) {
	t.Parallel()

	if provider.ProviderOpenAI != "openai" {
		t.Errorf("ProviderOpenAI should be 'openai'")
	}

	if provider.ProviderSimple != "simple" {
		t.Errorf("ProviderSimple should be 'simple'")
	}
}

func TestBaseURLPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "normal URL",
			input: "https://api.openai.com/v1",
		},
		{
			name:  "custom URL",
			input: "https://code.coolyeah.net/v1",
		},
		{
			name:  "URL with trailing slash",
			input: "https://api.openai.com/v1/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := provider.NewOpenAIProvider(provider.OpenAIConfig{
				APIKey:  "test-key",
				Model:   "gpt-4",
				BaseURL: tc.input,
			})

			if p == nil {
				t.Fatal("NewOpenAIProvider returned nil")
			}
		})
	}
}
