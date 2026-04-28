package stats

import (
	"context"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type usageModel struct {
	rt     command.Runtime
	lines  []string
	width  int
	height int
	offset int
}

func loadUsageModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := usageModel{rt: rt}
	m.refresh()
	return m, nil
}

func (m *usageModel) refresh() {
	m.lines = usageLines(m.rt)
	if m.offset > usageMaxOffset(len(m.lines), m.bodyHeight()) {
		m.offset = usageMaxOffset(len(m.lines), m.bodyHeight())
	}
}

func (m usageModel) Init() tea.Cmd { return nil }

func (m usageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.offset > usageMaxOffset(len(m.lines), m.bodyHeight()) {
			m.offset = usageMaxOffset(len(m.lines), m.bodyHeight())
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.offset > 0 {
				m.offset--
			}
			return m, nil
		case tea.KeyDown:
			if m.offset < usageMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "j":
			if m.offset < usageMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
		case "k":
			if m.offset > 0 {
				m.offset--
			}
		case "r":
			m.refresh()
		case "q":
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m usageModel) View() string {
	if len(m.lines) == 0 {
		return "Usage\n\nNo data.\n\nEsc close"
	}
	bodyH := m.bodyHeight()
	start := m.offset
	if start < 0 {
		start = 0
	}
	end := start + bodyH
	if end > len(m.lines) {
		end = len(m.lines)
	}
	if bodyH <= 0 {
		start = 0
		end = len(m.lines)
	}

	visible := strings.Join(m.lines[start:end], "\n")
	footer := "j/k scroll · r refresh · Esc close"
	return strings.TrimRight(visible, "\n") + "\n\n" + footer
}

func (m usageModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 2
	if h < 6 {
		return 6
	}
	return h
}

func usageMaxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}
