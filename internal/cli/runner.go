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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"claude-go/internal/app"
	"claude-go/internal/command"
	"claude-go/internal/components"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	mcpinfra "claude-go/internal/infra/mcp"
	"claude-go/internal/services"
	"claude-go/internal/task"
	"claude-go/internal/tool"
	bashperm "claude-go/internal/tool/bash"
	"claude-go/internal/tool/interaction"
	"claude-go/internal/types"
	"claude-go/internal/ui"
	"claude-go/internal/ui/collapse"
	uicomponents "claude-go/internal/ui/components"
	"claude-go/internal/ui/diff"
	"claude-go/internal/ui/paste"
	"claude-go/internal/utils"

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
	// Channel for interactive user question requests (permissions, AskUserQuestion).
	questionChan chan userQuestionRequest

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

type userQuestionRequest struct {
	question    interaction.Question
	multiSelect bool
	response    chan userQuestionResponse
	// Optional tool context for permission requests with diff preview
	toolName  string
	toolInput map[string]any // Tool input as parsed JSON for file edits
}

type userQuestionResponse struct {
	answer  string
	answers []string
	err     error
}

func CreateChatRunner(application *app.App, stdout, stderr io.Writer) *ChatRunner {
	r := &ChatRunner{
		app:          application,
		stdout:       stdout,
		stderr:       stderr,
		components:   components.ChatAppFor(),
		streamChan:   make(chan streamUpdate, 100),
		questionChan: make(chan userQuestionRequest),
	}

	// Route AskUserQuestion/permission prompts through Bubble Tea so prompts
	// can be answered interactively while a streaming turn is running.
	interaction.SetUserInputHandler(&chatUserInputHandler{runner: r})

	// Set up permission handler for tool execution
	application.Services().SetAskPermissionHandler(func(ctx context.Context, toolName string, input tool.Input, message string) (bool, error) {
		return r.handlePermissionRequest(ctx, toolName, input, message)
	})

	// Sync any pre-loaded messages from engine (e.g., from session resume)
	// This ensures entries are initialized before the UI starts
	if application.Engine() != nil && len(application.Engine().Messages()) > 0 {
		r.syncEntriesFromEngine("")
	}

	return r
}

// handlePermissionRequest handles permission requests from the engine
func (r *ChatRunner) handlePermissionRequest(ctx context.Context, toolName string, input tool.Input, message string) (bool, error) {
	// Build permission question
	filePath := tool.GetString(input, "file_path")

	// Build detailed question text
	questionText := message

	// For Edit tool, add diff preview info if available
	if toolName == "Edit" && filePath != "" {
		// The preview will be rendered by the permission dialog using toolInput
		questionText = fmt.Sprintf("Edit needs permission to run:\nEdit file %s", filePath)
	}

	// Create permission request question
	req := userQuestionRequest{
		question: interaction.Question{
			Question: questionText,
			Header:   "Permission",
			Options: []interaction.QuestionOption{
				{
					Label:       "Allow once",
					Description: "Approve this operation for this attempt only.",
				},
				{
					Label:       "Always allow",
					Description: "Remember this pattern for the session.",
				},
				{
					Label:       "Deny",
					Description: "Do not run this operation.",
				},
			},
		},
		multiSelect: false,
		response:    make(chan userQuestionResponse, 1),
		toolName:    toolName,
		toolInput:   input,
	}

	if err := r.enqueueQuestionRequest(req); err != nil {
		return false, err
	}

	resp := <-req.response
	if resp.err != nil {
		return false, resp.err
	}

	answer := strings.ToLower(strings.TrimSpace(resp.answer))

	// Check for "always allow" - set permission mode to AcceptEdits for session
	if strings.Contains(answer, "always allow") {
		bashperm.SetGlobalPermissionMode(bashperm.PermissionModeAcceptEdits)
		return true, nil
	}

	// "Allow once" just returns true for this specific operation
	return strings.Contains(answer, "allow"), nil
}

type chatUserInputHandler struct {
	runner *ChatRunner
}

func (h *chatUserInputHandler) AskQuestion(q interaction.Question) (string, error) {
	if h == nil || h.runner == nil {
		return "", fmt.Errorf("interactive question handler unavailable")
	}
	req := userQuestionRequest{
		question:    q,
		multiSelect: false,
		response:    make(chan userQuestionResponse, 1),
	}
	if err := h.runner.enqueueQuestionRequest(req); err != nil {
		return "", err
	}
	resp := <-req.response
	return strings.TrimSpace(resp.answer), resp.err
}

func (h *chatUserInputHandler) AskMultiSelect(q interaction.Question) ([]string, error) {
	if h == nil || h.runner == nil {
		return nil, fmt.Errorf("interactive question handler unavailable")
	}
	req := userQuestionRequest{
		question:    q,
		multiSelect: true,
		response:    make(chan userQuestionResponse, 1),
	}
	if err := h.runner.enqueueQuestionRequest(req); err != nil {
		return nil, err
	}
	resp := <-req.response
	return resp.answers, resp.err
}

func (r *ChatRunner) enqueueQuestionRequest(req userQuestionRequest) error {
	if r == nil {
		return fmt.Errorf("chat runner unavailable")
	}
	select {
	case r.questionChan <- req:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out while waiting to open interactive prompt")
	}
}

func (r *ChatRunner) Run(ctx context.Context) error {
	r.drainNotices()
	program := tea.NewProgram(
		createChatModel(ctx, r),
		tea.WithContext(ctx),
		tea.WithInput(r.app.Input()),
		tea.WithOutput(r.stdout),
		// No alt screen - use main screen buffer for native terminal scrollback
		// Historical entries are flushed via tea.Println() for native scrolling
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
	// Ensure session is saved on exit
	saveCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if eng := r.app.Engine(); eng != nil {
		_ = eng.Close(saveCtx, "exit")
	}
	if err == tea.ErrProgramKilled || err == io.EOF {
		return nil
	}
	return err
}

func (r *ChatRunner) drainNotices() {
	r.drainAgentNotices()
	r.drainPluginNotices()
}

// findCommand finds a command by name from the registry
func (r *ChatRunner) findCommand(name string) command.Command {
	if r.app == nil || r.app.Commands() == nil {
		return nil
	}
	for _, cmd := range r.app.Commands().List() {
		if cmd.GetBase().Name == name {
			return cmd
		}
	}
	return nil
}

func (r *ChatRunner) transformEntriesForDisplay(entries []components.TranscriptEntry, mode components.ViewMode) ([]components.TranscriptEntry, map[string]bool) {
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

	// Apply transformation pipeline (matching TS Messages.tsx)
	verbose := mode == components.ViewModeVerbose || mode == components.ViewModeTranscript

	// Step 1: Group tool uses (non-verbose only)
	if !verbose {
		entries = collapse.GroupToolUses(entries, r.app.Services().Tools(), verbose)
	}

	// Step 2: Collapse read/search groups (non-verbose only)
	entries = collapse.ReadSearchGroups(entries, r.app.Services().Tools(), verbose)

	// Step 3: Collapse teammate shutdown attachments
	entries = collapse.TeammateShutdowns(entries)

	// Step 4: Collapse hook summaries with the same label
	entries = collapse.HookSummaries(entries)

	// Step 5: Collapse background bash notifications (non-verbose only)
	if !verbose {
		entries = collapse.BackgroundBashNotifications(entries)
	}

	// Update IsActive on any remaining tool_use/grouped_tool_use entries
	for i := range entries {
		if entries[i].ToolUseID != "" && inProgressToolIDs[entries[i].ToolUseID] {
			entries[i].IsActive = true
		}
	}

	return entries, inProgressToolIDs
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

	entries, inProgressToolIDs := r.transformEntriesForDisplay(entries, mode)

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
		PermissionMode:       state.permissionMode,
		FooterRightHint:      state.footerRightHint,
		InProgressToolIDs:    inProgressToolIDs,
	})
}

type renderState struct {
	busy              bool
	streamingText     string
	spinnerTick       int
	toolName          string
	toolCallID        string // ID of the currently streaming tool call
	toolInput         string // JSON arguments of the streaming tool
	statusText        string
	startedAt         time.Time
	toolInProgress    bool
	verb              string // Randomly selected verb, constant during request
	tokenCount        int    // Current token count for display
	transcriptScroll  int
	teammates         []ui.TeammateSpinnerNode
	teammateVerb      string
	teammateTokens    int
	contextTokenUsage int
	permissionMode    string
	footerRightHint   string
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

	// Preserve entries that shouldn't be rebuilt from engine messages.
	// These entries are either added during streaming or should persist across syncs.
	aux := make([]components.TranscriptEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		switch entry.Kind {
		case "panel", "command", "error":
			aux = append(aux, entry)
		case "attachment":
			aux = append(aux, entry)
		case "notice":
			if entry.Title != "System" {
				aux = append(aux, entry)
			}
		case "system":
			aux = append(aux, entry)
		case "user":
			// Preserve user entries that were added directly (e.g., from slash commands)
			// before the engine processed them. These will be superseded by entries
			// from buildTranscriptEntries if duplicates, but we keep them as fallback.
			aux = append(aux, entry)
		case "assistant_streaming":
			// Preserve streaming placeholder - will be superseded by engine entries
			// when streaming completes
			aux = append(aux, entry)
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
				ToolInput: streamingTool.ToolInput,
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
	cursorPos          int // Cursor position in current input
	selectedSuggestion int
	width              int
	height             int
	state              renderState
	mode               components.ViewMode // View mode (normal, verbose, transcript)
	// History navigation
	history        []string                     // Command history
	historyIndex   int                          // Current history index (-1 means editing draft)
	historyDraft   string                       // Draft saved when navigating history
	historyManager *utils.CommandHistoryManager // Persistent history manager
	// Paste manager for collapsing large pastes
	pasteManager *paste.Manager
	// Thinking block visibility tracking
	lastThinkingBlockID     string    // UUID of the last thinking block to show
	streamingThinkingEndsAt time.Time // When streaming thinking grace period ends
	// Scrollback mode: track which entries have been printed to terminal scrollback
	printedEntriesCount int             // Index-based tracking for incremental flushes
	printedEntryUUIDs   map[string]bool // UUID-based deduplication
	// Text selection state for input field
	selectionStart int // Start position of text selection (0 means no selection)
	selectionEnd   int // End position of text selection
	// LocalJSX interactive sub-UI state (TS parity: isLocalJSXCommand)
	activeLocalJSXModel tea.Model
	activeLocalJSXName  string
	activeLocalJSXArgs  string
	activeLocalJSXLife  *localJSXLifecycle
	questionPrompt      *activeQuestionPrompt
	questionQueue       []userQuestionRequest
	// Double-press Ctrl+C exit mechanism (matches TS useExitOnCtrlCD)
	interruptPending     bool      // First Ctrl+C pressed, waiting for second
	interruptPendingTime time.Time // When first Ctrl+C was pressed
	// Initial flush flag: when true, include user entries in scrollback flush
	// (for session resume, user entries haven't been printed yet)
	isInitialFlush bool
}

type activeQuestionPrompt struct {
	request      userQuestionRequest
	selected     int
	selectedMany map[int]bool
	previousHint string
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

type questionRequestMsg struct {
	request userQuestionRequest
}

// commandResultMsg is sent when a slash command completes (local or prompt)
// This triggers scrollback flush to display the command result
type commandResultMsg struct{}

// compactResultMsg is sent when /compact completes, carrying before/after counts
type compactResultMsg struct {
	before int
	after  int
}

type interruptMsg struct {
	signal string
}

// interruptTimeoutMsg is sent when the double-press Ctrl+C timeout expires
type interruptTimeoutMsg struct{}

// interruptTimeout is the time window for double-press Ctrl+C (matches TS DEFAULT_TIMEOUT_MS)
const interruptTimeout = 500 * time.Millisecond

// localJSXActivatedMsg mounts a LocalJSX model as an interactive sub-UI.
type localJSXActivatedMsg struct {
	commandName string
	commandArgs string
	model       tea.Model
	lifecycle   *localJSXLifecycle
	resetBusy   bool
}

// localJSXCloseMsg closes the active LocalJSX sub-UI.
type localJSXCloseMsg struct{}

// localJSXDoneMsg completes the active LocalJSX sub-UI with onDone payload.
type localJSXDoneMsg struct {
	commandName string
	commandArgs string
	payload     localJSXDonePayload
}

type localJSXDonePayload struct {
	result  string
	options command.LocalJSXDoneOptions
}

const localJSXNoContentMessage = "(no content)"

type localJSXLifecycle struct {
	closeRequested atomic.Bool
	mu             sync.Mutex
	done           *localJSXDonePayload
}

func newLocalJSXLifecycle() *localJSXLifecycle {
	return &localJSXLifecycle{}
}

func (l *localJSXLifecycle) RequestClose() {
	if l == nil {
		return
	}
	l.closeRequested.Store(true)
}

func (l *localJSXLifecycle) Done(result string, options command.LocalJSXDoneOptions) {
	if l == nil {
		return
	}
	cloned := options
	if len(options.MetaMessages) > 0 {
		cloned.MetaMessages = append([]string(nil), options.MetaMessages...)
	}
	l.mu.Lock()
	l.done = &localJSXDonePayload{
		result:  result,
		options: cloned,
	}
	l.mu.Unlock()
	l.closeRequested.Store(true)
}

func (l *localJSXLifecycle) TakeDone() (localJSXDonePayload, bool) {
	if l == nil {
		return localJSXDonePayload{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.done == nil {
		return localJSXDonePayload{}, false
	}
	payload := *l.done
	l.done = nil
	l.closeRequested.Store(false)
	return payload, true
}

func (l *localJSXLifecycle) TakeCloseRequested() bool {
	if l == nil {
		return false
	}
	return l.closeRequested.Swap(false)
}

func clampCursorPos(input string, cursor int) int {
	if cursor < 0 {
		return 0
	}
	if cursor > len(input) {
		return len(input)
	}
	return cursor
}

func setInputValue(cursor int, next string) (string, int) {
	return next, clampCursorPos(next, cursor)
}

func clearInputValue() (string, int) {
	return "", 0
}

func insertAtCursor(current string, cursor int, insert string) (string, int) {
	cursor = clampCursorPos(current, cursor)
	next := current[:cursor] + insert + current[cursor:]
	return next, cursor + len(insert)
}

func deleteAtCursorOrBackspace(current string, cursor int) (string, int) {
	cursor = clampCursorPos(current, cursor)
	// Backspace: delete the character BEFORE the cursor position
	// This matches TS: Cursor.backspace() -> this.left().modifyText(this)
	if cursor > 0 {
		// Find the rune before cursor position
		_, runeLen := utf8.DecodeLastRuneInString(current[:cursor])
		prevCursor := cursor - runeLen
		next := current[:prevCursor] + current[cursor:]
		return next, prevCursor
	}
	return current, cursor
}

func createChatModel(ctx context.Context, runner *ChatRunner) chatModel {
	// Get initial terminal size instead of using hardcoded values
	// This matches the original TS behavior where process.stdout.columns is used
	size := ui.GetTerminalSize()

	// Create persistent history manager
	historyManager := utils.CreateCommandHistoryManager(utils.CommandHistoryConfig{
		ProjectRoot: runner.app.State().Snapshot().ProjectRoot,
		SessionID:   runner.app.State().Snapshot().SessionID,
	})

	// Load history from persistent storage
	var history []string
	if historyManager != nil {
		history = historyManager.GetAll()
	}

	model := chatModel{
		ctx:                ctx,
		runner:             runner,
		selectedSuggestion: -1,
		width:              size.Width,
		height:             size.Height,
		mode:               components.ViewModeNormal,
		printedEntryUUIDs:  make(map[string]bool),
		history:            history,
		historyIndex:       -1,
		historyManager:     historyManager,
	}
	model.refreshFooterState()
	return model
}

func (m chatModel) Init() tea.Cmd {
	// Print header immediately to scrollback, then start normal operations
	width := maxInt(72, m.width)
	snapshot := m.runner.app.State().Snapshot()
	header := ui.RenderHeader(width, m.runner.app.Version(), m.runner.app.Config(), snapshot, snapshot.SessionID, snapshot.TurnCount)

	otherCmds := []tea.Cmd{
		tickNoticesCmd(),
		spinnerTickCmd(),
		listenForStreamUpdates(m.runner),
		listenForQuestionRequests(m.runner),
	}

	// Flush any pre-loaded entries (e.g., from session resume) to scrollback
	// Set isInitialFlush so user entries are included (they haven't been printed via Enter)
	m.isInitialFlush = true
	flushCmd := m.flushNewEntriesToScrollback()
	m.isInitialFlush = false

	// Use tea.Sequence to guarantee header prints BEFORE history entries.
	// tea.Batch does not guarantee order, which caused the header to appear below history.
	if flushCmd != nil {
		otherCmds = append(otherCmds, tea.Sequence(tea.Println(header), flushCmd))
	} else {
		otherCmds = append(otherCmds, tea.Println(header))
	}

	return tea.Batch(otherCmds...)
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
	// Matches TS Messages.tsx:395-419 behavior:
	// - Find last thinking in most recent assistant turn
	// - Stop at user message without tool_result (previous turn boundary)
	lastThinkingID := ""
	if m.runner != nil {
		m.runner.mu.RLock()
		for i := len(m.runner.entries) - 1; i >= 0; i-- {
			entry := m.runner.entries[i]

			// Found thinking block
			if entry.Kind == "thinking" || entry.Kind == "redacted_thinking" {
				lastThinkingID = entry.UUID
				break
			}

			// Hit user message without tool_result -> previous turn boundary
			if entry.Kind == "user" {
				// Check if any following entries are tool_result
				hasToolResult := false
				for j := i + 1; j < len(m.runner.entries); j++ {
					if m.runner.entries[j].Kind == "tool_result" {
						hasToolResult = true
						break
					}
				}
				if !hasToolResult {
					// Previous turn, don't show stale thinking
					lastThinkingID = "no-thinking"
					break
				}
			}
		}
		m.runner.mu.RUnlock()
	}
	m.lastThinkingBlockID = lastThinkingID

	if m.runner != nil && m.runner.app != nil {
		m.refreshFooterState()
	}
}

func (m *chatModel) prepareStreamingTurn() {
	m.state.busy = true
	m.state.streamingText = ""
	m.state.toolName = ""
	m.state.toolCallID = ""
	m.state.toolInput = ""
	m.state.statusText = "Waiting for model response"
	m.state.startedAt = time.Now()
	m.state.toolInProgress = false
	m.state.verb = ui.GetRandomVerb()
	m.state.teammateVerb = m.state.verb
	m.state.tokenCount = 0
	m.refreshTeammateSpinnerState()
	m.state.transcriptScroll = 0
	m.selectedSuggestion = -1
}

func (m *chatModel) refreshFooterState() {
	m.state.contextTokenUsage = m.estimateContextTokenUsage()
	m.state.permissionMode = currentFooterPermissionMode()
	m.state.footerRightHint = buildAutoCompactFooterHint(
		m.runner.app.Config().AutoCompactEnabled,
		m.runner.app.Config().ContextWindowOverride,
		m.currentMainLoopModel(),
		m.state.contextTokenUsage,
	)
}

func (m *chatModel) enqueueQuestionPrompt(req userQuestionRequest) {
	if len(req.question.Options) == 0 {
		select {
		case req.response <- userQuestionResponse{err: fmt.Errorf("interactive question has no options")}:
		default:
		}
		return
	}
	if m.questionPrompt == nil {
		m.activateQuestionPrompt(req)
		return
	}
	m.questionQueue = append(m.questionQueue, req)
}

func (m *chatModel) activateQuestionPrompt(req userQuestionRequest) {
	prompt := &activeQuestionPrompt{
		request:      req,
		selected:     0,
		selectedMany: map[int]bool{},
		previousHint: m.state.statusText,
	}
	m.questionPrompt = prompt
	if header := strings.TrimSpace(req.question.Header); header != "" {
		m.state.statusText = fmt.Sprintf("Waiting for %s", strings.ToLower(header))
	} else {
		m.state.statusText = "Waiting for your input"
	}

	// Set tool context for permission dialogs (diff preview needs toolInput as JSON)
	if req.toolName != "" {
		m.state.toolName = req.toolName
	}
	if req.toolInput != nil {
		if inputJSON, err := json.Marshal(req.toolInput); err == nil {
			m.state.toolInput = string(inputJSON)
		}
	}
}

func (m *chatModel) completeQuestionPrompt(resp userQuestionResponse) {
	if m.questionPrompt == nil {
		return
	}
	req := m.questionPrompt.request
	select {
	case req.response <- resp:
	default:
	}

	previous := m.questionPrompt.previousHint
	m.questionPrompt = nil

	// Clear tool context from permission dialog
	m.state.toolName = ""
	m.state.toolInput = ""

	if len(m.questionQueue) > 0 {
		next := m.questionQueue[0]
		m.questionQueue = m.questionQueue[1:]
		m.activateQuestionPrompt(next)
		return
	}
	m.state.statusText = previous
}

func (m *chatModel) cycleQuestionSelection(delta int) {
	if m.questionPrompt == nil {
		return
	}
	count := len(m.questionPrompt.request.question.Options)
	if count == 0 {
		return
	}
	next := (m.questionPrompt.selected + delta) % count
	if next < 0 {
		next += count
	}
	m.questionPrompt.selected = next
}

func (m *chatModel) toggleQuestionMultiSelection(index int) {
	if m.questionPrompt == nil {
		return
	}
	if index < 0 || index >= len(m.questionPrompt.request.question.Options) {
		return
	}
	if m.questionPrompt.selectedMany[index] {
		delete(m.questionPrompt.selectedMany, index)
		return
	}
	m.questionPrompt.selectedMany[index] = true
}

func (m *chatModel) submitSingleQuestionAnswer(index int) {
	if m.questionPrompt == nil {
		return
	}
	options := m.questionPrompt.request.question.Options
	if index < 0 || index >= len(options) {
		return
	}
	m.completeQuestionPrompt(userQuestionResponse{answer: options[index].Label})
}

func (m *chatModel) submitMultiQuestionAnswers() {
	if m.questionPrompt == nil {
		return
	}
	options := m.questionPrompt.request.question.Options
	selected := make([]string, 0, len(m.questionPrompt.selectedMany))
	for i, opt := range options {
		if m.questionPrompt.selectedMany[i] {
			selected = append(selected, opt.Label)
		}
	}
	if len(selected) == 0 {
		cur := m.questionPrompt.selected
		if cur >= 0 && cur < len(options) {
			selected = append(selected, options[cur].Label)
		}
	}
	m.completeQuestionPrompt(userQuestionResponse{
		answers: selected,
		answer:  strings.Join(selected, ", "),
	})
}

func (m *chatModel) handleQuestionPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.questionPrompt == nil {
		return m, nil
	}

	// Handle immediate commands (like /btw) during permission prompts
	// These can execute without blocking the permission dialog
	if msg.Type == tea.KeyEnter {
		line := strings.TrimSpace(m.currentInput)
		if strings.HasPrefix(line, "/") {
			// Check if this is an immediate command
			cmdName := strings.TrimPrefix(line, "/")
			cmdArgs := ""
			if i := strings.Index(cmdName, " "); i > 0 {
				cmdArgs = strings.TrimSpace(cmdName[i+1:])
				cmdName = cmdName[:i]
			}
			matchingCmd := m.runner.findCommand(cmdName)
			if matchingCmd != nil && matchingCmd.GetBase().Immediate {
				// Execute immediate command during permission prompt
				m.currentInput, m.cursorPos = clearInputValue()
				m.selectedSuggestion = -1

				// Handle based on command kind
				switch matchingCmd.GetKind() {
				case command.KindLocal:
					// For Local commands, execute handler directly and show result inline
					runtime := m.runner.buildCommandRuntime(m.ctx)
					result, handled, err := m.runner.app.Commands().Execute(m.ctx, line, runtime)
					if err != nil {
						m.runner.mu.Lock()
						m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
						m.runner.mu.Unlock()
						return m, nil
					}
					if !handled {
						m.runner.mu.Lock()
						m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Command", Content: "Command not handled"})
						m.runner.mu.Unlock()
						return m, nil
					}
					// Show result in transcript
					m.runner.mu.Lock()
					entryKind := "command"
					if result.Display == "user" {
						entryKind = "user"
					} else if result.Display == "system" {
						entryKind = "system"
					}
					m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
						Kind:    entryKind,
						Title:   "Side Question",
						Content: result.Value,
					})
					m.runner.mu.Unlock()
					return m, nil

				case command.KindLocalJSX:
					// For JSX commands, use modal with lifecycle
					lifecycle := newLocalJSXLifecycle()
					localRuntime := m.runner.buildLocalJSXCommandRuntime(m.ctx, lifecycle)
					model, _, modelHandled, modelErr := m.runner.app.Commands().LoadModel(m.ctx, line, localRuntime)
					if modelErr != nil {
						m.runner.mu.Lock()
						m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: modelErr.Error()})
						m.runner.mu.Unlock()
						return m, nil
					}
					if modelHandled && model != nil {
						return m, func() tea.Msg {
							return localJSXActivatedMsg{
								commandName: cmdName,
								commandArgs: cmdArgs,
								model:       model,
								lifecycle:   lifecycle,
							}
						}
					}
					return m, nil

				default:
					// Unknown kind - show notice
					m.runner.mu.Lock()
					m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
						Kind:    "notice",
						Title:   "Command",
						Content: fmt.Sprintf("Cannot execute /%s (unsupported command type) during permission prompt.", cmdName),
					})
					m.runner.mu.Unlock()
					return m, nil
				}
			}
		}
	}

	if !m.questionPrompt.request.multiSelect && isPermissionQuestion(m.questionPrompt.request.question) {
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "y":
			if idx := findQuestionOptionIndex(m.questionPrompt.request.question.Options, "allow once", "allow"); idx >= 0 {
				m.questionPrompt.selected = idx
				m.submitSingleQuestionAnswer(idx)
				return m, nil
			}
		case "a":
			if idx := findQuestionOptionIndex(m.questionPrompt.request.question.Options, "always allow"); idx >= 0 {
				m.questionPrompt.selected = idx
				m.submitSingleQuestionAnswer(idx)
				return m, nil
			}
		case "n":
			if idx := findQuestionOptionIndex(m.questionPrompt.request.question.Options, "deny"); idx >= 0 {
				m.questionPrompt.selected = idx
				m.submitSingleQuestionAnswer(idx)
				return m, nil
			}
		}
	}

	switch msg.Type {
	case tea.KeyUp:
		m.cycleQuestionSelection(-1)
		return m, nil
	case tea.KeyDown:
		m.cycleQuestionSelection(1)
		return m, nil
	case tea.KeyTab:
		m.cycleQuestionSelection(1)
		return m, nil
	case tea.KeyShiftTab:
		m.cycleQuestionSelection(-1)
		return m, nil
	case tea.KeyEsc, tea.KeyCtrlC:
		m.completeQuestionPrompt(userQuestionResponse{err: fmt.Errorf("user canceled interactive prompt")})
		return m, nil
	case tea.KeyEnter:
		if m.questionPrompt.request.multiSelect {
			m.submitMultiQuestionAnswers()
			return m, nil
		}
		m.submitSingleQuestionAnswer(m.questionPrompt.selected)
		return m, nil
	case tea.KeySpace:
		if m.questionPrompt.request.multiSelect {
			m.toggleQuestionMultiSelection(m.questionPrompt.selected)
			return m, nil
		}
		// For non-multiSelect, treat space as text input for typing commands
		m.cursorPos = clampCursorPos(m.currentInput, m.cursorPos)
		m.currentInput, m.cursorPos = insertAtCursor(m.currentInput, m.cursorPos, " ")
		m.selectedSuggestion = -1
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		// Allow backspace during permission prompts for editing immediate commands
		m.currentInput, m.cursorPos = deleteAtCursorOrBackspace(m.currentInput, m.cursorPos)
		return m, nil
	}

	s := strings.TrimSpace(msg.String())
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		index := int(s[0] - '1')
		if index < len(m.questionPrompt.request.question.Options) {
			m.questionPrompt.selected = index
			if m.questionPrompt.request.multiSelect {
				m.toggleQuestionMultiSelection(index)
				return m, nil
			}
			m.submitSingleQuestionAnswer(index)
		}
	}

	// Handle text input for typing immediate commands (like /btw)
	// Allow typing when not a permission shortcut key
	if msg.Type == tea.KeyRunes || len(msg.String()) > 0 {
		// Don't intercept y/a/n keys for permission questions
		if !m.questionPrompt.request.multiSelect && isPermissionQuestion(m.questionPrompt.request.question) {
			lowerS := strings.ToLower(s)
			if lowerS == "y" || lowerS == "a" || lowerS == "n" {
				return m, nil // Already handled above
			}
		}
		// Allow typing for slash commands - forward to normal key handling
		// This enables typing /btw during permission prompts
		return m.handleTextInputKey(msg)
	}

	return m, nil
}

// handleTextInputKey handles text input for command typing during permission prompts.
// This allows users to type immediate commands like /btw even when permission dialogs are active.
func (m *chatModel) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.cursorPos = clampCursorPos(m.currentInput, m.cursorPos)
	if msg.Type == tea.KeyRunes {
		// Insert at cursor position
		runes := string(msg.Runes)
		// Handle bracketed paste: collapse large/multi-line pastes
		if msg.Paste {
			if m.pasteManager == nil {
				m.pasteManager = paste.NewManager()
			}
			runes = m.pasteManager.AddPaste(runes)
		}
		m.currentInput, m.cursorPos = insertAtCursor(m.currentInput, m.cursorPos, runes)
		m.selectedSuggestion = -1
		m.selectionStart = -1
		m.selectionEnd = -1
	} else if s := msg.String(); s != "" && utf8.ValidString(s) && !strings.HasPrefix(s, "ctrl+") {
		// Handle arrow keys that might come as strings
		switch s {
		case "left":
			if m.cursorPos > 0 {
				_, runeLen := utf8.DecodeLastRuneInString(m.currentInput[:m.cursorPos])
				m.cursorPos -= runeLen
			}
			return m, nil
		case "right":
			if m.cursorPos < len(m.currentInput) {
				_, runeLen := utf8.DecodeRuneInString(m.currentInput[m.cursorPos:])
				m.cursorPos += runeLen
			}
			return m, nil
		case "home":
			m.cursorPos = 0
			return m, nil
		case "end":
			m.cursorPos = len(m.currentInput)
			return m, nil
		case "up", "down", "pgup", "pgdown":
			// These are handled by other cases
			return m, nil
		}
		// Insert at cursor position
		m.currentInput, m.cursorPos = insertAtCursor(m.currentInput, m.cursorPos, s)
		m.selectedSuggestion = -1
		m.selectionStart = -1
		m.selectionEnd = -1
	}
	return m, nil
}

func findQuestionOptionIndex(options []interaction.QuestionOption, candidates ...string) int {
	loweredCandidates := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" {
			loweredCandidates = append(loweredCandidates, candidate)
		}
	}

	for i, opt := range options {
		label := strings.ToLower(strings.TrimSpace(opt.Label))
		for _, candidate := range loweredCandidates {
			if strings.Contains(label, candidate) {
				return i
			}
		}
	}
	return -1
}

func (m *chatModel) appendLocalJSXMetaMessages(metaMessages []string) {
	if len(metaMessages) == 0 || m.runner == nil || m.runner.app == nil {
		return
	}
	eng := m.runner.app.Engine()
	if eng == nil {
		return
	}
	messages := eng.Messages()
	for _, content := range metaMessages {
		if strings.TrimSpace(content) == "" {
			continue
		}
		messages = append(messages, types.Message{
			Role:      types.RoleUser,
			Content:   content,
			IsMeta:    true,
			Timestamp: time.Now(),
		})
	}
	eng.ReplaceMessages(messages)
}

func (m *chatModel) appendLocalJSXSystemLocalCommandMessage(commandName string, commandArgs string, result string, display string) bool {
	if strings.ToLower(strings.TrimSpace(display)) != "system" {
		return false
	}
	if m.runner == nil || m.runner.app == nil {
		return false
	}
	eng := m.runner.app.Engine()
	if eng == nil {
		return false
	}
	messages := eng.Messages()
	messages = append(messages, buildLocalJSXSystemLocalCommandMessage(commandName, commandArgs, result))
	eng.ReplaceMessages(messages)
	m.runner.syncEntriesFromEngine("")
	return true
}

func (m *chatModel) appendLocalJSXUserLocalCommandMessages(commandName string, commandArgs string, result string, display string) bool {
	if strings.ToLower(strings.TrimSpace(display)) != "user" {
		return false
	}
	if m.runner == nil || m.runner.app == nil {
		return false
	}
	eng := m.runner.app.Engine()
	if eng == nil {
		return false
	}
	messages := eng.Messages()
	messages = append(messages, buildLocalJSXUserCommandMessages(commandName, commandArgs, result)...)
	eng.ReplaceMessages(messages)
	m.runner.syncEntriesFromEngine("")
	return true
}

func (m *chatModel) estimateContextTokenUsage() int {
	messages := m.runner.app.Engine().Messages()
	if len(messages) == 0 {
		return 0
	}
	return services.EstimateMessagesTokenCount(services.ConvertToCompactMessages(messages))
}

func (m *chatModel) currentMainLoopModel() string {
	model := strings.TrimSpace(m.runner.app.State().Snapshot().CurrentModel)
	if model == "" {
		model = strings.TrimSpace(m.runner.app.Config().Model)
	}
	return model
}

func currentFooterPermissionMode() string {
	switch bashperm.GetPermissionChecker().GetMode() {
	case bashperm.PermissionModeAcceptEdits:
		return string(types.PermissionModeAcceptEdits)
	case bashperm.PermissionModeBypassPermissions:
		return string(types.PermissionModeBypass)
	case bashperm.PermissionModeLimitTools:
		return string(types.PermissionModeAuto)
	default:
		return string(types.PermissionModeDefault)
	}
}

func cycleFooterPermissionMode() {
	current := bashperm.GetPermissionChecker().GetMode()
	switch current {
	case bashperm.PermissionModeAsk:
		bashperm.SetGlobalPermissionMode(bashperm.PermissionModeAcceptEdits)
	case bashperm.PermissionModeAcceptEdits:
		bashperm.SetGlobalPermissionMode(bashperm.PermissionModeBypassPermissions)
	default:
		bashperm.SetGlobalPermissionMode(bashperm.PermissionModeAsk)
	}
}

func buildAutoCompactFooterHint(autoCompactEnabled bool, contextWindowOverride int, model string, tokenUsage int) string {
	if !autoCompactEnabled || tokenUsage <= 0 || services.IsCompactWarningSuppressed() {
		return ""
	}
	warning := services.CalculateTokenWarningState(tokenUsage, model, true, contextWindowOverride)
	if !warning.IsAboveWarningThreshold {
		return ""
	}
	return fmt.Sprintf("%d%% until auto-compact", warning.PercentLeft)
}

func (m *chatModel) containsHistory(command string) bool {
	for _, h := range m.history {
		if h == command {
			return true
		}
	}
	return false
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

	entries, _ = m.runner.transformEntriesForDisplay(entries, m.mode)

	// Find entries to print using UUID-based deduplication
	// This prevents duplicates when entries array is rebuilt during streaming
	var toPrint []components.TranscriptEntry
	nextPrintedCount := len(entries)
	for _, entry := range entries {
		if entry.Kind == "assistant_streaming" {
			continue
		}
		key := entry.UUID
		// Use content-based dedup for user entries to handle entry rebuilding
		// (user entries get new UUIDs when rebuilt from engine messages via buildTranscriptEntries)
		if key == "" || entry.Kind == "user" {
			key = fmt.Sprintf("%s|%s|%s|%s", entry.Kind, entry.Title, entry.ToolUseID, entry.Content)
		}
		// UUID-based deduplication: skip if already printed
		if m.printedEntryUUIDs[key] {
			continue
		}
		toPrint = append(toPrint, entry)
		m.printedEntryUUIDs[key] = true
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

func listenForQuestionRequests(runner *ChatRunner) tea.Cmd {
	return func() tea.Msg {
		req := <-runner.questionChan
		return questionRequestMsg{request: req}
	}
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Track commands to batch together
	var cmds []tea.Cmd

	if m.activeLocalJSXModel != nil {
		// For core runtime messages, we need to handle them in the parent model
		// but also forward them to LocalJSX to keep its state (especially spinner ticks)
		switch msg.(type) {
		case localJSXActivatedMsg, localJSXCloseMsg, localJSXDoneMsg:
			// Lifecycle messages - only handled in main switch below
		case spinnerTickMsg, noticeTickMsg:
			// Tick messages - forward to LocalJSX for its spinner, then handle below
			if localCmd := (&m).updateActiveLocalJSXSilent(msg); localCmd != nil {
				cmds = append(cmds, localCmd)
			}
		case streamUpdateMsg, commandResultMsg:
			// Stream/result messages - forward to LocalJSX, then handle below
			if localCmd := (&m).updateActiveLocalJSXSilent(msg); localCmd != nil {
				cmds = append(cmds, localCmd)
			}
		case questionRequestMsg:
			// Permission request - handle in main switch, LocalJSX stays active
			// Do not forward to LocalJSX (it doesn't need permission context)
		case interruptMsg, tea.KeyMsg, tea.WindowSizeMsg:
			// Input events - handled in main switch with special logic
		default:
			// All other messages (including btwTickMsg) - forward to LocalJSX only
			return m, (&m).updateActiveLocalJSX(msg)
		}
	}

	switch msg := msg.(type) {
	case localJSXActivatedMsg:
		if msg.model == nil {
			return m, nil
		}
		if msg.resetBusy {
			m.resetBusyState()
		}
		return m, (&m).activateLocalJSX(msg.model, msg.commandName, msg.lifecycle, msg.commandArgs)
	case localJSXCloseMsg:
		(&m).deactivateLocalJSX()
		if m.runner == nil || m.runner.app == nil {
			return m, nil
		}
		m.refreshFooterState()
		return m, m.flushNewEntriesToScrollback()
	case localJSXDoneMsg:
		(&m).deactivateLocalJSX()
		trimmedResult := strings.TrimSpace(msg.payload.result)
		systemLocalCommandShown := (&m).appendLocalJSXSystemLocalCommandMessage(
			msg.commandName,
			msg.commandArgs,
			msg.payload.result,
			msg.payload.options.Display,
		)
		userLocalCommandShown := (&m).appendLocalJSXUserLocalCommandMessages(
			msg.commandName,
			msg.commandArgs,
			msg.payload.result,
			msg.payload.options.Display,
		)
		if msg.payload.options.Display != "skip" && trimmedResult != "" && m.runner != nil && !systemLocalCommandShown && !userLocalCommandShown {
			entry := components.TranscriptEntry{
				Kind:    "command",
				Title:   "Command /" + strings.TrimSpace(msg.commandName),
				Content: trimmedResult,
			}
			m.runner.mu.Lock()
			m.runner.entries = append(m.runner.entries, entry)
			m.runner.mu.Unlock()
		}
		(&m).appendLocalJSXMetaMessages(buildLocalJSXModelContextMetaMessages(
			msg.commandName,
			msg.payload.result,
			msg.payload.options.Display,
		))
		(&m).appendLocalJSXMetaMessages(msg.payload.options.MetaMessages)
		if nextInput := msg.payload.options.NextInput; nextInput != "" {
			m.currentInput, m.cursorPos = setInputValue(len(nextInput), nextInput)
			m.selectedSuggestion = -1
		}
		shouldSubmitNextInput := msg.payload.options.SubmitNextInput && strings.TrimSpace(m.currentInput) != ""
		shouldContinueQuery := msg.payload.options.ShouldQuery
		if m.runner == nil || m.runner.app == nil {
			if shouldSubmitNextInput {
				return m, submitCurrentInputCmd()
			}
			return m, nil
		}
		m.refreshFooterState()
		if shouldContinueQuery {
			m.prepareStreamingTurn()
			turnCtx, cancel := context.WithCancel(m.ctx)
			m.runner.setActiveCancel(cancel)
			return m, tea.Batch(
				m.flushNewEntriesToScrollback(),
				submitLocalJSXContinuationStreamCmd(turnCtx, m.runner, cancel),
				spinnerTickCmd(),
			)
		}
		if shouldSubmitNextInput {
			return m, tea.Batch(
				m.flushNewEntriesToScrollback(),
				submitCurrentInputCmd(),
			)
		}
		return m, m.flushNewEntriesToScrollback()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// With alternate screen, content is auto re-rendered in View() on resize
		// However, we need to clear the screen first to remove old artifacts
		// This ensures a clean re-render with the new dimensions
		clearCmd := func() tea.Msg { return tea.ClearScreen() }
		if m.activeLocalJSXModel != nil {
			return m, tea.Batch(clearCmd, (&m).updateActiveLocalJSX(msg))
		}
		return m, clearCmd
	case noticeTickMsg:
		m.runner.mu.Lock()
		m.runner.entries = cloneEntries(m.runner.entries)
		m.runner.mu.Unlock()
		m.runner.drainNotices()
		cmds = append(cmds, tickNoticesCmd())
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case spinnerTickMsg:
		if m.state.busy {
			// Increment by 50ms to match TS useAnimationFrame(50) interval
			m.state.spinnerTick += 50
			// Update verb from current task's activeForm (matching TS logic)
			m.updateVerbFromCurrentTask()
			m.refreshTeammateSpinnerState()
			cmds = append(cmds, spinnerTickCmd())
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
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
				if msg.toolCallID != "" {
					m.state.toolCallID = msg.toolCallID
				}
				if msg.toolInput != "" {
					m.state.toolInput = msg.toolInput
				}
				if msg.toolResult == "" {
					m.state.toolInProgress = true
				}
			}
			if msg.toolResult != "" {
				m.state.toolInProgress = false
				m.state.toolInput = ""
			}
			if msg.status != "" {
				m.state.statusText = msg.status
			} else if msg.text != "" && !m.state.toolInProgress {
				m.state.statusText = "Receiving response"
			}
			// Sync entries with streaming tool info for live tool display
			m.runner.syncEntriesFromEngineWithTool(m.state.streamingText, StreamingToolInfo{
				ToolName:   m.state.toolName,
				ToolCallID: m.state.toolCallID,
				ToolInput:  m.state.toolInput,
				InProgress: m.state.toolInProgress,
			})
			// Always flush - dedup logic prevents duplicate prints,
			// and this ensures user entries are printed immediately
			return m, tea.Batch(
				listenForStreamUpdates(m.runner),
				m.flushNewEntriesToScrollback(),
			)
		} else if msg.refresh {
			// Refresh also needs streaming tool info
			m.runner.syncEntriesFromEngineWithTool(m.state.streamingText, StreamingToolInfo{
				ToolName:   m.state.toolName,
				ToolCallID: m.state.toolCallID,
				ToolInput:  m.state.toolInput,
				InProgress: m.state.toolInProgress,
			})
			m.refreshFooterState()
			return m, tea.Batch(
				listenForStreamUpdates(m.runner),
				m.flushNewEntriesToScrollback(),
			)
		}
		return m, listenForStreamUpdates(m.runner)
	case questionRequestMsg:
		m.enqueueQuestionPrompt(msg.request)
		return m, listenForQuestionRequests(m.runner)
	case commandResultMsg:
		// Slash command completed - reset busy state and flush entries
		m.resetBusyState()
		m.refreshFooterState()
		return m, m.flushNewEntriesToScrollback()
	case compactResultMsg:
		// /compact completed - rebuild entries from compacted engine, reset scrollback tracking
		m.refreshFooterState()
		m.resetBusyState()
		m.runner.syncEntriesFromEngine("")
		m.printedEntriesCount = 0
		m.printedEntryUUIDs = make(map[string]bool)
		// Set isInitialFlush so system entries (compact boundary) are also printed
		m.isInitialFlush = true
		cmd := m.flushNewEntriesToScrollback()
		m.isInitialFlush = false
		return m, cmd
	case interruptMsg:
		// Handle interrupt (Ctrl+C) - matches TS useExitOnCtrlCD behavior
		// Double-press mechanism: first press shows warning, second press exits

		// 1. If LocalJSX is active, forward interrupt to it
		if m.activeLocalJSXModel != nil && msg.signal == os.Interrupt.String() {
			if cmd := (&m).updateActiveLocalJSX(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd != nil {
				return m, cmd
			}
			if m.activeLocalJSXLife != nil {
				return m, nil
			}
			return m, localJSXCloseCmd()
		}

		// 2. If busy (streaming), cancel the active operation
		if m.state.busy {
			if m.runner.cancelActive() {
				m.runner.finalizeStreamingEntry(m.state.streamingText)
				m.resetBusyState()
				m.interruptPending = true
				m.interruptPendingTime = time.Now()
				// Show feedback and start timeout timer
				return m, tea.Tick(interruptTimeout, func(t time.Time) tea.Msg {
					return interruptTimeoutMsg{}
				})
			}
		}

		// 3. Double-press check for exit
		if m.interruptPending && time.Since(m.interruptPendingTime) < interruptTimeout {
			// Second Ctrl+C within timeout - exit immediately
			return m, closeEngineAndQuit(m.ctx, m.runner.app.Engine())
		}

		// 4. First Ctrl+C when idle - set pending and show feedback
		m.interruptPending = true
		m.interruptPendingTime = time.Now()
		return m, tea.Tick(interruptTimeout, func(t time.Time) tea.Msg {
			return interruptTimeoutMsg{}
		})

	case interruptTimeoutMsg:
		// Timeout expired - clear interrupt pending state
		m.interruptPending = false
		return m, nil
	case tea.KeyMsg:
		if m.questionPrompt != nil {
			return m.handleQuestionPromptKey(msg)
		}
		if m.activeLocalJSXModel != nil {
			switch msg.Type {
			case tea.KeyEsc, tea.KeyCtrlC:
				if cmd := (&m).updateActiveLocalJSX(msg); cmd != nil {
					return m, cmd
				}
				if m.activeLocalJSXLife != nil {
					return m, nil
				}
				return m, localJSXCloseCmd()
			default:
				return m, (&m).updateActiveLocalJSX(msg)
			}
		}
		switch msg.Type {
		// Removed: PgUp, PgDown, Up, Down, Home, End, CtrlU, CtrlD scroll handling
		// Terminal native scrolling is now used via scrollback output
		case tea.KeyCtrlC:
			// Double-press Ctrl+C mechanism (matches TS useExitOnCtrlCD)
			if m.state.busy {
				// First Ctrl+C when busy: cancel active operation
				if m.runner.cancelActive() {
					m.runner.finalizeStreamingEntry(m.state.streamingText)
					m.resetBusyState()
					m.interruptPending = true
					m.interruptPendingTime = time.Now()
					return m, tea.Tick(interruptTimeout, func(t time.Time) tea.Msg {
						return interruptTimeoutMsg{}
					})
				}
			}
			// Check for double-press
			if m.interruptPending && time.Since(m.interruptPendingTime) < interruptTimeout {
				// Second Ctrl+C - exit immediately
				return m, closeEngineAndQuit(m.ctx, m.runner.app.Engine())
			}
			// First Ctrl+C - set pending
			m.interruptPending = true
			m.interruptPendingTime = time.Now()
			return m, tea.Tick(interruptTimeout, func(t time.Time) tea.Msg {
				return interruptTimeoutMsg{}
			})
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
		case tea.KeyCtrlA:
			// Ctrl+A: Select all text in input field
			m.selectionStart = 0
			m.selectionEnd = len(m.currentInput)
			return m, nil
		case tea.KeyTab, tea.KeyShiftTab:
			if m.state.busy {
				return m, nil
			}
			suggestions := buildSlashSuggestions(m.currentInput, m.runner.app.Commands())
			if len(suggestions) == 0 {
				m.selectedSuggestion = -1
				if msg.Type == tea.KeyShiftTab {
					cycleFooterPermissionMode()
					m.refreshFooterState()
				}
				return m, nil
			}
			delta := 1
			if msg.Type == tea.KeyShiftTab {
				delta = -1
			}
			m.advanceSuggestionSelection(len(suggestions), delta)
			if m.selectedSuggestion >= 0 && m.selectedSuggestion < len(suggestions) {
				m.currentInput, m.cursorPos = setInputValue(
					m.cursorPos,
					applySlashSuggestion(m.currentInput, suggestions[m.selectedSuggestion].Command),
				)
				m.cursorPos = len(m.currentInput) // Move cursor to end after suggestion
			}
			return m, nil
		case tea.KeyEnter:
			if !m.state.busy && m.applySelectedSuggestionIfActive() {
				return m, nil
			}
			line := strings.TrimSpace(m.currentInput)

			// Handle immediate commands (like /btw) during streaming
			// These can execute even when busy without interrupting the main agent
			if m.state.busy && strings.HasPrefix(line, "/") {
				// Check if this is an immediate command
				cmdName := strings.TrimPrefix(line, "/")
				cmdArgs := ""
				if i := strings.Index(cmdName, " "); i > 0 {
					cmdArgs = strings.TrimSpace(cmdName[i+1:])
					cmdName = cmdName[:i]
				}
				matchingCmd := m.runner.findCommand(cmdName)
				if matchingCmd != nil && matchingCmd.GetBase().Immediate {
					// Execute immediate command during streaming
					m.currentInput, m.cursorPos = clearInputValue()
					m.selectedSuggestion = -1

					// Handle based on command kind
					switch matchingCmd.GetKind() {
					case command.KindLocal:
						// For Local commands, execute handler directly and show result inline
						runtime := m.runner.buildCommandRuntime(m.ctx)
						result, handled, err := m.runner.app.Commands().Execute(m.ctx, line, runtime)
						if err != nil {
							m.runner.mu.Lock()
							m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
							m.runner.mu.Unlock()
							return m, nil
						}
						if !handled {
							m.runner.mu.Lock()
							m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Command", Content: "Command not handled"})
							m.runner.mu.Unlock()
							return m, nil
						}
						// Show result in transcript
						m.runner.mu.Lock()
						entryKind := "command"
						if result.Display == "user" {
							entryKind = "user"
						} else if result.Display == "system" {
							entryKind = "system"
						}
						m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
							Kind:    entryKind,
							Title:   "Side Question",
							Content: result.Value,
						})
						m.runner.mu.Unlock()
						return m, nil

					case command.KindLocalJSX:
						// For JSX commands, use modal with lifecycle
						lifecycle := newLocalJSXLifecycle()
						localRuntime := m.runner.buildLocalJSXCommandRuntime(m.ctx, lifecycle)
						model, _, modelHandled, modelErr := m.runner.app.Commands().LoadModel(m.ctx, line, localRuntime)
						if modelErr != nil {
							m.runner.mu.Lock()
							m.runner.entries = append(m.runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: modelErr.Error()})
							m.runner.mu.Unlock()
							return m, nil
						}
						if modelHandled && model != nil {
							return m, func() tea.Msg {
								return localJSXActivatedMsg{
									commandName: cmdName,
									commandArgs: cmdArgs,
									model:       model,
									lifecycle:   lifecycle,
								}
							}
						}
						return m, nil

					default:
						// Unknown kind - show notice
						m.runner.mu.Lock()
						m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
							Kind:    "notice",
							Title:   "Command",
							Content: fmt.Sprintf("Cannot execute /%s (unsupported command type) while agent is running.", cmdName),
						})
						m.runner.mu.Unlock()
						return m, nil
					}
				}
				// Non-immediate slash command during busy: show notice and clear input
				m.runner.mu.Lock()
				m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
					Kind:    "notice",
					Title:   "Agent Running",
					Content: fmt.Sprintf("Cannot execute /%s while agent is running. Use /btw for side questions or wait for completion.", cmdName),
				})
				m.runner.mu.Unlock()
				m.currentInput, m.cursorPos = clearInputValue()
				return m, nil
			}

			if m.state.busy {
				// Non-command input during busy: show notice and clear input
				if line != "" {
					m.runner.mu.Lock()
					m.runner.entries = append(m.runner.entries, components.TranscriptEntry{
						Kind:    "notice",
						Title:   "Agent Running",
						Content: "Cannot submit input while agent is running. Use /btw for side questions or Ctrl+C to stop.",
					})
					m.runner.mu.Unlock()
					m.currentInput, m.cursorPos = clearInputValue()
				}
				return m, nil
			}

			if line == "" {
				return m, nil
			}
			switch line {
			case "/exit", "/quit":
				return m, closeEngineAndQuit(m.ctx, m.runner.app.Engine())
			case "/clear":
				m.currentInput, m.cursorPos = clearInputValue()
				m.selectedSuggestion = -1
				m.runner.mu.Lock()
				m.runner.entries = nil
				m.printedEntriesCount = 0 // Reset scrollback tracking
				m.printedEntryUUIDs = make(map[string]bool)
				m.runner.mu.Unlock()
				return m, nil
			}

			// Check if this is a slash command
			if strings.HasPrefix(line, "/") {
				m.currentInput, m.cursorPos = clearInputValue()
				m.selectedSuggestion = -1
				m.state.busy = true
				m.state.startedAt = time.Now()
				// For /compact, show specific verb
				compactCmd := strings.TrimPrefix(line, "/")
				if compactCmd == "compact" || strings.HasPrefix(compactCmd, "compact ") {
					m.state.verb = "Compacting"
					m.state.statusText = "Compacting session..."
				}
				turnCtx, cancel := context.WithCancel(m.ctx)
				m.runner.setActiveCancel(cancel)
				return m, tea.Batch(
					runSlashCommandCmd(turnCtx, m.runner, line, cancel),
					spinnerTickCmd(),
				)
			}

			m.prepareStreamingTurn()
			// User message will be rendered in View() automatically (alternate screen mode)
			// No need to print to scrollback - entries are added to runner.entries and rendered in View()

			// Add to history (both in-memory and persistent)
			if line != "" && !m.containsHistory(line) {
				m.history = append([]string{line}, m.history...)
				// Trim history to reasonable size
				if len(m.history) > 100 {
					m.history = m.history[:100]
				}
				// Save to persistent storage
				if m.historyManager != nil {
					m.historyManager.Add(line)
				}
			}

			// Expand paste references before submitting to API
			if m.pasteManager != nil {
				line = m.pasteManager.ExpandInput(line)
			}
			m.currentInput, m.cursorPos = clearInputValue()
			m.selectionStart = -1
			m.selectionEnd = -1
			turnCtx, cancel := context.WithCancel(m.ctx)
			m.runner.setActiveCancel(cancel)
			return m, tea.Batch(
				submitLineStreamCmd(turnCtx, m.runner, line, cancel),
				spinnerTickCmd(),
			)
		case tea.KeyBackspace, tea.KeyDelete:
			// Allow backspace during busy state for editing immediate commands
			// If there's a selection, delete the selected text
			if m.selectionStart >= 0 && m.selectionEnd > m.selectionStart {
				m.currentInput = m.currentInput[:m.selectionStart] + m.currentInput[m.selectionEnd:]
				m.cursorPos = m.selectionStart
				m.selectionStart = -1
				m.selectionEnd = -1
			} else {
				m.currentInput, m.cursorPos = deleteAtCursorOrBackspace(m.currentInput, m.cursorPos)
			}
			m.selectedSuggestion = -1
			return m, nil
		case tea.KeyUp:
			// Navigate to previous history entry (only when not busy)
			if !m.state.busy && len(m.history) > 0 {
				if m.historyIndex == -1 {
					// Save current draft before navigating
					m.historyDraft = m.currentInput
					m.historyIndex = len(m.history) - 1
				} else if m.historyIndex > 0 {
					m.historyIndex--
				}
				m.currentInput = m.history[m.historyIndex]
				m.cursorPos = len(m.currentInput) // Move cursor to end
				m.selectedSuggestion = -1
			}
			return m, nil
		case tea.KeyDown:
			// Navigate to next history entry (only when not busy)
			if !m.state.busy && m.historyIndex != -1 {
				if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
					m.currentInput = m.history[m.historyIndex]
				} else {
					// Back to draft
					m.historyIndex = -1
					m.currentInput = m.historyDraft
				}
				m.cursorPos = len(m.currentInput) // Move cursor to end
				m.selectedSuggestion = -1
			}
			return m, nil
		case tea.KeyLeft:
			// Move cursor left (by rune, not byte) - allow during busy state
			if m.selectionStart >= 0 && m.selectionEnd > m.selectionStart {
				m.cursorPos = m.selectionStart
				m.selectionStart = -1
				m.selectionEnd = -1
			} else if m.cursorPos > 0 {
				_, runeLen := utf8.DecodeLastRuneInString(m.currentInput[:m.cursorPos])
				m.cursorPos -= runeLen
			}
			return m, nil
		case tea.KeyRight:
			// Move cursor right (by rune, not byte) - allow during busy state
			if m.selectionStart >= 0 && m.selectionEnd > m.selectionStart {
				m.cursorPos = m.selectionEnd
				m.selectionStart = -1
				m.selectionEnd = -1
			} else if m.cursorPos < len(m.currentInput) {
				_, runeLen := utf8.DecodeRuneInString(m.currentInput[m.cursorPos:])
				m.cursorPos += runeLen
			}
			return m, nil
		case tea.KeyHome:
			// Move cursor to start - allow during busy state
			m.cursorPos = 0
			return m, nil
		case tea.KeyEnd:
			// Move cursor to end - allow during busy state
			m.cursorPos = len(m.currentInput)
			return m, nil
		default:
			// Allow input during busy state for immediate commands (like /btw)
			m.cursorPos = clampCursorPos(m.currentInput, m.cursorPos)
			if msg.Type == tea.KeyRunes {
				// Insert at cursor position
				runes := string(msg.Runes)
				// Handle bracketed paste: collapse large/multi-line pastes
				if msg.Paste {
					if m.pasteManager == nil {
						m.pasteManager = paste.NewManager()
					}
					runes = m.pasteManager.AddPaste(runes)
				}
				// If there's a selection, replace it with the typed text
				if m.selectionStart >= 0 && m.selectionEnd > m.selectionStart {
					m.currentInput = m.currentInput[:m.selectionStart] + runes + m.currentInput[m.selectionEnd:]
					m.cursorPos = m.selectionStart + len(runes)
				} else {
					m.currentInput, m.cursorPos = insertAtCursor(m.currentInput, m.cursorPos, runes)
				}
				m.selectedSuggestion = -1
				m.selectionStart = -1
				m.selectionEnd = -1
			} else if s := msg.String(); s != "" && utf8.ValidString(s) && !strings.HasPrefix(s, "ctrl+") {
				// Handle arrow keys that might come as strings
				switch s {
				case "left":
					if m.cursorPos > 0 {
						_, runeLen := utf8.DecodeLastRuneInString(m.currentInput[:m.cursorPos])
						m.cursorPos -= runeLen
					}
					return m, nil
				case "right":
					if m.cursorPos < len(m.currentInput) {
						_, runeLen := utf8.DecodeRuneInString(m.currentInput[m.cursorPos:])
						m.cursorPos += runeLen
					}
					return m, nil
				case "home":
					m.cursorPos = 0
					return m, nil
				case "end":
					m.cursorPos = len(m.currentInput)
					return m, nil
				case "up", "down", "pgup", "pgdown":
					// These are handled by other cases
					return m, nil
				}
				// Insert at cursor position
				m.currentInput, m.cursorPos = insertAtCursor(m.currentInput, m.cursorPos, s)
				m.selectedSuggestion = -1
			}
			return m, nil
		}
		// Mouse scroll wheel not handled - terminal native scrolling used
	}
	return m, nil
}

func (m chatModel) View() string {
	// If both LocalJSX modal and questionPrompt are active, render as overlay
	// to preserve the permission prompt visibility
	if m.activeLocalJSXModel != nil && m.questionPrompt != nil {
		return m.renderLocalJSXOverlayView()
	}

	if m.activeLocalJSXModel != nil {
		return m.renderLocalJSXModalView()
	}

	return m.renderMainView()
}

// renderLocalJSXOverlayView renders LocalJSX as an overlay above the main view
// This preserves the question prompt visibility when both are active
func (m chatModel) renderLocalJSXOverlayView() string {
	// Render the main view (includes question prompt)
	mainView := m.renderMainView()

	// Render the LocalJSX modal content as a compact overlay
	modalTitle := strings.TrimSpace(m.activeLocalJSXName)
	if modalTitle == "" {
		modalTitle = "panel"
	}

	body := strings.TrimSpace(m.activeLocalJSXModel.View())
	header := fmt.Sprintf("/%s · Press Esc to close", modalTitle)

	// Build overlay with header and body
	var overlayParts []string
	overlayParts = append(overlayParts, "")
	overlayParts = append(overlayParts, ui.Style(&ui.Dark.Warning, nil, header, true))
	if body != "" {
		// Limit body height to not cover entire screen
		maxBodyLines := 10
		bodyLines := strings.Split(body, "\n")
		if len(bodyLines) > maxBodyLines {
			bodyLines = bodyLines[:maxBodyLines]
			bodyLines = append(bodyLines, "... (scroll in fullscreen mode)")
		}
		overlayParts = append(overlayParts, strings.Join(bodyLines, "\n"))
	}
	overlay := strings.Join(overlayParts, "\n")

	// Combine main view with overlay at the bottom
	// The overlay appears above the input area
	return mainView + "\n" + overlay
}

func (m chatModel) renderMainView() string {
	if m.runner == nil || m.runner.app == nil {
		if strings.TrimSpace(m.state.streamingText) == "" {
			return ""
		}
		return m.state.streamingText
	}

	// In scrollback mode: only render current streaming content + input area
	// Historical content has already been printed to terminal scrollback via tea.Println()

	width := maxInt(72, m.width)

	var parts []string

	// If there's active streaming content, show it
	if m.state.busy && m.state.streamingText != "" {
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

	// If there's a question prompt, show it
	if m.questionPrompt != nil {
		parts = append(parts, renderQuestionPromptBlock(width, m.questionPrompt, m.state.toolInput, m.state.toolName))
	}

	// Interrupt pending warning
	if m.interruptPending {
		warningStyle := ui.Style(&ui.Dark.Warning, nil, "⚠ Press Ctrl+C again to exit", true)
		parts = append(parts, warningStyle)
	}

	// Always show input area at the bottom
	suggestions := buildSlashSuggestions(m.currentInput, m.runner.app.Commands())
	if m.selectedSuggestion >= len(suggestions) {
		m.selectedSuggestion = -1
	}
	input := ui.RenderInputPanel(
		width,
		m.runner.app.State().Snapshot(),
		m.currentInput,
		m.cursorPos,
		m.selectionStart,
		m.selectionEnd,
		suggestions,
		m.selectedSuggestion,
		m.state.busy,
		m.state.spinnerTick,
		m.state.toolName,
		m.state.statusText,
		m.state.startedAt,
		m.state.verb,
		m.state.tokenCount,
		0,
		m.state.teammates,
		m.state.teammateVerb,
		m.state.teammateTokens,
		m.state.permissionMode,
		m.state.footerRightHint,
	)
	parts = append(parts, input)

	return strings.Join(parts, "\n")
}

func shouldRenderStandaloneStreaming(entries []components.TranscriptEntry, busy bool, streamingText string) bool {
	if !busy || strings.TrimSpace(streamingText) == "" {
		return false
	}
	for _, entry := range entries {
		if entry.Kind == "assistant_streaming" && strings.TrimSpace(entry.Content) != "" {
			return false
		}
	}
	return true
}

func (m chatModel) renderLocalJSXModalView() string {
	width := maxInt(72, m.width)
	height := m.height
	if height <= 0 {
		height = 24
	}

	mainView := m.renderMainView()
	modalTitle := strings.TrimSpace(m.activeLocalJSXName)
	if modalTitle == "" {
		modalTitle = "panel"
	}

	body := strings.TrimSpace(m.activeLocalJSXModel.View())
	header := fmt.Sprintf("/%s · Press Esc to close", modalTitle)
	modalView := header
	if body != "" {
		modalView = modalView + "\n" + body
	}

	layout := uicomponents.FullscreenLayoutStateFor(width, height)
	layout.ShowModal = true
	layout.SetContentHeight(countLines(mainView))

	return uicomponents.RenderFullscreenLayout(layout, uicomponents.FullscreenLayoutRegions{
		Scrollable: mainView,
		Modal:      modalView,
	})
}

func countLines(content string) int {
	if strings.TrimSpace(content) == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func renderQuestionPromptBlock(width int, prompt *activeQuestionPrompt, toolInput string, toolName string) string {
	if prompt == nil {
		return ""
	}

	dialogWidth := width - 6
	if dialogWidth < 56 {
		dialogWidth = 56
	}
	if dialogWidth > 96 {
		dialogWidth = 96
	}

	question := prompt.request.question
	if isPermissionQuestion(question) {
		return renderPermissionQuestionDialog(dialogWidth, prompt, toolInput, toolName)
	}
	return renderGenericQuestionDialog(dialogWidth, prompt)
}

func renderPermissionQuestionDialog(width int, prompt *activeQuestionPrompt, toolInput string, toolName string) string {
	question := prompt.request.question
	title, command, reason, hint, extraLines := parsePermissionQuestion(question.Question)
	if title == "" {
		title = "Permission Required"
	}

	content := make([]string, 0, 24)
	if command != "" {
		// Show command with visual indicator
		content = append(content, renderCommandBlock(command))
	}
	if reason != "" {
		if len(content) > 0 {
			content = append(content, "")
		}
		content = append(content, "Reason: "+reason)
	}
	if hint != "" {
		content = append(content, "Hint: "+hint)
	}
	if len(extraLines) > 0 {
		if len(content) > 0 {
			content = append(content, "")
		}
		content = append(content, extraLines...)
	}

	// For file edit operations, render diff preview
	if toolName == "Edit" && toolInput != "" {
		diffPreview := renderFileEditDiffPreview(toolInput, width)
		if diffPreview != "" {
			if len(content) > 0 {
				content = append(content, "")
			}
			content = append(content, diffPreview)
		}
	}

	if len(content) > 0 {
		content = append(content, "")
	}
	if isPermissionPotentiallyDangerous(command, reason) {
		warningText := "WARNING: This operation may modify files or system state."
		content = append(content, renderWarningText(warningText))
		content = append(content, "")
	}
	content = append(content, "Choose an action:")
	content = append(content, renderQuestionOptions(prompt, maxInt(32, width-12))...)

	inputHint := "Enter to select · y allow once · a always allow · n deny · Esc to cancel"
	if prompt.request.multiSelect {
		inputHint = "Enter to select · ↑/↓ move · Space toggle · Esc to cancel"
	}

	dialogColor := uicomponents.DialogColorPermission
	if isPermissionPotentiallyDangerous(command, reason) {
		dialogColor = uicomponents.DialogColorWarning
	}

	return uicomponents.RenderDialog(uicomponents.DialogConfig{
		Title:         title,
		Content:       content,
		Color:         dialogColor,
		Width:         width,
		InputHint:     inputHint,
		TopBorderOnly: true,
	})
}

// renderCommandBlock renders a command in a styled block (matches TS PermissionDialog styling)
func renderCommandBlock(command string) string {
	// Add visual indentation and subtle styling
	return "Command:\n  " + command
}

// renderWarningText renders warning text with appropriate styling
func renderWarningText(text string) string {
	// Use ANSI codes for warning color (yellow/orange)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", 255, 193, 7, text)
}

// renderFileEditDiffPreview renders a diff preview for file edit permission requests
// Matches TS FileEditToolDiff behavior
func renderFileEditDiffPreview(toolInputJSON string, width int) string {
	// Parse the tool input JSON
	var input struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}

	if err := json.Unmarshal([]byte(toolInputJSON), &input); err != nil {
		return ""
	}

	if input.FilePath == "" {
		return ""
	}

	// Read current file content if it exists
	currentContent, err := os.ReadFile(input.FilePath)
	if err != nil {
		// File doesn't exist or can't be read - show the proposed content
		if input.OldString == "" {
			// Creating new file
			return renderNewFilePreview(input.FilePath, input.NewString, width)
		}
		// Can't read file - just show old vs new
		return diff.RenderEditDiffPreview(input.FilePath, input.OldString, input.NewString, width)
	}

	// Generate diff between current content and proposed change
	oldContent := string(currentContent)
	// If oldString is specific, find where it is in the file
	if input.OldString != "" && strings.Contains(oldContent, input.OldString) {
		// We can show a focused diff
		return diff.RenderEditDiffPreview(input.FilePath, input.OldString, input.NewString, width)
	}

	// Show full file diff preview
	return diff.RenderEditDiffPreview(input.FilePath, oldContent, input.NewString, width)
}

// renderNewFilePreview renders a preview for creating a new file
func renderNewFilePreview(filePath, content string, width int) string {
	var lines []string
	lines = append(lines, uicomponents.RenderDimText("  Creating new file: "+filePath))
	lines = append(lines, "")

	// Show first few lines of proposed content
	contentLines := strings.Split(content, "\n")
	maxLines := 10
	for i, line := range contentLines {
		if i >= maxLines {
			lines = append(lines, uicomponents.RenderDimText("  … (more content below)"))
			break
		}
		// Truncate long lines
		if len(line) > width-8 {
			line = line[:width-11] + "…"
		}
		lines = append(lines, fmt.Sprintf("\033[38;2;%d;%d;%dm  + %s\033[0m", 78, 186, 101, line))
	}

	return strings.Join(lines, "\n")
}

func renderGenericQuestionDialog(width int, prompt *activeQuestionPrompt) string {
	question := prompt.request.question
	header := strings.TrimSpace(question.Header)
	if header == "" {
		header = "Question"
	}

	content := make([]string, 0, 24)
	for _, line := range strings.Split(strings.TrimSpace(question.Question), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		content = append(content, line)
	}
	if len(content) > 0 {
		content = append(content, "")
	}
	content = append(content, "Options:")
	content = append(content, renderQuestionOptions(prompt, maxInt(32, width-12))...)

	// Preview section with proper styling (matches TS PreviewQuestionView)
	if preview := strings.TrimSpace(selectedQuestionOptionPreview(prompt)); preview != "" {
		content = append(content, "")
		content = append(content, "\033[2mPreview:\033[0m")
		previewLines := strings.Split(preview, "\n")
		maxPreviewLines := 8
		if len(previewLines) > maxPreviewLines {
			previewLines = previewLines[:maxPreviewLines]
			previewLines = append(previewLines, "\033[2m...\033[0m")
		}
		for _, line := range previewLines {
			// Preview content with indentation and subtle styling
			content = append(content, "  "+line)
		}
	}

	inputHint := "Enter to select · ↑/↓ to navigate · 1-9 quick select · Esc to cancel"
	if prompt.request.multiSelect {
		inputHint = "Enter to select · ↑/↓ move · Space toggle · Esc to cancel"
	}

	return uicomponents.RenderDialog(uicomponents.DialogConfig{
		Title:         "Interactive Prompt",
		Subtitle:      header,
		Content:       content,
		Color:         uicomponents.DialogColorInfo,
		Width:         width,
		InputHint:     inputHint,
		TopBorderOnly: true,
	})
}

func renderQuestionOptions(prompt *activeQuestionPrompt, maxLabelWidth int) []string {
	lines := make([]string, 0, len(prompt.request.question.Options)*2)

	for i, opt := range prompt.request.question.Options {
		// Use figures.pointer (›) for focused selection indicator (matches TS Select)
		cursor := " "
		if i == prompt.selected {
			cursor = uicomponents.FigurePointer
		}

		label := strings.TrimSpace(opt.Label)
		if len(label) > maxLabelWidth {
			label = truncateString(label, maxLabelWidth)
		}

		// Apply suggestion color to focused option label (matches TS Select styling)
		if i == prompt.selected && !prompt.request.multiSelect {
			label = renderSuggestionText(label)
		}

		// Build option line
		if prompt.request.multiSelect {
			// Use figures.tick (✔) for checked items (matches TS SelectMulti)
			checked := " "
			if prompt.selectedMany[i] {
				checked = uicomponents.FigureTick
			}
			// Apply success color to checked checkbox (matches TS SelectMulti styling)
			checkBox := fmt.Sprintf("[%s]", checked)
			if prompt.selectedMany[i] {
				checkBox = renderSuccessText(checkBox)
			}
			// Dimmed number index (matches TS SelectMulti)
			indexText := uicomponents.RenderDimText(fmt.Sprintf("%d.", i+1))
			line := fmt.Sprintf("%s %s %s %s", cursor, checkBox, indexText, label)
			lines = append(lines, line)
		} else {
			// Single select: numbered style with dimmed index (matches TS Select)
			indexText := uicomponents.RenderDimText(fmt.Sprintf("%d.", i+1))
			line := fmt.Sprintf("%s %s %s", cursor, indexText, label)
			lines = append(lines, line)
		}

		// Add description with proper indentation and dimmed styling
		desc := strings.TrimSpace(opt.Description)
		if desc != "" {
			if len(desc) > maxLabelWidth {
				desc = truncateString(desc, maxLabelWidth)
			}
			// Dimmed description (matches TS OptionWithDescription)
			descLine := fmt.Sprintf("    %s", uicomponents.RenderDimText(desc))
			lines = append(lines, descLine)
		}
	}

	// Add "Chat about this" footer option (matches TS QuestionView footer)
	numOptions := len(prompt.request.question.Options)
	chatCursor := " "
	chatLabel := fmt.Sprintf("%d. Chat about this", numOptions+1)
	lines = append(lines, "")
	lines = append(lines, chatCursor+" "+uicomponents.RenderDimText(chatLabel))

	return lines
}

// renderSuggestionText renders text with suggestion color (rgb(177,185,249))
func renderSuggestionText(text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", 177, 185, 249, text)
}

// renderSuccessText renders text with success color (rgb(78,186,101))
func renderSuccessText(text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", 78, 186, 101, text)
}

// renderInactiveText renders text with inactive color (rgb(153,153,153))
func renderInactiveText(text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", 153, 153, 153, text)
}

func selectedQuestionOptionPreview(prompt *activeQuestionPrompt) string {
	if prompt == nil {
		return ""
	}
	index := prompt.selected
	options := prompt.request.question.Options
	if index < 0 || index >= len(options) {
		return ""
	}
	return options[index].Preview
}

func isPermissionQuestion(q interaction.Question) bool {
	header := strings.ToLower(strings.TrimSpace(q.Header))
	if header == "permission" {
		return true
	}

	hasAllow := false
	hasDeny := false
	for _, opt := range q.Options {
		label := strings.ToLower(strings.TrimSpace(opt.Label))
		if strings.Contains(label, "allow") {
			hasAllow = true
		}
		if strings.Contains(label, "deny") {
			hasDeny = true
		}
	}
	return hasAllow && hasDeny
}

func parsePermissionQuestion(raw string) (title, command, reason, hint string, extraLines []string) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	expectCommand := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasSuffix(lower, "needs permission to run:"):
			tool := strings.TrimSpace(line[:len(line)-len("needs permission to run:")])
			if tool != "" {
				title = tool + " Permission"
			}
			expectCommand = true
		case strings.HasPrefix(lower, "reason:"):
			reason = strings.TrimSpace(line[len("reason:"):])
		case strings.HasPrefix(lower, "hint:"):
			hint = strings.TrimSpace(line[len("hint:"):])
		case expectCommand && command == "":
			command = line
			expectCommand = false
		default:
			extraLines = append(extraLines, line)
		}
	}

	if title == "" {
		title = "Permission Required"
	}
	return title, command, reason, hint, extraLines
}

func isPermissionPotentiallyDangerous(command, reason string) bool {
	candidate := strings.ToLower(strings.TrimSpace(command + " " + reason))
	if candidate == "" {
		return false
	}
	dangerSignals := []string{
		"destructive",
		"rm ",
		"rmdir",
		"chmod",
		"chown",
		"mv ",
		"dd ",
		"mkfs",
		"shutdown",
		"reboot",
		"poweroff",
	}
	for _, signal := range dangerSignals {
		if strings.Contains(candidate, signal) {
			return true
		}
	}
	return false
}

func (m *chatModel) activateLocalJSX(model tea.Model, commandName string, lifecycle *localJSXLifecycle, commandArgs ...string) tea.Cmd {
	args := ""
	if len(commandArgs) > 0 {
		args = commandArgs[0]
	}
	m.activeLocalJSXModel = model
	m.activeLocalJSXName = commandName
	m.activeLocalJSXArgs = args
	m.activeLocalJSXLife = lifecycle

	var cmds []tea.Cmd
	if initCmd := wrapLocalJSXCmd(model.Init()); initCmd != nil {
		cmds = append(cmds, initCmd)
	}
	if m.width > 0 && m.height > 0 {
		if resizeCmd := m.updateActiveLocalJSX(tea.WindowSizeMsg{Width: m.width, Height: m.height}); resizeCmd != nil {
			cmds = append(cmds, resizeCmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *chatModel) deactivateLocalJSX() {
	m.activeLocalJSXModel = nil
	m.activeLocalJSXName = ""
	m.activeLocalJSXArgs = ""
	m.activeLocalJSXLife = nil
}

func (m *chatModel) updateActiveLocalJSX(msg tea.Msg) tea.Cmd {
	if m.activeLocalJSXModel == nil {
		return nil
	}
	if m.activeLocalJSXLife != nil {
		if done, ok := m.activeLocalJSXLife.TakeDone(); ok {
			return localJSXDoneCmd(m.activeLocalJSXName, m.activeLocalJSXArgs, done)
		}
	}
	if m.activeLocalJSXLife != nil && m.activeLocalJSXLife.TakeCloseRequested() {
		return localJSXCloseCmd()
	}
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		width, height := localJSXWindowSize(sizeMsg.Width, sizeMsg.Height)
		msg = tea.WindowSizeMsg{Width: width, Height: height}
	}
	nextModel, cmd := m.activeLocalJSXModel.Update(msg)
	if nextModel != nil {
		m.activeLocalJSXModel = nextModel
	}
	if m.activeLocalJSXLife != nil {
		if done, ok := m.activeLocalJSXLife.TakeDone(); ok {
			return localJSXDoneCmd(m.activeLocalJSXName, m.activeLocalJSXArgs, done)
		}
	}
	if m.activeLocalJSXLife != nil && m.activeLocalJSXLife.TakeCloseRequested() {
		return localJSXCloseCmd()
	}
	return wrapLocalJSXCmd(cmd)
}

// updateActiveLocalJSXSilent updates the LocalJSX model without checking lifecycle state.
// Used for tick messages that need to keep the LocalJSX spinner running while
// permission prompts are active.
func (m *chatModel) updateActiveLocalJSXSilent(msg tea.Msg) tea.Cmd {
	if m.activeLocalJSXModel == nil {
		return nil
	}
	// Don't check lifecycle done/close - those are handled in main switch
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		width, height := localJSXWindowSize(sizeMsg.Width, sizeMsg.Height)
		msg = tea.WindowSizeMsg{Width: width, Height: height}
	}
	nextModel, cmd := m.activeLocalJSXModel.Update(msg)
	if nextModel != nil {
		m.activeLocalJSXModel = nextModel
	}
	return wrapLocalJSXCmd(cmd)
}

func localJSXCloseCmd() tea.Cmd {
	return func() tea.Msg {
		return localJSXCloseMsg{}
	}
}

// closeEngineAndQuit saves the session before exiting
func closeEngineAndQuit(ctx context.Context, eng *engine.Engine) tea.Cmd {
	return func() tea.Msg {
		if eng != nil {
			// Save session before exit - use short timeout to avoid hanging
			saveCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = eng.Close(saveCtx, "exit")
		}
		return tea.QuitMsg{}
	}
}

func localJSXDoneCmd(commandName string, commandArgs string, payload localJSXDonePayload) tea.Cmd {
	return func() tea.Msg {
		return localJSXDoneMsg{
			commandName: commandName,
			commandArgs: commandArgs,
			payload:     payload,
		}
	}
}

func submitCurrentInputCmd() tea.Cmd {
	return func() tea.Msg {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
}

func buildLocalJSXModelContextMetaMessages(commandName string, result string, display string) []string {
	_ = commandName
	_ = result
	switch strings.ToLower(strings.TrimSpace(display)) {
	case "skip", "system", "user":
		return nil
	}
	return nil
}

func buildLocalJSXStdoutPayload(result string) string {
	content := result
	if strings.TrimSpace(content) == "" {
		content = localJSXNoContentMessage
	}
	return "<" + types.LocalCommandStdoutTag + ">" + content + "</" + types.LocalCommandStdoutTag + ">"
}

func buildLocalCommandOutputContent(stdout string, stderr string) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(stderr); trimmed != "" {
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, "\n")
}

func buildLocalJSXSystemLocalCommandMessage(commandName string, commandArgs string, result string) types.Message {
	return types.Message{
		Role:                      types.RoleSystem,
		Type:                      types.SystemSubtypeLocalCommand,
		Content:                   types.FormatCommandInputTags(strings.TrimSpace(commandName), strings.TrimSpace(commandArgs)) + "\n" + buildLocalJSXStdoutPayload(result),
		IsVisibleInTranscriptOnly: true,
		Timestamp:                 time.Now(),
	}
}

func buildLocalJSXUserCommandMessages(commandName string, commandArgs string, result string) []types.Message {
	return []types.Message{
		{
			Role:      types.RoleUser,
			Content:   types.FormatCommandInputTags(strings.TrimSpace(commandName), strings.TrimSpace(commandArgs)),
			Timestamp: time.Now(),
		},
		{
			Role:      types.RoleUser,
			Content:   buildLocalJSXStdoutPayload(result),
			Timestamp: time.Now(),
		},
	}
}

func wrapLocalJSXCmd(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if msg == nil {
			return nil
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return localJSXCloseMsg{}
		}
		return msg
	}
}

func localJSXWindowSize(width, height int) (int, int) {
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 24
	}
	modalHeight := height - uicomponents.ModalTranscriptPeek - 2
	if modalHeight < 6 {
		modalHeight = maxInt(6, height-1)
	}
	return width, modalHeight
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
	m.currentInput, m.cursorPos = setInputValue(
		m.cursorPos,
		applySlashSuggestion(m.currentInput, suggestions[m.selectedSuggestion].Command),
	)
	m.cursorPos = len(m.currentInput) // Move cursor to end after suggestion
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
			} else if chunk.ToolName != "" {
				toolName = chunk.ToolName
				if chunk.Status != "" {
					status = chunk.Status
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
				refresh:    len(chunk.ToolCalls) > 0 || chunk.ToolResult != "" || chunk.ToolName != "",
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

func submitLocalJSXContinuationStreamCmd(ctx context.Context, runner *ChatRunner, cancel context.CancelFunc) tea.Cmd {
	// Add streaming placeholder immediately (no visible user input in this path).
	runner.mu.Lock()
	runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "assistant_streaming", Title: "Claude", Content: ""})
	runner.mu.Unlock()

	go func() {
		defer runner.clearActiveCancel(cancel)
		start := time.Now()
		runner.sendStreamUpdate(streamUpdate{status: "Waiting for model response"}, false)
		_, err := runner.app.Engine().ContinueStream(ctx, func(chunk engine.StreamChunk) error {
			toolName := ""
			toolInput := ""
			status := ""
			if len(chunk.ToolCalls) > 0 {
				toolName = chunk.ToolCalls[0].Name
				if inputBytes, marshalErr := json.Marshal(chunk.ToolCalls[0].Input); marshalErr == nil {
					toolInput = string(inputBytes)
				}
				if chunk.Status != "" {
					status = chunk.Status
				} else {
					status = "Running tool: " + toolName
				}
			} else if chunk.ToolName != "" {
				toolName = chunk.ToolName
				if chunk.Status != "" {
					status = chunk.Status
				}
			} else if chunk.Text != "" {
				status = "Receiving response"
			} else if chunk.Status != "" {
				status = chunk.Status
			}
			runner.sendStreamUpdate(streamUpdate{
				text:       chunk.Text,
				toolName:   toolName,
				toolCallID: chunk.ToolCallID,
				toolInput:  toolInput,
				toolResult: chunk.ToolResult,
				status:     status,
				refresh:    len(chunk.ToolCalls) > 0 || chunk.ToolResult != "" || chunk.ToolName != "",
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
		runner.syncEntriesFromEngine("")

		runner.drainNotices()
		runner.sendStreamUpdate(streamUpdate{done: true}, true)
	}()

	return func() tea.Msg {
		time.Sleep(10 * time.Millisecond)
		return streamUpdateMsg{}
	}
}

func splitSlashCommandLine(line string) (name string, args string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", ""
	}
	withoutSlash := strings.TrimPrefix(trimmed, "/")
	if withoutSlash == "" {
		return "", ""
	}
	space := strings.IndexRune(withoutSlash, ' ')
	if space < 0 {
		return strings.TrimSpace(withoutSlash), ""
	}
	return strings.TrimSpace(withoutSlash[:space]), strings.TrimSpace(withoutSlash[space+1:])
}

func runSlashCommandCmd(ctx context.Context, runner *ChatRunner, line string, cancel context.CancelFunc) tea.Cmd {
	return func() tea.Msg {
		defer runner.clearActiveCancel(cancel)
		commandName, commandArgs := splitSlashCommandLine(line)

		// Check if this is a prompt command that should be streamed to the model
		cmd, ok := runner.app.Commands().Lookup(line)
		runtime := runner.buildCommandRuntime(ctx)

		if ok && cmd.GetKind() == command.KindPrompt {
			// For prompt commands, expand the prompt and stream to model
			out, handled, err := runner.app.Commands().Execute(ctx, line, runtime)
			if err != nil {
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
				runner.mu.Unlock()
				return commandResultMsg{}
			}
			if handled && out.Value != "" {
				name := commandName
				if name == "" {
					name = strings.TrimPrefix(strings.Fields(line)[0], "/")
				}
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
					return commandResultMsg{}
				}
				runner.app.State().SetSessionID(runner.app.Engine().SessionID())
				runner.app.State().RecordTurn(runner.app.Config().Model, time.Since(start))
				// Don't pass resp.Text as streamingText - the assistant message is already in engine.Messages
				runner.syncEntriesFromEngine("")
				runner.drainNotices()
				runner.sendStreamUpdate(streamUpdate{done: true}, true)
				return commandResultMsg{}
			}
			return commandResultMsg{}
		}

		if ok && cmd.GetKind() == command.KindLocalJSX {
			name := commandName
			if name == "" {
				name = strings.TrimPrefix(strings.Fields(line)[0], "/")
			}
			lifecycle := newLocalJSXLifecycle()
			localRuntime := runner.buildLocalJSXCommandRuntime(ctx, lifecycle)

			// Prefer model rendering path for native LocalJSX commands.
			model, _, modelHandled, modelErr := runner.app.Commands().LoadModel(ctx, line, localRuntime)
			if modelErr != nil {
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: modelErr.Error()})
				runner.mu.Unlock()
				return commandResultMsg{}
			}
			if modelHandled && model != nil {
				return localJSXActivatedMsg{
					commandName: name,
					commandArgs: commandArgs,
					model:       model,
					lifecycle:   lifecycle,
					resetBusy:   true,
				}
			}

			// Compatibility path for local-jsx handlers that return plain text.
			out, handled, err := runner.app.Commands().Execute(ctx, line, localRuntime)
			if err != nil {
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
				runner.mu.Unlock()
				return commandResultMsg{}
			}
			if handled {
				runner.mu.Lock()
				runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "panel", Title: "Panel /" + name, Content: out.Value})
				runner.mu.Unlock()
				return commandResultMsg{}
			}
			return commandResultMsg{}
		}

		// For local commands, execute and display output
		out, handled, err := runner.app.Commands().Execute(ctx, line, runtime)
		if err != nil {
			runner.mu.Lock()
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "error", Title: "Error", Content: err.Error()})
			runner.mu.Unlock()
			return commandResultMsg{}
		}
		if handled {
			name := commandName
			if name == "" {
				name = strings.TrimPrefix(strings.Fields(line)[0], "/")
			}

			// For /compact, engine already has the boundary marker from CompactSession
			if name == "compact" && out.Type == command.ResultTypeCompact {
				messages := runner.app.Engine().Messages()
				after := len(messages)
				before := 0
				for _, outLine := range strings.Split(out.Value, "\n") {
					if strings.HasPrefix(outLine, "messages_before=") {
						if n, err := strconv.Atoi(strings.TrimPrefix(outLine, "messages_before=")); err == nil {
							before = n
							break
						}
					}
				}
				return compactResultMsg{before: before, after: after}
			}

			runner.mu.Lock()
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "command", Title: "Command /" + name, Content: out.Value})
			runner.mu.Unlock()

			return commandResultMsg{}
		}

		// Command not found - check if it looks like a command name or regular text
		// Matches TS looksLikeCommand(): names should only contain [a-zA-Z0-9:_-]
		if !looksLikeCommand(commandName) {
			// Doesn't look like a command (e.g., "/var/log", "/path/to/file")
			// Treat as regular user input and submit to model
			runner.mu.Lock()
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "user", Title: "You", Content: line})
			runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "assistant_streaming", Title: "Claude", Content: ""})
			runner.mu.Unlock()

			// Run streaming in goroutine to allow UI updates
			go func() {
				defer runner.clearActiveCancel(cancel)
				start := time.Now()
				runner.sendStreamUpdate(streamUpdate{status: "Waiting for model response"}, false)
				_, err := runner.app.Engine().SubmitStream(ctx, line, func(chunk engine.StreamChunk) error {
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
					return
				}
				runner.app.State().SetSessionID(runner.app.Engine().SessionID())
				runner.app.State().RecordTurn(runner.app.Config().Model, time.Since(start))
				runner.syncEntriesFromEngine("")
				runner.drainNotices()
				runner.sendStreamUpdate(streamUpdate{done: true}, true)
			}()

			// Return immediately with stream update to trigger flush
			return func() tea.Msg {
				time.Sleep(10 * time.Millisecond)
				return streamUpdateMsg{}
			}
		}

		// Looks like a command name but is unknown - show error
		runner.mu.Lock()
		runner.entries = append(runner.entries, components.TranscriptEntry{Kind: "notice", Title: "Unknown skill", Content: "Unknown skill: " + commandName})
		runner.mu.Unlock()
		return commandResultMsg{}
	}
}

// looksLikeCommand checks whether a name looks like a command identifier.
// Command names should only contain [a-zA-Z0-9:_-].
// Matches TS processSlashCommand.tsx looksLikeCommand().
func looksLikeCommand(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ':' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func renderLocalJSXModelSnapshot(model tea.Model, width, height int) string {
	if model == nil {
		return ""
	}
	if width <= 0 {
		width = 100
	}
	if height <= 0 {
		height = 24
	}

	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	if updatedModel != nil {
		model = updatedModel
	}

	return strings.TrimSpace(model.View())
}

func buildTranscriptEntries(messages []types.Message, streamingText string) []components.TranscriptEntry {
	entries := make([]components.TranscriptEntry, 0, len(messages))
	toolNames := map[string]string{}
	lastUserCommandName := ""

	for idx, msg := range messages {
		if msg.IsMeta {
			continue
		}
		uuid := fmt.Sprintf("msg-%d", idx)

		if msg.Type == types.MessageTypeAttachment {
			attachmentSubtype, attachmentData, attachmentContent := parseAttachmentTranscriptPayload(msg.Content)
			if shouldSuppressAttachmentEntry(attachmentSubtype, attachmentData, attachmentContent) {
				continue
			}
			entries = append(entries, components.TranscriptEntry{
				Kind:      "attachment",
				Title:     "Attachment",
				Content:   attachmentContent,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
				Subtype:   attachmentSubtype,
				Data:      attachmentData,
			})
			continue
		}
		if strings.EqualFold(strings.TrimSpace(msg.Type), types.MessageTypeProgress) ||
			strings.EqualFold(strings.TrimSpace(msg.Role), types.MessageTypeProgress) {
			progressSubtype, progressData, progressContent := parseProgressTranscriptPayload(msg)
			if strings.TrimSpace(progressContent) == "" {
				continue
			}
			entries = append(entries, components.TranscriptEntry{
				Kind:      "progress",
				Title:     "Progress",
				Content:   progressContent,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
				Subtype:   progressSubtype,
				Data:      progressData,
				ToolUseID: msg.ToolCallID,
			})
			continue
		}

		switch msg.Role {
		case types.RoleUser:
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}

			commandName := strings.TrimSpace(types.ExtractTag(msg.Content, types.CommandNameTag))
			if commandName != "" {
				lastUserCommandName = commandName
			}
			hasLocalStdout := strings.Contains(msg.Content, "<"+types.LocalCommandStdoutTag)
			hasLocalStderr := strings.Contains(msg.Content, "<"+types.LocalCommandStderrTag)
			hasLocalCommandOutput := hasLocalStdout || hasLocalStderr
			stdout := types.ExtractTag(msg.Content, types.LocalCommandStdoutTag)
			stderr := types.ExtractTag(msg.Content, types.LocalCommandStderrTag)
			if commandName != "" && !hasLocalCommandOutput {
				// Breadcrumb-only message, defer rendering until stdout message arrives.
				continue
			}
			if hasLocalCommandOutput {
				title := "Command"
				if commandName != "" {
					title = "Command " + commandName
				} else if lastUserCommandName != "" {
					title = "Command " + lastUserCommandName
				}
				content := buildLocalCommandOutputContent(stdout, stderr)
				if strings.TrimSpace(content) == "" {
					content = localJSXNoContentMessage
				}
				entries = append(entries, components.TranscriptEntry{
					Kind:      "command",
					Title:     title,
					Content:   content,
					UUID:      uuid,
					Timestamp: msg.Timestamp,
				})
				lastUserCommandName = ""
				continue
			}

			// Filter control messages (matching TS UserTextMessage.tsx:39-59)
			if isControlMessage(msg.Content) {
				continue
			}

			// Compact summary messages render with markdown enabled
			kind := "user"
			if msg.IsCompactSummary {
				kind = "compact_summary"
			}

			entries = append(entries, components.TranscriptEntry{
				Kind:      kind,
				Title:     "You",
				Content:   msg.Content,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
			})
		case types.RoleAssistant:
			// Process content blocks (thinking, text, tool_use)
			blockIdx := 0
			for _, block := range msg.Blocks {
				switch block.Type {
				case "thinking":
					entries = append(entries, components.TranscriptEntry{
						Kind:      "thinking",
						Content:   block.Thinking,
						UUID:      fmt.Sprintf("%s-block-%d", uuid, blockIdx),
						Timestamp: msg.Timestamp,
					})
					blockIdx++
				case "redacted_thinking":
					entries = append(entries, components.TranscriptEntry{
						Kind:      "redacted_thinking",
						UUID:      fmt.Sprintf("%s-block-%d", uuid, blockIdx),
						Timestamp: msg.Timestamp,
					})
					blockIdx++
				case "text":
					if strings.TrimSpace(block.Text) != "" {
						entries = append(entries, components.TranscriptEntry{
							Kind:      "assistant",
							Title:     "Claude",
							Content:   block.Text,
							UUID:      fmt.Sprintf("%s-block-%d", uuid, blockIdx),
							Timestamp: msg.Timestamp,
						})
						blockIdx++
					}
				case "tool_use":
					toolNames[block.ID] = block.Name
					displayName, summary := summarizeToolUseDisplay(block.Name, string(block.Input))
					entries = append(entries, components.TranscriptEntry{
						Kind:      "tool_use",
						Title:     displayName,
						Content:   summary,
						UUID:      fmt.Sprintf("%s-block-%d", uuid, blockIdx),
						Timestamp: msg.Timestamp,
						ToolName:  displayName,
						ToolInput: string(block.Input),
						ToolUseID: block.ID,
					})
					blockIdx++
				}
			}

			// Fallback: if no blocks but has simple content
			if len(msg.Blocks) == 0 && strings.TrimSpace(msg.Content) != "" {
				entries = append(entries, components.TranscriptEntry{
					Kind:      "assistant",
					Title:     "Claude",
					Content:   msg.Content,
					UUID:      uuid,
					Timestamp: msg.Timestamp,
				})
			}

			// Legacy tool calls support (if not using blocks)
			if len(msg.Blocks) == 0 {
				for i, call := range msg.ToolCalls {
					toolNames[call.ID] = call.Name
					displayName, summary := summarizeToolUseDisplay(call.Name, string(call.Arguments))
					entries = append(entries, components.TranscriptEntry{
						Kind:      "tool_use",
						Title:     displayName,
						Content:   summary,
						UUID:      fmt.Sprintf("%s-tool-%d", uuid, i),
						Timestamp: msg.Timestamp,
						ToolName:  displayName,
						ToolInput: string(call.Arguments),
						ToolUseID: call.ID,
					})
				}
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
			systemSubtype := strings.TrimSpace(msg.Type)
			systemData := ""
			systemContent := msg.Content
			trimmed := strings.TrimSpace(msg.Content)
			if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
				var payload map[string]any
				if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
					if subtype, ok := payload["subtype"].(string); ok && strings.TrimSpace(subtype) != "" {
						systemSubtype = strings.TrimSpace(subtype)
					}
					if contentText, ok := payload["content"].(string); ok && strings.TrimSpace(contentText) != "" {
						systemContent = contentText
					}
					systemData = trimmed
				}
			}
			if strings.EqualFold(systemSubtype, types.SystemSubtypeLocalCommand) {
				commandName := strings.TrimSpace(types.ExtractTag(systemContent, types.CommandNameTag))
				stdout := types.ExtractTag(systemContent, types.LocalCommandStdoutTag)
				stderr := types.ExtractTag(systemContent, types.LocalCommandStderrTag)
				content := buildLocalCommandOutputContent(stdout, stderr)
				if strings.TrimSpace(content) == "" {
					content = localJSXNoContentMessage
				}
				title := "Command"
				if commandName != "" {
					title = "Command " + commandName
				}
				entries = append(entries, components.TranscriptEntry{
					Kind:      "command",
					Title:     title,
					Content:   content,
					UUID:      uuid,
					Timestamp: msg.Timestamp,
					Subtype:   systemSubtype,
					Data:      systemData,
				})
				continue
			}
			entries = append(entries, components.TranscriptEntry{
				Kind:      "system",
				Title:     "System",
				Content:   systemContent,
				UUID:      uuid,
				Timestamp: msg.Timestamp,
				Subtype:   systemSubtype,
				Data:      systemData,
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

func parseAttachmentTranscriptPayload(content string) (subtype string, data string, display string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", ""
	}
	if !(strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) {
		return "", "", trimmed
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", "", trimmed
	}
	attType, _ := payload["type"].(string)
	if attType == "" {
		attType, _ = payload["attachmentType"].(string)
	}
	if contentText, ok := payload["content"].(string); ok && strings.TrimSpace(contentText) != "" {
		return strings.TrimSpace(attType), trimmed, contentText
	}
	return strings.TrimSpace(attType), trimmed, trimmed
}

func parseProgressTranscriptPayload(msg types.Message) (subtype string, data string, display string) {
	payload, ok := extractProgressPayload(msg)
	if !ok {
		trimmed := strings.TrimSpace(msg.Content)
		if trimmed == "" {
			return "", "", ""
		}
		return "", "", trimmed
	}

	raw, err := json.Marshal(payload)
	if err == nil {
		data = string(raw)
	}

	if value, ok := payload["type"].(string); ok {
		subtype = strings.TrimSpace(value)
	}
	display = renderProgressDisplay(subtype, payload)
	if strings.TrimSpace(display) == "" {
		display = subtype
	}
	return strings.TrimSpace(subtype), strings.TrimSpace(data), strings.TrimSpace(display)
}

func extractProgressPayload(msg types.Message) (map[string]any, bool) {
	if payload, ok := extractProgressPayloadFromValue(msg.ToolUseResult); ok {
		return payload, true
	}
	return extractProgressPayloadFromValue(msg.Content)
}

func extractProgressPayloadFromValue(value any) (map[string]any, bool) {
	switch raw := value.(type) {
	case map[string]any:
		if nested, ok := raw["data"].(map[string]any); ok && len(nested) > 0 {
			return nested, true
		}
		if _, ok := raw["type"].(string); ok {
			return raw, true
		}
		return nil, false
	case json.RawMessage:
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, false
		}
		if nested, ok := payload["data"].(map[string]any); ok && len(nested) > 0 {
			return nested, true
		}
		return payload, true
	case []byte:
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, false
		}
		if nested, ok := payload["data"].(map[string]any); ok && len(nested) > 0 {
			return nested, true
		}
		return payload, true
	case string:
		trimmed := strings.TrimSpace(raw)
		if !(strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) {
			return nil, false
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
			return nil, false
		}
		if nested, ok := payload["data"].(map[string]any); ok && len(nested) > 0 {
			return nested, true
		}
		return payload, true
	default:
		return nil, false
	}
}

func renderProgressDisplay(subtype string, payload map[string]any) string {
	if message, ok := payload["message"].(string); ok && strings.TrimSpace(message) != "" {
		return strings.TrimSpace(message)
	}

	switch strings.ToLower(strings.TrimSpace(subtype)) {
	case "repl_tool_call":
		toolName, _ := payload["toolName"].(string)
		if strings.TrimSpace(toolName) == "" {
			toolName = "REPL"
		}
		phase, _ := payload["phase"].(string)
		switch strings.ToLower(strings.TrimSpace(phase)) {
		case "start":
			return fmt.Sprintf("%s started", strings.TrimSpace(toolName))
		case "end", "done", "finish":
			if errText, ok := payload["error"].(string); ok && strings.TrimSpace(errText) != "" {
				return fmt.Sprintf("%s failed: %s", strings.TrimSpace(toolName), strings.TrimSpace(errText))
			}
			if status, ok := payload["status"].(string); ok && strings.EqualFold(strings.TrimSpace(status), "error") {
				return fmt.Sprintf("%s failed", strings.TrimSpace(toolName))
			}
			return fmt.Sprintf("%s completed", strings.TrimSpace(toolName))
		}
	}

	if status, ok := payload["status"].(string); ok && strings.TrimSpace(status) != "" {
		return strings.TrimSpace(status)
	}

	return ""
}

// shouldSuppressAttachmentEntry hides noisy placeholder hook attachments from transcript UI.
// Specifically suppresses default post_turn hook echoes like "post_turn" / "Hook completed".
func shouldSuppressAttachmentEntry(_ string, data string, content string) bool {
	trimmedData := strings.TrimSpace(data)
	if trimmedData == "" {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmedData), &payload); err != nil {
		return false
	}

	hookEvent, _ := payload["hookEvent"].(string)
	if !strings.EqualFold(strings.TrimSpace(hookEvent), "post_turn") {
		return false
	}

	contentNorm := strings.TrimSpace(content)
	if strings.EqualFold(contentNorm, "post_turn") || strings.EqualFold(contentNorm, "hook completed") {
		return true
	}

	commandText, _ := payload["command"].(string)
	if strings.EqualFold(strings.TrimSpace(commandText), "echo post_turn") {
		return true
	}

	return false
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
	if suggestions := buildSlashSubcommandSuggestions(current); len(suggestions) > 0 {
		return suggestions
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
			Details:     slashSuggestionDetails(base),
		})
		if len(suggestions) >= 6 {
			break
		}
	}
	return suggestions
}

func buildSlashSubcommandSuggestions(current string) []ui.SlashSuggestion {
	fields := strings.Fields(current)
	if len(fields) < 2 {
		return nil
	}
	commandName := strings.TrimPrefix(strings.ToLower(fields[0]), "/")
	switch commandName {
	case "model", "models":
		return buildModelSubcommandSuggestions(fields)
	default:
		return nil
	}
}

func buildModelSubcommandSuggestions(fields []string) []ui.SlashSuggestion {
	type modelSubcommand struct {
		name        string
		description string
		details     []string
	}
	subcommands := []modelSubcommand{
		{name: "list", description: "List saved API profiles", details: []string{"Usage: /model list"}},
		{name: "current", description: "Show the active API profile and runtime config", details: []string{"Usage: /model current"}},
		{name: "add", description: "Add or update an API profile", details: []string{"Usage: /model add <name> key=<api-key> url=<base-url> model=<model> [summary=<model>]"}},
		{name: "use", description: "Switch to a saved API profile", details: []string{"Usage: /model use <name>"}},
		{name: "remove", description: "Delete a saved API profile", details: []string{"Usage: /model remove <name>"}},
		{name: "default", description: "Reset to configured model", details: []string{"Usage: /model default"}},
	}

	query := ""
	if len(fields) >= 2 {
		query = strings.ToLower(fields[1])
	}
	out := make([]ui.SlashSuggestion, 0, len(subcommands))
	for _, subcommand := range subcommands {
		if query != "" && !strings.HasPrefix(subcommand.name, query) {
			continue
		}
		out = append(out, ui.SlashSuggestion{
			Command:     "/model " + subcommand.name,
			Description: subcommand.description,
			Details:     subcommand.details,
		})
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func slashSuggestionDetails(base command.CommandBase) []string {
	name := strings.ToLower(strings.TrimSpace(base.Name))
	switch name {
	case "model":
		return []string{
			"Enter /model for guided API profile setup",
			"/model list · /model current",
			"/model use <name> · /model remove <name>",
			"/model add <name> key=<api-key> url=<base-url> model=<model> [summary=<model>]",
		}
	}
	if strings.TrimSpace(base.ArgumentHint) == "" {
		return nil
	}
	return []string{"Usage: /" + base.Name + " " + strings.TrimSpace(base.ArgumentHint)}
}

func applySlashSuggestion(current, selected string) string {
	if selected == "" {
		return current
	}
	trimmedLeft := strings.TrimLeft(current, " \t")
	if !strings.HasPrefix(trimmedLeft, "/") {
		return selected
	}
	currentFields := strings.Fields(trimmedLeft)
	selectedFields := strings.Fields(selected)
	if len(currentFields) == 0 || len(selectedFields) == 0 {
		return selected
	}
	preserveFrom := len(selectedFields)
	if preserveFrom > len(currentFields) {
		preserveFrom = len(currentFields)
	}
	if len(currentFields) <= preserveFrom {
		return selected
	}
	return selected + " " + strings.Join(currentFields[preserveFrom:], " ")
}

func (r *ChatRunner) buildCommandRuntime(ctx context.Context) command.Runtime {
	return command.Runtime{
		Engine:   r.app.Engine(),
		Provider: r.app.Provider(),
		Agents:   r.app.Agents(),
		Tools:    r.app.Services().Tools(),
		State:    r.app.State(),
		Config:   r.app.Config(),
		CompactSession: func(maxMessages int) (before int, after int) {
			messages := r.app.Engine().Messages()
			before = len(messages)
			if before == 0 {
				return 0, 0
			}
			// Convert to CompactMessage for compaction
			compactMessages := services.ConvertToCompactMessages(messages)
			result, err := r.app.Services().Compact().Compact(ctx, compactMessages, "", false)
			if err != nil {
				return before, before
			}
			// Order matches TS: boundaryMarker, summaryMessages, attachments, hookResults
			compacted := make([]services.CompactMessage, 0, 4)
			compacted = append(compacted, result.BoundaryMarker)
			compacted = append(compacted, result.SummaryMessages...)
			compacted = append(compacted, result.Attachments...)
			compacted = append(compacted, result.HookResults...)
			// Convert back to types.Message
			r.app.Engine().ReplaceMessages(services.ConvertFromCompactMessages(compacted))
			after = len(compacted)
			// Reset runner entries so they will be rebuilt from the compacted engine messages
			// This is crucial for UI consistency after compact
			r.mu.Lock()
			r.entries = nil
			r.mu.Unlock()
			return before, after
		},
		SaveSessionTitle: func(sessionID, title string) error {
			// Save custom title to transcript
			tm := r.app.Services().Transcripts()
			if tm != nil {
				return tm.SaveCustomTitle(sessionID, title)
			}
			return nil
		},
		RewindMessages: func(toIndex int) error {
			return r.app.Engine().RewindMessages(toIndex)
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
		OnExit:  func() {},
		OnClear: func() {},
		OnThemeChange: func(theme string) {
			_ = theme
		},
		OnModelChange: func(model string) {
			next := strings.TrimSpace(model)
			if next == "" {
				return
			}
			r.app.State().SetCurrentModel(next)
		},
		OnConfigChange: func(cfg config.Config) error {
			r.app.ApplyConfig(cfg)
			return nil
		},
		OnLocalJSXDone: func(result string, options command.LocalJSXDoneOptions) {
			_ = result
			_ = options
		},
		Commands: func() []command.Command {
			return r.app.Commands().List()
		},
	}
}

func (r *ChatRunner) buildLocalJSXCommandRuntime(ctx context.Context, lifecycle *localJSXLifecycle) command.Runtime {
	rt := r.buildCommandRuntime(ctx)
	if lifecycle == nil {
		return rt
	}
	rt.OnExit = lifecycle.RequestClose
	rt.OnClear = lifecycle.RequestClose
	rt.OnLocalJSXDone = lifecycle.Done
	return rt
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
