package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"claude-go/internal/ui/components"
	"claude-go/internal/ui/dialogs"
	"claude-go/internal/ui/input"
	"claude-go/internal/ui/messages"
	"claude-go/internal/ui/status"
)

// Model represents the main Bubble Tea model for the CLI
// This integrates all UI components into a single interactive model
type Model struct {
	// Screen dimensions
	width  int
	height int

	// View state
	mode             ViewMode
	transcriptScroll int

	// Messages
	messages []messages.Message
	// Per-message verbose toggle from click interaction
	clickExpanded map[int]bool
	// Last clicked message index in transcript
	clickedMessageIndex int

	// Input
	input *input.PromptInputState
	// History search mode (Ctrl+R)
	historySearchActive bool
	historyQuery        string
	historyMatch        string
	historyFailed       bool
	historyBrowseOffset int
	historyDraft        string

	// Spinner state
	spinner *SpinnerState

	// Dialog (nil when no dialog shown)
	dialog *DialogState
	globalSearch *dialogs.GlobalSearchState
	modelPicker  *dialogs.ModelPickerState
	quickOpen    *dialogs.QuickOpenState

	// Status
	statusConfig status.StatusLineConfig

	// Timing
	startedAt time.Time
	tickMs    int

	// Flags
	busy          bool
	streaming     bool
	reducedMotion bool

	// Phase 2 layout/scroll primitives
	layoutState   *components.FullscreenLayoutState
	virtualState  *components.VirtualListState
	messageZones  *components.ZoneRegistry
	zoneToMessage map[string]int
}

// DialogState holds the current dialog state
type DialogState struct {
	Type    DialogType
	Content interface{} // Type-specific content
	OnYes   func()
	OnNo    func()
}

// DialogType represents different dialog types
type DialogType int

const (
	DialogPermission DialogType = iota
	DialogTrust
	DialogAutoMode
	DialogExport
	DialogCostThreshold
)

// Model creates a new UI model
func ModelFor() Model {
	// Get initial terminal size instead of using hardcoded values
	// This matches the original TS behavior where process.stdout.columns is used
	size := GetTerminalSize()
	width := size.Width
	height := size.Height

	layout := components.FullscreenLayoutStateFor(width, height)
	in := input.PromptInputStateFor(width - 4)
	// Phase 3 baseline: keep disabled by default to preserve existing behavior.
	in.EnableVimMode(false)
	return Model{
		width:               width,
		height:              height,
		mode:                ViewModeNormal,
		input:               in,
		spinner:             SpinnerStateFor(),
		layoutState:         layout,
		virtualState:        components.VirtualListStateFor(nil, width-4, max(3, height-7)),
		messageZones:        components.ZoneRegistryFor(),
		zoneToMessage:       make(map[string]int),
		clickExpanded:       make(map[int]bool),
		clickedMessageIndex: -1,
		startedAt:           time.Now(),
		statusConfig: status.StatusLineConfig{
			Width: width,
		},
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tick()
}

// tickMsg is sent on each animation tick
type tickMsg time.Time

// tick returns a command that sends tick messages
func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*120, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		m.statusConfig.Width = msg.Width
		m.layoutState.SetSize(msg.Width, msg.Height)
		m.virtualState.SetViewport(msg.Width-4, max(3, msg.Height-7))
		return m, nil

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tickMsg:
		m.tickMs += 120
		return m, tick()

	case messages.Message:
		m.messages = append(m.messages, msg)
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle dialog keys first
	if m.quickOpen != nil {
		return m.handleQuickOpenKey(msg)
	}
	if m.modelPicker != nil {
		return m.handleModelPickerKey(msg)
	}
	if m.globalSearch != nil {
		return m.handleGlobalSearchKey(msg)
	}
	if m.dialog != nil {
		return m.handleDialogKey(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "ctrl+r":
		if m.busy {
			return m, nil
		}
		if !m.historySearchActive {
			m.historySearchActive = true
			m.historyQuery = ""
			m.historyMatch = m.input.Value
			m.historyFailed = false
			m.historyBrowseOffset = 0
			m.historyDraft = m.input.Value
			return m, nil
		}
		m.historyBrowseOffset++
		m.updateHistorySearchMatch()
		return m, nil

	case "ctrl+o":
		// Toggle view mode
		switch m.mode {
		case ViewModeNormal:
			m.mode = ViewModeVerbose
		case ViewModeVerbose:
			m.mode = ViewModeTranscript
		default:
			m.mode = ViewModeNormal
		}
		return m, nil
	case "ctrl+g":
		m.globalSearch = dialogs.GlobalSearchStateFor(max(60, m.width-4), max(16, m.height-6))
		return m, nil
	case "ctrl+k":
		m.quickOpen = dialogs.QuickOpenStateFor(max(60, m.width-4), max(16, m.height-6))
		return m, nil
	case "ctrl+p":
		current := strings.TrimSpace(m.statusConfig.Model)
		if current == "" {
			current = "gpt-4.1"
		}
		m.modelPicker = dialogs.ModelPickerStateFor(current)
		return m, nil

	case "up":
		if m.historySearchActive {
			m.historyBrowseOffset++
			m.updateHistorySearchMatch()
			return m, nil
		}
		if !m.busy && m.input.PrevHistory() {
			return m, nil
		}
		if m.virtualState != nil && m.virtualState.HandleKey("up") {
			m.transcriptScroll = m.virtualState.ScrollOffset
			return m, nil
		}
		if m.transcriptScroll < len(m.messages)-1 {
			m.transcriptScroll++
		}
		return m, nil

	case "down":
		if m.historySearchActive {
			if m.historyBrowseOffset > 0 {
				m.historyBrowseOffset--
			}
			m.updateHistorySearchMatch()
			return m, nil
		}
		if !m.busy && m.input.NextHistory() {
			return m, nil
		}
		if m.virtualState != nil && m.virtualState.HandleKey("down") {
			m.transcriptScroll = m.virtualState.ScrollOffset
			return m, nil
		}
		if m.transcriptScroll > 0 {
			m.transcriptScroll--
		}
		return m, nil

	case "pgup", "ctrl+b":
		if m.virtualState != nil && m.virtualState.HandleKey("pgup") {
			m.transcriptScroll = m.virtualState.ScrollOffset
			return m, nil
		}
		return m, nil

	case "pgdown", "ctrl+f":
		if m.virtualState != nil && m.virtualState.HandleKey("pgdown") {
			m.transcriptScroll = m.virtualState.ScrollOffset
			return m, nil
		}
		return m, nil

	case "enter":
		if m.historySearchActive {
			if !m.historyFailed && strings.TrimSpace(m.historyMatch) != "" {
				m.input.SetValue(m.historyMatch)
				m.input.MoveEnd()
			}
			m.exitHistorySearch(false)
			return m, nil
		}
		if m.input.Value != "" && !m.busy {
			// Submit input - return a command to handle it
			value := m.input.Value
			m.input.AddHistory(value)
			m.input.Clear()
			return m, func() tea.Msg {
				return submitMsg{text: value}
			}
		}
		return m, nil

	case "backspace":
		if m.historySearchActive {
			queryRunes := []rune(m.historyQuery)
			if len(queryRunes) == 0 {
				m.exitHistorySearch(true)
				return m, nil
			}
			m.historyQuery = string(queryRunes[:len(queryRunes)-1])
			m.historyBrowseOffset = 0
			m.updateHistorySearchMatch()
			return m, nil
		}
		m.input.Backspace()
		return m, nil

	case "left":
		if m.historySearchActive {
			return m, nil
		}
		m.input.MoveLeft()
		return m, nil

	case "right":
		if m.historySearchActive {
			return m, nil
		}
		m.input.MoveRight()
		return m, nil

	case "home", "ctrl+a":
		if m.historySearchActive {
			return m, nil
		}
		m.input.MoveHome()
		return m, nil

	case "end", "ctrl+e":
		if m.historySearchActive {
			return m, nil
		}
		m.input.MoveEnd()
		return m, nil

	case "esc":
		if m.historySearchActive {
			m.exitHistorySearch(true)
			return m, nil
		}
		return m, nil

	default:
		if m.historySearchActive {
			if len(msg.String()) == 1 {
				m.historyQuery += msg.String()
				m.historyBrowseOffset = 0
				m.updateHistorySearchMatch()
			}
			return m, nil
		}
		if handled, swallow := m.input.HandleVimKey(msg.String()); handled || swallow {
			return m, nil
		}
		// Handle paste (bracketed paste mode)
		if msg.Paste {
			text := string(msg.Runes)
			ref := m.input.HandlePaste(text)
			m.input.Insert(ref)
			return m, nil
		}
		// Insert character
		if len(msg.String()) == 1 {
			m.input.Insert(msg.String())
		}
		return m, nil
	}
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.dialog != nil {
		return m, nil
	}

	if m.virtualState != nil && m.virtualState.HandleMouse(msg) {
		m.transcriptScroll = m.virtualState.ScrollOffset
		return m, nil
	}

	if m.messageZones != nil {
		for zoneID, idx := range m.zoneToMessage {
			if components.GlobalInBounds(zoneID, msg) {
				if idx < 0 || idx >= len(m.messages) || !messages.IsClickableForExpand(m.messages[idx]) {
					return m, nil
				}
				if m.clickedMessageIndex == idx {
					delete(m.clickExpanded, idx)
					m.clickedMessageIndex = -1
				} else {
					m.clickExpanded[idx] = true
					m.clickedMessageIndex = idx
				}
				return m, nil
			}
		}
	}
	return m, nil
}

// handleDialogKey handles keyboard input when a dialog is shown
func (m Model) handleDialogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if m.dialog.OnYes != nil {
			m.dialog.OnYes()
		}
		m.dialog = nil
		return m, nil

	case "n", "N", "esc":
		if m.dialog.OnNo != nil {
			m.dialog.OnNo()
		}
		m.dialog = nil
		return m, nil

	case "ctrl+c":
		return m, tea.Quit
	}

	return m, nil
}

// submitMsg is sent when input is submitted
type submitMsg struct {
	text string
}

// View implements tea.Model
func (m Model) View() string {
	if m.quickOpen != nil {
		return m.quickOpen.View()
	}
	if m.modelPicker != nil {
		return m.modelPicker.View()
	}
	if m.globalSearch != nil {
		return m.globalSearch.View()
	}
	// If dialog is shown, render just the dialog
	if m.dialog != nil {
		return m.renderDialog()
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Messages
	if len(m.messages) > 0 {
		sections = append(sections, m.renderMessages())
	}

	// Spinner (if busy)
	if m.busy {
		sections = append(sections, m.renderSpinner())
	}

	// Input
	sections = append(sections, m.renderInput())

	// Status line
	sections = append(sections, m.renderStatusLine())

	return components.GlobalScan(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderHeader renders the header section
func (m Model) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D77757")).
		Bold(true)

	return titleStyle.Render(" Claude Code ") + lipgloss.NewStyle().
		Foreground(lipgloss.Color("#869198")).
		Render("v1.0")
}

// renderMessages renders the message list
func (m Model) renderMessages() string {
	verbose := m.mode == ViewModeVerbose || m.mode == ViewModeTranscript
	width := max(20, m.width-4)
	if m.virtualState == nil {
		m.virtualState = components.VirtualListStateFor(nil, width, max(3, m.height-7))
	}
	m.virtualState.SetViewport(width, max(3, m.height-7))

	items := make([]components.VirtualListItem, 0, len(m.messages))
	for i := range m.messages {
		msg := m.messages[i]
		msgVerbose := verbose || m.clickExpanded[i]
		rendered := messages.RenderMessage(msg, width, msgVerbose)
		if messages.IsClickableForExpand(msg) {
			rendered = components.GlobalMark(fmt.Sprintf("message-%d", i), rendered)
		}
		items = append(items, messageVirtualItem{
			id:      fmt.Sprintf("msg-%d", i),
			content: rendered,
		})
	}
	m.virtualState.SetItems(items)

	cfg := components.VirtualListConfig{
		Items:       items,
		Width:       width,
		Height:      max(3, m.height-7),
		ShowPointer: false,
	}
	rendered := components.RenderVirtualList(m.virtualState, cfg)
	if m.messageZones == nil {
		m.messageZones = components.ZoneRegistryFor()
	}
	m.zoneToMessage = make(map[string]int, len(m.messages))
	for i := range m.messages {
		if messages.IsClickableForExpand(m.messages[i]) {
			zoneID := fmt.Sprintf("message-%d", i)
			m.zoneToMessage[zoneID] = i
		}
	}

	return rendered
}

// renderSpinner renders the loading spinner
func (m Model) renderSpinner() string {
	verb := m.spinner.GetVerb()
	frame := GetSpinnerFrame(m.tickMs)

	spinnerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D77757"))

	return spinnerStyle.Render(frame+" ") + verb + "…"
}

// renderInput renders the input section
func (m Model) renderInput() string {
	m.input.IsFocused = !m.busy
	if m.historySearchActive {
		prompt := input.RenderPromptInput(m.input)
		status := "search history"
		if m.historyQuery != "" {
			status = "search history: " + m.historyQuery
		}
		if m.historyFailed {
			status += " (no match)"
		}
		footer := input.RenderPromptFooter(max(10, m.width-4), []string{"Enter accept", "Esc cancel", "Ctrl+R next"}, status)
		if footer == "" {
			return prompt
		}
		return prompt + "\n" + footer
	}
	return input.RenderPromptInput(m.input)
}

// renderStatusLine renders the bottom status line
func (m Model) renderStatusLine() string {
	return status.RenderStatusLine(m.statusConfig)
}

// renderDialog renders the current dialog
func (m Model) renderDialog() string {
	if m.dialog == nil {
		return ""
	}

	switch m.dialog.Type {
	case DialogPermission:
		if req, ok := m.dialog.Content.(dialogs.PermissionRequest); ok {
			return dialogs.RenderPermissionDialog(req, m.width-4)
		}

	case DialogTrust:
		if path, ok := m.dialog.Content.(string); ok {
			return dialogs.RenderTrustDialog(path, m.width-4)
		}

	case DialogAutoMode:
		return dialogs.RenderAutoModeDialog(m.width - 4)

	case DialogCostThreshold:
		if costs, ok := m.dialog.Content.([2]float64); ok {
			return dialogs.RenderCostThresholdDialog(costs[0], costs[1], m.width-4)
		}
	}

	return ""
}

// Public methods for external control

// SetBusy sets the busy state
func (m *Model) SetBusy(busy bool) {
	m.busy = busy
	if busy {
		m.spinner.Reset()
	}
}

// SetStreaming sets the streaming state
func (m *Model) SetStreaming(streaming bool) {
	m.streaming = streaming
}

// AddMessage adds a message to the display
func (m *Model) AddMessage(msg messages.Message) {
	m.messages = append(m.messages, msg)
	if m.virtualState != nil && m.virtualState.IsAtBottom() {
		m.virtualState.ScrollToBottom()
	}
}

// ShowPermissionDialog shows a permission request dialog
func (m *Model) ShowPermissionDialog(req dialogs.PermissionRequest, onYes, onNo func()) {
	m.dialog = &DialogState{
		Type:    DialogPermission,
		Content: req,
		OnYes:   onYes,
		OnNo:    onNo,
	}
}

// ShowTrustDialog shows the trust dialog
func (m *Model) ShowTrustDialog(path string, onYes, onNo func()) {
	m.dialog = &DialogState{
		Type:    DialogTrust,
		Content: path,
		OnYes:   onYes,
		OnNo:    onNo,
	}
}

// ShowAutoModeDialog shows the auto mode opt-in dialog
func (m *Model) ShowAutoModeDialog(onYes, onNo func()) {
	m.dialog = &DialogState{
		Type:  DialogAutoMode,
		OnYes: onYes,
		OnNo:  onNo,
	}
}

// SetStatusConfig updates the status line configuration
func (m *Model) SetStatusConfig(cfg status.StatusLineConfig) {
	m.statusConfig = cfg
}

// Lip Gloss styles matching the TS theme
var (
	styleClaudeColor     = lipgloss.Color("#D77757")
	stylePermissionColor = lipgloss.Color("#B1B9F9")
	styleSuccessColor    = lipgloss.Color("#4EBA65")
	styleErrorColor      = lipgloss.Color("#FF6B80")
	styleWarningColor    = lipgloss.Color("#FFC107")
	styleMutedColor      = lipgloss.Color("#869198")
	styleTextColor       = lipgloss.Color("#FFFFFF")
)

// Helper to create styled messages
func styledMessage(color lipgloss.Color, text string) string {
	return lipgloss.NewStyle().Foreground(color).Render(text)
}

func boldMessage(color lipgloss.Color, text string) string {
	return lipgloss.NewStyle().Foreground(color).Bold(true).Render(text)
}

func dimMessage(text string) string {
	return lipgloss.NewStyle().Faint(true).Render(text)
}

// RenderToolUseIndicator renders a tool use status indicator
func RenderToolUseIndicator(toolName string, isActive, isError bool) string {
	indicator := components.BlackCircle
	var color lipgloss.Color

	if isActive {
		color = styleClaudeColor
	} else if isError {
		color = styleErrorColor
	} else {
		color = styleSuccessColor
	}

	style := lipgloss.NewStyle().Foreground(color)
	nameStyle := lipgloss.NewStyle().Bold(true)

	return style.Render(indicator) + " " + nameStyle.Render(toolName)
}

// RenderCollapsedSummary renders a collapsed operation summary
func RenderCollapsedSummary(summary string, isActive bool) string {
	prefix := "  ⎿ "
	if isActive {
		return styledMessage(styleClaudeColor, prefix+summary+"…")
	}
	return styledMessage(styleMutedColor, prefix+summary)
}

// RenderExpandHint renders the Ctrl+O expand hint
func RenderExpandHint() string {
	return dimMessage("    Ctrl+O to expand")
}

// CreateProgressBar creates a progress bar with the given ratio
func CreateProgressBar(ratio float64, width int) string {
	return components.ProgressBarSimple(ratio, width)
}

// FormatElapsed formats an elapsed duration
func FormatElapsed(d time.Duration) string {
	return FormatDuration(int(d.Milliseconds()))
}

// FormatTokenCount formats a token count with commas
func FormatTokenCount(count int) string {
	return FormatNumber(count) + " tokens"
}

type messageVirtualItem struct {
	id      string
	content string
}

func (i messageVirtualItem) Key() string {
	return i.id
}

func (i messageVirtualItem) Render(_ int, _ bool) string {
	return i.content
}

func (i messageVirtualItem) Height() int {
	return strings.Count(i.content, "\n") + 1
}

func (m Model) handleGlobalSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.globalSearch == nil {
		return m, nil
	}
	act := m.globalSearch.HandleKey(msg.String())
	if act.Err != nil {
		m.messages = append(m.messages, messages.Message{
			Type:    messages.MessageTypeSystem,
			Content: "Global search failed: " + act.Err.Error(),
		})
	}
	if act.Insert != "" {
		m.input.Insert(act.Insert)
	}
	if act.Open != nil {
		m.messages = append(m.messages, messages.Message{
			Type:    messages.MessageTypeSystem,
			Content: fmt.Sprintf("open %s:%d", act.Open.File, act.Open.Line),
		})
	}
	if act.Done {
		m.globalSearch = nil
	}
	return m, nil
}

func (m Model) handleModelPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modelPicker == nil {
		return m, nil
	}
	act := m.modelPicker.HandleKey(msg.String())
	if act.Selected != "" {
		m.statusConfig.Model = act.Selected
		m.messages = append(m.messages, messages.Message{
			Type:    messages.MessageTypeSystem,
			Content: "model set to " + act.Selected,
		})
	}
	if act.Done {
		m.modelPicker = nil
	}
	return m, nil
}

func (m Model) handleQuickOpenKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.quickOpen == nil {
		return m, nil
	}
	act := m.quickOpen.HandleKey(msg.String())
	if act.Err != nil {
		m.messages = append(m.messages, messages.Message{
			Type:    messages.MessageTypeSystem,
			Content: "Quick open failed: " + act.Err.Error(),
		})
	}
	if act.Insert != "" {
		m.input.Insert(act.Insert)
	}
	if act.Open != "" {
		m.messages = append(m.messages, messages.Message{
			Type:    messages.MessageTypeSystem,
			Content: "open " + act.Open,
		})
	}
	if act.Done {
		m.quickOpen = nil
	}
	return m, nil
}

func (m *Model) exitHistorySearch(restoreDraft bool) {
	if restoreDraft {
		m.input.SetValue(m.historyDraft)
		m.input.MoveEnd()
	}
	m.historySearchActive = false
	m.historyQuery = ""
	m.historyMatch = ""
	m.historyFailed = false
	m.historyBrowseOffset = 0
	m.historyDraft = ""
}

func (m *Model) updateHistorySearchMatch() {
	if len(m.input.History) == 0 {
		m.historyFailed = true
		return
	}

	query := strings.ToLower(strings.TrimSpace(m.historyQuery))
	matches := make([]string, 0, len(m.input.History))
	for i := len(m.input.History) - 1; i >= 0; i-- {
		h := m.input.History[i]
		if query == "" || strings.Contains(strings.ToLower(h), query) {
			matches = append(matches, h)
		}
	}
	if len(matches) == 0 {
		m.historyFailed = true
		return
	}
	idx := m.historyBrowseOffset
	if idx >= len(matches) {
		idx = len(matches) - 1
		m.historyBrowseOffset = idx
	}
	m.historyMatch = matches[idx]
	m.historyFailed = false
	m.input.SetValue(m.historyMatch)
	m.input.MoveEnd()
}
