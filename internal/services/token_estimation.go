package services

// Token estimation with file type and content type awareness.
// Ported from src/services/tokenEstimation.ts

import (
	"encoding/json"
	"strings"
)

// Token counting constants
const (
	// Default bytes per token ratio (conservative estimate)
	DefaultBytesPerToken = 4
	// Bytes per token for dense JSON files (many single-char tokens)
	JSONBytesPerToken = 2
	// Max tokens for images/documents (matches microCompact IMAGE_MAX_TOKEN_SIZE)
	ImageDocumentMaxTokens = 2000
	// Conservative padding factor (multiply by 4/3)
	TokenEstimationPadding = 1.33
)

// BytesPerTokenForFileType returns an estimated bytes-per-token ratio for a file extension.
// Dense JSON has many single-character tokens ({, }, :, ,, ") which makes the ratio closer to 2.
// Ported from src/services/tokenEstimation.ts:bytesPerTokenForFileType
func BytesPerTokenForFileType(fileExtension string) int {
	ext := strings.ToLower(strings.TrimPrefix(fileExtension, "."))
	switch ext {
	case "json", "jsonl", "jsonc":
		return JSONBytesPerToken
	default:
		return DefaultBytesPerToken
	}
}

// RoughTokenCountEstimation provides a rough estimate of token count for text.
// Uses the default bytes-per-token ratio.
// Ported from src/services/tokenEstimation.ts:roughTokenCountEstimation
func RoughTokenCountEstimation(content string) int {
	if len(content) == 0 {
		return 0
	}
	// Multiply first to avoid truncation, then apply padding
	return int(float64(len(content)) / DefaultBytesPerToken * TokenEstimationPadding)
}

// RoughTokenCountEstimationForFileType provides a more accurate estimate when file type is known.
// Ported from src/services/tokenEstimation.ts:roughTokenCountEstimationForFileType
func RoughTokenCountEstimationForFileType(content string, fileExtension string) int {
	if len(content) == 0 {
		return 0
	}
	bytesPerToken := BytesPerTokenForFileType(fileExtension)
	// Multiply first to avoid truncation, then apply padding
	return int(float64(len(content)) / float64(bytesPerToken) * TokenEstimationPadding)
}

// ContentBlock represents a content block for token estimation.
type ContentBlock struct {
	Type string // "text", "image", "document", "tool_use", "tool_result", "thinking", "redacted_thinking"
	// For text blocks
	Text string
	// For tool_use blocks
	Name  string
	Input map[string]any
	// For tool_result blocks (can have nested content)
	Content interface{} // string or []ContentBlock
	// For thinking blocks
	Thinking string
	// For redacted_thinking blocks
	Data string
}

// RoughTokenCountEstimationForBlock estimates tokens for a single content block.
// Ported from src/services/tokenEstimation.ts:roughTokenCountEstimationForBlock
func RoughTokenCountEstimationForBlock(block ContentBlock) int {
	switch block.Type {
	case "text":
		return RoughTokenCountEstimation(block.Text)

	case "image", "document":
		// Images/documents have a fixed max token cost
		// https://platform.claude.com/docs/en/build-with-claude/vision#calculate-image-costs
		// tokens = (width px * height px)/750, max 2000x2000 = 5333 tokens
		// Use conservative 2000 to match microCompact
		return ImageDocumentMaxTokens

	case "tool_use":
		// tool_use: input is JSON the model generated - can be arbitrarily large
		// Stringify for char count since API re-serializes anyway
		inputJSON, _ := json.Marshal(block.Input)
		return RoughTokenCountEstimation(block.Name + string(inputJSON))

	case "tool_result":
		return estimateContentValue(block.Content)

	case "thinking":
		return RoughTokenCountEstimation(block.Thinking)

	case "redacted_thinking":
		return RoughTokenCountEstimation(block.Data)

	default:
		// server_tool_use, web_search_tool_result, mcp_tool_use, etc.
		// Stringify-length tracks serialized form the API sees
		blockJSON, _ := json.Marshal(block)
		return RoughTokenCountEstimation(string(blockJSON))
	}
}

// estimateContentValue estimates tokens for content that can be string or array of blocks.
func estimateContentValue(content interface{}) int {
	if content == nil {
		return 0
	}

	switch v := content.(type) {
	case string:
		return RoughTokenCountEstimation(v)
	case []ContentBlock:
		total := 0
		for _, block := range v {
			total += RoughTokenCountEstimationForBlock(block)
		}
		return total
	case []interface{}:
		// Generic array - try to convert to ContentBlock
		total := 0
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				block := convertMapToBlock(m)
				total += RoughTokenCountEstimationForBlock(block)
			}
		}
		return total
	default:
		// Fallback: stringify
		contentJSON, _ := json.Marshal(content)
		return RoughTokenCountEstimation(string(contentJSON))
	}
}

// convertMapToBlock converts a map to ContentBlock.
func convertMapToBlock(m map[string]interface{}) ContentBlock {
	block := ContentBlock{}
	if t, ok := m["type"].(string); ok {
		block.Type = t
	}
	if text, ok := m["text"].(string); ok {
		block.Text = text
	}
	if name, ok := m["name"].(string); ok {
		block.Name = name
	}
	if input, ok := m["input"].(map[string]interface{}); ok {
		block.Input = input
	}
	if thinking, ok := m["thinking"].(string); ok {
		block.Thinking = thinking
	}
	if data, ok := m["data"].(string); ok {
		block.Data = data
	}
	if content, ok := m["content"]; ok {
		block.Content = content
	}
	return block
}

// RoughTokenCountEstimationForMessage estimates tokens for a message with content blocks.
// Ported from src/services/tokenEstimation.ts:roughTokenCountEstimationForMessage
func RoughTokenCountEstimationForMessage(content interface{}) int {
	return estimateContentValue(content)
}

// EstimateTokenCount provides a simple token count estimate for plain text.
// This is the legacy function from the original implementation.
func EstimateTokenCount(text string) int {
	return RoughTokenCountEstimation(text)
}

// EstimateToolResultTokens estimates tokens for a tool result.
// Ported from original token_estimation.go
func EstimateToolResultTokens(result ToolResultContent) int {
	if result.Content == "" {
		return 0
	}
	tokens := EstimateTokenCount(result.Content)

	// Add tokens for any images/documents in the result
	for range result.Images {
		tokens += ImageDocumentMaxTokens
	}
	for range result.Documents {
		tokens += ImageDocumentMaxTokens
	}

	return tokens
}