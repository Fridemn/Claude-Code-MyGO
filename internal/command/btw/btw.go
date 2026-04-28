package btw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"claude-go/internal/command"
	"claude-go/internal/engine"
	"claude-go/internal/prompt"
	"claude-go/internal/types"
	"claude-go/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// btwResponseMsg is sent when the side question response is received
type btwResponseMsg struct {
	response string
	err      error
}

// btwTickMsg is sent for spinner animation
type btwTickMsg struct{}

// btwModel is the Bubble Tea model for the btw side question UI
type btwModel struct {
	rt        command.Runtime
	question  string
	response  string
	error     string
	frame     int
	scrollPos int
	maxHeight int
	loading   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

const btwSpinnerInterval = 80 * time.Millisecond
const btwScrollLines = 3

// Register registers the btw command with the registry
func Register(r *command.Registry) {
	r.Register(command.LocalJSXCommand{
		CommandBase: command.CommandBase{
			Name:          "btw",
			Description:   "Ask a quick side question without interrupting the main conversation",
			ArgumentHint:  "<question>",
			UserInvocable: true,
			Source:        "builtin",
			Immediate:     true, // Can execute during streaming without interrupting main agent
		},
		Load: loadBtwModel,
	})
}

func loadBtwModel(ctx context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	question := strings.Join(args, " ")
	if strings.TrimSpace(question) == "" {
		// No question provided - return a simple model that shows usage and exits
		return btwEmptyModel{rt: rt}, nil
	}

	// Create context for the side question
	sideCtx, cancel := context.WithCancel(ctx)

	return btwModel{
		rt:       rt,
		question: question,
		loading:  true,
		ctx:      sideCtx,
		cancel:   cancel,
	}, nil
}

// btwEmptyModel handles the empty question case
type btwEmptyModel struct {
	rt command.Runtime
}

func (m btwEmptyModel) Init() tea.Cmd {
	return nil
}

func (m btwEmptyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || msg.Type == tea.KeyCtrlC {
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone("Usage: /btw <your question>", command.LocalJSXDoneOptions{Display: "system"})
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m btwEmptyModel) View() string {
	return ui.Style(&ui.Dark.Error, nil, "Usage: /btw <your question>", true) +
		"\n\n" + ui.Style(&ui.Dark.Muted, nil, "Press Enter or Escape to dismiss", true)
}

func (m btwModel) Init() tea.Cmd {
	if m.loading && m.rt.Engine != nil && m.rt.Provider != nil {
		return tea.Batch(
			m.fetchSideQuestion(),
			m.tickSpinner(),
		)
	}
	// If engine or provider is not available, return error immediately
	if m.loading {
		return func() tea.Msg {
			return btwResponseMsg{err: fmt.Errorf("engine or provider not available")}
		}
	}
	return nil
}

func (m btwModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case btwTickMsg:
		if m.loading {
			m.frame++
			return m, m.tickSpinner()
		}
		return m, nil

	case btwResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.error = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.response = msg.response
		}
		return m, nil

	case tea.KeyMsg:
		// Dismiss keys: Escape, Return, Space, Ctrl+C, Ctrl+D
		if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter || msg.String() == " " ||
			(msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlD) {
			m.dismiss()
			return m, tea.Quit
		}

		// Scroll keys: Up, Down, Ctrl+P, Ctrl+N
		if msg.Type == tea.KeyUp || (msg.Type == tea.KeyCtrlP) {
			m.scrollPos -= btwScrollLines
			if m.scrollPos < 0 {
				m.scrollPos = 0
			}
			return m, nil
		}
		if msg.Type == tea.KeyDown || (msg.Type == tea.KeyCtrlN) {
			m.scrollPos += btwScrollLines
			// Clamp scroll position
			if m.response != "" {
				lines := strings.Split(m.response, "\n")
				maxScroll := len(lines) - m.maxHeight
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.scrollPos > maxScroll {
					m.scrollPos = maxScroll
				}
			}
			return m, nil
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.maxHeight = msg.Height - 10 // Reserve space for header/footer
		if m.maxHeight < 5 {
			m.maxHeight = 5
		}
		return m, nil
	}

	return m, nil
}

func (m btwModel) View() string {
	var b strings.Builder

	// Header: /btw <question>
	b.WriteString(ui.Style(&ui.Dark.Warning, nil, "/btw ", true))
	b.WriteString(ui.Style(&ui.Dark.Muted, nil, m.question, true))
	b.WriteString("\n")

	// Content area
	if m.error != "" {
		b.WriteString("\n")
		b.WriteString(ui.Style(&ui.Dark.Error, nil, m.error, true))
	} else if m.response != "" {
		b.WriteString("\n")
		// Render response with scrollable content
		b.WriteString(m.renderScrollableResponse())
	} else if m.loading {
		b.WriteString("\n")
		frame := ui.GetSpinnerFrame(m.frame * int(btwSpinnerInterval.Milliseconds()))
		b.WriteString(ui.Style(&ui.Dark.Warning, nil, frame+" ", true))
		b.WriteString(ui.Style(&ui.Dark.Warning, nil, "Answering...", true))
	}

	// Footer hint (when response or error is shown)
	if m.response != "" || m.error != "" {
		b.WriteString("\n\n")
		b.WriteString(ui.Style(&ui.Dark.Muted, nil, "↑/↓ to scroll · Space, Enter, or Escape to dismiss", true))
	}

	return b.String()
}

func (m btwModel) renderScrollableResponse() string {
	if m.response == "" {
		return ""
	}

	lines := strings.Split(m.response, "\n")

	// Calculate visible range
	start := m.scrollPos
	if start < 0 {
		start = 0
	}
	end := start + m.maxHeight
	if end > len(lines) {
		end = len(lines)
	}

	// Render visible lines
	visibleLines := lines[start:end]
	return strings.Join(visibleLines, "\n")
}

func (m btwModel) tickSpinner() tea.Cmd {
	return tea.Tick(btwSpinnerInterval, func(_ time.Time) tea.Msg {
		return btwTickMsg{}
	})
}

// isCompactBoundaryMessage checks if a message is a compact boundary marker.
// Matches TS: isCompactBoundaryMessage in messages.ts
func isCompactBoundaryMessage(msg types.Message) bool {
	if msg.Type != types.MessageTypeSystem {
		return false
	}
	// Check if content contains subtype compact_boundary
	// The compact boundary is stored as JSON in content
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return false
	}
	// Try to parse as JSON to check subtype
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err == nil {
		if subtype, ok := payload["subtype"].(string); ok {
			return subtype == "compact_boundary" || subtype == "microcompact_boundary"
		}
	}
	// Fallback: check if content contains the subtype string
	return strings.Contains(content, `"subtype":"compact_boundary"`) ||
		strings.Contains(content, `"subtype": "compact_boundary"`) ||
		strings.Contains(content, `"subtype":"microcompact_boundary"`) ||
		strings.Contains(content, `"subtype": "microcompact_boundary"`)
}

// findLastCompactBoundaryIndex finds the index of the last compact boundary marker.
// Matches TS: findLastCompactBoundaryIndex in messages.ts
func findLastCompactBoundaryIndex(messages []types.Message) int {
	// Scan backwards to find the most recent compact boundary
	for i := len(messages) - 1; i >= 0; i-- {
		if isCompactBoundaryMessage(messages[i]) {
			return i
		}
	}
	return -1 // No boundary found
}

// getMessagesAfterCompactBoundary returns messages from the last compact boundary onward.
// Matches TS: getMessagesAfterCompactBoundary in messages.ts
func getMessagesAfterCompactBoundary(messages []types.Message) []types.Message {
	boundaryIndex := findLastCompactBoundaryIndex(messages)
	if boundaryIndex == -1 {
		return messages
	}
	return messages[boundaryIndex:]
}

// stripInProgressAssistantMessage removes the last assistant message if it has no stop reason.
// This matches TS: stripInProgressAssistantMessage in btw.tsx and messages.ts
// An in-progress assistant message has StopReason == "" (null in TS)
func stripInProgressAssistantMessage(messages []types.Message) []types.Message {
	if len(messages) == 0 {
		return messages
	}
	last := messages[len(messages)-1]
	// If the last message is an assistant message with no stop reason, it's in-progress
	// StopReason == "" indicates streaming/in-progress (matches TS: stop_reason === null)
	if last.Role == types.RoleAssistant && last.StopReason == "" {
		return messages[:len(messages)-1]
	}
	return messages
}

// buildSideQuestionMessages builds the message list for a side question.
// Matches TS: buildCacheSafeParams -> forkContextMessages in btw.tsx
// Prepends the system prompt as the first system message.
func buildSideQuestionMessages(baseMessages []types.Message, wrappedQuestion string, systemPrompt string) []types.Message {
	// Strip in-progress assistant message
	stripped := stripInProgressAssistantMessage(baseMessages)

	// Get messages after compact boundary
	forkContextMessages := getMessagesAfterCompactBoundary(stripped)

	// Build the final message list
	var sideMessages []types.Message

	// Prepend system prompt as the first system message
	// Matches TS: systemPrompt in CacheSafeParams passed to runForkedAgent
	if systemPrompt != "" {
		sideMessages = append(sideMessages, types.Message{
			Role:    types.RoleSystem,
			Content: systemPrompt,
		})
	}

	// Add fork context messages
	for _, msg := range forkContextMessages {
		sideMessages = append(sideMessages, msg)
	}

	// If no context messages, just use the side question alone
	if len(forkContextMessages) == 0 {
		sideMessages = append(sideMessages, types.Message{
			Role:    types.RoleUser,
			Content: wrappedQuestion,
		})
		return sideMessages
	}

	// Add the side question as a user message
	sideMessages = append(sideMessages, types.Message{
		Role:    types.RoleUser,
		Content: wrappedQuestion,
	})

	return sideMessages
}

// buildSideQuestionSystemPrompt builds the system prompt for a side question.
// Matches TS: getSystemPrompt -> asSystemPrompt in buildCacheSafeParams
func buildSideQuestionSystemPrompt(rt command.Runtime) string {
	// Use the base system prompt without tools (side questions have no tools)
	// Matches TS: systemPrompt in CacheSafeParams from getSystemPrompt(tools, model, [], mcpClients)
	return prompt.System(rt.Config)
}

func (m btwModel) fetchSideQuestion() tea.Cmd {
	return func() tea.Msg {
		// Debug: check if engine and provider are available
		if m.rt.Engine == nil {
			return btwResponseMsg{err: fmt.Errorf("engine not available")}
		}
		if m.rt.Provider == nil {
			return btwResponseMsg{err: fmt.Errorf("provider not available")}
		}

		// Wrap the question with side question context
		// Matches TS: wrappedQuestion in sideQuestion.ts
		wrappedQuestion := fmt.Sprintf(`<system-reminder>
This is a side question from the user. You must answer this question directly in a single response.

IMPORTANT CONTEXT:
- You are a separate, lightweight agent spawned to answer this one question
- The main agent is NOT interrupted - it continues working independently in the background
- You share the conversation context but are a completely separate instance
- Do NOT reference being interrupted or what you were "previously doing" - that framing is incorrect

CRITICAL CONSTRAINTS:
- You have NO tools available - you cannot read files, run commands, search, or take any actions
- This is a one-off response - there will be no follow-up turns
- You can ONLY provide information based on what you already know from the conversation context
- NEVER say things like "Let me try...", "I'll now...", "Let me check...", or promise to take any action
- If you don't know the answer, say so - do not offer to look it up or investigate

Simply answer the question with the information you have.</system-reminder>

%s`, m.question)

		// Build messages from current engine state + side question
		// Matches TS: buildCacheSafeParams in btw.tsx
		baseMessages := m.rt.Engine.Messages()

		// Build system prompt for side question (no tools)
		// Matches TS: systemPrompt in CacheSafeParams (but without tools for side questions)
		systemPrompt := buildSideQuestionSystemPrompt(m.rt)

		// Build side question messages with compact boundary handling
		// System prompt is prepended as the first system message
		sideMessages := buildSideQuestionMessages(baseMessages, wrappedQuestion, systemPrompt)

		// Build request without tools (side questions cannot use tools)
		// Matches TS: runSideQuestion -> runForkedAgent with canUseTool returning deny
		req := engine.Request{
			Model:    m.rt.Config.Model,
			Messages: sideMessages,
			Tools:    nil, // No tools for side questions - matches TS: canUseTool returns deny
		}

		// Call the provider directly without modifying engine state
		// Matches TS: runForkedAgent -> query
		response, err := m.rt.Provider.Complete(m.ctx, req)
		if err != nil {
			return btwResponseMsg{err: fmt.Errorf("API call failed: %v", err)}
		}

		// Extract text content from response
		textContent := response.Text

		if textContent == "" {
			return btwResponseMsg{response: "No response received from API"}
		}

		return btwResponseMsg{response: textContent}
	}
}

func (m btwModel) dismiss() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.rt.OnLocalJSXDone != nil {
		// Skip adding to transcript when dismissed
		m.rt.OnLocalJSXDone("", command.LocalJSXDoneOptions{Display: "skip"})
	} else if m.rt.OnExit != nil {
		m.rt.OnExit()
	}
}

// Legacy handler for non-TUI fallback
func handleBtw(ctx context.Context, rt command.Runtime, args []string) (command.CommandResult, error) {
	question := strings.Join(args, " ")
	if strings.TrimSpace(question) == "" {
		return command.CommandResult{
			Type:  command.ResultTypeText,
			Value: "btw requires a question argument: /btw <question>",
		}, nil
	}

	// Wrap the question with side question context
	wrappedQuestion := fmt.Sprintf(`<system-reminder>
This is a side question from the user. You must answer this question directly in a single response.

IMPORTANT CONTEXT:
- You are a separate, lightweight agent spawned to answer this one question
- The main agent is NOT interrupted - it continues working independently in the background
- You share the conversation context but are a completely separate instance
- Do NOT reference being interrupted or what you were "previously doing" - that framing is incorrect

CRITICAL CONSTRAINTS:
- You have NO tools available - you cannot read files, run commands, search, or take any actions
- This is a one-off response - there will be no follow-up turns
- You can ONLY provide information based on what you already know from the conversation context
- NEVER say things like "Let me try...", "I'll now...", "Let me check...", or promise to take any action
- If you don't know the answer, say so - do not offer to look it up or investigate

Simply answer the question with the information you have.</system-reminder>

%s`, question)

	if rt.Engine == nil || rt.Provider == nil {
		return command.CommandResult{
			Type:  command.ResultTypeText,
			Value: "Side question failed: engine or provider not available",
		}, nil
	}

	// Build messages from current engine state + side question
	baseMessages := rt.Engine.Messages()

	// Build system prompt for side question
	systemPrompt := buildSideQuestionSystemPrompt(rt)

	// Build side question messages with compact boundary handling
	sideMessages := buildSideQuestionMessages(baseMessages, wrappedQuestion, systemPrompt)

	// Build request without tools
	req := engine.Request{
		Model:    rt.Config.Model,
		Messages: sideMessages,
		Tools:    nil,
	}

	// Call the provider directly
	response, err := rt.Provider.Complete(ctx, req)
	if err != nil {
		return command.CommandResult{
			Type:  command.ResultTypeText,
			Value: fmt.Sprintf("Side question failed: %v", err),
		}, nil
	}

	// Build the result text
	resultText := fmt.Sprintf("Side Question: %s\n\n%s", question, response.Text)

	return command.CommandResult{
		Type:    command.ResultTypeText,
		Value:   resultText,
		Display: "user", // Show in user-visible transcript
	}, nil
}