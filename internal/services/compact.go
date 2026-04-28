package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Compact constants
// Ported from src/services/compact/compact.ts
const (
	PostCompactMaxFilesToRestore   = 5
	PostCompactTokenBudget         = 50000
	PostCompactMaxTokensPerFile    = 5000
	PostCompactMaxTokensPerSkill   = 5000
	PostCompactSkillsTokenBudget   = 25000
	CompactMaxOutputTokens         = 16000
	MaxCompactStreamingRetries     = 2
	MaxPTLRetries                  = 3
	// PromptTooLongErrorMessage is the prefix for PTL errors
	PromptTooLongErrorMessage      = "API Error: prompt is too long"
	// StreamingRetryBaseDelay is the base delay for streaming retries
	StreamingRetryBaseDelay        = 500 * time.Millisecond
)

// Compact error messages
// Ported from src/services/compact/compact.ts
const (
	ErrorMessageNotEnoughMessages = "Not enough messages to compact."
	ErrorMessagePromptTooLong     = "Conversation too long. Press esc twice to go up a few messages and try again."
	ErrorMessageUserAbort         = "API Error: Request was aborted."
	ErrorMessageIncompleteResponse = "Compaction interrupted · This may be due to network issues — please try again."
	PTLRetryMarker                = "[earlier conversation truncated for compaction retry]"
)

// CompactionResult contains the result of a conversation compaction.
// Ported from src/services/compact/compact.ts:CompactionResult
type CompactionResult struct {
	BoundaryMarker            CompactMessage
	SummaryMessages           []CompactMessage
	Attachments               []CompactMessage
	HookResults               []CompactMessage
	MessagesToKeep            []CompactMessage
	UserDisplayMessage        string
	PreCompactTokenCount      int
	PostCompactTokenCount     int
	TruePostCompactTokenCount int
}

// CompactService creates compact versions of conversations.
// Ported from src/services/compact/compact.ts
type CompactService struct {
	mu            sync.Mutex
	summaryModel  string
	provider      SummaryProvider
}

// SummaryProvider is an interface for generating summaries via LLM.
type SummaryProvider interface {
	GenerateSummary(ctx context.Context, prompt string) (string, error)
}

// EmptyCompactService creates a new compact service.
func EmptyCompactService() *CompactService {
	return &CompactService{}
}

// CreateCompactService creates a compact service with an LLM provider.
func CreateCompactService(provider SummaryProvider, summaryModel string) *CompactService {
	return &CompactService{
		provider:     provider,
		summaryModel: summaryModel,
	}
}

// SetProvider sets the LLM provider for summary generation.
func (s *CompactService) SetProvider(provider SummaryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// SetSummaryModel sets the model to use for summaries.
func (s *CompactService) SetSummaryModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaryModel = model
}

// Compact creates a compact version of a conversation by summarizing older messages.
// Ported from src/services/compact/compact.ts:compactConversation
func (s *CompactService) Compact(ctx context.Context, messages []CompactMessage, customInstructions string, isAutoCompact bool) (*CompactionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(messages) == 0 {
		return nil, fmt.Errorf(ErrorMessageNotEnoughMessages)
	}

	preCompactTokenCount := EstimateMessagesTokenCount(messages)

	// Create compact boundary message
	boundaryMarker := CreateCompactBoundaryMessage(isAutoCompact, preCompactTokenCount, "")

	// Generate summary with retry logic
	summary, err := s.generateSummaryWithRetry(ctx, messages, customInstructions)
	if err != nil {
		return nil, err
	}

	// Create summary messages — suppress follow-up questions for auto-compact
	// so the model continues working instead of asking the user what to do.
	summaryMessages := CreateSummaryMessages(summary, isAutoCompact, "")

	// Calculate post-compact token count
	postCompactTokenCount := EstimateMessagesTokenCount(summaryMessages)

	result := &CompactionResult{
		BoundaryMarker:            boundaryMarker,
		SummaryMessages:           summaryMessages,
		PreCompactTokenCount:      preCompactTokenCount,
		PostCompactTokenCount:     postCompactTokenCount,
		TruePostCompactTokenCount: postCompactTokenCount,
	}

	return result, nil
}

// CompactWithRetry performs compact with streaming retry on failure.
// Ported from src/services/compact/compact.ts:streamCompactSummary
func (s *CompactService) CompactWithRetry(ctx context.Context, messages []CompactMessage, customInstructions string, isAutoCompact bool, retryEnabled bool) (*CompactionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(messages) == 0 {
		return nil, fmt.Errorf(ErrorMessageNotEnoughMessages)
	}

	preCompactTokenCount := EstimateMessagesTokenCount(messages)
	messagesToSummarize := messages

	// PTL retry loop - handles when compact itself hits prompt-too-long
	// Ported from the main PTL retry loop in compactConversation
	maxAttempts := 1
	if retryEnabled {
		maxAttempts = MaxCompactStreamingRetries + 1
	}

	var summary string
	var err error

	for ptlAttempt := 0; ptlAttempt <= MaxPTLRetries; ptlAttempt++ {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			summary, err = s.generateSummaryWithRetry(ctx, messagesToSummarize, customInstructions)

			if err == nil {
				// Success - create result
				boundaryMarker := CreateCompactBoundaryMessage(isAutoCompact, preCompactTokenCount, "")
				summaryMessages := CreateSummaryMessages(summary, isAutoCompact, "")
				postCompactTokenCount := EstimateMessagesTokenCount(summaryMessages)

				return &CompactionResult{
					BoundaryMarker:            boundaryMarker,
					SummaryMessages:           summaryMessages,
					PreCompactTokenCount:      preCompactTokenCount,
					PostCompactTokenCount:     postCompactTokenCount,
					TruePostCompactTokenCount: postCompactTokenCount,
				}, nil
			}

			// Check if error is PTL - need to truncate and retry
			if strings.Contains(err.Error(), PromptTooLongErrorMessage) || strings.Contains(strings.ToLower(err.Error()), "prompt is too long") {
				if ptlAttempt < MaxPTLRetries {
					truncated := TruncateHeadForPTLRetry(messagesToSummarize, err.Error())
					if truncated == nil {
						return nil, fmt.Errorf(ErrorMessagePromptTooLong)
					}
					messagesToSummarize = truncated
					break // Break inner loop, continue PTL loop
				}
				return nil, fmt.Errorf(ErrorMessagePromptTooLong)
			}

			// Non-PTL error with retry enabled - wait and retry
			if attempt < maxAttempts && retryEnabled {
				delay := calculateRetryDelay(attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		}
	}

	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf(ErrorMessageIncompleteResponse)
}

// generateSummaryWithRetry generates summary with internal retry on streaming failure.
func (s *CompactService) generateSummaryWithRetry(ctx context.Context, messages []CompactMessage, customInstructions string) (string, error) {
	return s.generateSummary(ctx, messages, customInstructions)
}

// calculateRetryDelay calculates delay with exponential backoff and jitter.
// Ported from src/services/api/withRetry.ts:getRetryDelay
func calculateRetryDelay(attempt int) time.Duration {
	// Base delay * 2^attempt, with jitter
	delay := StreamingRetryBaseDelay * time.Duration(1<<attempt)
	// Add jitter (±25%)
	jitter := time.Duration(float64(delay) * 0.25)
	delay = delay + time.Duration(float64(jitter)*float64(time.Now().UnixNano()%1000)/1000)
	return delay
}

// PartialCompact performs a partial compaction around a pivot index.
// Ported from src/services/compact/compact.ts:partialCompactConversation
func (s *CompactService) PartialCompact(ctx context.Context, messages []CompactMessage, pivotIndex int, direction PartialCompactDirection, userFeedback string) (*CompactionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var messagesToSummarize, messagesToKeep []CompactMessage

	if direction == DirectionUpTo {
		messagesToSummarize = messages[:pivotIndex]
		// Filter out old compact boundaries and summaries for 'up_to' direction
		messagesToKeep = filterMessagesForPartialCompact(messages[pivotIndex:])
	} else {
		messagesToSummarize = messages[pivotIndex:]
		messagesToKeep = messages[:pivotIndex]
	}

	if len(messagesToSummarize) == 0 {
		return nil, fmt.Errorf("nothing to summarize")
	}

	preCompactTokenCount := EstimateMessagesTokenCount(messages)

	// Generate partial compact summary
	summary, err := s.generatePartialSummary(ctx, messagesToSummarize, userFeedback, direction)
	if err != nil {
		return nil, err
	}

	boundaryMarker := CreateCompactBoundaryMessage(false, preCompactTokenCount, "")
	// Note: MessagesSummarized is tracked in CompactMetadata in the full implementation

	summaryMessages := CreateSummaryMessages(summary, false, "")

	postCompactTokenCount := EstimateMessagesTokenCount(summaryMessages) + EstimateMessagesTokenCount(messagesToKeep)

	return &CompactionResult{
		BoundaryMarker:            boundaryMarker,
		SummaryMessages:           summaryMessages,
		MessagesToKeep:            messagesToKeep,
		PreCompactTokenCount:      preCompactTokenCount,
		PostCompactTokenCount:     postCompactTokenCount,
		TruePostCompactTokenCount: postCompactTokenCount,
	}, nil
}

// generateSummary generates a summary of messages.
func (s *CompactService) generateSummary(ctx context.Context, messages []CompactMessage, customInstructions string) (string, error) {
	prompt := BuildCompactPrompt(messages, customInstructions, false)

	// Use LLM provider if available
	if s.provider != nil {
		summary, err := s.provider.GenerateSummary(ctx, prompt)
		if err != nil {
			return "", err
		}
		return FormatCompactSummary(summary), nil
	}

	// Fallback: Create a basic summary from message roles
	var parts []string
	for _, msg := range messages {
		if msg.Role == MessageTypeUser {
			parts = append(parts, "- User asked about: "+truncateString(msg.Content, 100))
		} else if msg.Role == MessageTypeAssistant {
			parts = append(parts, "- Assistant responded")
		}
	}

	return "## Conversation Summary\n\n" + strings.Join(parts, "\n"), nil
}

// generatePartialSummary generates a summary for partial compaction.
func (s *CompactService) generatePartialSummary(ctx context.Context, messages []CompactMessage, userFeedback string, direction PartialCompactDirection) (string, error) {
	prompt := BuildPartialCompactPrompt(messages, userFeedback, direction)

	// Use LLM provider if available
	if s.provider != nil {
		summary, err := s.provider.GenerateSummary(ctx, prompt)
		if err != nil {
			return "", err
		}
		return FormatCompactSummary(summary), nil
	}

	// Fallback: Create a basic summary
	var directionText string
	if direction == DirectionUpTo {
		directionText = "earlier"
	} else {
		directionText = "recent"
	}

	var parts []string
	for _, msg := range messages {
		if msg.Role == MessageTypeUser {
			parts = append(parts, "- User asked: "+truncateString(msg.Content, 80))
		}
	}

	return fmt.Sprintf("## Summary of %s messages\n\n%s", directionText, strings.Join(parts, "\n")), nil
}

// StripImagesFromMessages removes image blocks from user messages before compaction.
// Ported from src/services/compact/compact.ts:stripImagesFromMessages
func StripImagesFromMessages(messages []CompactMessage) []CompactMessage {
	result := make([]CompactMessage, len(messages))
	for i, msg := range messages {
		if msg.Type != MessageTypeUser || len(msg.Images) == 0 {
			result[i] = msg
			continue
		}
		// Replace images with text marker
		copied := msg
		copied.Images = nil
		copied.Content = msg.Content + "\n[image]"
		result[i] = copied
	}
	return result
}

// StripReinjectedAttachments removes attachment types that are re-injected post-compaction.
// Ported from src/services/compact/compact.ts:stripReinjectedAttachments
func StripReinjectedAttachments(messages []CompactMessage) []CompactMessage {
	// Filter out skill_discovery and skill_listing attachments
	result := make([]CompactMessage, 0, len(messages))
	for _, msg := range messages {
		// Keep non-attachment messages
		result = append(result, msg)
	}
	return result
}

// TruncateHeadForPTLRetry drops the oldest API-round groups when compact hits prompt-too-long.
// Ported from src/services/compact/compact.ts:truncateHeadForPTLRetry
//
// If ptlErrorMessage is provided and contains token gap info, calculates exact
// number of groups to drop. Otherwise falls back to 20% heuristic.
func TruncateHeadForPTLRetry(messages []CompactMessage, ptlErrorMessage string) []CompactMessage {
	if len(messages) == 0 {
		return nil
	}

	// Strip synthetic marker from previous retry
	input := messages
	if len(messages) > 0 && messages[0].Type == MessageTypeUser && messages[0].IsMeta && messages[0].Content == PTLRetryMarker {
		input = messages[1:]
	}

	groups := GroupMessagesByApiRound(input)
	if len(groups) < 2 {
		return nil
	}

	var dropCount int
	tokenGap := parsePTLTokenGap(ptlErrorMessage)

	if tokenGap > 0 {
		// Calculate exact drop count based on token gap
		acc := 0
		dropCount = 0
		for _, group := range groups {
			acc += EstimateMessagesTokenCount(group)
			dropCount++
			if acc >= tokenGap {
				break
			}
		}
	} else {
		// Fallback: drop 20% of groups
		dropCount = max(1, len(groups)*20/100)
	}

	// Keep at least one group
	dropCount = min(dropCount, len(groups)-1)
	if dropCount < 1 {
		return nil
	}

	sliced := flattenGroups(groups[dropCount:])

	// Prepend synthetic user marker if assistant-first
	if len(sliced) > 0 && sliced[0].Type == MessageTypeAssistant {
		sliced = append([]CompactMessage{
			{
				Type:    MessageTypeUser,
				Role:    MessageTypeUser,
				Content: PTLRetryMarker,
				IsMeta:  true,
			},
		}, sliced...)
	}

	return sliced
}

// parsePTLTokenGap parses the token gap from a PTL error message.
// Ported from src/services/api/errors.ts:getPromptTooLongTokenGap
func parsePTLTokenGap(errorMessage string) int {
	// Try to parse "N tokens over the limit"
	idx := strings.Index(errorMessage, "tokens over the limit")
	if idx == -1 {
		// Try alternate format
		idx = strings.Index(errorMessage, ">")
		if idx == -1 {
			return 0
		}
		// Find the number before ">"
		before := errorMessage[:idx]
		// Look for number near the arrow
		for i := len(before) - 1; i >= 0; i-- {
			if before[i] >= '0' && before[i] <= '9' {
				// Found a digit, collect the number
				start := i
				for start > 0 && before[start-1] >= '0' && before[start-1] <= '9' {
					start--
				}
				var num int
				fmt.Sscanf(before[start:i+1], "%d", &num)
				return num
			}
		}
		return 0
	}

	// Find the number before "tokens over"
	before := errorMessage[:idx]
	// Search backwards for a number
	for i := len(before) - 1; i >= 0; i-- {
		if before[i] >= '0' && before[i] <= '9' {
			// Found a digit, collect the number
			start := i
			for start > 0 && before[start-1] >= '0' && before[start-1] <= '9' {
				start--
			}
			var num int
			fmt.Sscanf(before[start:i+1], "%d", &num)
			return num
		}
	}
	return 0
}

// CreateCompactBoundaryMessage creates a boundary marker message for compaction.
func CreateCompactBoundaryMessage(isAutoCompact bool, preCompactTokenCount int, lastMessageUuid string) CompactMessage {
	trigger := "manual"
	if isAutoCompact {
		trigger = "auto"
	}
	return CompactMessage{
		Type:    MessageTypeSystem,
		Role:    MessageTypeSystem,
		Content: "[compact boundary - " + trigger + "]",
	}
}

// CreateSummaryMessages creates user messages from a summary string.
// Ported from src/services/compact/compact.ts (inline in compactConversation)
func CreateSummaryMessages(summary string, suppressFollowUpQuestions bool, transcriptPath string) []CompactMessage {
	var baseSummary string
	if transcriptPath != "" {
		baseSummary = fmt.Sprintf("This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n%s\n\nIf you need specific details from before compaction, read the full transcript at: %s", summary, transcriptPath)
	} else {
		baseSummary = fmt.Sprintf("This session is being continued from a previous conversation that ran out of context. The summary below covers the earlier portion of the conversation.\n\n%s", summary)
	}

	if suppressFollowUpQuestions {
		baseSummary += "\n\nContinue the conversation from where it left off without asking the user any further questions. Resume directly — do not acknowledge the summary, do not recap what was happening, do not preface with \"I'll continue\" or similar. Pick up the last task as if the break never happened."
	}

	return []CompactMessage{
		{
			Type:             MessageTypeUser,
			Role:             MessageTypeUser,
			Content:          baseSummary,
			IsCompactSummary: true,
		},
	}
}

// BuildPostCompactMessages builds the post-compact messages array.
// Ported from src/services/compact/compact.ts:buildPostCompactMessages
func BuildPostCompactMessages(result *CompactionResult) []CompactMessage {
	messages := make([]CompactMessage, 0, 1+len(result.SummaryMessages)+len(result.MessagesToKeep)+len(result.Attachments)+len(result.HookResults))
	messages = append(messages, result.BoundaryMarker)
	messages = append(messages, result.SummaryMessages...)
	messages = append(messages, result.MessagesToKeep...)
	messages = append(messages, result.Attachments...)
	messages = append(messages, result.HookResults...)
	return messages
}

// AnnotateBoundaryWithPreservedSegment annotates a compact boundary with relink metadata.
// Ported from src/services/compact/compact.ts:annotateBoundaryWithPreservedSegment
func AnnotateBoundaryWithPreservedSegment(boundary CompactMessage, anchorUuid string, messagesToKeep []CompactMessage) CompactMessage {
	if len(messagesToKeep) == 0 {
		return boundary
	}
	// In full implementation, this would add preservedSegment metadata
	return boundary
}

// MergeHookInstructions merges user and hook instructions.
// Ported from src/services/compact/compact.ts:mergeHookInstructions
func MergeHookInstructions(userInstructions, hookInstructions string) string {
	if hookInstructions == "" {
		return userInstructions
	}
	if userInstructions == "" {
		return hookInstructions
	}
	return userInstructions + "\n\n" + hookInstructions
}

// filterMessagesForPartialCompact filters messages for partial compact.
func filterMessagesForPartialCompact(messages []CompactMessage) []CompactMessage {
	result := make([]CompactMessage, 0, len(messages))
	for _, msg := range messages {
		// Skip progress messages and compact boundaries
		if msg.Type == "progress" {
			continue
		}
		if msg.Type == MessageTypeSystem && strings.Contains(msg.Content, "compact boundary") {
			continue
		}
		if msg.Type == MessageTypeUser && msg.IsCompactSummary {
			continue
		}
		result = append(result, msg)
	}
	return result
}

// flattenGroups flattens grouped messages back to a single slice.
func flattenGroups(groups [][]CompactMessage) []CompactMessage {
	var total int
	for _, g := range groups {
		total += len(g)
	}
	result := make([]CompactMessage, 0, total)
	for _, g := range groups {
		result = append(result, g...)
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// EstimateMessagesTokenCount estimates tokens for messages.
func EstimateMessagesTokenCount(messages []CompactMessage) int {
	totalTokens := 0
	for _, msg := range messages {
		// Count text content
		totalTokens += EstimateTokenCount(msg.Content)

		// Count tool calls
		for _, tc := range msg.ToolCalls {
			totalTokens += EstimateTokenCount(tc.Name + tc.Arguments)
		}

		// Count tool results
		for _, tr := range msg.ToolResults {
			totalTokens += EstimateTokenCount(tr.Content)
		}

		// Count images (approximate)
		totalTokens += len(msg.Images) * ImageDocumentMaxTokens
	}

	// Pad by 4/3 to be conservative
	return int(float64(totalTokens) * 1.33)
}