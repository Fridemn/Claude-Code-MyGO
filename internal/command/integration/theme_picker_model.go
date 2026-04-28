package integration

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

var themePickerOptions = []string{
	"auto",
	"dark",
	"light",
	"dark-daltonized",
	"light-daltonized",
	"dark-ansi",
	"light-ansi",
}

type themePickerModel struct {
	rt      command.Runtime
	options []string
	index   int
	width   int
}

func loadThemeModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	return themePickerModel{
		rt:      rt,
		options: append([]string(nil), themePickerOptions...),
		index:   0,
	}, nil
}

func (m themePickerModel) Init() tea.Cmd { return nil }

func (m themePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.rt.OnThemeChange != nil {
				m.rt.OnThemeChange(selected)
			}
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone(fmt.Sprintf("Theme set to %s", selected), command.LocalJSXDoneOptions{})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone("Theme picker dismissed", command.LocalJSXDoneOptions{
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
					m.rt.OnLocalJSXDone("Theme picker dismissed", command.LocalJSXDoneOptions{
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

func (m themePickerModel) View() string {
	var b strings.Builder
	b.WriteString("Theme\n")
	b.WriteString("Choose the text style that looks best with your terminal.\n\n")

	for i, option := range m.options {
		cursor := "  "
		if i == m.index {
			cursor = "> "
		}
		b.WriteString(cursor + option + "\n")
	}

	b.WriteString("\n")
	b.WriteString("Enter select · Esc cancel")
	if m.width > 0 && m.width < 36 {
		// Keep footer short in very narrow terminals.
		return "Theme\n\n" + strings.Join(m.options, "\n") + "\n\nEsc close"
	}
	return strings.TrimRight(b.String(), "\n")
}
