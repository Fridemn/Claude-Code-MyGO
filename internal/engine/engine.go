package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"claude-code-go/internal/config"
	"claude-code-go/internal/prompt"
	"claude-code-go/internal/session"
	"claude-code-go/internal/tool"
	"claude-code-go/internal/tool/mcp"
	"claude-code-go/internal/types"
)

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
	Trigger(context.Context, HookEvent) ([]HookExecution, error)
}

type HookEvent struct {
	Name    string
	Target  string
	Payload map[string]any
}

type HookExecution struct {
	Event     string
	Target    string
	Hook      string
	Blocking  bool
	Result    string
	Output    string
	Error     string
	Timestamp string
	Payload   map[string]any
}

type Options struct {
	Config          config.Config
	Provider        Provider
	Tools           *tool.Registry
	ToolRuntime     tool.Runtime
	Hooks           HookRunner
	Sessions        *session.Manager
	InitialMessages []types.Message
	SessionID       string
	OnStreamChunk   func(StreamChunk) error // Optional callback for streaming
}

type Engine struct {
	cfg           config.Config
	provider      Provider
	tools         *tool.Registry
	runtime       tool.Runtime
	hooks         HookRunner
	session       *session.Session
	manager       *session.Manager
	onStreamChunk func(StreamChunk) error
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
		cfg:           opts.Config,
		provider:      opts.Provider,
		tools:         opts.Tools,
		runtime:       opts.ToolRuntime,
		hooks:         opts.Hooks,
		session:       s,
		manager:       opts.Sessions,
		onStreamChunk: opts.OnStreamChunk,
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
	if err := e.persistSession(); err != nil {
		return Response{}, err
	}
	e.trimHistory()

	resp, err := e.runLoop(ctx)
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
		_ = e.manager.Save(e.session)
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
	if err := e.manager.Save(e.session); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// SubmitStream submits input and streams the response
func (e *Engine) SubmitStream(ctx context.Context, input string, onChunk func(StreamChunk) error) (Response, error) {
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
		_ = e.manager.Save(e.session)
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
	if err := e.manager.Save(e.session); err != nil {
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
func (e *Engine) Runtime() tool.Runtime {
	return e.runtime
}

func (e *Engine) ReplaceMessages(messages []types.Message) {
	e.session.Messages = append([]types.Message(nil), messages...)
}

func (e *Engine) runLoop(ctx context.Context) (Response, error) {
	last := Response{}
	toolDefs := tool.DefinitionsToTypes(mcp.MergedDefinitions(e.tools, e.runtime))
	for round := 0; round < e.toolLoopLimit(); round++ {
		resp, err := e.provider.Complete(ctx, Request{
			Model:    e.cfg.Model,
			Messages: append([]types.Message(nil), e.session.Messages...),
			Tools:    toolDefs,
		})
		if err != nil {
			return Response{}, err
		}
		resp.ToolCalls = ensureToolCallIDs(resp.ToolCalls)

		calls, err := parseToolCalls(resp)
		if err != nil {
			return Response{}, err
		}
		clean := cleanAssistantText(resp)
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
		if len(calls) == 0 {
			if clean != "" {
				last.Text = clean
			} else {
				last = resp
			}
			return last, nil
		}

		if clean != "" {
			last.Text = clean
		}
		for _, call := range calls {
			result := e.callTool(ctx, call)
			e.session.Messages = append(e.session.Messages, buildToolResultMessage(call, result))
			if err := e.persistSession(); err != nil {
				return Response{}, err
			}
		}
		e.trimHistory()
		if err := e.persistSession(); err != nil {
			return Response{}, err
		}
	}
	if last.Text == "" {
		last.Text = "tool loop limit reached"
	}
	return last, nil
}

// runLoopStream runs the main loop with streaming support
func (e *Engine) runLoopStream(ctx context.Context, onChunk func(StreamChunk) error) (Response, error) {
	// Check if provider supports streaming
	streamingProvider, ok := e.provider.(StreamingProvider)
	if !ok {
		// Fall back to non-streaming
		return e.runLoop(ctx)
	}

	last := Response{}
	toolDefs := tool.DefinitionsToTypes(mcp.MergedDefinitions(e.tools, e.runtime))
	for round := 0; round < e.toolLoopLimit(); round++ {
		resp, err := streamingProvider.CompleteStream(ctx, Request{
			Model:    e.cfg.Model,
			Messages: append([]types.Message(nil), e.session.Messages...),
			Tools:    toolDefs,
		}, func(chunk StreamChunk) error {
			if onChunk != nil {
				return onChunk(chunk)
			}
			return nil
		})
		if err != nil {
			return Response{}, err
		}
		resp.ToolCalls = ensureToolCallIDs(resp.ToolCalls)

		calls, err := parseToolCalls(resp)
		if err != nil {
			return Response{}, err
		}
		clean := cleanAssistantText(resp)
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
		if len(calls) == 0 {
			if clean != "" {
				last.Text = clean
			} else {
				last = resp
			}
			return last, nil
		}

		if clean != "" {
			last.Text = clean
		}
		for _, call := range calls {
			result := e.callTool(ctx, call)
			e.session.Messages = append(e.session.Messages, buildToolResultMessage(call, result))
			if err := e.persistSession(); err != nil {
				return Response{}, err
			}
		}
		e.trimHistory()
		if err := e.persistSession(); err != nil {
			return Response{}, err
		}
	}
	if last.Text == "" {
		last.Text = "tool loop limit reached"
	}
	return last, nil
}

func (e *Engine) toolLoopLimit() int {
	if e.cfg.MaxTurns > 0 {
		return e.cfg.MaxTurns
	}
	return 4
}

func (e *Engine) callTool(ctx context.Context, call tool.CallSpec) tool.Result {
	if err := e.triggerHooks(ctx, HookEvent{
		Name:   "pre_tool",
		Target: call.Name,
		Payload: map[string]any{
			"tool":  call.Name,
			"input": call.Input,
		},
	}); err != nil {
		return tool.Result{Error: err.Error()}
	}
	if e.tools == nil {
		return tool.Result{Error: "tool registry is not configured"}
	}
	definition, ok := mcp.LookupDefinition(e.tools, e.runtime, call.Name)
	if !ok {
		return tool.Result{Error: fmt.Sprintf("unknown tool: %s", call.Name)}
	}
	result, err := definition.Call(ctx, call.Input, e.runtime)
	if err != nil {
		result.Error = err.Error()
	}
	_ = e.triggerHooks(ctx, HookEvent{
		Name:   "post_tool",
		Target: call.Name,
		Payload: map[string]any{
			"tool":   call.Name,
			"input":  call.Input,
			"error":  result.Error,
			"result": summarizeToolResult(result),
		},
	})
	return result
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
	if len(resp.ToolCalls) > 0 {
		return strings.TrimSpace(resp.Text)
	}
	return tool.StripCalls(resp.Text)
}

func buildToolResultMessage(call tool.CallSpec, result tool.Result) types.Message {
	msg := types.Message{
		Content:   tool.RenderResult(call.Name, result),
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
	if !shouldPersistHookReports(event.Name) {
		return err
	}
	if len(reports) > 0 {
		summary := formatHookReports(reports)
		if strings.TrimSpace(summary) != "" {
			e.session.Messages = append(e.session.Messages, types.Message{
				Role:      types.RoleSystem,
				Content:   summary,
				Timestamp: time.Now(),
			})
		}
	}
	return err
}

func shouldPersistHookReports(eventName string) bool {
	switch eventName {
	case "pre_tool", "post_tool":
		return false
	default:
		return true
	}
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
	return e.manager.Save(e.session)
}
