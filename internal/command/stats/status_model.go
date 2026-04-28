package stats

import (
	"context"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type statusModel struct {
	rt     command.Runtime
	lines  string
}

func loadStatusModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := statusModel{rt: rt}
	m.lines = buildStatusText(rt)
	return m, nil
}

func (m statusModel) Init() tea.Cmd { return nil }

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyEnter:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statusModel) View() string {
	return m.lines + "\n\nPress Enter or Esc to close"
}