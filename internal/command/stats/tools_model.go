package stats

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type toolsModel struct {
	rt     command.Runtime
	lines  []string
	width  int
	height int
	offset int
}

func loadToolsModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := toolsModel{rt: rt}
	m.refresh()
	return m, nil
}

func (m *toolsModel) refresh() {
	m.lines = renderToolsLines(m.rt)
	max := toolsMaxOffset(len(m.lines), m.bodyHeight())
	if m.offset > max {
		m.offset = max
	}
}

func (m toolsModel) Init() tea.Cmd { return nil }

func (m toolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		max := toolsMaxOffset(len(m.lines), m.bodyHeight())
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
			if m.offset < toolsMaxOffset(len(m.lines), m.bodyHeight()) {
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
			if m.offset < toolsMaxOffset(len(m.lines), m.bodyHeight()) {
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

func (m toolsModel) View() string {
	if len(m.lines) == 0 {
		return "Tools\n\nNo data.\n\nEsc close"
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

func (m toolsModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 2
	if h < 6 {
		return 6
	}
	return h
}

func renderToolsLines(runtime command.Runtime) []string {
	if runtime.Tools == nil {
		return []string{"tool registry is not configured"}
	}
	definitions := runtime.Tools.List()
	lines := make([]string, 0, len(definitions)+1)
	lines = append(lines, "Tools")
	for _, definition := range definitions {
		mode := "write"
		if definition.IsReadOnly(nil) {
			mode = "read"
		}
		lines = append(lines, fmt.Sprintf("%s  [%s]  %s", definition.Name(), mode, definition.Description()))
	}
	if len(lines) == 1 {
		lines = append(lines, "(no tools)")
	}
	sort.Strings(lines[1:])
	return lines
}

func toolsMaxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}
