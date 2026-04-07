package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"claude-code-go/internal/app"
	"claude-code-go/internal/command"
	"claude-code-go/internal/components"
	"claude-code-go/internal/engine"
	mcpinfra "claude-code-go/internal/infra/mcp"
	"claude-code-go/internal/services"
	"claude-code-go/internal/task"
	"claude-code-go/internal/types"
	"claude-code-go/internal/ui"
	"claude-code-go/internal/ui/collapse"

	tea "github.com/charmbracelet/bubbletea"
)

type ChatRunner struct {
	app        *app.App
	stdout     io.Writer
	stderr     io.Writer
	components *components.ChatApp
	entries    []components.TranscriptEntry
	mu         sync.RWMutex

	// Channel for streaming updates
	streamChan chan streamUpdate

	cancelMu     sync.Mutex
	activeCancel context.CancelFunc
}

type streamUpdate struct {
	text       string
	toolName   string
	toolCallID string
	toolInput  string // JSON tool arguments
	toolResult string
	toolCalls  []engine.StreamChunk
	status     string
	done       bool
	refresh    bool
	err        error
}

func CreateChatRunner(application *app.App, stdout, stderr io.Writer) *ChatRunner {
	return &ChatRunner{
		app:        application,
		stdout:     stdout,
		stderr:     stderr,
		components: components.ChatAppFor(),
		streamChan: make(chan streamUpdate, 100),
	}
}

func (r *ChatRunner) Run(ctx context.Context) error {
	r.drainNotices()
	program := tea.NewProgram(
		createChatModel(ctx, r),
		tea.WithContext(ctx),
		tea.WithInput(r.app.Input()),
		tea.WithOutput(r.stdout),
		// Note: Mouse capture disabled to enable:
		// - Native terminal scroll wheel
		// - Text selection and copy
		// The scrollback mode uses tea.Println() which works with native terminal features
	)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-sigCh:
				if sig == nil {
					return
				}
				program.Send(interruptMsg{signal: sig.String()})
			}
		}
	}()
	_, err := program.Run()
	if err == tea.ErrProgramKilled || err == io.EOF {
		return nil
	}
	return err
}

func (r *ChatRunner) drainNotices() {
	r.drainAgentNotices()
	r.drainPluginNotices()
}

func (r *ChatRunner) Render(currentInput string, width, height int, state renderState, mode components.ViewMode, lastThinkingBlockID string, latestBashOutputUUID string) string {
	displayInput := currentInput
	if state.busy {
		displayInput += " ..."
	}

	r.mu.RLock()
	entries := make([]components.TranscriptEntry, len(r.entries))
	copy(entries, r.entries)
	r.mu.RUnlock()

	// Track in-progress tool IDs for active state display
	// A tool is in-progress if tool_use exists but no matching tool_result
	inProgressToolIDs := computeInProgressToolIDs(entries)

	// Set IsActive on tool_use entries BEFORE collapse runs
	// This allows collapse logic to inherit active state
	for i := range entries {
		if entries[i].Kind == "tool_use" && entries[i].ToolUseID != "" {
			entries[i].IsActive = inProgressToolIDs[entries[i].ToolUseID]
		}
	}

	// Apply transformation pipeline (matching TS Messages.tsx:379-521)
	// Step 1: Normalize and filter - already done in buildTranscriptEntries

	// Step 2: Group tool uses (non-verbose only)
	// Evidence: src/components/Messages.tsx:517, src/utils/groupToolUses.ts:52-64
	verbose := mode == components.ViewModeVerbose || mode == components.ViewModeTranscript
	if !verbose {
		entries = collapse.GroupToolUses(entries, r.app.Services().Tools(), verbose)
	}

	// Step 3: Collapse read/search groups (non-verbose only)
	// Evidence: src/components/Messages.tsx:518-521
	entries = collapse.ReadSearchGroups(entries, r.app.Services().Tools(), verbose)

	// Update IsActive on any remaining tool_use/grouped_tool_use entries
	// (in case they weren't collapsed)
	for i := range entries {
		if entries[i].ToolUseID != "" && inProgressToolIDs[entries[i].ToolUseID] {
			entries[i].IsActive = true
		}
	}

	// TODO: Step 4 - Collapse teammate shutdowns
	// entries = collapse.TeammateShutdowns(entries)

	// TODO: Step 5 - Collapse hook summaries
	// entries = collapse.HookSummaries(entries)

	// TODO: Step 6 - Collapse background bash notifications (non-verbose only)
	// if !verbose {
	//     entries = collapse.BackgroundBashNotifications(entries)
	// }

	return r.components.Render(components.ChatProps{
		Version:              r.app.Version(),
		Config:               r.app.Config(),
		Width:                width,
		Height:               height,
		State:                r.app.State().Snapshot(),
		Entries:              entries,
		CurrentInput:         displayInput,
		Suggestions:          buildSlashSuggestions(currentInput, r.app.Commands()),
		SelectedSuggestion:   -1,
		Busy:                 state.busy,
		SpinnerTick:          state.spinnerTick,
		StreamingText:        state.streamingText,
		ToolName:             state.toolName,
		ToolCallID:           state.toolCallID,
		StatusText:           state.statusText,
		StartedAt:            state.startedAt,
		Verb:                 state.verb,
		TokenCount:           state.tokenCount,
		Mode:                 mode,
		TranscriptScroll:     state.transcriptScroll,
		LastThinkingBlockID:  lastThinkingBlockID,
		LatestBashOutputUUID: latestBashOutputUUID,
		Teammates:            state.teammates,
		TeammateLeaderVerb:   state.teammateVerb,
		TeammateLeaderTokens: state.teammateTokens,
		InProgressToolIDs:    inProgressToolIDs,
	})
}

type renderState struct {
	busy             bool
	streamingText    string
	spinnerTick      int
	toolName         string
	toolCallID       string // ID of the currently streaming tool call
	toolInput        string // JSON arguments of the streaming tool
	statusText       string
	startedAt        time.Time
	toolInProgress   bool
	verb             string // Randomly selected verb, constant during request
	tokenCount       int    // Current token count for display
	transcriptScroll int
	teammates        []ui.TeammateSpinnerNode
	teammateVerb     string
	teammateTokens   int
}

func (r *ChatRunner) appendError(line string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.app.State().RecordError(err)
	if strings.TrimSpace(line) != "" {
		r.entries = append(r.entries, components.TranscriptEntry{Kind: "user", Title: "You", Content: line})
	}
	r.entries = append(r.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
}

func cloneEntries(entries []components.TranscriptEntry) []components.TranscriptEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]components.TranscriptEntry, len(entries))
	copy(out, entries)
	return out
}

func (r *ChatRunner) setActiveCancel(cancel context.CancelFunc) {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	r.activeCancel = cancel
}

func (r *ChatRunner) clearActiveCancel(cancel context.CancelFunc) {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	r.activeCancel = nil
}

func (r *ChatRunner) cancelActive() bool {
	r.cancelMu.Lock()
	cancel := r.activeCancel
	r.activeCancel = nil
	r.cancelMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (r *ChatRunner) finalizeStreamingEntry(streamingText string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.entries) == 0 {
		return
	}
	last := &r.entries[len(r.entries)-1]
	if last.Kind != "assistant_streaming" {
		return
	}
	if strings.TrimSpace(streamingText) == "" {
		r.entries = r.entries[:len(r.entries)-1]
		return
	}
	last.Kind = "assistant"
	last.Content = streamingText
}

// StreamingToolInfo holds info about a currently streaming tool call
type StreamingToolInfo struct {
	ToolName   string
	ToolCallID string
	ToolInput  string // JSON arguments for the tool
	InProgress bool
}

func (r *ChatRunner) syncEntriesFromEngine(streamingText string) bool {
	return r.syncEntriesFromEngineWithTool(streamingText, StreamingToolInfo{})
}

// syncEntriesFromEngineWithTool syncs entries from engine and optionally adds streaming tool entry
// Returns true if a new streaming tool entry was added
func (r *ChatRunner) syncEntriesFromEngineWithTool(streamingText string, streamingTool StreamingToolInfo) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	aux := make([]components.TranscriptEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		switch entry.Kind {
		case "panel", "command", "error":
			aux = append(aux, entry)
		case "notice":
			if entry.Title != "System" {
				aux = append(aux, entry)
			}
		}
	}

	oldEntriesCount := len(r.entries)
	entries := buildTranscriptEntries(r.app.Engine().Messages(), streamingText)
	addedStreamingTool := false

	// Add synthetic streaming tool_use entry if a tool is in progress
	// This shows the tool call before it's added to the engine messages
	if streamingTool.InProgress && streamingTool.ToolName != "" {
		// Check if this tool call is not already in the entries
		alreadyExists := false
		for _, e := range entries {
			if e.Kind == "tool_use" && e.ToolUseID == streamingTool.ToolCallID {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			// Summarize tool input for display
			displayName, summary := summarizeToolUseDisplay(streamingTool.ToolName, streamingTool.ToolInput)
			entries = append(entries, components.TranscriptEntry{
				Kind:      "tool_use",
				Title:     displayName,
				Content:   summary,
				UUID:      "streaming-tool-" + streamingTool.ToolCallID,
				ToolName:  displayName,
				ToolUseID: streamingTool.ToolCallID,
				IsActive:  true, // Mark as active since it's streaming
			})
			addedStreamingTool = true
		}
	}

	r.entries = append(entries, aux...)

	// Return true if we added a new streaming tool OR if entries increased
	return addedStreamingTool || len(r.entries) > oldEntriesCount
}

func (r *ChatRunner) sendStreamUpdate(update streamUpdate, critical bool) {
	if !critical {
		select {
		case r.streamChan <- update:
		default:
		}
		return
	}

	select {
	case r.streamChan <- update:
		return
	default:
	}

	select {
	case <-r.streamChan:
	default:
	}

	select {
	case r.streamChan <- update:
	default:
	}
}

type chatModel struct {
	ctx                context.Context
	runner             *ChatRunner
	currentInput       string
	selectedSuggestion int
	width              int
	height             int
	state              renderState
	mode               components.ViewMode // View mode (normal, verbose, transcript)
	// Thinking block visibility tracking
	lastThinkingBlockID     string    // UUID of the last thinking block to show
	streamingThinkingEndsAt time.Time // When streaming thinking grace period ends
	// Scrollback mode: track how many entries have been printed to terminal scrollback
	printedEntriesCount int
}

type submitDoneMsg struct {
	err error
}

type noticeTickMsg struct{}

type spinnerTickMsg struct{}

type streamUpdateMsg struct {
	text       string
	toolName   string
	toolCallID string
	toolInput  string // JSON tool arguments
	toolResult string
	toolCalls  []engine.StreamChunk
	status     string
	done       bool
	refresh    bool
	err        error
}

type interruptMsg struct {
	signal string
}

func createChatModel(ctx context.Context, runner *ChatRunner) chatModel {
	// Get initial terminal size instead of using hardcoded values
	// This matches the original TS behavior where process.stdout.columns is used
	size := ui.GetTerminalSize()
	return chatModel{
		ctx:                ctx,
		runner:             runner,
		selectedSuggestion: -1,
		width:              size.Width,
		height:             size.Height,
		mode:               components.ViewModeNormal,
	}
}

func (m chatModel) Init() tea.Cmd {
	// Print header immediately to scrollback, then start normal operations
	width := maxInt(72, m.width)
	snapshot := m.runner.app.State().Snapshot()
	header := ui.RenderHeader(width, m.runner.app.Version(), m.runner.app.Config(), snapshot, snapshot.SessionID, snapshot.TurnCount)
	return tea.Batch(
		tea.Println(header),
		tickNoticesCmd(),
		spinnerTickCmd(),
		listenForStreamUpdates(m.runner),
	)
}

func (m *chatModel) resetBusyState() {
	m.state.busy = false
	m.state.streamingText = ""
	m.state.toolName = ""
	m.state.toolCallID = ""
	m.state.toolInput = ""
	m.state.statusText = ""
	m.state.startedAt = time.Time{}
	m.state.toolInProgress = false
	m.state.verb = ""
	m.state.tokenCount = 0
	m.state.teammates = nil
	m.state.teammateVerb = ""
	m.state.teammateTokens = 0

	// Set 30-second grace period for thinking block visibility
	m.streamingThinkingEndsAt = time.Now().Add(30 * time.Second)

	// Find the last thinking block in entries
	m.runner.mu.RLock()
	for i := len(m.runner.entries) - 1; i >= 0; i-- {
		entry := m.runner.entries[i]
		if entry.Kind == "thinking" || entry.Kind == "redacted_thinking" {
			m.lastThinkingBlockID = entry.UUID
			break
		}
	}
	m.runner.mu.RUnlock()
}

func (m *chatModel) refreshTeammateSpinnerState() {
	tasks := m.runner.app.Agents().Tasks().List()
	nodes := make([]ui.TeammateSpinnerNode, 0, len(tasks))
	for _, agentTask := range tasks {
		if !agentTask.Background {
			continue
		}
		if agentTask.Status != task.StatusPending && agentTask.Status != task.StatusRunning {
			continue
		}
		activity := "working"
		if agentTask.Status == task.StatusPending {
			activity = "pending"
		}
		if strings.TrimSpace(agentTask.Description) != "" {
			activity = strings.TrimSpace(agentTask.Description)
		}
		nodes = append(nodes, ui.TeammateSpinnerNode{
			Name:           strings.TrimSpace(agentTask.AgentType),
			Activity:       activity,
			TokenCount:     0,
			ToolUseCount:   0,
			IsIdle:         agentTask.Status == task.StatusPending,
			IsForegrounded: false,
		})
	}
	m.state.teammates = nodes
	m.state.teammateVerb = m.state.verb
	m.state.teammateTokens = m.state.tokenCount
}

// flushNewEntriesToScrollback prints any new completed entries to terminal scrollback
// and returns a Cmd to do so. This enables native terminal scrolling.
func (m *chatModel) flushNewEntriesToScrollback() tea.Cmd {
	m.runner.mu.RLock()
	entries := make([]components.TranscriptEntry, len(m.runner.entries))
	copy(entries, m.runner.entries)
	m.runner.mu.RUnlock()

	// Skip if nothing new to print
	if len(entries) <= m.printedEntriesCount {
		return nil
	}

	// Find entries to print (exclude streaming and user entries - user already printed on Enter)
	var toPrint []components.TranscriptEntry
	nextPrintedCount := m.printedEntriesCount
	for i := m.printedEntriesCount; i < len(entries); i++ {
		entry := entries[i]
		// Streaming entry is a barrier: don't advance beyond it.
		// A later sync replaces this same index with final assistant text.
		// If we continue scanning and advance past it, the final text won't print.
		if entry.Kind == "assistant_streaming" {
			break
		}
		// Skip user entries - already printed immediately on Enter
		if entry.Kind == "user" {
			nextPrintedCount = i + 1
			continue
		}
		toPrint = append(toPrint, entry)
		nextPrintedCount = i + 1
	}

	if len(toPrint) == 0 {
		m.printedEntriesCount = nextPrintedCount
		return nil
	}

	// Render and print
	width := maxInt(72, m.width)
	rendered := ui.RenderTranscript(width, 0, toPrint, m.mode, 0, m.lastThinkingBlockID, m.findLatestBashOutputUUID())
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		m.printedEntriesCount = nextPrintedCount
		return nil
	}

	m.printedEntriesCount = nextPrintedCount

	return tea.Println(rendered)
}

// updateVerbFromCurrentTask updates the spinner verb based on current task's activeForm
// Matches TS logic: overrideMessage ?? currentTodo?.activeForm ?? currentTodo?.subject ?? randomVerb
func (m *chatModel) updateVerbFromCurrentTask() {
	// Get current in-progress task from TaskList via Engine's Runtime
	runtime := m.runner.app.Engine().Runtime()
	if runtime.TaskList == nil {
		return
	}

	tasks, err := runtime.TaskList.List()
	if err != nil || len(tasks) == 0 {
		return
	}

	// Find first non-pending, non-completed task (currently in progress)
	for _, task := range tasks {
		if task.Status != "pending" && task.Status != "completed" {
			// Use activeForm if available, otherwise use subject
			if task.ActiveForm != "" {
				m.state.verb = task.ActiveForm
				return
			}
			if task.Subject != "" {
				m.state.verb = task.Subject
				return
			}
		}
	}
	// If no in-progress task found, keep the random verb (already set)
}

// listenForStreamUpdates waits for the next streaming update from the channel.
// Blocking here avoids a constant refresh loop that can interfere with native
// terminal scrollback/mouse-wheel scrolling in main-screen mode.
func listenForStreamUpdates(runner *ChatRunner) tea.Cmd {
	return func() tea.Msg {
		update := <-runner.streamChan
		return streamUpdateMsg{
			text:       update.text,
			toolName:   update.toolName,
			toolCallID: update.toolCallID,
			toolInput:  update.toolInput,
			toolResult: update.toolResult,
			status:     update.status,
			done:       update.done,
			refresh:    update.refresh,
			err:        update.err,
		}
	}
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case noticeTickMsg:
		m.runner.mu.Lock()
		m.runner.entries = cloneEntries(m.runner.entries)
		m.runner.mu.Unlock()
		m.runner.drainNotices()
		return m, tickNoticesCmd()
	case spinnerTickMsg:
		if m.state.busy {
			// Increment by 50ms to match TS useAnimationFrame(50) interval
			m.state.spinnerTick += 50
			// Update verb from current task's activeForm (matching TS logic)
			m.updateVerbFromCurrentTask()
			m.refreshTeammateSpinnerState()
			return m, spinnerTickCmd()
		}
		return m, nil
	case streamUpdateMsg:
		if msg.err != nil {
			// Stream ended with error - entries already synced by goroutine
			m.resetBusyState()
			return m, tea.Batch(
				listenForStreamUpdates(m.runner),
				m.flushNewEntriesToScrollback(),
			)
		}
		if msg.done {
			// Stream completed - entries already synced with resp.Text by goroutine
			// DO NOT call syncEntriesFromEngine("") here - it would lose the streaming text
			m.resetBusyState()
			return m, tea.Batch(
				listenForStreamUpdates(m.runner),
				m.flushNewEntriesToScrollback(),
			)
		}
		if msg.text != "" || msg.toolName != "" || msg.status != "" {
			// Streaming update
			m.state.streamingText += msg.text
			// Update token count based on streaming text length (approximate tokens)
			m.state.tokenCount = len(m.state.streamingText) / 4
			m.state.teammateTokens = m.state.tokenCount
			if msg.toolName != "" {
				m.state.toolName = msg.toolName
				m.state.toolCallID = msg.toolCallID
				m.state.toolInput = msg.toolInput
				m.state.toolInProgress = true
			}
			if msg.status != "" {
				m.state.statusText = msg.status
			} else if msg.text != "" && !m.state.toolInProgress {
				m.state.statusText = "Receiving response"
			}
			// Sync entries with streaming tool info for live tool display
			addedNew := m.runner.syncEntriesFromEngineWithTool(m.state.streamingText, StreamingToolInfo{
				ToolName:   m.state.toolName,
				ToolCallID: m.state.toolCallID,
				ToolInput:  m.state.toolInput,
				InProgress: m.state.toolInProgress,
			})
			// If a new streaming tool was added, flush it to scrollback immediately
			if addedNew {
				return m, tea.Batch(
					listenForStreamUpdates(m.runner),
					m.flushNewEntriesToScrollback(),
				)
			}
		} else if msg.refresh {
			// Refresh also needs streaming tool info
			addedNew := m.runner.syncEntriesFromEngineWithTool(m.state.streamingText, StreamingToolInfo{
				ToolName:   m.state.toolName,
				ToolCallID: m.state.toolCallID,
				ToolInput:  m.state.toolInput,
				InProgress: m.state.toolInProgress,
			})
			// If a new streaming tool was added, flush it to scrollback immediately
			if addedNew {
				return m, tea.Batch(
					listenForStreamUpdates(m.runner),
					m.flushNewEntriesToScrollback(),
				)
			}
		}
		return m, listenForStreamUpdates(m.runner)
	case interruptMsg:
		if m.state.busy {
			if m.runner.cancelActive() {
				m.runner.finalizeStreamingEntry(m.state.streamingText)
				m.resetBusyState()
				return m, nil
			}
		}
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.Type {
		// Removed: PgUp, PgDown, Up, Down, Home, End, CtrlU, CtrlD scroll handling
		// Terminal native scrolling is now used via scrollback output
		case tea.KeyCtrlC:
			if m.state.busy {
				if m.runner.cancelActive() {
					m.runner.finalizeStreamingEntry(m.state.streamingText)
					m.resetBusyState()
				}
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyEsc:
			if m.state.busy {
				if m.runner.cancelActive() {
					m.runner.finalizeStreamingEntry(m.state.streamingText)
					m.resetBusyState()
				}
				return m, nil
			}
			return m, nil
		case tea.KeyCtrlO:
			// Toggle transcript mode
			if m.mode == components.ViewModeTranscript {
				m.mode = components.ViewModeNormal
			} else {
				m.mode = components.ViewModeTranscript
			}
			return m, nil
		case tea.KeyTab, tea.KeyShiftTab:
			if m.state.busy {
				return m, nil
			}
			suggestions := buildSlashSuggestions(m.currentInput, m.runner.app.Commands())
			if len(suggestions) == 0 {
				m.selectedSuggestion = -1
				return m, nil
			}
			delta := 1
			if msg.Type == tea.KeyShiftTab {
				delta = -1
			}
			m.advanceSuggestionSelection(len(suggestions), delta)
			if m.selectedSuggestion >= 0 && m.selectedSuggestion < len(suggestions) {
				m.currentInput = applySlashSuggestion(m.currentInput, suggestions[m.selectedSuggestion].Command)
			}
			return m, nil
		case tea.KeyEnter:
			if !m.state.busy && m.applySelectedSuggestionIfActive() {
				return m, nil
			}
			line := strings.TrimSpace(m.currentInput)
			if m.state.busy || line == "" {
				return m, nil
			}
			switch line {
			case "/exit", "/quit":
				return m, tea.Quit
			case "/clear":
				m.currentInput = ""
				m.selectedSuggestion = -1
				m.runner.mu.Lock()
				m.runner.entries = nil
				m.printedEntriesCount = 0 // Reset scrollback tracking
				m.runner.mu.Unlock()
				return m, nil
			}

			// Check if this is a slash command
			if strings.HasPrefix(line, "/") {
				m.currentInput = ""
				m.selectedSuggestion = -1
				turnCtx, cancel := context.WithCancel(m.ctx)
				m.runner.setActiveCancel(cancel)
				return m, tea.Batch(
					runSlashCommandCmd(turnCtx, m.runner, line, cancel),
				)
			}

			m.state.busy = true
			m.state.streamingText = ""
			m.state.toolName = ""
			m.state.toolCallID = ""
			m.state.statusText = "Waiting for model response"
			m.state.startedAt = time.Now()
			m.state.toolInProgress = false
			// Select verb ONCE at request start (matching TS: useState(() => sample(getSpinnerVerbs())))
			m.state.verb = ui.GetRandomVerb()
			m.state.teammateVerb = m.state.verb
			m.state.tokenCount = 0
			m.refreshTeammateSpinnerState()
			m.state.transcriptScroll = 0
			m.selectedSuggestion = -1
			// Print user message to scrollback immediately
			userEntry := components.TranscriptEntry{Kind: "user", Title: "You", Content: line}
			userRendered := ui.RenderTranscript(maxInt(72, m.width), 0, []components.TranscriptEntry{userEntry}, m.mode, 0, "", "")
			m.currentInput = ""
			turnCtx, cancel := context.WithCancel(m.ctx)
			m.runner.setActiveCancel(cancel)
			return m, tea.Batch(
				tea.Println(strings.TrimSpace(userRendered)),
				submitLineStreamCmd(turnCtx, m.runner, line, cancel),
				spinnerTickCmd(),
			)
		case tea.KeyBackspace, tea.KeyDelete:
			if !m.state.busy {
				m.currentInput = dropLastRune(m.currentInput)
				m.selectedSuggestion = -1
			}
			return m, nil
		default:
			if !m.state.busy {
				if msg.Type == tea.KeyRunes {
					m.currentInput += string(msg.Runes)
					m.selectedSuggestion = -1
				} else if s := msg.String(); s != "" && utf8.ValidString(s) && !strings.HasPrefix(s, "ctrl+") {
					m.currentInput += s
					m.selectedSuggestion = -1
				}
			}
			return m, nil
		}
		// Removed: MouseMsg scroll wheel handling - terminal native scrolling is used
	}
	return m, nil
}

func (m chatModel) View() string {
	// In scrollback mode: only render current streaming content + input area
	// Historical content has already been printed to terminal scrollback via tea.Println()

	width := maxInt(72, m.width)

	// Build the view with only active/streaming content
	var parts []string

	// If there's active streaming content, show it
	if m.state.busy && m.state.streamingText != "" {
		// Show streaming assistant response
		streamEntry := components.TranscriptEntry{
			Kind:    "assistant_streaming",
			Title:   "Claude",
			Content: m.state.streamingText,
		}
		rendered := ui.RenderTranscript(width, 0, []components.TranscriptEntry{streamEntry}, m.mode, 0, "", "")
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}

	// Always show input area at the bottom
	suggestions := buildSlashSuggestions(m.currentInput, m.runner.app.Commands())
	if m.selectedSuggestion >= len(suggestions) {
		m.selectedSuggestion = -1
	}
	input := ui.RenderInputPanel(width, m.runner.app.State().Snapshot(), m.currentInput, suggestions, m.selectedSuggestion, m.state.busy, m.state.spinnerTick, m.state.toolName, m.state.statusText, m.state.startedAt, m.state.verb, m.state.tokenCount, 0, m.state.teammates, m.state.teammateVerb, m.state.teammateTokens)
	parts = append(parts, input)

	return strings.Join(parts, "\n")
}

func (m *chatModel) advanceSuggestionSelection(total int, delta int) {
	if total <= 0 {
		m.selectedSuggestion = -1
		return
	}
	if m.selectedSuggestion < 0 || m.selectedSuggestion >= total {
		if delta < 0 {
			m.selectedSuggestion = total - 1
		} else {
			m.selectedSuggestion = 0
		}
		return
	}
	next := m.selectedSuggestion + delta
	if next < 0 {
		next = total - 1
	}
	if next >= total {
		next = 0
	}
	m.selectedSuggestion = next
}

func (m *chatModel) applySelectedSuggestionIfActive() bool {
	suggestions := buildSlashSuggestions(m.currentInput, m.runner.app.Commands())
	if len(suggestions) == 0 || m.selectedSuggestion < 0 || m.selectedSuggestion >= len(suggestions) {
		return false
	}
	m.currentInput = applySlashSuggestion(m.currentInput, suggestions[m.selectedSuggestion].Command)
	m.selectedSuggestion = -1
	return true
}

// findLatestBashOutputUUID finds the UUID of the most recent bash tool result
func (m chatModel) findLatestBashOutputUUID() string {
	m.runner.mu.RLock()
	defer m.runner.mu.RUnlock()

	// Scan backwards to find the most recent bash tool result
	for i := len(m.runner.entries) - 1; i >= 0; i-- {
		entry := m.runner.entries[i]
		if entry.Kind == "tool_result" && entry.ToolName == "Bash" {
			return entry.UUID
		}
		// Also check for user messages containing bash output
		if entry.Kind == "user" && (strings.Contains(entry.Content, "<bash-stdout") || strings.Contains(entry.Content, "<bash-stderr")) {
			return entry.UUID
		}
	}
	return ""
}

func submitLineStreamCmd(ctx context.Context, runner *ChatRunner, line string, cancel context.CancelFunc) tea.Cmd {
	// Add user message and placeholder immediately
	runner.mu.Lock()
	runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "user", Title: "You", Content: line})
	runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "assistant_streaming", Title: "Claude", Content: ""})
	runner.mu.Unlock()

	// Start streaming in a goroutine
	go func() {
		defer runner.clearActiveCancel(cancel)
		start := time.Now()
		runner.sendStreamUpdate(streamUpdate{status: "Waiting for model response"}, false)
		_, err := runner.app.Engine().SubmitStream(ctx, line, func(chunk engine.StreamChunk) error {
			toolName := ""
			toolInput := ""
			status := ""
			if len(chunk.ToolCalls) > 0 {
				toolName = chunk.ToolCalls[0].Name
				// Capture tool input as JSON for display
				if inputBytes, err := json.Marshal(chunk.ToolCalls[0].Input); err == nil {
					toolInput = string(inputBytes)
				}
				if chunk.Status != "" {
					status = chunk.Status
				} else {
					status = "Running tool: " + toolName
				}
			} else if chunk.Text != "" {
				status = "Receiving response"
			} else if chunk.Status != "" {
				status = chunk.Status
			}
			// Send update through channel (non-blocking)
			runner.sendStreamUpdate(streamUpdate{
				text:       chunk.Text,
				toolName:   toolName,
				toolCallID: chunk.ToolCallID,
				toolInput:  toolInput,
				toolResult: chunk.ToolResult,
				status:     status,
				refresh:    len(chunk.ToolCalls) > 0 || chunk.ToolResult != "",
			}, false)
			return nil
		})

		if err != nil {
			runner.mu.Lock()
			if len(runner.entries) > 0 && runner.entries[len(runner.entries)-1].Kind == "assistant_streaming" {
				runner.entries = runner.entries[:len(runner.entries)-1]
			}
			if errors.Is(err, context.Canceled) {
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Stopped", Content: "Current request was canceled."})
			} else {
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
			}
			runner.mu.Unlock()
			runner.sendStreamUpdate(streamUpdate{err: err}, true)
			return
		}

		runner.app.State().SetSessionID(runner.app.Engine().SessionID())
		runner.app.State().RecordTurn(runner.app.Config().Model, time.Since(start))
		// Don't pass resp.Text as streamingText - the assistant message is already in engine.Messages
		// Passing it would create a duplicate assistant_streaming entry
		runner.syncEntriesFromEngine("")

		runner.drainNotices()
		runner.sendStreamUpdate(streamUpdate{done: true}, true)
	}()

	// Return immediately - updates come through the channel
	return func() tea.Msg {
		// Small delay to let the goroutine start
		time.Sleep(10 * time.Millisecond)
		return streamUpdateMsg{}
	}
}

func runSlashCommandCmd(ctx context.Context, runner *ChatRunner, line string, cancel context.CancelFunc) tea.Cmd {
	return func() tea.Msg {
		defer runner.clearActiveCancel(cancel)

		// Check if this is a prompt command that should be streamed to the model
		cmd, ok := runner.app.Commands().Lookup(line)

		if ok && cmd.GetKind() == command.KindPrompt {
			// For prompt commands, expand the prompt and stream to model
			runtime := runner.buildCommandRuntime(ctx)
			out, handled, err := runner.app.Commands().Execute(ctx, line, runtime)
			if err != nil {
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
				runner.mu.Unlock()
				return streamUpdateMsg{}
			}
			if handled && out.Value != "" {
				name := strings.TrimPrefix(strings.Fields(line)[0], "/")
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "command", Title: "Prompt /" + name, Content: out.Value})
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "user", Title: "You", Content: out.Value})
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "assistant_streaming", Title: "Claude", Content: ""})
				runner.mu.Unlock()

				// Stream the expanded prompt to the model
				start := time.Now()
				_, err := runner.app.Engine().SubmitStream(ctx, out.Value, func(chunk engine.StreamChunk) error {
					runner.sendStreamUpdate(streamUpdate{text: chunk.Text, status: "Receiving response"}, false)
					return nil
				})
				if err != nil {
					runner.mu.Lock()
					if len(runner.entries) > 0 && runner.entries[len(runner.entries)-1].Kind == "assistant_streaming" {
						runner.entries = runner.entries[:len(runner.entries)-1]
					}
					if errors.Is(err, context.Canceled) {
						runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Stopped", Content: "Current request was canceled."})
					} else {
						runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
					}
					runner.mu.Unlock()
					runner.sendStreamUpdate(streamUpdate{err: err}, true)
					return streamUpdateMsg{}
				}
				runner.app.State().SetSessionID(runner.app.Engine().SessionID())
				runner.app.State().RecordTurn(runner.app.Config().Model, time.Since(start))
				// Don't pass resp.Text as streamingText - the assistant message is already in engine.Messages
				runner.syncEntriesFromEngine("")
				runner.drainNotices()
				runner.sendStreamUpdate(streamUpdate{done: true}, true)
				return streamUpdateMsg{}
			}
			return streamUpdateMsg{}
		}

		// For local commands, execute and display output
		runtime := runner.buildCommandRuntime(ctx)
		out, handled, err := runner.app.Commands().Execute(ctx, line, runtime)
		if err != nil {
			runner.mu.Lock()
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
			runner.mu.Unlock()
			return streamUpdateMsg{}
		}
		if handled {
			name := strings.TrimPrefix(strings.Fields(line)[0], "/")
			runner.mu.Lock()
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "command", Title: "Command /" + name, Content: out.Value})
			runner.mu.Unlock()
			return streamUpdateMsg{}
		}

		// Unknown command
		runner.mu.Lock()
		runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Unknown Command", Content: line})
		runner.mu.Unlock()
		return streamUpdateMsg{}
	}
}

func buildTranscriptEntries(messages []types.Message, streamingText string) []components.TranscriptEntry {
	entries := make([]components.TranscriptEntry, 0, len(messages))
	toolNames := map[string]string{}

	for idx, msg := range messages {
		uuid := fmt.Sprintf("msg-%d", idx)
		switch msg.Role {
		case types.RoleUser:
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}

			// Filter control messages (matching TS UserTextMessage.tsx:39-59)
			if isControlMessage(msg.Content) {
				continue
			}

			entries = append(entries, components.TranscriptEntry{
				Kind:      "user",
				Title:     "You",
				Content:   msg.Content,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
			})
		case types.RoleAssistant:
			if strings.TrimSpace(msg.Content) != "" {
				entries = append(entries, components.TranscriptEntry{
					Kind:      "assistant",
					Title:     "Claude",
					Content:   msg.Content,
					UUID:      uuid,
					Timestamp: msg.Timestamp,
				})
			}
			for i, call := range msg.ToolCalls {
				toolNames[call.ID] = call.Name
				displayName, summary := summarizeToolUseDisplay(call.Name, call.Arguments)
				entries = append(entries, components.TranscriptEntry{
					Kind:      "tool_use",
					Title:     displayName,
					Content:   summary,
					UUID:      fmt.Sprintf("%s-tool-%d", uuid, i),
					Timestamp: msg.Timestamp,
					ToolName:  displayName,
					ToolUseID: call.ID,
				})
			}
		case types.RoleTool:
			entries = append(entries, components.TranscriptEntry{
				Kind:      "tool_result",
				Content:   msg.Content,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
				ToolName:  toolNames[msg.ToolCallID],
				ToolUseID: msg.ToolCallID,
			})
		case types.RoleSystem:
			if strings.HasPrefix(strings.TrimSpace(msg.Content), "hook_reports:") {
				continue
			}
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			entries = append(entries, components.TranscriptEntry{
				Kind:      "notice",
				Title:     "System",
				Content:   msg.Content,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
			})
		}
	}

	if strings.TrimSpace(streamingText) != "" {
		entries = append(entries, components.TranscriptEntry{
			Kind:    "assistant_streaming",
			Title:   "Claude",
			Content: streamingText,
			UUID:    "streaming",
		})
	}

	return entries
}

// computeInProgressToolIDs finds all tool_use IDs that don't have a matching tool_result
// This indicates tools that are still executing
func computeInProgressToolIDs(entries []components.TranscriptEntry) map[string]bool {
	// Collect all tool_use IDs
	toolUseIDs := make(map[string]bool)
	for _, e := range entries {
		if e.Kind == "tool_use" && e.ToolUseID != "" {
			toolUseIDs[e.ToolUseID] = true
		}
	}

	// Remove IDs that have a tool_result
	for _, e := range entries {
		if e.Kind == "tool_result" && e.ToolUseID != "" {
			delete(toolUseIDs, e.ToolUseID)
		}
	}

	return toolUseIDs
}

// isControlMessage filters out UI control messages that should not be displayed
// Matches TS logic from UserTextMessage.tsx:39-59
func isControlMessage(content string) bool {
	trimmed := strings.TrimSpace(content)
	// NO_CONTENT_MESSAGE sentinel
	if trimmed == "NO_CONTENT_MESSAGE" {
		return true
	}
	// <tick> markers
	if strings.HasPrefix(trimmed, "<tick>") && strings.HasSuffix(trimmed, "</tick>") {
		return true
	}
	// <local-command-caveat> markers
	if strings.HasPrefix(trimmed, "<local-command-caveat>") {
		return true
	}
	return false
}

// summarizeToolArguments creates a human-readable summary of tool arguments
// Matches the TS tool.renderToolUseMessage() pattern for common tools
func summarizeToolArguments(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		// If not valid JSON, return truncated raw string
		if len(raw) > 80 {
			return raw[:77] + "..."
		}
		return raw
	}

	// Extract common human-readable fields based on tool conventions
	// This matches TS renderToolUseMessage() logic from various tools

	// Bash tool: show command
	if cmd, ok := args["command"].(string); ok {
		return truncateCommand(cmd)
	}

	// Read/Edit/Write tools: show file path
	if path, ok := args["file_path"].(string); ok {
		displayPath := path
		// Strip common prefixes for cleaner display
		if strings.HasPrefix(path, "./") {
			displayPath = path[2:]
		}

		// For Read tool with offset/limit, show line range
		if offset, hasOffset := args["offset"].(float64); hasOffset {
			if limit, hasLimit := args["limit"].(float64); hasLimit {
				return fmt.Sprintf("%s · lines %d-%d", displayPath, int(offset), int(offset+limit-1))
			}
			return fmt.Sprintf("%s · from line %d", displayPath, int(offset))
		}

		// For Read tool with pages
		if pages, ok := args["pages"].(string); ok && pages != "" {
			return fmt.Sprintf("%s · pages %s", displayPath, pages)
		}

		return displayPath
	}

	// Glob tool: show pattern
	if pattern, ok := args["pattern"].(string); ok {
		return pattern
	}

	// Grep tool: show search + pattern
	if search, ok := args["search"].(string); ok {
		if pattern, ok := args["pattern"].(string); ok {
			return fmt.Sprintf("%s in %s", truncateString(search, 40), pattern)
		}
		return truncateString(search, 60)
	}

	// Agent tool: show agent type + prompt
	if agentType, ok := args["agent_type"].(string); ok {
		if agentType == "worker" {
			agentType = "Agent"
		}
		if prompt, ok := args["prompt"].(string); ok {
			return fmt.Sprintf("%s: %s", agentType, truncateString(prompt, 50))
		}
		return agentType
	}
	if subagentType, ok := args["subagent_type"].(string); ok {
		displayType := subagentType
		if displayType == "" || displayType == "general-purpose" || displayType == "worker" {
			displayType = "Agent"
		}
		if prompt, ok := args["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
			return fmt.Sprintf("%s: %s", displayType, truncateString(prompt, 50))
		}
		if desc, ok := args["description"].(string); ok && strings.TrimSpace(desc) != "" {
			return fmt.Sprintf("%s: %s", displayType, truncateString(desc, 50))
		}
		return displayType
	}

	// MCP tool: show server + tool name
	if server, ok := args["server_name"].(string); ok {
		if tool, ok := args["tool_name"].(string); ok {
			return fmt.Sprintf("%s/%s", server, tool)
		}
		return server
	}

	// Fallback: return first string value found, or empty
	for _, val := range args {
		if str, ok := val.(string); ok && str != "" {
			return truncateString(str, 80)
		}
	}

	return ""
}

// summarizeToolUseDisplay returns TS-like tool display name + argument summary.
func summarizeToolUseDisplay(toolName, raw string) (string, string) {
	name := toolName
	summary := summarizeToolArguments(raw)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if name == "" {
			name = "Tool"
		}
		return name, summary
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		if name == "" {
			name = "Tool"
		}
		return name, summary
	}
	if name == "Read" {
		if filePath, ok := args["file_path"].(string); ok {
			if isPlanPath(filePath) {
				name = "Reading Plan"
				summary = ""
			} else if taskID := extractAgentOutputTaskID(filePath); taskID != "" {
				name = "Read agent output"
				summary = taskID
			}
		}
	}
	if name == "Write" {
		if filePath, ok := args["file_path"].(string); ok && isPlanPath(filePath) {
			name = "Updated plan"
			summary = ""
		}
	}
	if name == "Edit" {
		if filePath, ok := args["file_path"].(string); ok && isPlanPath(filePath) {
			name = "Updated plan"
			summary = ""
		}
	}
	if name == "Agent" {
		if subagentType, ok := args["subagent_type"].(string); ok {
			if subagentType == "" || subagentType == "general-purpose" || subagentType == "worker" {
				name = "Agent"
			} else {
				name = subagentType
			}
		}
	}
	if name == "" {
		name = toolName
	}
	if name == "" {
		name = "Tool"
	}
	return name, summary
}

func isPlanPath(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	plansDir := filepath.Join(home, ".claude", "plans")
	cleanPath := filepath.Clean(path)
	cleanPlansDir := filepath.Clean(plansDir)
	return strings.HasPrefix(cleanPath, cleanPlansDir)
}

func extractAgentOutputTaskID(path string) string {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".output") {
		taskID := strings.TrimSuffix(base, ".output")
		if taskID != "" {
			return taskID
		}
	}
	return ""
}

// truncateCommand truncates a bash command for display
// Matches TS BashTool renderToolUseMessage logic
func truncateCommand(cmd string) string {
	const maxLines = 2
	const maxChars = 160

	lines := strings.Split(cmd, "\n")
	needsLineTruncation := len(lines) > maxLines
	needsCharTruncation := len(cmd) > maxChars

	if !needsLineTruncation && !needsCharTruncation {
		return cmd
	}

	truncated := cmd

	// First truncate by lines if needed
	if needsLineTruncation {
		truncated = strings.Join(lines[:maxLines], "\n")
	}

	// Then truncate by chars if still too long
	if len(truncated) > maxChars {
		truncated = truncated[:maxChars]
	}

	return strings.TrimSpace(truncated) + "…"
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tickNoticesCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return noticeTickMsg{}
	})
}

func spinnerTickCmd() tea.Cmd {
	// TS uses useAnimationFrame(50) - 50ms interval for smooth animation
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func buildSlashSuggestions(current string, registry *command.Registry) []ui.SlashSuggestion {
	current = strings.TrimSpace(current)
	if !strings.HasPrefix(current, "/") {
		return nil
	}
	namePart := strings.TrimPrefix(current, "/")
	if i := strings.IndexAny(namePart, " \t"); i >= 0 {
		namePart = namePart[:i]
	}
	query := strings.ToLower(namePart)
	var suggestions []ui.SlashSuggestion
	for _, cmd := range registry.List() {
		base := cmd.GetBase()
		if base.Hidden {
			continue
		}
		name := strings.ToLower(base.Name)
		if query != "" && !strings.HasPrefix(name, query) {
			continue
		}
		suggestions = append(suggestions, ui.SlashSuggestion{
			Command:     "/" + base.Name,
			Description: base.Description,
		})
		if len(suggestions) >= 6 {
			break
		}
	}
	return suggestions
}

func applySlashSuggestion(current, selected string) string {
	if selected == "" {
		return current
	}
	trimmedLeft := strings.TrimLeft(current, " \t")
	if !strings.HasPrefix(trimmedLeft, "/") {
		return selected
	}
	spaceIdx := strings.IndexAny(trimmedLeft, " \t")
	if spaceIdx < 0 {
		return selected
	}
	rest := strings.TrimLeft(trimmedLeft[spaceIdx:], " \t")
	if rest == "" {
		return selected
	}
	return selected + " " + rest
}

func (r *ChatRunner) buildCommandRuntime(ctx context.Context) command.Runtime {
	return command.Runtime{
		Engine: r.app.Engine(),
		Agents: r.app.Agents(),
		Tools:  r.app.Services().Tools(),
		State:  r.app.State(),
		Config: r.app.Config(),
		CompactSession: func(maxMessages int) (before int, after int) {
			messages := r.app.Engine().Messages()
			before = len(messages)
			// Convert to CompactMessage for compaction
			compactMessages := services.ConvertToCompactMessages(messages)
			result, err := r.app.Services().Compact().Compact(ctx, compactMessages, "", false)
			if err != nil {
				return before, before
			}
			compacted := append(result.MessagesToKeep, result.BoundaryMarker)
			compacted = append(compacted, result.SummaryMessages...)
			compacted = append(compacted, result.Attachments...)
			compacted = append(compacted, result.HookResults...)
			// Convert back to types.Message
			r.app.Engine().ReplaceMessages(services.ConvertFromCompactMessages(compacted))
			after = len(compacted)
			return before, after
		},
		MCPStatus: func() string {
			return r.app.Services().MCP().Status()
		},
		MCPServers: func() []command.MCPServerInfo {
			servers := r.app.Services().MCP().Servers()
			out := make([]command.MCPServerInfo, 0, len(servers))
			for _, server := range servers {
				out = append(out, command.MCPServerInfo{
					Name:          server.Name,
					Transport:     string(server.Transport),
					Status:        server.Status,
					ToolCount:     server.ToolCount,
					ResourceCount: server.ResourceCount,
					Description:   server.Description,
					Enabled:       server.Enabled,
					URL:           server.URL,
					Auth:          server.Auth,
					Channel:       server.Channel,
					Dev:           server.Dev,
					Command:       server.Command,
					Connected:     server.Connected,
					LastConnected: server.LastConnected,
					LastCalledAt:  server.LastCalledAt,
					LastResult:    server.LastResult,
				})
			}
			return out
		},
		MCPTools: func(server string) []command.MCPToolInfo {
			tools := r.app.Services().MCP().ListTools(server)
			out := make([]command.MCPToolInfo, 0, len(tools))
			for _, item := range tools {
				out = append(out, command.MCPToolInfo{
					Name:        item.Name,
					Description: item.Description,
					Response:    item.Response,
					ReadOnly:    item.ReadOnly,
				})
			}
			return out
		},
		MCPResources: func(server string) []command.MCPResourceInfo {
			resources := r.app.Services().MCP().ListResources(server)
			out := make([]command.MCPResourceInfo, 0, len(resources))
			for _, item := range resources {
				out = append(out, command.MCPResourceInfo{
					URI:         item.URI,
					Name:        item.Name,
					Description: item.Description,
					MimeType:    item.MimeType,
					Content:     item.Content,
				})
			}
			return out
		},
		MCPTemplates: func(server string) []command.MCPTemplateInfo {
			templates := r.app.Services().MCP().ListTemplates(server)
			out := make([]command.MCPTemplateInfo, 0, len(templates))
			for _, item := range templates {
				out = append(out, command.MCPTemplateInfo{
					URI:         item.URI,
					Description: item.Description,
				})
			}
			return out
		},
		MCPSearchTools: func(query string) []command.MCPToolMatchInfo {
			matches := r.app.Services().MCP().SearchTools(query)
			out := make([]command.MCPToolMatchInfo, 0, len(matches))
			for _, item := range matches {
				out = append(out, command.MCPToolMatchInfo{
					Server:      item.Server,
					Name:        item.Name,
					Description: item.Description,
					ReadOnly:    item.ReadOnly,
				})
			}
			return out
		},
		MCPCallTool: func(server, tool string, args map[string]any) (string, error) {
			return r.app.Services().MCP().CallTool(server, tool, args)
		},
		MCPReadResource: func(server, uri string) (command.MCPResourceInfo, error) {
			resource, err := r.app.Services().MCP().ReadResource(server, uri)
			if err != nil {
				return command.MCPResourceInfo{}, err
			}
			return command.MCPResourceInfo{
				URI:         resource.URI,
				Name:        resource.Name,
				Description: resource.Description,
				MimeType:    resource.MimeType,
				Content:     resource.Content,
			}, nil
		},
		ReloadMCP: func() string {
			return r.app.Services().MCP().Reload()
		},
		ResetMCP: func() string {
			return r.app.Services().MCP().Reset()
		},
		ConnectMCP: func(server string) bool {
			return r.app.Services().MCP().Connect(server)
		},
		DisconnectMCP: func(server string) bool {
			return r.app.Services().MCP().Disconnect(server)
		},
		RestartMCP: func(server string) bool {
			return r.app.Services().MCP().Restart(server)
		},
		PingMCP: func(server string) (string, bool) {
			return r.app.Services().MCP().Ping(server)
		},
		AuthenticateMCP: func(server, token string) bool {
			return r.app.Services().MCP().Authenticate(server, token)
		},
		SetMCPEnabledAll: func(enabled bool) {
			r.app.Services().MCP().SetEnabled(enabled)
		},
		SetMCPServiceStatus: func(status string) {
			r.app.Services().MCP().SetStatus(status)
		},
		SetMCPEnabled: func(name string, enabled bool) bool {
			return r.app.Services().MCP().SetServerEnabled(name, enabled)
		},
		SetMCPStatus: func(name, status string) bool {
			return r.app.Services().MCP().SetServerStatus(name, status)
		},
		AddMCPServer: func(server command.MCPServerInfo) {
			r.app.Services().MCP().AddServer(servicesMCPServer(server))
		},
		RemoveMCPServer: func(name string) bool {
			return r.app.Services().MCP().RemoveServer(name)
		},
		PluginStatus: func() string {
			return r.app.Services().Plugins().Status()
		},
		PluginList: func() []command.PluginInfo {
			plugins := r.app.Services().Plugins().List()
			out := make([]command.PluginInfo, 0, len(plugins))
			for _, plugin := range plugins {
				out = append(out, command.PluginInfo{
					Name:         plugin.Name,
					Source:       plugin.Source,
					Status:       plugin.Status,
					SourceType:   plugin.SourceType,
					Version:      plugin.Version,
					Description:  plugin.Description,
					Enabled:      plugin.Enabled,
					Marketplace:  plugin.Marketplace,
					Category:     plugin.Category,
					Path:         plugin.Path,
					Dev:          plugin.Dev,
					CommandCount: plugin.CommandCount,
					AgentCount:   plugin.AgentCount,
					SkillCount:   plugin.SkillCount,
					HookCount:    plugin.HookCount,
				})
			}
			return out
		},
		ReloadPlugins: func() string {
			return r.app.Services().ReloadPluginsRuntime()
		},
		ResetPlugins: func() string {
			return r.app.Services().ResetPluginsRuntime()
		},
		SetPluginsEnabledAll: func(enabled bool) {
			r.app.Services().Plugins().SetEnabledAll(enabled)
			r.app.Services().SyncExtensions()
		},
		SetPluginsServiceStatus: func(status string) {
			r.app.Services().Plugins().SetServiceStatus(status)
			r.app.Services().SyncExtensions()
		},
		SetPluginEnabled: func(name string, enabled bool) bool {
			ok := r.app.Services().Plugins().SetEnabled(name, enabled)
			r.app.Services().SyncExtensions()
			return ok
		},
		SetPluginStatus: func(name, status string) bool {
			ok := r.app.Services().Plugins().SetStatus(name, status)
			r.app.Services().SyncExtensions()
			return ok
		},
		AddPlugin: func(plugin command.PluginInfo) {
			r.app.Services().Plugins().Add(servicesPlugin(plugin))
			r.app.Services().SyncExtensions()
		},
		RemovePlugin: func(name string) bool {
			ok := r.app.Services().Plugins().Remove(name)
			r.app.Services().SyncExtensions()
			return ok
		},
		ReloadSkills: func() string {
			return r.app.Services().ReloadSkillsRuntime()
		},
		SkillStatus: func() string {
			return r.app.Services().Skills().Status()
		},
		SkillList: func() []command.SkillInfo {
			skills := r.app.Services().Skills().List()
			out := make([]command.SkillInfo, 0, len(skills))
			for _, skill := range skills {
				out = append(out, command.SkillInfo{
					Name:                   skill.Name,
					DisplayName:            skill.DisplayName,
					Aliases:                append([]string(nil), skill.Aliases...),
					Description:            skill.Description,
					WhenToUse:              skill.WhenToUse,
					ArgumentHint:           skill.ArgumentHint,
					AllowedTools:           append([]string(nil), skill.AllowedTools...),
					Version:                skill.Version,
					Model:                  skill.Model,
					Context:                skill.Context,
					Agent:                  skill.Agent,
					Source:                 skill.Source,
					LoadedFrom:             skill.LoadedFrom,
					Path:                   skill.Path,
					BaseDir:                skill.BaseDir,
					UserInvocable:          skill.UserInvocable,
					DisableModelInvocation: skill.DisableModelInvocation,
				})
			}
			return out
		},
		HookStatus: func() string {
			return r.app.Services().Hooks().Status()
		},
		HookList: func() []command.HookInfo {
			hooks := r.app.Services().Hooks().List()
			out := make([]command.HookInfo, 0, len(hooks))
			for _, hook := range hooks {
				out = append(out, command.HookInfo{
					Event:       hook.Event,
					Source:      hook.Source,
					Status:      hook.Status,
					Command:     hook.Command,
					Description: hook.Description,
					Enabled:     hook.Enabled,
					Matcher:     hook.Matcher,
					TimeoutMs:   hook.TimeoutMs,
					Blocking:    hook.Blocking,
					Shell:       hook.Shell,
					RunCount:    hook.RunCount,
					LastRunAt:   hook.LastRunAt,
					LastResult:  hook.LastResult,
					LastOutput:  hook.LastOutput,
					LastError:   hook.LastError,
				})
			}
			return out
		},
		ReloadHooks: func() string {
			return r.app.Services().Hooks().Reload()
		},
		ResetHooks: func() string {
			return r.app.Services().Hooks().Reset()
		},
		SetHooksEnabledAll: func(enabled bool) {
			r.app.Services().Hooks().SetEnabledAll(enabled)
		},
		SetHooksServiceStatus: func(status string) {
			r.app.Services().Hooks().SetServiceStatus(status)
		},
		SetHookEnabled: func(event string, enabled bool) bool {
			return r.app.Services().Hooks().SetEnabled(event, enabled)
		},
		SetHookStatus: func(event, status string) bool {
			return r.app.Services().Hooks().SetStatus(event, status)
		},
		AddHook: func(hook command.HookInfo) {
			r.app.Services().Hooks().Add(servicesHook(hook))
		},
		RemoveHook: func(event string) bool {
			return r.app.Services().Hooks().Remove(event)
		},
	}
}

func (r *ChatRunner) runCommand(ctx context.Context, line string) error {
	runtime := r.buildCommandRuntime(ctx)
	cmd, ok := r.app.Commands().Lookup(line)
	out, handled, err := r.app.Commands().Execute(ctx, line, runtime)
	if err != nil {
		return err
	}
	if handled {
		name := strings.TrimPrefix(strings.Fields(line)[0], "/")
		switch {
		case ok && cmd.GetKind() == command.KindPrompt:
			if out.Value == "" {
				return nil
			}
			r.mu.Lock()
			r.entries = append(r.entries, components.TranscriptEntry{Kind: "command", Title: "Prompt /" + name, Content: out.Value})
			r.mu.Unlock()
			_, err := r.app.Engine().Submit(ctx, out.Value)
			return err
		case ok && cmd.GetKind() == command.KindLocalJSX:
			r.mu.Lock()
			r.entries = append(r.entries, components.TranscriptEntry{Kind: "panel", Title: "Panel /" + name, Content: out.Value})
			r.mu.Unlock()
		default:
			r.mu.Lock()
			r.entries = append(r.entries, components.TranscriptEntry{Kind: "command", Title: "Command /" + name, Content: out.Value})
			r.mu.Unlock()
		}
		return nil
	}

	r.mu.Lock()
	r.entries = append(r.entries, components.TranscriptEntry{Kind: "notice", Title: "Unknown Command", Content: line})
	r.mu.Unlock()
	if r.stderr != nil {
		_, _ = fmt.Fprint(r.stderr, "")
	}
	return nil
}

func servicesMCPServer(info command.MCPServerInfo) services.MCPServer {
	return services.MCPServer{
		Name:          info.Name,
		Transport:     mcpinfra.Transport(info.Transport),
		Status:        info.Status,
		ToolCount:     info.ToolCount,
		ResourceCount: info.ResourceCount,
		Description:   info.Description,
		Enabled:       info.Enabled,
		URL:           info.URL,
		Auth:          info.Auth,
		Channel:       info.Channel,
		Dev:           info.Dev,
		Command:       info.Command,
	}
}

func servicesPlugin(info command.PluginInfo) services.Plugin {
	return services.Plugin{
		Name:        info.Name,
		Source:      info.Source,
		Status:      info.Status,
		Version:     info.Version,
		Description: info.Description,
		Enabled:     info.Enabled,
		Marketplace: info.Marketplace,
		Category:    info.Category,
		Path:        info.Path,
		Dev:         info.Dev,
	}
}

func servicesHook(info command.HookInfo) services.Hook {
	return services.Hook{
		Event:       info.Event,
		Source:      info.Source,
		Status:      info.Status,
		Command:     info.Command,
		Description: info.Description,
		Enabled:     info.Enabled,
		Matcher:     info.Matcher,
		TimeoutMs:   info.TimeoutMs,
		Blocking:    info.Blocking,
		Shell:       info.Shell,
	}
}

func (r *ChatRunner) drainAgentNotices() {
	for _, notice := range r.app.Agents().Tasks().DrainNotices() {
		title := "Background Agent Update"
		switch notice.Kind {
		case "launched":
			title = "Background Agent Launched"
		case "continued":
			title = "Background Agent Continued"
		case "completed":
			title = "Background Agent Completed"
		case "failed":
			title = "Background Agent Failed"
		case "killed":
			title = "Background Agent Stopped"
		}

		content := fmt.Sprintf("task=%s\ntype=%s\nstatus=%s\ndescription=%s", notice.TaskID, notice.AgentType, notice.Status, notice.Description)
		if notice.Error != "" {
			content += "\nerror=" + notice.Error
		}
		if notice.Output != "" {
			content += "\n\n" + notice.Output
		}
		r.mu.Lock()
		r.entries = append(r.entries, components.TranscriptEntry{
			Kind:    "notice",
			Title:   title,
			Content: content,
		})
		r.mu.Unlock()
	}
}

func (r *ChatRunner) drainPluginNotices() {
	for _, notice := range r.app.Services().Plugins().DrainNotices() {
		content := "event=" + notice.Kind
		if strings.TrimSpace(notice.Target) != "" {
			content += "\ntarget=" + notice.Target
		}
		if strings.TrimSpace(notice.Timestamp) != "" {
			content += "\nat=" + notice.Timestamp
		}
		if strings.TrimSpace(notice.Message) != "" {
			content += "\n\n" + notice.Message
		}
		r.mu.Lock()
		r.entries = append(r.entries, components.TranscriptEntry{
			Kind:    "notice",
			Title:   "Plugin Update",
			Content: content,
		})
		r.mu.Unlock()
	}
}
