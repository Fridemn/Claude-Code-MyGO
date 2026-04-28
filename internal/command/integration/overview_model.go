package integration

import (
	"context"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type overviewModel struct {
	renderFn func() string
	onExit   func()
	footer   string

	lines  []string
	width  int
	height int
	offset int
}

func loadMCPModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := overviewModel{
		renderFn: func() string { return renderMCPOverview(rt) },
		onExit:   rt.OnExit,
		footer:   "j/k scroll · r refresh · Esc close",
	}
	m.refresh()
	return m, nil
}

func loadPluginsModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := overviewModel{
		renderFn: func() string { return renderPluginsOverview(rt) },
		onExit:   rt.OnExit,
		footer:   "j/k scroll · r refresh · Esc close",
	}
	m.refresh()
	return m, nil
}

func loadHooksModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := overviewModel{
		renderFn: func() string { return renderHooksOverview(rt) },
		onExit:   rt.OnExit,
		footer:   "j/k scroll · r refresh · Esc close",
	}
	m.refresh()
	return m, nil
}

func (m *overviewModel) refresh() {
	content := ""
	if m.renderFn != nil {
		content = strings.TrimSpace(m.renderFn())
	}
	if content == "" {
		m.lines = []string{"No data."}
	} else {
		m.lines = strings.Split(content, "\n")
	}
	max := integrationViewerMaxOffset(len(m.lines), m.bodyHeight())
	if m.offset > max {
		m.offset = max
	}
}

func (m overviewModel) Init() tea.Cmd { return nil }

func (m overviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		max := integrationViewerMaxOffset(len(m.lines), m.bodyHeight())
		if m.offset > max {
			m.offset = max
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
			if m.offset < integrationViewerMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.onExit != nil {
				m.onExit()
			}
			return m, tea.Quit
		}
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "j":
			if m.offset < integrationViewerMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
		case "k":
			if m.offset > 0 {
				m.offset--
			}
		case "r":
			m.refresh()
		case "q":
			if m.onExit != nil {
				m.onExit()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m overviewModel) View() string {
	if len(m.lines) == 0 {
		return "No data.\n\nEsc close"
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
	footer := m.footer
	if strings.TrimSpace(footer) == "" {
		footer = "Esc close"
	}
	return strings.TrimRight(visible, "\n") + "\n\n" + footer
}

func (m overviewModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 2
	if h < 6 {
		return 6
	}
	return h
}

func integrationViewerMaxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}
