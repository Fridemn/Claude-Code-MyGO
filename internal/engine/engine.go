package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-go/internal/config"
	"claude-go/internal/memory"
	"claude-go/internal/prompt"
	"claude-go/internal/session"
	"claude-go/internal/tool"
	"claude-go/internal/tool/mcp"
	"claude-go/internal/tool/repl"
	"claude-go/internal/types"
)

// isPromptTooLongError checks if an error is a prompt-too-long error
func isPromptTooLongError(err error) bool {
	if err == nil {
		return false
	}
	lowerMsg := strings.ToLower(err.Error())
	return strings.Contains(lowerMsg, "prompt is too long") ||
		strings.Contains(lowerMsg, "prompt exceeds max length") ||
		strings.Contains(lowerMsg, "input characters limit") ||
		strings.Contains(lowerMsg, "input character limit")
}

// CompactMessage represents a message for compaction (simplified interface)
type CompactMessage struct {
	UUID    string
	Type    string
	Role    string
	Content string
	Name    string
	ToolID  string
}

// CompactResult contains the result of compaction
type CompactResult struct {
	SummaryMessages       []CompactMessage
	PreCompactTokenCount  int
	PostCompactTokenCount int
}

// AutoCompactState tracks auto-compact state across turns
type AutoCompactState struct {
	Compacted              bool
	TurnCounter            int
	ConsecutiveFailures    int
	compactedThisTurn      bool // tracks if reactive compact was already attempted this turn
	firstRoundEmptyRetries int  // tracks retries for empty first response
}

// CompactService interface for dependency injection
type CompactService interface {
	Compact(ctx context.Context, messages []CompactMessage, customInstructions string, isAutoCompact bool) (*CompactResult, error)
}

// StreamChunk represents a chunk of streaming response
type StreamChunk struct {
	Text       string
	Done       bool
	ToolCalls  []tool.CallSpec
	ToolName   string // Current tool being executed, if any
	ToolCallID string
	ToolResult string
	Status     string
}

// Provider interface for LLM providers
type Provider interface {
	Complete(context.Context, Request) (Response, error)
}

// StreamingProvider extends Provider with streaming support
type StreamingProvider interface {
	Provider
	CompleteStream(ctx context.Context, req Request, onChunk func(StreamChunk) error) (Response, error)
}

type HookRunner interface {
	Trigger(ctx context.Context, event HookEvent) ([]HookExecution, error)
}

type HookEvent struct {
	Name    string
	Target  string
	Payload map[string]any
}

type HookExecution struct {
	Event               string
	Target              string
	Hook                string
	Command             string
	Blocking            bool
	Result              string
	Output              string
	Error               string
	DurationMs          int
	StopReason          string
	Timestamp           string
	Payload             map[string]any
	PreventContinuation bool // Set to true if hook returned {"continue": false}
}

type Options struct {
	Config          config.Config
	Provider        Provider
	Tools           *tool.Registry
	ToolRuntime     tool.Runtime
	Hooks           HookRunner
	Sessions        *session.Manager
	Transcripts     *session.EnhancedManager
	InitialMessages []types.Message
	SessionID       string
	OnStreamChunk   func(StreamChunk) error // Optional callback for streaming
	CompactService  CompactService
}

type Engine struct {
	cfg           config.Config
	provider      Provider
	tools         *tool.Registry
	runtime       tool.Runtime
	hooks         HookRunner
	session       *session.Session
	manager       *session.Manager
	transcripts   *session.EnhancedManager
	transcriptPos int
	onStreamChunk func(StreamChunk) error
	compact       CompactService
	autoCompact   *AutoCompactState
	// Tracks denied non-retryable tool calls within the current user turn
	// to avoid same-turn retry loops on blocked commands.
	nonRetryableFailures map[string]string
}

func Create(ctx context.Context, opts Options) (*Engine, error) {
	if opts.Provider == nil {
		return nil, fmt.Errorf("provider is required")
	}
	if opts.Tools == nil {
		return nil, fmt.Errorf("tool registry is required")
	}
	if opts.Sessions == nil {
		return nil, fmt.Errorf("session manager is required")
	}

	var (
		s   *session.Session
		err error
	)
	switch {
	case opts.SessionID != "":
		s, err = opts.Sessions.Load(opts.SessionID)
		if err != nil {
			s = opts.Sessions.CreateWithID(opts.SessionID)
		} else if opts.Transcripts != nil {
			// Load full conversation history from transcript (.jsonl)
			// Manager.Load only reads .json metadata; the actual messages
			// (including user/assistant turns) are in the transcript.
			transcriptMsgs, tErr := opts.Transcripts.ReadTranscript(opts.SessionID)
			if tErr == nil && len(transcriptMsgs) > 0 {
				s.Messages = transcriptMsgs
			}
		}
	case len(opts.InitialMessages) > 0:
		s, err = opts.Sessions.Create(ctx)
		if err != nil {
			return nil, err
		}
		s.Messages = append([]types.Message(nil), opts.InitialMessages...)
	default:
		s, err = opts.Sessions.Create(ctx)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	eng := &Engine{
		cfg:                  opts.Config,
		provider:             opts.Provider,
		tools:                opts.Tools,
		runtime:              opts.ToolRuntime,
		hooks:                opts.Hooks,
		session:              s,
		manager:              opts.Sessions,
		transcripts:          opts.Transcripts,
		transcriptPos:        len(s.Messages),
		onStreamChunk:        opts.OnStreamChunk,
		compact:              opts.CompactService,
		autoCompact:          &AutoCompactState{},
		nonRetryableFailures: map[string]string{},
	}

	// Trigger SessionStart hook if this is a new session
	if len(s.Messages) == 0 || (opts.SessionID == "" && len(opts.InitialMessages) == 0) {
		_ = eng.triggerHooks(ctx, HookEvent{
			Name:   string(types.HookEventSessionStart),
			Target: "create",
			Payload: map[string]any{
				"session_id": s.ID,
			},
		})
	}

	if system := prompt.WithTools(opts.Config.SystemPrompt, mcp.MergedDefinitions(opts.Tools, opts.ToolRuntime)); system != "" && len(s.Messages) == 0 {
		eng.session.Messages = append(eng.session.Messages, types.Message{
			Role:      types.RoleSystem,
			Content:   system,
			Timestamp: time.Now(),
		})
	}
	if err := eng.persistSession(); err != nil {
		return nil, err
	}
	return eng, nil
}

func (e *Engine) Submit(ctx context.Context, input string) (Response, error) {
	// Reset non-retryable cache for a new user turn. This preserves same-turn
	// loop protection but still allows users to retry a previously denied command
	// after giving new instructions.
	e.nonRetryableFailures = map[string]string{}

	if err := e.triggerHooks(ctx, HookEvent{
		Name:   string(types.HookEventUserPromptSubmit),
		Target: input,
		Payload: map[string]any{
			"input":   input,
			"session": e.session.ID,
		},
	}); err != nil {
		return Response{}, err
	}
	e.session.Messages = append(e.session.Messages, types.Message{
		Role:      types.RoleUser,
		Content:   input,
		Timestamp: time.Now(),
	})
	e.appendRelevantMemoriesAttachment(ctx, input)
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	e.trimHistory()

	resp, err := e.runLoop(ctx)
	if err != nil {
		_ = e.triggerHooks(ctx, HookEvent{
			Name:   string(types.HookEventStop),
			Target: input,
			Payload: map[string]any{
				"input":     input,
				"output":    "",
				"session":   e.session.ID,
				"messages":  len(e.session.Messages),
				"has_error": true,
				"error":     err.Error(),
			},
		})
		_ = e.persistSession()
		return Response{}, err
	}

	_ = e.triggerHooks(ctx, HookEvent{
		Name:   string(types.HookEventStop),
		Target: input,
		Payload: map[string]any{
			"input":     input,
			"output":    resp.Text,
			"session":   e.session.ID,
			"messages":  len(e.session.Messages),
			"has_error": false,
		},
	})
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// SubmitStream submits input and streams the response
func (e *Engine) SubmitStream(ctx context.Context, input string, onChunk func(StreamChunk) error) (Response, error) {
	// Reset non-retryable cache for a new user turn.
	e.nonRetryableFailures = map[string]string{}

	if err := e.triggerHooks(ctx, HookEvent{
		Name:   "pre_turn",
		Target: input,
		Payload: map[string]any{
			"input":   input,
			"session": e.session.ID,
		},
	}); err != nil {
		return Response{}, err
	}
	e.session.Messages = append(e.session.Messages, types.Message{
		Role:      types.RoleUser,
		Content:   input,
		Timestamp: time.Now(),
	})
	e.appendRelevantMemoriesAttachment(ctx, input)
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	e.trimHistory()

	resp, err := e.runLoopStream(ctx, onChunk)
	if err != nil {
		_ = e.triggerHooks(ctx, HookEvent{
			Name:   "post_turn",
			Target: input,
			Payload: map[string]any{
				"input":     input,
				"output":    "",
				"session":   e.session.ID,
				"messages":  len(e.session.Messages),
				"has_error": true,
				"error":     err.Error(),
			},
		})
		_ = e.persistSession()
		return Response{}, err
	}

	_ = e.triggerHooks(ctx, HookEvent{
		Name:   "post_turn",
		Target: input,
		Payload: map[string]any{
			"input":     input,
			"output":    resp.Text,
			"session":   e.session.ID,
			"messages":  len(e.session.Messages),
			"has_error": false,
		},
	})
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// ContinueStream runs a follow-up query loop without appending a new visible user input.
// This is used by LocalJSX lifecycle flows where command handlers inject meta/system context
// and request a model follow-up turn.
func (e *Engine) ContinueStream(ctx context.Context, onChunk func(StreamChunk) error) (Response, error) {
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	e.trimHistory()

	resp, err := e.runLoopStream(ctx, onChunk)
	if err != nil {
		_ = e.persistSession()
		return Response{}, err
	}
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	return resp, nil
}

func (e *Engine) Messages() []types.Message {
	return append([]types.Message(nil), e.session.Messages...)
}

func (e *Engine) SessionID() string {
	return e.session.ID
}

// Runtime returns the tool runtime for accessing task list and other runtime services
func (e *Engine) Runtime() *tool.Runtime {
	return &e.runtime
}

func (e *Engine) SetConfig(cfg config.Config) {
	e.cfg = cfg
}

func (e *Engine) ReplaceMessages(messages []types.Message) {
	e.session.Messages = append([]types.Message(nil), messages...)
	e.transcriptPos = len(e.session.Messages)
}

// RewindMessages truncates the message list to keep only messages up to (and including) the given index.
func (e *Engine) RewindMessages(toIndex int) error {
	if toIndex < 0 || toIndex >= len(e.session.Messages) {
		return fmt.Errorf("invalid rewind index: %d (valid: 0-%d)", toIndex, len(e.session.Messages)-1)
	}

	e.session.Messages = append([]types.Message(nil), e.session.Messages[:toIndex+1]...)
	e.transcriptPos = len(e.session.Messages)

	// Persist the updated session
	return e.persistSession()
}

// maxToolLoopIterations is a safety limit for tool calls within a single turn.
// This prevents runaway loops but is much higher than MaxTurns (which limits user turns).
// TS version uses while(true) with no explicit limit; we add a safety guard.
const maxToolLoopIterations = 100

// maxEmptyAssistantRetriesAfterTool limits retries when model returns an empty
// assistant turn right after tool execution. This avoids silently ending a turn
// when a proxy/provider emits an empty follow-up response.
const maxEmptyAssistantRetriesAfterTool = 2

// maxEmptyFirstResponseRetries limits retries when model returns an empty
// response on the first API round (not after tool calls).
const maxEmptyFirstResponseRetries = 2

// Auto-compact constants
const (
	DefaultContextWindow              = 200000
	AutocompactBufferTokens           = 13000
	MaxConsecutiveAutocompactFailures = 3
)

// shouldAutoCompact determines if auto-compact should trigger based on estimated token count
func (e *Engine) shouldAutoCompact() bool {
	if e.compact == nil {
		return false
	}
	if !e.cfg.AutoCompactEnabled {
		return false
	}
	// Circuit breaker: stop trying after consecutive failures
	if e.autoCompact != nil && e.autoCompact.ConsecutiveFailures >= MaxConsecutiveAutocompactFailures {
		return false
	}

	// Estimate token count from messages
	tokenCount := e.estimateTokenCount()
	contextWindow := e.cfg.ContextWindowOverride
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}
	threshold := contextWindow - AutocompactBufferTokens

	return tokenCount >= threshold
}

// estimateTokenCount estimates token count from messages
// Uses simple heuristic: ~4 characters per token
func (e *Engine) estimateTokenCount() int {
	total := 0
	for _, msg := range e.session.Messages {
		total += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += len(tc.Name) + len(tc.Arguments)
		}
	}
	// Rough estimate: 4 chars per token, with 4/3 padding factor
	return (total / 4) * 4 / 3
}

// postCompactMaxPreservedPairs is the maximum number of recent assistant+tool
// exchange pairs to preserve after auto-compact. Matching TS behavior which
// re-injects file attachments and recent context post-compact.
const postCompactMaxPreservedPairs = 3

// performAutoCompact performs auto-compact if needed.
// Matching TS: buildPostCompactMessages builds [boundaryMarker, summaryMessages,
// messagesToKeep, attachments, hookResults]. In Go we preserve recent messages
// as messagesToKeep and emit a boundary marker + summary.
func (e *Engine) performAutoCompact(ctx context.Context) bool {
	if !e.shouldAutoCompact() {
		return false
	}

	// Snapshot messages before compact for preservation logic
	preCompactMessages := e.session.Messages

	// Convert messages to compact format
	compactMessages := make([]CompactMessage, len(preCompactMessages))
	for i, msg := range preCompactMessages {
		compactMessages[i] = CompactMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Perform compaction
	result, err := e.compact.Compact(ctx, compactMessages, "", true)
	if err != nil {
		if e.autoCompact != nil {
			e.autoCompact.ConsecutiveFailures++
		}
		return false
	}

	// Reset consecutive failures on success
	if e.autoCompact != nil {
		e.autoCompact.ConsecutiveFailures = 0
		e.autoCompact.Compacted = true
	}

	// Build post-compact messages matching TS: buildPostCompactMessages
	// Order: system prompt, boundary marker, summary, preserved recent messages
	if len(result.SummaryMessages) > 0 {
		var newMessages []types.Message

		// 1. Keep system prompt if present
		if len(preCompactMessages) > 0 && preCompactMessages[0].Role == types.RoleSystem {
			newMessages = append(newMessages, preCompactMessages[0])
		}

		// 2. Add boundary marker (system message)
		newMessages = append(newMessages, types.Message{
			Role:      types.RoleSystem,
			Type:      "system",
			Content:   "[compact boundary - auto]",
			Timestamp: time.Now(),
		})

		// 3. Add summary messages
		for _, sm := range result.SummaryMessages {
			newMessages = append(newMessages, types.Message{
				Role:             sm.Role,
				Content:          sm.Content,
				IsCompactSummary: true,
			})
		}

		// 4. Preserve recent message pairs (assistant + tool results + user)
		//    This is the Go equivalent of TS messagesToKeep + file attachments.
		//    We keep the last N "rounds" of conversation to preserve context.
		messagesToKeep := e.collectRecentMessages(preCompactMessages, postCompactMaxPreservedPairs)
		newMessages = append(newMessages, messagesToKeep...)

		e.session.Messages = newMessages
	}

	return true
}

// collectRecentMessages collects recent messages to preserve after auto-compact.
// It keeps the last N "rounds" (user messages as round boundaries), preserving
// the assistant responses, tool calls, and tool results within those rounds.
// This matches TS behavior of preserving recent context and file state.
func (e *Engine) collectRecentMessages(messages []types.Message, maxPairs int) []types.Message {
	if len(messages) <= 1 {
		return nil
	}

	// Skip system prompt
	start := 1
	if len(messages) > 0 && messages[0].Role == types.RoleSystem {
		start = 1
	}

	// Count user messages from the end to find the cutoff point
	userCount := 0
	cutoff := len(messages)
	for i := len(messages) - 1; i >= start; i-- {
		if messages[i].Role == types.RoleUser && !messages[i].IsCompactSummary {
			userCount++
			if userCount > maxPairs {
				cutoff = i
				break
			}
		}
	}

	if cutoff >= len(messages) {
		return nil
	}

	// Filter out progress messages and compact boundaries
	var kept []types.Message
	for i := cutoff; i < len(messages); i++ {
		msg := messages[i]
		// Skip progress messages
		if msg.Type == "progress" || msg.Role == "progress" {
			continue
		}
		// Skip compact boundaries from previous compacts
		if msg.Role == types.RoleSystem && strings.Contains(msg.Content, "[compact boundary") {
			continue
		}
		// Skip previous compact summaries
		if msg.IsCompactSummary {
			continue
		}
		kept = append(kept, msg)
	}

	return kept
}

// performReactiveCompact performs reactive compaction when prompt-too-long error occurs
// Returns true if compaction was performed successfully
func (e *Engine) performReactiveCompact(ctx context.Context) bool {
	if e.compact == nil {
		return false
	}

	// Mark as attempted this turn to prevent retry loops
	if e.autoCompact != nil {
		e.autoCompact.compactedThisTurn = true
	}

	// Snapshot messages before compact for preservation logic
	preCompactMessages := e.session.Messages

	// Convert messages to compact format
	compactMessages := make([]CompactMessage, len(preCompactMessages))
	for i, msg := range preCompactMessages {
		compactMessages[i] = CompactMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	// Perform compaction
	result, err := e.compact.Compact(ctx, compactMessages, "", true)
	if err != nil {
		if e.autoCompact != nil {
			e.autoCompact.ConsecutiveFailures++
		}
		return false
	}

	// Reset consecutive failures on success
	if e.autoCompact != nil {
		e.autoCompact.ConsecutiveFailures = 0
		e.autoCompact.Compacted = true
	}

	// Build post-compact messages matching TS: buildPostCompactMessages
	if len(result.SummaryMessages) > 0 {
		var newMessages []types.Message

		// 1. Keep system prompt if present
		if len(preCompactMessages) > 0 && preCompactMessages[0].Role == types.RoleSystem {
			newMessages = append(newMessages, preCompactMessages[0])
		}

		// 2. Add boundary marker (system message)
		newMessages = append(newMessages, types.Message{
			Role:      types.RoleSystem,
			Type:      "system",
			Content:   "[compact boundary - auto (reactive)]",
			Timestamp: time.Now(),
		})

		// 3. Add summary messages
		for _, sm := range result.SummaryMessages {
			newMessages = append(newMessages, types.Message{
				Role:             sm.Role,
				Content:          sm.Content,
				IsCompactSummary: true,
			})
		}

		// 4. Preserve recent message pairs
		messagesToKeep := e.collectRecentMessages(preCompactMessages, postCompactMaxPreservedPairs)
		newMessages = append(newMessages, messagesToKeep...)

		e.session.Messages = newMessages
	}

	return true
}
func (e *Engine) runLoop(ctx context.Context) (Response, error) {
	last := Response{}
	toolDefs := tool.DefinitionsToTypes(mcp.MergedDefinitions(e.tools, e.runtime))
	previousRoundHadToolCalls := false
	consecutiveEmptyPostToolResponses := 0

	// Reset firstRoundEmptyRetries for new turn
	if e.autoCompact != nil {
		e.autoCompact.firstRoundEmptyRetries = 0
	}

	// Perform auto-compact if needed before first API call
	e.performAutoCompact(ctx)

	// Use infinite loop like TS version: while (true) { ... }
	// Exit when no tool calls are returned by the model.
	// Safety limit prevents runaway loops.
	for round := 0; round < maxToolLoopIterations; round++ {
		// Check for context cancellation (matching TS abort handling)
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		default:
		}

		resp, err := e.provider.Complete(ctx, Request{
			Model:    e.cfg.Model,
			Messages: modelRequestMessages(e.session.Messages),
			Tools:    toolDefs,
		})
		if err != nil {
			// Handle prompt-too-long errors with reactive compact
			if isPromptTooLongError(err) {
				// Try reactive compact once
				if e.compact != nil && (e.autoCompact == nil || !e.autoCompact.compactedThisTurn) {
					if e.performReactiveCompact(ctx) {
						// Compact succeeded, retry the request
						continue
					}
				}
			}
			return Response{}, err
		}
		resp.ToolCalls = ensureToolCallIDs(resp.ToolCalls)

		calls, err := parseToolCalls(resp)
		if err != nil {
			return Response{}, err
		}
		clean := cleanAssistantText(resp)
		isEmptyAssistantTurn := strings.TrimSpace(clean) == "" && len(calls) == 0

		// Handle empty response after tool execution (existing behavior)
		if previousRoundHadToolCalls && isEmptyAssistantTurn {
			consecutiveEmptyPostToolResponses++
			if consecutiveEmptyPostToolResponses < maxEmptyAssistantRetriesAfterTool {
				continue
			}
			return Response{}, fmt.Errorf("model returned empty assistant response after tool execution")
		}

		// Handle empty response on first round (not after tool calls) with retry
		if round == 0 && isEmptyAssistantTurn {
			// Only retry on first round; no previousRoundHadToolCalls
			// Use a counter for first-round empty responses
			if e.autoCompact == nil || e.autoCompact.firstRoundEmptyRetries < maxEmptyFirstResponseRetries {
				if e.autoCompact != nil {
					e.autoCompact.firstRoundEmptyRetries++
				}
				continue
			}
			// Exhausted retries for first round - return empty response (not an error)
			// matching TS behavior where empty first response ends the turn
		}
		consecutiveEmptyPostToolResponses = 0
		assistantMsg := types.Message{
			Role:      types.RoleAssistant,
			Content:   clean,
			Timestamp: time.Now(),
		}
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = append([]types.ToolCall(nil), resp.ToolCalls...)
		}
		e.session.Messages = append(e.session.Messages, assistantMsg)
		if err := e.persistSession(); err != nil {
			return Response{}, err
		}
		// Exit when no tool calls - matching TS: if (!needsFollowUp) return { reason: 'completed' }
		if len(calls) == 0 {
			if clean != "" {
				last.Text = clean
			} else {
				last = resp
			}
			return last, nil
		}
		previousRoundHadToolCalls = true

		if clean != "" {
			last.Text = clean
		}
		for _, call := range calls {
			// Check for context cancellation before each tool call
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			default:
			}

			toolResult := e.callToolWithRetryGuard(ctx, call)
			e.session.Messages = append(e.session.Messages, buildToolResultMessage(call, toolResult.result))
			if err := e.persistSession(); err != nil {
				return Response{}, err
			}
			// Check if hook prevented continuation (matching TS: if (shouldPreventContinuation) return)
			if toolResult.preventContinuation {
				if last.Text == "" {
					last.Text = "hook stopped continuation"
				}
				return last, nil
			}
		}
		// Note: We intentionally do NOT call trimHistory here during tool execution.
		// Trimming history mid-tool-loop can break context for ongoing tool calls.
		// History management should happen between turns (in Submit), not during tool loops.
		// This matches TS version which uses compact mechanism instead of aggressive trimming.
	}
	// Safety limit reached - return last response
	if last.Text == "" {
		last.Text = "tool loop limit reached"
	}
	return last, nil
}

func modelRequestMessages(messages []types.Message) []types.Message {
	if len(messages) == 0 {
		return nil
	}
	filtered := make([]types.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.IsVirtual {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(msg.Type), types.MessageTypeProgress) ||
			strings.EqualFold(strings.TrimSpace(msg.Role), types.MessageTypeProgress) {
			continue
		}
		if msg.IsVisibleInTranscriptOnly {
			continue
		}
		if msg.Role == types.RoleSystem && strings.EqualFold(strings.TrimSpace(msg.Type), types.SystemSubtypeLocalCommand) {
			continue
		}
		filtered = append(filtered, msg)
	}
	return filtered
}

// runLoopStream runs the main loop with streaming support
// Matching TS version: uses while(true) loop, exits when no tool calls
func (e *Engine) runLoopStream(ctx context.Context, onChunk func(StreamChunk) error) (Response, error) {
	// Check if provider supports streaming
	streamingProvider, ok := e.provider.(StreamingProvider)
	if !ok {
		// Fall back to non-streaming
		return e.runLoop(ctx)
	}

	// Reset firstRoundEmptyRetries for new turn
	if e.autoCompact != nil {
		e.autoCompact.firstRoundEmptyRetries = 0
	}

	// Perform auto-compact if needed before first API call
	e.performAutoCompact(ctx)

	last := Response{}
	toolDefs := tool.DefinitionsToTypes(mcp.MergedDefinitions(e.tools, e.runtime))
	previousRoundHadToolCalls := false
	consecutiveEmptyPostToolResponses := 0
	// Use infinite loop like TS version: while (true) { ... }
	// Exit when no tool calls are returned by the model.
	// Safety limit prevents runaway loops.
	for round := 0; round < maxToolLoopIterations; round++ {
		// Check for context cancellation (matching TS abort handling)
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		default:
		}

		resp, err := streamingProvider.CompleteStream(ctx, Request{
			Model:    e.cfg.Model,
			Messages: modelRequestMessages(e.session.Messages),
			Tools:    toolDefs,
		}, func(chunk StreamChunk) error {
			return e.emitStreamChunk(chunk, onChunk)
		})
		if err != nil {
			// Handle prompt-too-long errors with reactive compact
			if isPromptTooLongError(err) {
				if e.compact != nil && (e.autoCompact == nil || !e.autoCompact.compactedThisTurn) {
					if e.performReactiveCompact(ctx) {
						continue
					}
				}
			}
			return Response{}, err
		}
		resp.ToolCalls = ensureToolCallIDs(resp.ToolCalls)

		calls, err := parseToolCalls(resp)
		if err != nil {
			return Response{}, err
		}
		clean := cleanAssistantText(resp)
		isEmptyAssistantTurn := strings.TrimSpace(clean) == "" && len(calls) == 0

		// Handle empty response after tool execution (existing behavior)
		if previousRoundHadToolCalls && isEmptyAssistantTurn {
			consecutiveEmptyPostToolResponses++
			if consecutiveEmptyPostToolResponses < maxEmptyAssistantRetriesAfterTool {
				continue
			}
			return Response{}, fmt.Errorf("model returned empty assistant response after tool execution")
		}

		// Handle empty response on first round (not after tool calls) with retry
		if round == 0 && isEmptyAssistantTurn {
			// Only retry on first round; no previousRoundHadToolCalls
			// Use a counter for first-round empty responses
			if e.autoCompact == nil || e.autoCompact.firstRoundEmptyRetries < maxEmptyFirstResponseRetries {
				if e.autoCompact != nil {
					e.autoCompact.firstRoundEmptyRetries++
				}
				continue
			}
			// Exhausted retries for first round - return empty response (not an error)
			// matching TS behavior where empty first response ends the turn
		}
		consecutiveEmptyPostToolResponses = 0
		assistantMsg := types.Message{
			Role:      types.RoleAssistant,
			Content:   clean,
			Timestamp: time.Now(),
		}
		if len(resp.ToolCalls) > 0 {
			assistantMsg.ToolCalls = append([]types.ToolCall(nil), resp.ToolCalls...)
		}
		e.session.Messages = append(e.session.Messages, assistantMsg)
		if err := e.persistSession(); err != nil {
			return Response{}, err
		}
		// Exit when no tool calls - matching TS: if (!needsFollowUp) return { reason: 'completed' }
		if len(calls) == 0 {
			if clean != "" {
				last.Text = clean
			} else {
				last = resp
			}
			return last, nil
		}
		previousRoundHadToolCalls = true

		if clean != "" {
			last.Text = clean
		}
		for _, call := range calls {
			// Check for context cancellation before each tool call
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			default:
			}
			if err := e.emitStreamChunk(StreamChunk{
				ToolCalls:  []tool.CallSpec{call},
				ToolName:   call.Name,
				ToolCallID: call.ID,
				Status:     "Running tool: " + call.Name,
			}, onChunk); err != nil {
				return Response{}, err
			}

			toolResult := e.callToolWithRetryGuard(ctx, call)
			e.session.Messages = append(e.session.Messages, buildToolResultMessage(call, toolResult.result))
			if err := e.persistSession(); err != nil {
				return Response{}, err
			}
			toolStatus := "Tool completed: " + call.Name
			toolResultSummary := "ok"
			if strings.TrimSpace(toolResult.result.Error) != "" {
				toolStatus = "Tool failed: " + call.Name
				toolResultSummary = "error"
			}
			if err := e.emitStreamChunk(StreamChunk{
				ToolName:   call.Name,
				ToolCallID: call.ID,
				ToolResult: toolResultSummary,
				Status:     toolStatus,
			}, onChunk); err != nil {
				return Response{}, err
			}
			// Check if hook prevented continuation (matching TS: if (shouldPreventContinuation) return)
			if toolResult.preventContinuation {
				if last.Text == "" {
					last.Text = "hook stopped continuation"
				}
				return last, nil
			}
		}
		// Note: We intentionally do NOT call trimHistory here during tool execution.
		// Trimming history mid-tool-loop can break context for ongoing tool calls.
		// History management should happen between turns (in Submit), not during tool loops.
		// This matches TS version which uses compact mechanism instead of aggressive trimming.
	}
	// Safety limit reached - return last response
	if last.Text == "" {
		last.Text = "tool loop limit reached"
	}
	return last, nil
}

func (e *Engine) emitStreamChunk(chunk StreamChunk, onChunk func(StreamChunk) error) error {
	if onChunk != nil {
		if err := onChunk(chunk); err != nil {
			return err
		}
	}
	if e.onStreamChunk != nil {
		if err := e.onStreamChunk(chunk); err != nil {
			return err
		}
	}
	return nil
}

// callToolResult wraps tool.Result with a flag indicating if hooks prevented continuation.
type callToolResult struct {
	result              tool.Result
	preventContinuation bool
}

func (e *Engine) callTool(ctx context.Context, call tool.CallSpec) callToolResult {
	// Pre-tool hooks
	preReports, err := e.triggerHooksWithReports(ctx, HookEvent{
		Name:   string(types.HookEventPreToolUse),
		Target: call.Name,
		Payload: map[string]any{
			"tool":  call.Name,
			"input": call.Input,
		},
	})
	if err != nil {
		return callToolResult{result: tool.Result{Error: err.Error()}}
	}
	// Check if pre-tool hooks prevented continuation (matching TS version)
	if shouldPreventContinuation(preReports) {
		return callToolResult{
			result:              tool.Result{Error: "hook prevented continuation"},
			preventContinuation: true,
		}
	}

	if e.tools == nil {
		return callToolResult{result: tool.Result{Error: "tool registry is not configured"}}
	}
	definition, ok := mcp.LookupDefinition(e.tools, e.runtime, call.Name)
	if !ok {
		return callToolResult{result: tool.Result{Error: fmt.Sprintf("unknown tool: %s", call.Name)}}
	}

	// Permission check: if tool implements PermissionChecker, check before execution
	if permChecker, ok := definition.(tool.PermissionChecker); ok {
		permResult := permChecker.CheckPermissions(ctx, call.Input)
		switch permResult.Decision {
		case tool.PermissionDeny:
			return callToolResult{
				result: tool.Result{Error: permResult.Message},
			}
		case tool.PermissionAsk:
			// Need user approval - call AskPermission callback
			if e.runtime.AskPermission != nil {
				approved, err := e.runtime.AskPermission(ctx, call.Name, call.Input, permResult.Message)
				if err != nil {
					return callToolResult{result: tool.Result{Error: "permission request failed: " + err.Error()}}
				}
				if !approved {
					return callToolResult{
						result: tool.Result{Error: "Permission denied: " + permResult.Message},
					}
				}
			} else {
				// No permission handler available - deny by default
				return callToolResult{
					result: tool.Result{Error: "Permission required but no handler available: " + permResult.Message},
				}
			}
		case tool.PermissionAllow:
			// Continue with execution
		}
	}

	runtime := e.runtime
	runtime.EmitProgress = func(data map[string]any) {
		if len(data) == 0 {
			return
		}
		parentID := strings.TrimSpace(call.ID)
		if parentID == "" {
			parentID = fmt.Sprintf("repl_progress_%d", time.Now().UnixNano())
		}
		e.appendProgressMessage(parentID, parentID, data)
	}

	result, err := definition.Call(ctx, call.Input, runtime)
	if err != nil {
		result.Error = err.Error()
	}

	// Determine hook event based on success/failure
	hookEvent := types.HookEventPostToolUse
	if result.Error != "" {
		hookEvent = types.HookEventPostToolUseFailure
	}

	// Post-tool hooks
	postReports, _ := e.triggerHooksWithReports(ctx, HookEvent{
		Name:   string(hookEvent),
		Target: call.Name,
		Payload: map[string]any{
			"tool":   call.Name,
			"input":  call.Input,
			"error":  result.Error,
			"result": summarizeToolResult(result),
		},
	})

	return callToolResult{
		result:              result,
		preventContinuation: shouldPreventContinuation(postReports),
	}
}

func (e *Engine) trimHistory() {
	if e.cfg.MaxTurns <= 0 {
		return
	}
	if len(e.session.Messages) == 0 {
		return
	}

	systemPrefix := 0
	if e.cfg.SystemPrompt != "" && len(e.session.Messages) > 0 && e.session.Messages[0].Role == types.RoleSystem {
		systemPrefix = 1
	}

	userCount := 0
	start := len(e.session.Messages)
	for i := len(e.session.Messages) - 1; i >= systemPrefix; i-- {
		if e.session.Messages[i].Role != types.RoleUser {
			continue
		}
		userCount++
		if userCount == e.cfg.MaxTurns {
			start = i
			break
		}
	}
	if start == len(e.session.Messages) {
		return
	}

	if systemPrefix == 1 && start > 1 {
		kept := []types.Message{e.session.Messages[0]}
		kept = append(kept, e.session.Messages[start:]...)
		e.session.Messages = kept
		return
	}
	e.session.Messages = append([]types.Message(nil), e.session.Messages[start:]...)
}

func parseToolCalls(resp Response) ([]tool.CallSpec, error) {
	if len(resp.ToolCalls) > 0 {
		return tool.ParseNativeCalls(resp.ToolCalls)
	}
	return tool.ParseCalls(resp.Text)
}

func cleanAssistantText(resp Response) string {
	var text string
	if len(resp.ToolCalls) > 0 {
		text = strings.TrimSpace(resp.Text)
	} else {
		text = tool.StripCalls(resp.Text)
	}
	return stripAssistantThinkingTags(text)
}

func stripAssistantThinkingTags(content string) string {
	if strings.TrimSpace(content) == "" {
		return content
	}

	text := content
	patterns := [][2]string{
		{"<think>", "</think>"},
		{"<thinking>", "</thinking>"},
	}

	for _, tag := range patterns {
		startTag := tag[0]
		endTag := tag[1]
		for {
			startIdx := strings.Index(text, startTag)
			if startIdx == -1 {
				break
			}
			endRel := strings.Index(text[startIdx:], endTag)
			if endRel == -1 {
				text = text[:startIdx]
				break
			}
			endIdx := startIdx + endRel
			text = text[:startIdx] + text[endIdx+len(endTag):]
		}
	}

	return strings.TrimSpace(text)
}

func (e *Engine) callToolWithRetryGuard(ctx context.Context, call tool.CallSpec) callToolResult {
	if e.nonRetryableFailures == nil {
		e.nonRetryableFailures = map[string]string{}
	}

	if key, ok := toolCallRetryKey(call); ok {
		if previousErr, exists := e.nonRetryableFailures[key]; exists {
			msg := fmt.Sprintf(
				"non-retryable tool error: this call was already denied (%s). Retrying without changing permissions or command will fail. Ask the user for approval or use a different approach.",
				previousErr,
			)
			return callToolResult{
				result: tool.Result{
					Error: msg,
					Content: map[string]any{
						"error":          msg,
						"original_error": previousErr,
						"retryable":      false,
					},
				},
			}
		}
	}

	result := e.callTool(ctx, call)
	if key, ok := toolCallRetryKey(call); ok && isNonRetryableToolError(result.result.Error) {
		e.nonRetryableFailures[key] = result.result.Error
	}
	return result
}

func toolCallRetryKey(call tool.CallSpec) (string, bool) {
	toolName := strings.ToLower(strings.TrimSpace(call.Name))
	if toolName == "" {
		return "", false
	}

	switch toolName {
	case "bash", "powershell":
		command, _ := call.Input["command"].(string)
		command = strings.TrimSpace(command)
		if command == "" {
			return "", false
		}
		return toolName + "::" + command, true
	}

	raw, err := json.Marshal(call.Input)
	if err != nil {
		return "", false
	}
	return toolName + "::" + string(raw), true
}

func isNonRetryableToolError(errText string) bool {
	errText = strings.ToLower(strings.TrimSpace(errText))
	if errText == "" {
		return false
	}
	prefixes := []string{
		"permission required:",
		"permission denied:",
		"command blocked:",
		"non-retryable tool error:",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(errText, p) {
			return true
		}
	}
	return false
}

func buildToolResultMessage(call tool.CallSpec, result tool.Result) types.Message {
	renderToolName := call.Name
	if isREPLToolCall(call) {
		if result.Meta != nil {
			if inner, ok := result.Meta["wrapped_tool_name"].(string); ok && strings.TrimSpace(inner) != "" {
				renderToolName = strings.TrimSpace(inner)
			}
		}
	}

	msg := types.Message{
		Content:   tool.RenderResult(renderToolName, result),
		Timestamp: time.Now(),
	}
	if call.ID != "" {
		msg.Role = types.RoleTool
		msg.ToolCallID = call.ID
		return msg
	}
	msg.Role = types.RoleSystem
	return msg
}

func isREPLToolCall(call tool.CallSpec) bool {
	return strings.EqualFold(strings.TrimSpace(call.Name), repl.REPLToolName)
}

func buildREPLToolProgressData(call tool.CallSpec, phase string, result *tool.Result) map[string]any {
	toolName := call.Name
	toolInput := map[string]any{}
	for k, v := range call.Input {
		toolInput[k] = v
	}
	if script, ok := call.Input["script"].(string); ok {
		if innerName, innerInput, parsed := repl.ExtractFirstPrimitiveCall(script); parsed {
			toolName = innerName
			toolInput = map[string]any{}
			for k, v := range innerInput {
				toolInput[k] = v
			}
		}
	}
	if result != nil && result.Meta != nil {
		if innerName, ok := result.Meta["wrapped_tool_name"].(string); ok && strings.TrimSpace(innerName) != "" {
			toolName = strings.TrimSpace(innerName)
		}
		if innerInput, ok := result.Meta["wrapped_tool_input"].(map[string]any); ok {
			toolInput = map[string]any{}
			for k, v := range innerInput {
				toolInput[k] = v
			}
		}
	}

	data := map[string]any{
		"type":      "repl_tool_call",
		"phase":     strings.ToLower(strings.TrimSpace(phase)),
		"toolName":  toolName,
		"toolInput": toolInput,
	}

	if result == nil {
		return data
	}

	if strings.TrimSpace(result.Error) != "" {
		data["status"] = "error"
		data["error"] = result.Error
		return data
	}
	data["status"] = "ok"
	return data
}

func (e *Engine) appendProgressMessage(toolUseID, parentToolUseID string, data map[string]any) {
	if len(data) == 0 {
		return
	}

	toolUseID = strings.TrimSpace(toolUseID)
	parentToolUseID = strings.TrimSpace(parentToolUseID)
	if toolUseID == "" {
		toolUseID = fmt.Sprintf("progress_%d", time.Now().UnixNano())
	}
	if parentToolUseID == "" {
		parentToolUseID = toolUseID
	}

	payload, err := json.Marshal(data)
	content := ""
	if err == nil {
		content = string(payload)
	}

	e.session.Messages = append(e.session.Messages, types.Message{
		UUID:       types.GenerateUUID(),
		Type:       types.MessageTypeProgress,
		Role:       types.RoleSystem,
		Content:    content,
		ToolCallID: toolUseID,
		ToolUseResult: map[string]any{
			"toolUseID":       toolUseID,
			"parentToolUseID": parentToolUseID,
			"data":            data,
		},
		Timestamp: time.Now(),
	})
}

func ensureToolCallIDs(calls []types.ToolCall) []types.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := append([]types.ToolCall(nil), calls...)
	prefix := fmt.Sprintf("call_%d", time.Now().UnixNano())
	for i := range out {
		if strings.TrimSpace(out[i].ID) == "" {
			out[i].ID = fmt.Sprintf("%s_%d", prefix, i+1)
		}
	}
	return out
}

func (e *Engine) triggerHooks(ctx context.Context, event HookEvent) error {
	if e.hooks == nil {
		return nil
	}
	reports, err := e.hooks.Trigger(ctx, event)
	e.persistHookReports(event, reports)
	return err
}

// triggerHooksWithReports triggers hooks and returns execution reports.
// Used when caller needs to check PreventContinuation or other hook flags.
func (e *Engine) triggerHooksWithReports(ctx context.Context, event HookEvent) ([]HookExecution, error) {
	if e.hooks == nil {
		return nil, nil
	}
	reports, err := e.hooks.Trigger(ctx, event)
	e.persistHookReports(event, reports)
	return reports, err
}

func (e *Engine) persistHookReports(event HookEvent, reports []HookExecution) {
	if len(reports) == 0 || !shouldPersistHookReports(event.Name) {
		return
	}
	if shouldEmitStopHookSummary(event.Name) {
		e.persistStopHookSummary(event, reports)
		return
	}
	summary := formatHookReports(reports)
	if strings.TrimSpace(summary) == "" {
		return
	}
	e.session.Messages = append(e.session.Messages, types.Message{
		Type:      types.MessageTypeSystem,
		Role:      types.RoleSystem,
		Content:   summary,
		Timestamp: time.Now(),
	})
}

func shouldEmitStopHookSummary(eventName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(eventName))
	return normalized == strings.ToLower(string(types.HookEventStop)) ||
		normalized == strings.ToLower(string(types.HookEventSubagentStop)) ||
		normalized == "post_turn"
}

func (e *Engine) persistStopHookSummary(event HookEvent, reports []HookExecution) {
	hookInfos := make([]map[string]any, 0, len(reports))
	hookErrors := make([]string, 0)
	totalDurationMs := 0
	hasOutput := false
	preventedContinuation := false
	stopReason := ""
	toolUseID := extractEventToolUseID(event.Payload)

	for _, report := range reports {
		info := map[string]any{}
		if hookName := strings.TrimSpace(report.Hook); hookName != "" {
			info["hookName"] = hookName
		}
		if cmd := strings.TrimSpace(report.Command); cmd != "" {
			info["command"] = cmd
		}
		if report.DurationMs > 0 {
			info["durationMs"] = report.DurationMs
			totalDurationMs += report.DurationMs
		}
		if len(info) > 0 {
			hookInfos = append(hookInfos, info)
		}

		if errText := strings.TrimSpace(report.Error); errText != "" {
			hookErrors = append(hookErrors, errText)
			hasOutput = true
			if stopReason == "" {
				stopReason = errText
			}
		}
		if strings.TrimSpace(report.Output) != "" {
			hasOutput = true
		}
		if report.PreventContinuation {
			preventedContinuation = true
			if stopReason == "" {
				if reason := strings.TrimSpace(report.StopReason); reason != "" {
					stopReason = reason
				} else {
					stopReason = "Stop hook prevented continuation"
				}
			}
		}

		e.appendHookAttachmentMessage(event.Name, toolUseID, report)
	}

	payload := map[string]any{
		"type":                  types.MessageTypeSystem,
		"subtype":               types.SystemSubtypeStopHookSummary,
		"hookCount":             len(reports),
		"hookInfos":             hookInfos,
		"hookErrors":            hookErrors,
		"preventedContinuation": preventedContinuation,
		"stopReason":            stopReason,
		"hasOutput":             hasOutput,
		"level":                 "suggestion",
		"totalDurationMs":       totalDurationMs,
		"content":               formatStopHookSummaryContent(len(reports), totalDurationMs, len(hookErrors), preventedContinuation),
	}
	if toolUseID != "" {
		payload["toolUseID"] = toolUseID
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	e.session.Messages = append(e.session.Messages, types.Message{
		Type:      types.MessageTypeSystem,
		Role:      types.RoleSystem,
		Content:   string(raw),
		Timestamp: time.Now(),
	})
}

func (e *Engine) appendHookAttachmentMessage(eventName, toolUseID string, report HookExecution) {
	attachmentType := types.AttachmentTypeHookSuccess
	if strings.TrimSpace(report.Error) != "" {
		if report.Blocking {
			attachmentType = types.AttachmentTypeHookBlockingError
		} else {
			attachmentType = types.AttachmentTypeHookNonBlockingError
		}
	}

	payload := map[string]any{
		"type":      attachmentType,
		"hookEvent": eventName,
		"hookName":  report.Hook,
	}
	if cmd := strings.TrimSpace(report.Command); cmd != "" {
		payload["command"] = cmd
	}
	if report.DurationMs > 0 {
		payload["durationMs"] = report.DurationMs
	}
	if toolUseID != "" {
		payload["toolUseID"] = toolUseID
	}
	if out := strings.TrimSpace(report.Output); out != "" {
		payload["stdout"] = out
		payload["content"] = out
	}
	if errText := strings.TrimSpace(report.Error); errText != "" {
		payload["stderr"] = errText
		payload["content"] = errText
	}
	if _, hasContent := payload["content"]; !hasContent {
		payload["content"] = "Hook completed"
	}

	raw, err := json.Marshal(payload)
	if err == nil {
		e.session.Messages = append(e.session.Messages, types.Message{
			Type:      types.MessageTypeAttachment,
			Role:      types.RoleSystem,
			Content:   string(raw),
			Timestamp: time.Now(),
		})
	}

	if report.PreventContinuation {
		stopReason := strings.TrimSpace(report.StopReason)
		if stopReason == "" {
			stopReason = "Stop hook prevented continuation"
		}
		haltPayload := map[string]any{
			"type":      types.AttachmentTypeHookStoppedContinuation,
			"hookEvent": eventName,
			"hookName":  report.Hook,
			"message":   stopReason,
			"content":   stopReason,
		}
		if toolUseID != "" {
			haltPayload["toolUseID"] = toolUseID
		}
		if rawStop, stopErr := json.Marshal(haltPayload); stopErr == nil {
			e.session.Messages = append(e.session.Messages, types.Message{
				Type:      types.MessageTypeAttachment,
				Role:      types.RoleSystem,
				Content:   string(rawStop),
				Timestamp: time.Now(),
			})
		}
	}
}

func extractEventToolUseID(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	for _, key := range []string{"toolUseID", "toolUseId", "tool_use_id"} {
		if raw, ok := payload[key]; ok {
			if toolUseID := strings.TrimSpace(stringFromAny(raw)); toolUseID != "" {
				return toolUseID
			}
		}
	}
	return ""
}

func formatStopHookSummaryContent(hookCount, totalDurationMs, hookErrorCount int, prevented bool) string {
	parts := []string{fmt.Sprintf("%d stop hook%s", hookCount, pluralSuffix(hookCount))}
	if totalDurationMs > 0 {
		parts = append(parts, fmt.Sprintf("%dms", totalDurationMs))
	}
	if hookErrorCount > 0 {
		parts = append(parts, fmt.Sprintf("%d error%s", hookErrorCount, pluralSuffix(hookErrorCount)))
	}
	if prevented {
		parts = append(parts, "continuation stopped")
	}
	return strings.Join(parts, " \u00b7 ")
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func (e *Engine) appendRelevantMemoriesAttachment(ctx context.Context, input string) {
	query := strings.TrimSpace(input)
	if query == "" {
		return
	}

	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		return
	}

	memoryDir := filepath.Join(cwd, ".claude-go", "memory")
	cfg := memory.DefaultRelevantMemoriesConfig()
	alreadySurfaced, surfacedBytes := collectSurfacedRelevantMemories(e.session.Messages)

	retriever := memory.CreateMemoryRetriever(memoryDir, cfg)
	selected, err := retriever.FindRelevantMemories(ctx, query, nil, alreadySurfaced)
	if err != nil || len(selected) == 0 {
		return
	}

	attachments, err := memory.GetRelevantMemoryAttachments(ctx, memoryDir, selected, nil)
	if err != nil || len(attachments) == 0 {
		return
	}

	memories := make([]map[string]any, 0, len(selected))
	usedBytes := surfacedBytes
	for _, mem := range selected {
		if alreadySurfaced[mem.Path] {
			continue
		}
		content, ok := attachments[mem.Path]
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		if cfg.MaxSessionBytes > 0 && usedBytes+len(content) > cfg.MaxSessionBytes {
			continue
		}
		usedBytes += len(content)
		memories = append(memories, map[string]any{
			"path":    mem.Path,
			"mtimeMs": mem.MtimeMs,
			"content": content,
		})
	}
	if len(memories) == 0 {
		return
	}

	payload := map[string]any{
		"type":     "relevant_memories",
		"memories": memories,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	e.session.Messages = append(e.session.Messages, types.Message{
		Type:      types.MessageTypeAttachment,
		Role:      types.RoleSystem,
		Content:   string(raw),
		Timestamp: time.Now(),
	})
}

func collectSurfacedRelevantMemories(messages []types.Message) (map[string]bool, int) {
	paths := make(map[string]bool)
	totalBytes := 0

	for _, msg := range messages {
		if msg.Type != types.MessageTypeAttachment {
			continue
		}
		payload := parseJSONPayloadMap(msg.Content)
		if len(payload) == 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(stringFromAny(payload["type"])), "relevant_memories") {
			continue
		}
		memories, ok := payload["memories"].([]any)
		if !ok {
			continue
		}
		for _, memoryItem := range memories {
			memoryObj, ok := memoryItem.(map[string]any)
			if !ok {
				continue
			}
			if path := strings.TrimSpace(stringFromAny(memoryObj["path"])); path != "" {
				paths[path] = true
			}
			if content := stringFromAny(memoryObj["content"]); content != "" {
				totalBytes += len(content)
			}
		}
	}

	return paths, totalBytes
}

func parseJSONPayloadMap(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if !(strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}
	return payload
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func shouldPersistHookReports(eventName string) bool {
	switch eventName {
	case string(types.HookEventPreToolUse),
		string(types.HookEventPostToolUse),
		string(types.HookEventPostToolUseFailure):
		return false
	default:
		return true
	}
}

// shouldPreventContinuation checks if any hook execution set PreventContinuation to true.
// Matching TS version: if (shouldPreventContinuation) { return { reason: 'hook_stopped' } }
func shouldPreventContinuation(reports []HookExecution) bool {
	for _, report := range reports {
		if report.PreventContinuation {
			return true
		}
	}
	return false
}

func summarizeToolResult(result tool.Result) string {
	if result.Error != "" {
		return "error"
	}
	switch v := result.Content.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "ok"
		}
		return strings.TrimSpace(v)
	default:
		return "ok"
	}
}

func formatHookReports(reports []HookExecution) string {
	lines := []string{"hook_reports:"}
	for _, report := range reports {
		line := fmt.Sprintf("- %s:%s result=%s", report.Event, report.Hook, report.Result)
		if strings.TrimSpace(report.Target) != "" {
			line += " target=" + report.Target
		}
		lines = append(lines, line)
		if strings.TrimSpace(report.Error) != "" {
			lines = append(lines, "  error="+report.Error)
		}
		if strings.TrimSpace(report.Output) != "" {
			lines = append(lines, "  output="+strings.TrimSpace(report.Output))
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Engine) persistSession() error {
	if e.manager == nil || e.session == nil {
		return nil
	}
	if err := e.manager.Save(e.session); err != nil {
		return err
	}

	if e.transcripts == nil {
		return nil
	}
	if e.transcriptPos < 0 || e.transcriptPos > len(e.session.Messages) {
		e.transcriptPos = len(e.session.Messages)
		return nil
	}
	if e.transcriptPos == len(e.session.Messages) {
		return nil
	}

	delta := append([]types.Message(nil), e.session.Messages[e.transcriptPos:]...)
	e.transcriptPos = len(e.session.Messages)

	cwd := "."
	if e.runtime.Store != nil {
		if current := strings.TrimSpace(e.runtime.Store.GetCWD()); current != "" {
			cwd = current
		}
	}
	userType := strings.TrimSpace(os.Getenv("USER_TYPE"))
	if userType == "" {
		userType = "external"
	}
	return e.transcripts.RecordTranscript(e.session.ID, delta, cwd, userType)
}

// Close triggers SessionEnd hook and saves the session.
func (e *Engine) Close(ctx context.Context, reason string) error {
	// Trigger SessionEnd hook
	_ = e.triggerHooks(ctx, HookEvent{
		Name:   string(types.HookEventSessionEnd),
		Target: reason,
		Payload: map[string]any{
			"session_id": e.session.ID,
			"reason":     reason,
		},
	})

	// Save final session state
	return e.persistSession()
}

// Session returns the current session.
func (e *Engine) Session() *session.Session {
	return e.session
}
