package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"claude-code-go/internal/engine"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/types"
)

// Complete implements engine.Provider interface
func (c *OpenAICompatibleClient) Complete(ctx context.Context, req engine.Request) (engine.Response, error) {
	// Convert engine.Request to ChatCompletionRequest
	chatReq := ChatCompletionRequest{
		Model:    firstNonEmptyString(req.Model, c.model),
		Messages: BuildMessagesFromTypes(req.Messages),
		Tools:    BuildToolsFromTypes(req.Tools),
	}

	// Call the internal completion method
	resp, err := c.doComplete(ctx, chatReq)
	if err != nil {
		return engine.Response{}, err
	}

	// Extract text from response
	var text string
	if len(resp.Choices) > 0 && resp.Choices[0].Message != nil {
		if content, ok := resp.Choices[0].Message.Content.(string); ok {
			text = content
		}
	}

	return engine.Response{
		Text:      text,
		ToolCalls: buildTypesToolCalls(resp),
	}, nil
}

// CompleteStream implements engine.StreamingProvider interface
// Uses streaming with idle timeout watchdog and non-streaming fallback for robustness
func (c *OpenAICompatibleClient) CompleteStream(ctx context.Context, req engine.Request, onChunk func(engine.StreamChunk) error) (engine.Response, error) {
	// Convert engine.Request to ChatCompletionRequest
	chatReq := ChatCompletionRequest{
		Model:    firstNonEmptyString(req.Model, c.model),
		Messages: BuildMessagesFromTypes(req.Messages),
		Tools:    BuildToolsFromTypes(req.Tools),
	}

	// Use streaming with watchdog and fallback
	fallbackConfig := DefaultNonStreamingFallbackConfig()
	result, err := c.CompleteStreamWithWatchdog(ctx, chatReq, func(chunk *StreamChunk) error {
		// Convert api.StreamChunk to engine.StreamChunk
		engineChunk := engine.StreamChunk{}

		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].Delta != nil {
				if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
					engineChunk.Text = content
				}
				if chunk.Choices[0].Delta.ToolCalls != nil {
					// Convert tool calls to tool.CallSpec
					engineChunk.ToolCalls = make([]tool.CallSpec, len(chunk.Choices[0].Delta.ToolCalls))
					for i, tc := range chunk.Choices[0].Delta.ToolCalls {
						// Parse arguments JSON into Input map
						var input tool.Input
						if tc.Function.Arguments != "" {
							if unmarshalErr := json.Unmarshal([]byte(tc.Function.Arguments), &input); unmarshalErr != nil {
								input = tool.Input{}
							}
						}
						engineChunk.ToolCalls[i] = tool.CallSpec{
							Name:  tc.Function.Name,
							Input: input,
							Raw:   tc.ID, // Use ID as Raw for tracking
						}
					}
				}
			}
			if chunk.Choices[0].FinishReason != nil {
				engineChunk.Done = true
			}
		}

		if onChunk != nil {
			return onChunk(engineChunk)
		}
		return nil
	}, fallbackConfig)

	if err != nil {
		return engine.Response{}, err
	}

	// Extract text from response
	var text string
	if result.Response != nil && len(result.Response.Choices) > 0 && result.Response.Choices[0].Message != nil {
		if content, ok := result.Response.Choices[0].Message.Content.(string); ok {
			text = content
		}
	}

	return engine.Response{
		Text:      text,
		ToolCalls: buildTypesToolCalls(result.Response),
	}, nil
}

// doComplete performs the actual completion (internal method) with enhanced retry
func (c *OpenAICompatibleClient) doComplete(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	cfg := DefaultRetryConfig()
	cfg.MaxRetries = c.maxRetries
	if c.retryDelay > 0 {
		cfg.BaseDelay = c.retryDelay
	}

	return RetryWithResult(ctx, cfg, func() (*ChatCompletionResponse, error) {
		return c.executeRequest(ctx, body)
	})
}

// executeRequest performs a single HTTP request
func (c *OpenAICompatibleClient) executeRequest(ctx context.Context, body []byte) (*ChatCompletionResponse, error) {
	respBody, err := c.doRequestWithContext(ctx, c.baseURL, body)
	if err != nil {
		return nil, err
	}

	var response ChatCompletionResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// Chat performs a chat completion (public API)
func (c *OpenAICompatibleClient) Chat(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	return c.doComplete(ctx, req)
}

// ChatStream performs a streaming chat completion (public API)
func (c *OpenAICompatibleClient) ChatStream(ctx context.Context, req ChatCompletionRequest, onChunk func(chunk *StreamChunk) error) (*ChatCompletionResponse, error) {
	return c.doCompleteStream(ctx, req, onChunk)
}

// doCompleteStream performs streaming completion (internal method) with enhanced retry
func (c *OpenAICompatibleClient) doCompleteStream(ctx context.Context, req ChatCompletionRequest, onChunk func(chunk *StreamChunk) error) (*ChatCompletionResponse, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	cfg := StreamingRetryConfig()
	cfg.MaxRetries = c.maxRetries
	if c.retryDelay > 0 {
		cfg.BaseDelay = c.retryDelay
	}

	return RetryWithResult(ctx, cfg, func() (*ChatCompletionResponse, error) {
		return c.tryCompleteStream(ctx, body, onChunk)
	})
}

// tryCompleteStream attempts a streaming request
func (c *OpenAICompatibleClient) tryCompleteStream(ctx context.Context, body []byte, onChunk func(chunk *StreamChunk) error) (*ChatCompletionResponse, error) {
	httpReq, err := createHTTPRequest(ctx, c.baseURL, body, c.apiKey)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, wrapConnectionError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := readAll(resp.Body)
		headers := ExtractHeadersFromResponse(resp)
		return nil, ParseErrorWithHeaders(resp.StatusCode, respBody, headers)
	}

	aggregated := &ChatCompletionResponse{
		Choices: []Choice{
			{Message: &ChatMessage{Role: "assistant", Content: ""}},
		},
	}

	scanner := bufio.NewScanner(resp.Body)
	receivedChunks := false
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || hasPrefix(line, ":") {
			continue
		}
		if !hasPrefix(line, "data: ") {
			continue
		}

		data := trimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		receivedChunks = true

		if onChunk != nil {
			if err := onChunk(&chunk); err != nil {
				return nil, err
			}
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
			if content, ok := chunk.Choices[0].Delta.Content.(string); ok {
				aggregated.Choices[0].Message.Content = aggregated.Choices[0].Message.Content.(string) + content
			}
			if chunk.Choices[0].Delta.ToolCalls != nil {
				aggregated.Choices[0].Message.ToolCalls = mergeToolCalls(
					aggregated.Choices[0].Message.ToolCalls,
					chunk.Choices[0].Delta.ToolCalls,
				)
			}
			if chunk.Choices[0].FinishReason != nil {
				aggregated.Choices[0].FinishReason = *chunk.Choices[0].FinishReason
			}
		}

		aggregated.ID = chunk.ID
		aggregated.Model = chunk.Model
	}

	if err := scanner.Err(); err != nil {
		if receivedChunks {
			return nil, wrapConnectionError(err)
		}
		return nil, fmt.Errorf("stream read failed before first chunk: %w", wrapConnectionError(err))
	}

	return aggregated, nil
}

// doRequestWithContext performs an HTTP request with proper error handling
func (c *OpenAICompatibleClient) doRequestWithContext(ctx context.Context, endpoint string, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, wrapConnectionError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapConnectionError(err)
	}

	if resp.StatusCode >= 300 {
		headers := ExtractHeadersFromResponse(resp)
		return nil, ParseErrorWithHeaders(resp.StatusCode, respBody, headers)
	}

	return respBody, nil
}

// wrapConnectionError wraps connection-related errors with detailed information
func wrapConnectionError(err error) error {
	if err == nil {
		return nil
	}

	// Check for context cancellation
	if err == context.Canceled {
		return err
	}
	if err == context.DeadlineExceeded {
		return WrapAsAPIError(err, 408, ErrorTypeTimeout)
	}

	// Check for URL errors (timeouts, etc.)
	if urlErr, ok := err.(*url.Error); ok {
		// Check if it's a timeout
		if urlErr.Timeout() {
			return WrapAsAPIError(err, 408, ErrorTypeTimeout)
		}

		// Check for TLS errors
		if strings.Contains(urlErr.Error(), "tls:") ||
			strings.Contains(urlErr.Error(), "certificate") ||
			strings.Contains(urlErr.Error(), "SSL") {
			return WrapAsAPIError(err, 0, ErrorTypeSSL)
		}

		// Check for connection refused
		if strings.Contains(urlErr.Error(), "connection refused") {
			return WrapAsAPIError(err, 0, ErrorTypeConnection)
		}

		// Other URL errors are connection errors
		return WrapAsAPIError(err, 0, ErrorTypeConnection)
	}

	// Check for net errors
	if netErr, ok := err.(*net.OpError); ok {
		// Check for timeout
		if netErr.Timeout() {
			return WrapAsAPIError(err, 408, ErrorTypeTimeout)
		}

		// Check for TLS handshake errors
		if strings.Contains(netErr.Error(), "tls:") {
			return WrapAsAPIError(err, 0, ErrorTypeSSL)
		}

		// Other network operation errors
		return WrapAsAPIError(err, 0, ErrorTypeConnection)
	}

	// Check for TLS certificate verification errors
	if _, ok := err.(*tls.CertificateVerificationError); ok {
		return WrapAsAPIError(err, 0, ErrorTypeSSL)
	}

	// Check error message for common patterns
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "tls handshake"):
		return WrapAsAPIError(err, 0, ErrorTypeSSL)
	case strings.Contains(errStr, "timeout"):
		return WrapAsAPIError(err, 408, ErrorTypeTimeout)
	case strings.Contains(errStr, "connection reset"):
		return WrapAsAPIError(err, 0, ErrorTypeConnection)
	case strings.Contains(errStr, "broken pipe"):
		return WrapAsAPIError(err, 0, ErrorTypeConnection)
	}

	return err
}

func buildTypesToolCalls(resp *ChatCompletionResponse) []types.ToolCall {
	if resp == nil || len(resp.Choices) == 0 || resp.Choices[0].Message == nil || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return nil
	}

	out := make([]types.ToolCall, 0, len(resp.Choices[0].Message.ToolCalls))
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		out = append(out, types.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return out
}

func mergeToolCalls(existing []ToolCall, deltas []ToolCall) []ToolCall {
	if len(deltas) == 0 {
		return existing
	}
	if existing == nil {
		existing = make([]ToolCall, 0, len(deltas))
	}

	for _, delta := range deltas {
		target := -1
		if delta.Index >= 0 && delta.Index < len(existing) {
			target = delta.Index
		} else if delta.ID != "" {
			for i := range existing {
				if existing[i].ID == delta.ID {
					target = i
					break
				}
			}
		}

		if target == -1 {
			existing = append(existing, delta)
			continue
		}

		if delta.ID != "" {
			existing[target].ID = delta.ID
		}
		if delta.Type != "" {
			existing[target].Type = delta.Type
		}
		if delta.Function.Name != "" {
			existing[target].Function.Name = delta.Function.Name
		}
		if delta.Function.Arguments != "" {
			existing[target].Function.Arguments += delta.Function.Arguments
		}
	}

	return existing
}

// Helper functions
func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// BuildMessagesFromTypes converts types.Message slice to ChatMessage slice
func BuildMessagesFromTypes(messages []types.Message) []ChatMessage {
	result := make([]ChatMessage, 0, len(messages))
	for _, msg := range messages {
		chatMsg := ChatMessage{
			Role: msg.Role,
		}

		// Handle different content types
		if len(msg.Images) > 0 {
			// Multimodal content
			parts := make([]ContentPart, 0, 1+len(msg.Images))
			parts = append(parts, ContentPart{
				Type: "text",
				Text: msg.Content,
			})
			for _, img := range msg.Images {
				parts = append(parts, ContentPart{
					Type: "image_url",
					ImageURL: &ImageURL{
						URL: img,
					},
				})
			}
			chatMsg.Content = parts
		} else {
			chatMsg.Content = msg.Content
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			chatMsg.ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				chatMsg.ToolCalls[i] = ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: FunctionCallData{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
		}

		// Handle tool results
		if msg.ToolCallID != "" {
			chatMsg.ToolCallID = msg.ToolCallID
		}

		if msg.Name != "" {
			chatMsg.Name = msg.Name
		}

		result = append(result, chatMsg)
	}
	return result
}

// BuildToolsFromTypes converts tool definitions
func BuildToolsFromTypes(tools []types.ToolDefinition) []ToolDefinition {
	result := make([]ToolDefinition, 0, len(tools))
	for _, t := range tools {
		result = append(result, ToolDefinition{
			Type: "function",
			Function: FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}

// Ensure OpenAICompatibleClient implements engine.Provider
var _ engine.Provider = (*OpenAICompatibleClient)(nil)

// Ensure OpenAICompatibleClient implements engine.StreamingProvider
var _ engine.StreamingProvider = (*OpenAICompatibleClient)(nil)

// ChatCompletionAdapter provides a bridge for tool support
type ChatCompletionAdapter struct {
	client *OpenAICompatibleClient
}

// ChatCompletionAdapter creates a new adapter
func CreateChatCompletionAdapter(client *OpenAICompatibleClient) *ChatCompletionAdapter {
	return &ChatCompletionAdapter{client: client}
}

// CompleteWithTools performs a completion with tool support
func (a *ChatCompletionAdapter) CompleteWithTools(ctx context.Context, messages []types.Message, tools []types.ToolDefinition) (*ChatCompletionResponse, error) {
	req := ChatCompletionRequest{
		Model:    a.client.model,
		Messages: BuildMessagesFromTypes(messages),
		Tools:    BuildToolsFromTypes(tools),
	}

	return a.client.Chat(ctx, req)
}

// CompleteStreamWithTools performs a streaming completion with tool support
func (a *ChatCompletionAdapter) CompleteStreamWithTools(ctx context.Context, messages []types.Message, tools []types.ToolDefinition, onChunk func(*StreamChunk) error) (*ChatCompletionResponse, error) {
	req := ChatCompletionRequest{
		Model:    a.client.model,
		Messages: BuildMessagesFromTypes(messages),
		Tools:    BuildToolsFromTypes(tools),
	}

	return a.client.ChatStream(ctx, req, onChunk)
}
