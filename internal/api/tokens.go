package api

import (
	"strings"
	"unicode"
)

// TokenEstimator provides token estimation utilities
type TokenEstimator struct {
	charsPerToken int
}

// CreateTokenEstimator creates a new token estimator
func CreateTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		charsPerToken: 4, // Rough approximation for English text
	}
}

// Estimate estimates the number of tokens in a string
func (te *TokenEstimator) Estimate(text string) int {
	if text == "" {
		return 0
	}

	// Count words and characters
	words := te.countWords(text)
	chars := len(text)

	// Use a heuristic combining words and characters
	// This is a rough approximation - use float math for accuracy
	charTokens := float64(chars) / float64(te.charsPerToken)
	estimated := int((float64(words) + charTokens) / 2)

	// Minimum of 1 token for any non-empty text
	if estimated < 1 {
		estimated = 1
	}

	// Add overhead for special tokens, formatting
	overhead := 0
	if strings.Contains(text, "```") {
		overhead += 5 // Code blocks
	}
	if strings.Contains(text, "http") {
		overhead += te.countURLs(text) * 2 // URLs
	}

	return estimated + overhead
}

// EstimateMessages estimates tokens for a slice of messages
func (te *TokenEstimator) EstimateMessages(messages []ChatMessage) int {
	total := 0

	for _, msg := range messages {
		// Add role overhead
		total += 4 // Every message has ~4 tokens overhead

		// Add content
		switch content := msg.Content.(type) {
		case string:
			total += te.Estimate(content)
		case []ContentPart:
			for _, part := range content {
				if part.Type == "text" {
					total += te.Estimate(part.Text)
				} else if part.Type == "image_url" {
					// Images use tokens based on detail level
					switch part.ImageURL.Detail {
					case "low":
						total += 85
					case "high":
						total += 1105
					default: // "auto"
						total += 85
					}
				}
			}
		}

		// Add name overhead
		if msg.Name != "" {
			total += te.Estimate(msg.Name)
		}

		// Add tool call overhead
		for _, tc := range msg.ToolCalls {
			total += te.Estimate(tc.Function.Name)
			total += te.Estimate(tc.Function.Arguments)
			total += 4 // Tool call overhead
		}
	}

	// Add conversation overhead
	total += 3 // Start/end tokens

	return total
}

// countWords counts words in text
func (te *TokenEstimator) countWords(text string) int {
	count := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}

	return count
}

// countURLs counts URLs in text
func (te *TokenEstimator) countURLs(text string) int {
	count := 0
	for {
		idx := strings.Index(text, "http")
		if idx == -1 {
			break
		}
		count++

		// Find end of URL
		end := idx
		for end < len(text) && !unicode.IsSpace(rune(text[end])) {
			end++
		}
		text = text[end:]
	}
	return count
}

// TruncateToTokenLimit truncates text to fit within token limit
func (te *TokenEstimator) TruncateToTokenLimit(text string, maxTokens int) string {
	if te.Estimate(text) <= maxTokens {
		return text
	}

	// Binary search for the right length
	left, right := 0, len(text)
	for left < right {
		mid := (left + right + 1) / 2
		if te.Estimate(text[:mid]) <= maxTokens {
			left = mid
		} else {
			right = mid - 1
		}
	}

	// Find a good break point (space or newline)
	breakPoint := left
	for breakPoint > 0 && !unicode.IsSpace(rune(text[breakPoint-1])) {
		breakPoint--
	}
	if breakPoint == 0 {
		breakPoint = left
	}

	return text[:breakPoint]
}

// CanFitInContext checks if messages fit within context window
func (te *TokenEstimator) CanFitInContext(messages []ChatMessage, maxTokens int) bool {
	return te.EstimateMessages(messages) <= maxTokens
}

// CalculateAvailableTokens calculates available tokens for completion
func (te *TokenEstimator) CalculateAvailableTokens(messages []ChatMessage, maxContextTokens, maxCompletionTokens int) int {
	used := te.EstimateMessages(messages)
	available := maxContextTokens - used

	if available < 0 {
		return 0
	}

	if available > maxCompletionTokens {
		available = maxCompletionTokens
	}

	return available
}

// TokenUsage tracks token usage
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Add adds usage from another TokenUsage
func (tu *TokenUsage) Add(other TokenUsage) {
	tu.PromptTokens += other.PromptTokens
	tu.CompletionTokens += other.CompletionTokens
	tu.TotalTokens += other.TotalTokens
}

// Reset resets the token usage
func (tu *TokenUsage) Reset() {
	tu.PromptTokens = 0
	tu.CompletionTokens = 0
	tu.TotalTokens = 0
}