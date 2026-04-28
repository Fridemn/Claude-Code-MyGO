package integration

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

var modelPickerOptions = []string{
	"claude-sonnet-4-5",
	"claude-opus-4-5",
	"claude-haiku-4-5",
	"gpt-4.1",
	"gpt-4.1-mini",
	"gpt-4.1-nano",
	"o3",
	"o4-mini",
}

type modelPickerModel struct {
	rt      command.Runtime
	options []string
	index   int
	width   int
}

func loadModelPickerModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	currentModel := rt.Config.Model
	idx := 0
	for i, m := range modelPickerOptions {
		if m == currentModel {
			idx = i
			break
		}
	}
	return modelPickerModel{
		rt:      rt,
		options: append([]string(nil), modelPickerOptions...),
		index:   idx,
	}, nil
}

func (m modelPickerModel) Init() tea.Cmd { return nil }

func (m modelPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		key := strings.ToLower(strings.TrimSpace(msg.String()))
		switch msg.Type {
		case tea.KeyUp:
			m.index = (m.index - 1 + len(m.options)) % len(m.options)
			return m, nil
		case tea.KeyDown:
			m.index = (m.index + 1) % len(m.options)
			return m, nil
		case tea.KeyEnter:
			selected := m.options[m.index]
			if m.rt.OnModelChange != nil {
				m.rt.OnModelChange(selected)
			}
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone(fmt.Sprintf("Model set to %s", selected), command.LocalJSXDoneOptions{})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone("Model picker dismissed", command.LocalJSXDoneOptions{
					Display: "system",
				})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		default:
			switch key {
			case "k":
				m.index = (m.index - 1 + len(m.options)) % len(m.options)
				return m, nil
			case "j":
				m.index = (m.index + 1) % len(m.options)
				return m, nil
			case "q":
				if m.rt.OnLocalJSXDone != nil {
					m.rt.OnLocalJSXDone("Model picker dismissed", command.LocalJSXDoneOptions{
						Display: "system",
					})
				} else if m.rt.OnExit != nil {
					m.rt.OnExit()
				}
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m modelPickerModel) View() string {
	var b strings.Builder
	b.WriteString("Model\n")
	b.WriteString("Select a model for this session.\n\n")

	currentModel := m.rt.Config.Model
	for i, option := range m.options {
		cursor := "  "
		if i == m.index {
			cursor = "> "
		}
		suffix := ""
		if option == currentModel {
			suffix = " (current)"
		}
		b.WriteString(cursor + option + suffix + "\n")
	}

	b.WriteString("\n")
	b.WriteString("Enter select · Esc cancel")
	return strings.TrimRight(b.String(), "\n")
}
