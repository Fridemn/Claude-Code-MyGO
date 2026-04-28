package help

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"claude-go/internal/command"
)

// Tab indices
const (
	TabGeneral  = 0
	TabCommands = 1
	TabCustom   = 2
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")). // professional blue
			Bold(true).
			Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")). // dim gray
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")). // professional blue
			Bold(true).
			Padding(0, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // dim gray

	shortcutsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // dim gray

	commandItemStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	commandNameStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14")) // cyan

	commandDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")) // dim gray
)

// Key bindings
type keyMap struct {
	Left  key.Binding
	Right key.Binding
	Up    key.Binding
	Down  key.Binding
	Quit  key.Binding
	Tab   key.Binding
	Enter key.Binding
}

var keys = keyMap{
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next tab"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc", "close"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
}

// Model is the Bubble Tea model for the help TUI
type Model struct {
	activeTab   int
	tabs        []string
	builtinCmds []commandItem
	customCmds  []commandItem
	listIndex   int
	width       int
	height      int
	onClose     func()
	exitPending bool
}

// commandItem represents a command in the list
type commandItem struct {
	name        string
	description string
}

// Model creates a new help model
func CreateModel(builtinCommands, customCommands []command.Command, onClose func()) Model {
	builtinItems := make([]commandItem, 0)
	for _, cmd := range builtinCommands {
		base := cmd.GetBase()
		builtinItems = append(builtinItems, commandItem{
			name:        base.Name,
			description: base.Description,
		})
	}

	customItems := make([]commandItem, 0)
	for _, cmd := range customCommands {
		base := cmd.GetBase()
		customItems = append(customItems, commandItem{
			name:        base.Name,
			description: base.Description,
		})
	}

	return Model{
		activeTab:   TabGeneral,
		tabs:        []string{"general", "commands", "custom-commands"},
		builtinCmds: builtinItems,
		customCmds:  customItems,
		listIndex:   0,
		onClose:     onClose,
		exitPending: false,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			if m.exitPending {
				if m.onClose != nil {
					m.onClose()
				}
				return m, tea.Quit
			}
			m.exitPending = true
			return m, nil

		case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Right):
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.listIndex = 0
			m.exitPending = false

		case key.Matches(msg, keys.Left):
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.listIndex = 0
			m.exitPending = false

		case key.Matches(msg, keys.Up):
			m.exitPending = false
			if m.activeTab == TabCommands && len(m.builtinCmds) > 0 {
				m.listIndex = (m.listIndex - 1 + len(m.builtinCmds)) % len(m.builtinCmds)
			} else if m.activeTab == TabCustom && len(m.customCmds) > 0 {
				m.listIndex = (m.listIndex - 1 + len(m.customCmds)) % len(m.customCmds)
			}

		case key.Matches(msg, keys.Down):
			m.exitPending = false
			if m.activeTab == TabCommands && len(m.builtinCmds) > 0 {
				m.listIndex = (m.listIndex + 1) % len(m.builtinCmds)
			} else if m.activeTab == TabCustom && len(m.customCmds) > 0 {
				m.listIndex = (m.listIndex + 1) % len(m.customCmds)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height / 2 // max height is half of terminal
	}

	return m, nil
}

// View renders the model
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	// Render tabs
	tabsRow := m.renderTabs()

	// Render content based on active tab
	content := ""
	switch m.activeTab {
	case TabGeneral:
		content = m.renderGeneralTab()
	case TabCommands:
		content = m.renderCommandsTab(m.builtinCmds, "Browse default commands:")
	case TabCustom:
		content = m.renderCommandsTab(m.customCmds, "Browse custom commands:")
	}

	// Footer with help link and quit hint
	footer := m.renderFooter()

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tabsRow,
		content,
		footer,
	)
}

func (m Model) renderTabs() string {
	tabStrs := make([]string, len(m.tabs))
	for i, tab := range m.tabs {
		if i == m.activeTab {
			tabStrs[i] = activeTabStyle.Render(tab)
		} else {
			tabStrs[i] = tabStyle.Render(tab)
		}
	}

	// Join tabs
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Left, tabStrs...)
	return titleStyle.Render("Claude Code v"+getVersion()) + "\n" + tabsRow
}

func getVersion() string {
	// TODO: get actual version from config
	return "0.1.0"
}

func (m Model) renderGeneralTab() string {
	// Shortcuts columns similar to TS PromptInputHelpMenu
	col1 := shortcutsStyle.Render(`! for bash mode
/ for commands
@ for file paths
& for background
/btw for side question`)

	col2 := shortcutsStyle.Render(`double tap esc to clear input
shift + tab to auto-accept edits
ctrl + o for verbose output
ctrl + t to toggle tasks
alt + enter for newline`)

	col3 := shortcutsStyle.Render(`ctrl + z to undo
ctrl + z to suspend
ctrl + v to paste images
alt + p to switch model
ctrl + s to stash prompt
ctrl + g to edit in $EDITOR`)

	// Join columns with gap
	shortcuts := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Width(24).Render(col1),
		lipgloss.NewStyle().Width(35).Render(col2),
		lipgloss.NewStyle().Render(col3),
	)

	header := "Shortcuts"
	return lipgloss.NewStyle().Padding(1, 0).Render(
		"Claude understands your codebase, makes edits with your permission,\nand executes commands — right from your terminal.\n\n" +
			lipgloss.NewStyle().Bold(true).Render(header) + "\n" +
			shortcuts,
	)
}

func (m Model) renderCommandsTab(items []commandItem, title string) string {
	if len(items) == 0 {
		return shortcutsStyle.Render("No commands found")
	}

	// Render command list with selection
	lines := make([]string, 0, len(items))
	maxVisible := min(10, m.height-6)

	for i, item := range items {
		// Determine if this item is visible
		startIdx := max(0, m.listIndex-maxVisible/2)
		endIdx := min(len(items), startIdx+maxVisible)

		if i < startIdx || i >= endIdx {
			continue
		}

		// Render the item
		itemStr := commandNameStyle.Render("/"+item.name) + " " +
			commandDescStyle.Render(truncate(item.description, m.width-20))

		if i == m.listIndex {
			// Highlight selected item
			itemStr = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")). // white
				Background(lipgloss.Color("12")). // blue
				Render(" ▶ " + itemStr)
		} else {
			itemStr = commandItemStyle.Render("   " + itemStr)
		}

		lines = append(lines, itemStr)
	}

	return lipgloss.NewStyle().Padding(1, 0).Render(
		title + "\n" + strings.Join(lines, "\n"),
	)
}

func (m Model) renderFooter() string {
	helpLink := "For more help: https://code.claude.com/docs/en/overview"

	quitHint := ""
	if m.exitPending {
		quitHint = "Press ctrl+c again to exit"
	} else {
		quitHint = shortcutsStyle.Render("esc to cancel")
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		helpStyle.Render(helpLink),
		helpStyle.Render(quitHint),
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
