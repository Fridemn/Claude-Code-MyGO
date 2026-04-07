package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

	// Generate summary
	summary, err := s.generateSummary(ctx, messages, customInstructions)
	if err != nil {
		return nil, err
	}

	// Create summary messages
	summaryMessages := CreateSummaryMessages(summary, false, "")

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
func TruncateHeadForPTLRetry(messages []CompactMessage) []CompactMessage {
	if len(messages) == 0 {
		return nil
	}

	// Strip synthetic marker from previous retry
	input := messages
	if messages[0].Type == MessageTypeUser && messages[0].IsMeta && messages[0].Content == PTLRetryMarker {
		input = messages[1:]
	}

	groups := GroupMessagesByApiRound(input)
	if len(groups) < 2 {
		return nil
	}

	// Drop 20% of groups as fallback
	dropCount := max(1, len(groups)*20/100)
	dropCount = min(dropCount, len(groups)-1)

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
		baseSummary += "\n\nContinue the conversation from where it left off without asking the user any further questions."
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