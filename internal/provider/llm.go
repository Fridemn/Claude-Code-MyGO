package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents an LLM response
type Response struct {
	Content    string `json:"content"`
	StopReason string `json:"stop_reason,omitempty"`
	Usage      Usage  `json:"usage,omitempty"`
}

// Usage represents token usage
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	Content    string `json:"content,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Done       bool   `json:"done"`
}

// Provider is the interface for LLM providers
type Provider interface {
	// Complete generates a completion for the given messages
	Complete(ctx context.Context, messages []Message) (*Response, error)
	// CompleteStream generates a streaming completion
	CompleteStream(ctx context.Context, messages []Message) (<-chan StreamResponse, error)
	// Name returns the provider name
	Name() string
	// Model returns the model name
	Model() string
}

// OpenAIProvider implements Provider for OpenAI-compatible APIs
type OpenAIProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	headers    map[string]string
}

// OpenAIConfig contains configuration for OpenAI provider
type OpenAIConfig struct {
	APIKey  string            // Required: API key
	BaseURL string            // Required: API base URL (e.g., "https://api.openai.com/v1")
	Model   string            // Required: model name (e.g., "gpt-4")
	Headers map[string]string // Optional: additional headers
	Timeout time.Duration     // Optional: request timeout
}

// ProviderType represents the type of provider
type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderSimple ProviderType = "simple"
)

// CreateProvider creates a provider based on type
func CreateProvider(providerType ProviderType, apiKey, model string, baseURL string) Provider {
	switch providerType {
	case ProviderSimple:
		return NewSimpleProvider("simple", model)
	default:
		return NewOpenAIProvider(OpenAIConfig{
			APIKey:  apiKey,
			BaseURL: baseURL,
			Model:   model,
		})
	}
}

// chatRequest represents an OpenAI chat completion request
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// chatResponse represents an OpenAI chat completion response
type chatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete generates a completion using OpenAI API
func (p *OpenAIProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	req := chatRequest{
		Model:    p.model,
		Messages: messages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var chatResp chatResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && chatResp.Error != nil {
			return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
		}
		// Provide user-friendly error messages for common status codes
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("API error 401: Invalid API key. Please check your API key configuration.")
		case 403:
			return nil, fmt.Errorf("API error 403: Permission denied. Your API key may not have access to this model.")
		case 404:
			return nil, fmt.Errorf("API error 404: Model or endpoint not found. Please check the model name and base URL.")
		case 429:
			return nil, fmt.Errorf("API error 429: Rate limit exceeded. Please wait a moment and try again.")
		default:
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := chatResp.Choices[0]
	return &Response{
		Content:    choice.Message.Content,
		StopReason: mapFinishReason(choice.FinishReason),
		Usage: Usage{
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
		},
	}, nil
}

// CompleteStream generates a streaming completion using OpenAI API
func (p *OpenAIProvider) CompleteStream(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	req := chatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		// Try to parse error response
		var chatResp chatResponse
		if err := json.Unmarshal(respBody, &chatResp); err == nil && chatResp.Error != nil {
			return nil, fmt.Errorf("API error: %s", chatResp.Error.Message)
		}
		// Provide user-friendly error messages for common status codes
		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("API error 401: Invalid API key. Please check your API key configuration.")
		case 403:
			return nil, fmt.Errorf("API error 403: Permission denied. Your API key may not have access to this model.")
		case 404:
			return nil, fmt.Errorf("API error 404: Model or endpoint not found. Please check the model name and base URL.")
		case 429:
			return nil, fmt.Errorf("API error 429: Rate limit exceeded. Please wait a moment and try again.")
		default:
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}
	}

	ch := make(chan StreamResponse, 100)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			// Check for stream end
			if data == "[DONE]" {
				ch <- StreamResponse{Done: true}
				return
			}

			var chatResp chatResponse
			if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
				continue
			}

			if chatResp.Error != nil {
				ch <- StreamResponse{
					Content:    fmt.Sprintf("Error: %s", chatResp.Error.Message),
					Done:       true,
				}
				return
			}

			if len(chatResp.Choices) > 0 {
				choice := chatResp.Choices[0]
				if choice.Delta.Content != "" {
					ch <- StreamResponse{Content: choice.Delta.Content}
				}
				if choice.FinishReason != "" {
					ch <- StreamResponse{
						StopReason: mapFinishReason(choice.FinishReason),
						Done:       true,
					}
				}
			}
		}

		ch <- StreamResponse{Done: true}
	}()

	return ch, nil
}

// setHeaders sets the required headers for the request
func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Set additional headers
	for key, value := range p.headers {
		req.Header.Set(key, value)
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Model returns the model name
func (p *OpenAIProvider) Model() string {
	return p.model
}

// SetModel sets the model name
func (p *OpenAIProvider) SetModel(model string) {
	p.model = model
}

// SetBaseURL sets the base URL (for switching providers)
func (p *OpenAIProvider) SetBaseURL(url string) {
	p.baseURL = url
}

// mapFinishReason maps OpenAI finish reason to standard format
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filtered"
	case "tool_calls":
		return "tool_use"
	default:
		return reason
	}
}

// =============================================================================
// SimpleProvider - Mock provider for testing
// =============================================================================

// SimpleProvider is a simple mock provider for testing
type SimpleProvider struct {
	name       string
	model      string
	responses  map[string]string // Predefined responses
}

// NewSimpleProvider creates a new simple provider
func NewSimpleProvider(name, model string) *SimpleProvider {
	return &SimpleProvider{
		name:  name,
		model: model,
		responses: map[string]string{
			"help": "I can help you with various tasks. What would you like to do?",
		},
	}
}

// NewSimpleProviderWithResponses creates a simple provider with custom responses
func NewSimpleProviderWithResponses(name, model string, responses map[string]string) *SimpleProvider {
	return &SimpleProvider{
		name:      name,
		model:     model,
		responses: responses,
	}
}

// Complete generates a simple completion (mock)
func (p *SimpleProvider) Complete(ctx context.Context, messages []Message) (*Response, error) {
	// Find last user message
	var lastUserMsg string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserMsg = messages[i].Content
			break
		}
	}

	// Check for predefined responses
	lowerMsg := strings.ToLower(lastUserMsg)
	for keyword, response := range p.responses {
		if strings.Contains(lowerMsg, keyword) {
			return &Response{
				Content:    response,
				StopReason: "end_turn",
				Usage: Usage{
					InputTokens:  countTokens(lastUserMsg),
					OutputTokens: countTokens(response),
				},
			}, nil
		}
	}

	// Default response
	response := "I understand you said: " + truncateString(lastUserMsg, 100)
	return &Response{
		Content:    response,
		StopReason: "end_turn",
		Usage: Usage{
			InputTokens:  countTokens(lastUserMsg),
			OutputTokens: countTokens(response),
		},
	}, nil
}

// CompleteStream generates a streaming response
func (p *SimpleProvider) CompleteStream(ctx context.Context, messages []Message) (<-chan StreamResponse, error) {
	ch := make(chan StreamResponse, 100)

	go func() {
		defer close(ch)

		response, err := p.Complete(ctx, messages)
		if err != nil {
			return
		}

		// Stream word by word
		words := strings.Fields(response.Content)
		for i, word := range words {
			select {
			case <-ctx.Done():
				return
			case ch <- StreamResponse{Content: word + " "}:
			}

			// Small delay for realistic streaming
			if i < len(words)-1 {
				time.Sleep(10 * time.Millisecond)
			}
		}

		ch <- StreamResponse{StopReason: response.StopReason, Done: true}
	}()

	return ch, nil
}

// Name returns the provider name
func (p *SimpleProvider) Name() string {
	return p.name
}

// Model returns the model name
func (p *SimpleProvider) Model() string {
	return p.model
}

// =============================================================================
// Helper functions
// =============================================================================

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// countTokens estimates token count (rough approximation)
func countTokens(s string) int {
	// Rough estimate: ~4 characters per token
	return (len(s) + 3) / 4
}

// =============================================================================
// Provider Factory
// =============================================================================

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(config OpenAIConfig) *OpenAIProvider {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &OpenAIProvider{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		headers: config.Headers,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// NewOpenAIProviderWithClient creates a provider with custom HTTP client
func NewOpenAIProviderWithClient(config OpenAIConfig, client *http.Client) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		model:      config.Model,
		headers:    config.Headers,
		httpClient: client,
	}
}
