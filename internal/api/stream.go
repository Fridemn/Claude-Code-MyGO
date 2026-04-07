package api

import (
	"bufio"
	"io"
	"strings"
)

// StreamReader handles SSE (Server-Sent Events) streaming
type StreamReader struct {
	scanner *bufio.Scanner
	done    bool
}

// StreamReader creates a new stream reader
func CreateStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		scanner: bufio.NewScanner(r),
	}
}

// Read reads the next chunk from the stream
func (sr *StreamReader) Read() (data string, done bool, err error) {
	if sr.done {
		return "", true, nil
	}

	for sr.scanner.Scan() {
		line := sr.scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse data field
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if data == "[DONE]" {
				sr.done = true
				return "", true, nil
			}

			return data, false, nil
		}
	}

	if err := sr.scanner.Err(); err != nil {
		return "", false, err
	}

	sr.done = true
	return "", true, nil
}

// StreamCallback is called for each chunk in the stream
type StreamCallback func(chunk *StreamChunk) error

// StreamProcessor processes streaming responses
type StreamProcessor struct {
	onContent  func(content string)
	onToolCall func(toolCall ToolCall)
	onComplete func(finishReason string)
	onError    func(err error)
}

// StreamProcessor creates a new stream processor
func CreateStreamProcessor() *StreamProcessor {
	return &StreamProcessor{}
}

// OnContent sets the content callback
func (sp *StreamProcessor) OnContent(fn func(content string)) *StreamProcessor {
	sp.onContent = fn
	return sp
}

// OnToolCall sets the tool call callback
func (sp *StreamProcessor) OnToolCall(fn func(toolCall ToolCall)) *StreamProcessor {
	sp.onToolCall = fn
	return sp
}

// OnComplete sets the completion callback
func (sp *StreamProcessor) OnComplete(fn func(finishReason string)) *StreamProcessor {
	sp.onComplete = fn
	return sp
}

// OnError sets the error callback
func (sp *StreamProcessor) OnError(fn func(err error)) *StreamProcessor {
	sp.onError = fn
	return sp
}

// Process processes a stream chunk
func (sp *StreamProcessor) Process(chunk *StreamChunk) error {
	if chunk == nil || len(chunk.Choices) == 0 {
		return nil
	}

	choice := chunk.Choices[0]

	// Handle content delta
	if choice.Delta != nil {
		if content, ok := choice.Delta.Content.(string); ok && content != "" {
			if sp.onContent != nil {
				sp.onContent(content)
			}
		}

		// Handle tool calls
		if choice.Delta.ToolCalls != nil && sp.onToolCall != nil {
			for _, tc := range choice.Delta.ToolCalls {
				sp.onToolCall(tc)
			}
		}
	}

	// Handle completion
	if choice.FinishReason != nil && sp.onComplete != nil {
		sp.onComplete(*choice.FinishReason)
	}

	return nil
}

// Aggregator aggregates streaming content
type Aggregator struct {
	content   strings.Builder
	toolCalls []ToolCall
	usage     *Usage
}

// Aggregator creates a new aggregator
func CreateAggregator() *Aggregator {
	return &Aggregator{
		toolCalls: make([]ToolCall, 0),
	}
}

// AddChunk adds a chunk to the aggregator
func (a *Aggregator) AddChunk(chunk *StreamChunk) {
	if chunk == nil || len(chunk.Choices) == 0 {
		return
	}

	choice := chunk.Choices[0]

	if choice.Delta != nil {
		if content, ok := choice.Delta.Content.(string); ok {
			a.content.WriteString(content)
		}

		if choice.Delta.ToolCalls != nil {
			a.toolCalls = append(a.toolCalls, choice.Delta.ToolCalls...)
		}
	}
}

// GetContent returns aggregated content
func (a *Aggregator) GetContent() string {
	return a.content.String()
}

// GetToolCalls returns aggregated tool calls
func (a *Aggregator) GetToolCalls() []ToolCall {
	return a.toolCalls
}

// SetUsage sets the usage
func (a *Aggregator) SetUsage(usage Usage) {
	a.usage = &usage
}

// GetUsage returns the usage
func (a *Aggregator) GetUsage() *Usage {
	return a.usage
}

// ToMessage converts aggregated content to a message
func (a *Aggregator) ToMessage() ChatMessage {
	return ChatMessage{
		Role:      "assistant",
		Content:   a.content.String(),
		ToolCalls: a.toolCalls,
	}
}

// ToResponse converts aggregated content to a response
func (a *Aggregator) ToResponse() *ChatCompletionResponse {
	return &ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: &ChatMessage{
					Role:      "assistant",
					Content:   a.content.String(),
					ToolCalls: a.toolCalls,
				},
			},
		},
	}
}
