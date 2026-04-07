package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-code-go/internal/config"
	"claude-code-go/internal/constants"
)

// OpenAICompatibleClient implements an OpenAI-compatible API client
type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	client     *http.Client
	maxRetries int
	retryDelay time.Duration
}

// CreateOpenAICompatibleClient creates a new OpenAI-compatible client
func CreateOpenAICompatibleClient(cfg config.Config) *OpenAICompatibleClient {
	return CreateOpenAICompatibleClientWithHTTPClient(cfg, &http.Client{
		Timeout: constants.DefaultRequestTimeoutSeconds * time.Second,
	})
}

// CreateOpenAICompatibleClientWithHTTPClient creates a client with custom HTTP client
func CreateOpenAICompatibleClientWithHTTPClient(cfg config.Config, client *http.Client) *OpenAICompatibleClient {
	if client == nil {
		client = &http.Client{Timeout: constants.DefaultRequestTimeoutSeconds * time.Second}
	}
	return &OpenAICompatibleClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		client:     client,
		maxRetries: DefaultMaxRetries,
		retryDelay: DefaultBaseDelay,
	}
}

// ChatCompletionRequest represents an OpenAI chat completion request
type ChatCompletionRequest struct {
	Model            string           `json:"model"`
	Messages         []ChatMessage    `json:"messages"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	Stream           bool             `json:"stream,omitempty"`
	Tools            []ToolDefinition `json:"tools,omitempty"`
	ToolChoice       any              `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat  `json:"response_format,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	User             string           `json:"user,omitempty"`
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ContentPart represents a part of message content (for multimodal)
type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL or base64 data
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// ToolDefinition represents a tool definition
type ToolDefinition struct {
	Type     string             `json:"type"` // "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition represents a function definition
type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string           `json:"id"`
	Index    int              `json:"index,omitempty"`
	Type     string           `json:"type"` // "function"
	Function FunctionCallData `json:"function"`
}

// FunctionCallData represents function call data
type FunctionCallData struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseFormat specifies the response format
type ResponseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// ChatCompletionResponse represents the API response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int          `json:"index"`
	Message      *ChatMessage `json:"message"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamChunk represents a streaming response chunk
type StreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int          `json:"index"`
		Delta        *ChatMessage `json:"delta"`
		FinishReason *string      `json:"finish_reason"`
	} `json:"choices"`
}

// SetMaxRetries sets the maximum number of retries
func (c *OpenAICompatibleClient) SetMaxRetries(max int) {
	c.maxRetries = max
}

// SetRetryDelay sets the retry delay
func (c *OpenAICompatibleClient) SetRetryDelay(delay time.Duration) {
	c.retryDelay = delay
}

// GetModel returns the model name
func (c *OpenAICompatibleClient) GetModel() string {
	return c.model
}

// SetModel sets the model name
func (c *OpenAICompatibleClient) SetModel(model string) {
	c.model = model
}

// SetTimeout sets the HTTP client timeout
func (c *OpenAICompatibleClient) SetTimeout(timeout time.Duration) {
	if c.client != nil {
		c.client.Timeout = timeout
	}
}

// BuildMessages converts types.Message slice to ChatMessage slice
func BuildMessages(messages []any) []ChatMessage {
	result := make([]ChatMessage, 0)
	for _, msg := range messages {
		switch m := msg.(type) {
		case ChatMessage:
			result = append(result, m)
		case map[string]any:
			chatMsg := ChatMessage{}
			if role, ok := m["role"].(string); ok {
				chatMsg.Role = role
			}
			if content, ok := m["content"].(string); ok {
				chatMsg.Content = content
			}
			if name, ok := m["name"].(string); ok {
				chatMsg.Name = name
			}
			result = append(result, chatMsg)
		}
	}
	return result
}

// BuildTools converts tool definitions
func BuildTools(tools []any) []ToolDefinition {
	result := make([]ToolDefinition, 0)
	for _, t := range tools {
		switch tool := t.(type) {
		case ToolDefinition:
			result = append(result, tool)
		case map[string]any:
			if fn, ok := tool["function"].(map[string]any); ok {
				td := ToolDefinition{Type: "function"}
				td.Function.Name, _ = fn["name"].(string)
				td.Function.Description, _ = fn["description"].(string)
				td.Function.Parameters, _ = fn["parameters"].(map[string]any)
				result = append(result, td)
			}
		}
	}
	return result
}

// Helper functions for adapter
func createHTTPRequest(ctx context.Context, url string, body []byte, apiKey string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	return req, nil
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func trimPrefix(s, prefix string) string {
	return strings.TrimPrefix(s, prefix)
}