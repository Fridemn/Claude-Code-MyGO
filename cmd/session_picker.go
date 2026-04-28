package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionPickerModel is a Bubble Tea model for selecting sessions
// Matches TS LogSelector/TreeSelect UI pattern
type SessionPickerModel struct {
	allSessions  []SessionPickItem // Original list
	sessions     []SessionPickItem // Filtered list for display
	selected     int
	width        int
	height       int
	title        string
	showPreview  bool
	searchQuery  string
	searching    bool
}

// SessionPickItem represents a session in the picker
type SessionPickItem struct {
	SessionID   string
	Title       string
	Date        string
	ProjectName string
	MessageCount int
}

// SessionPickerResult contains the result of picker selection
type SessionPickerResult struct {
	SessionID string
	Cancelled bool
}

// NewSessionPickerModel creates a new session picker model
func NewSessionPickerModel(sessions []SessionPickItem, width, height int) SessionPickerModel {
	return SessionPickerModel{
		allSessions: sessions,
		sessions:    sessions,
		selected:    0,
		width:       width,
		height:      height,
		title:       "Select a conversation to resume",
		showPreview: false,
	}
}

func (m SessionPickerModel) Init() tea.Cmd {
	return nil
}

func (m SessionPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
			if m.selected > 0 {
				m.selected--
			} else if len(m.sessions) > 0 {
				m.selected = len(m.sessions) - 1 // Wrap to bottom
			}

		case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
			if m.selected < len(m.sessions)-1 {
				m.selected++
			} else {
				m.selected = 0 // Wrap to top
			}

		case tea.KeyEnter:
			// Return the selected session
			if len(m.sessions) > 0 && m.selected >= 0 && m.selected < len(m.sessions) {
				return m, tea.Quit
			}

		case tea.KeyEsc:
			// Cancel selection - start new session
			m.selected = -1
			return m, tea.Quit

		case tea.KeyCtrlC:
			// Exit entirely
			return m, tea.Quit

		case tea.KeyBackspace, tea.KeyDelete:
			// Remove last character from search query
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.filterSessions()
				m.selected = 0 // Reset selection after filtering
			}

		case tea.KeyRunes:
			// Search/filter functionality - add to query
			if len(msg.Runes) > 0 {
				m.searchQuery += string(msg.Runes)
				m.filterSessions()
				m.selected = 0 // Reset selection after filtering
			}
		}
	}

	return m, nil
}

func (m *SessionPickerModel) filterSessions() {
	// Filter sessions by search query (case-insensitive)
	if m.searchQuery == "" {
		m.sessions = m.allSessions
		return
	}

	query := strings.ToLower(m.searchQuery)
	var filtered []SessionPickItem
	for _, s := range m.allSessions {
		// Match against session ID, title, and project name
		if strings.Contains(strings.ToLower(s.SessionID), query) ||
			strings.Contains(strings.ToLower(s.Title), query) ||
			strings.Contains(strings.ToLower(s.ProjectName), query) {
			filtered = append(filtered, s)
		}
	}
	m.sessions = filtered
}

func (m SessionPickerModel) View() string {
	var b strings.Builder

	// Header
	b.WriteString("\n")
	headerStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(headerStyle.Render(m.title))

	// Search indicator if query exists
	if m.searchQuery != "" {
		searchStyle := lipgloss.NewStyle().Faint(true)
		b.WriteString(searchStyle.Render(fmt.Sprintf(" (filtering: %s)", m.searchQuery)))
	}
	b.WriteString("\n\n")

	// Handle empty results
	if len(m.sessions) == 0 && m.searchQuery != "" {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("  No matching conversations found"))
		b.WriteString("\n")
		hintStyle := lipgloss.NewStyle().Faint(true)
		b.WriteString(hintStyle.Render("  Press Backspace to clear filter  •  Esc to start new session"))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.allSessions) == 0 {
		return "\n  No conversations found to resume\n\n  Press Esc to start a new session\n"
	}

	// Session list
	visibleCount := m.height - 12
	if visibleCount < 5 {
		visibleCount = 5
	}
	if visibleCount > len(m.sessions) {
		visibleCount = len(m.sessions)
	}

	// Calculate scroll offset to keep selected item visible
	scrollOffset := 0
	if m.selected >= visibleCount {
		scrollOffset = m.selected - visibleCount + 1
	}

	for i := scrollOffset; i < scrollOffset+visibleCount && i < len(m.sessions); i++ {
		session := m.sessions[i]

		// Format the line
		var line string
		if i == m.selected {
			// Selected item - highlight with cursor indicator
			selectedStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62"))
			line = selectedStyle.Render(formatSessionLine(session, true))
		} else {
			// Normal item - dim style
			normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			line = normalStyle.Render(formatSessionLine(session, false))
		}
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Footer with hints
	b.WriteString("\n")
	hintStyle := lipgloss.NewStyle().Faint(true)
	hints := "  ↑/↓ navigate  •  Enter select  •  Esc new session"
	if m.searchQuery == "" {
		hints += "  •  type to filter"
	} else {
		hints += "  •  Backspace clear"
	}
	b.WriteString(hintStyle.Render(hints))
	b.WriteString("\n")

	return b.String()
}

func formatSessionLine(session SessionPickItem, selected bool) string {
	// Format: [N] session-id-short  date  [title] (project)
	idShort := session.SessionID
	if len(idShort) > 8 {
		idShort = idShort[:8]
	}

	title := session.Title
	if len(title) > 40 {
		title = title[:40] + "..."
	}

	project := session.ProjectName
	if len(project) > 15 {
		project = project[:15] + "..."
	}

	// Build the line
	prefix := "  "
	if selected {
		prefix = "▸ "
	}

	return fmt.Sprintf("%s%s  %s  [%s] (%s)", prefix, idShort, session.Date, title, project)
}

// GetResult returns the selected session ID or cancelled status
func (m SessionPickerModel) GetResult() SessionPickerResult {
	if m.selected < 0 || m.selected >= len(m.sessions) {
		return SessionPickerResult{Cancelled: true}
	}
	return SessionPickerResult{
		SessionID: m.sessions[m.selected].SessionID,
		Cancelled: false,
	}
}