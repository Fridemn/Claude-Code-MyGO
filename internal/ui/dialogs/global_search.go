package dialogs

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"claude-go/internal/ui/components"
)

type GlobalSearchMatch struct {
	File string
	Line int
	Text string
}

type GlobalSearchAction struct {
	Done   bool
	Open   *GlobalSearchMatch
	Insert string
	Err    error
}

type GlobalSearchState struct {
	Width  int
	Height int

	Picker *components.FuzzyPickerModel
	Matches []GlobalSearchMatch
	byID    map[string]GlobalSearchMatch
	query   string

	Search func(query string) ([]GlobalSearchMatch, error)
}

func GlobalSearchStateFor(width, height int) *GlobalSearchState {
	s := &GlobalSearchState{
		Width:  width,
		Height: height,
		byID:   map[string]GlobalSearchMatch{},
	}
	s.Search = s.searchWorkspace
	s.Picker = components.FuzzyPickerModelFor(components.FuzzyPickerConfig{
		Title:        "Global Search",
		Placeholder:  "Type to search…",
		VisibleCount: 10,
		Direction:    "up",
		EmptyMessage: "Type to search…",
		MatchLabel:   " ",
		SelectAction: "open",
		OnTab: &components.FuzzyPickerAction{
			Action: "mention",
		},
		OnShiftTab: &components.FuzzyPickerAction{
			Action: "insert path",
		},
	})
	return s
}

func (s *GlobalSearchState) SetSize(width, height int) {
	s.Width = width
	s.Height = height
}

func (s *GlobalSearchState) View() string {
	return s.Picker.View()
}

func (s *GlobalSearchState) HandleKey(key string) GlobalSearchAction {
	switch key {
	case "esc", "ctrl+c":
		return GlobalSearchAction{Done: true}
	case "tab":
		if m := s.focusedMatch(); m != nil {
			return GlobalSearchAction{Done: true, Insert: fmt.Sprintf("@%s#L%d ", m.File, m.Line)}
		}
		return GlobalSearchAction{Done: true}
	case "shift+tab":
		if m := s.focusedMatch(); m != nil {
			return GlobalSearchAction{Done: true, Insert: fmt.Sprintf("%s:%d ", m.File, m.Line)}
		}
		return GlobalSearchAction{Done: true}
	case "enter":
		if m := s.focusedMatch(); m != nil {
			return GlobalSearchAction{Done: true, Open: m}
		}
		return GlobalSearchAction{Done: true}
	}

	closed := s.Picker.HandleKey(key)
	if closed {
		return GlobalSearchAction{Done: true}
	}

	nextQuery := s.Picker.State.Query
	if nextQuery != s.query {
		s.query = nextQuery
		if err := s.refreshMatches(nextQuery); err != nil {
			return GlobalSearchAction{Err: err}
		}
	}
	return GlobalSearchAction{}
}

func (s *GlobalSearchState) refreshMatches(query string) error {
	query = strings.TrimSpace(query)
	if query == "" {
		s.Matches = nil
		s.byID = map[string]GlobalSearchMatch{}
		s.Picker.State.AllItems = nil
		s.Picker.State.UpdateQuery("")
		s.Picker.Config.EmptyMessage = "Type to search…"
		s.Picker.Config.MatchLabel = " "
		return nil
	}

	matches, err := s.Search(query)
	if err != nil {
		s.Picker.Config.EmptyMessage = "Search failed"
		s.Picker.Config.MatchLabel = "0 matches"
		return err
	}

	s.Matches = matches
	s.byID = make(map[string]GlobalSearchMatch, len(matches))
	items := make([]components.FuzzyPickerItem, 0, len(matches))
	for i, m := range matches {
		id := fmt.Sprintf("match-%d", i)
		s.byID[id] = m
		items = append(items, components.FuzzyPickerItem{
			ID:          id,
			Label:       fmt.Sprintf("%s:%d %s", m.File, m.Line, m.Text),
			Description: "",
		})
	}
	s.Picker.State.AllItems = items
	s.Picker.State.UpdateQuery(query)
	s.Picker.Config.EmptyMessage = "No matches"
	s.Picker.Config.MatchLabel = fmt.Sprintf("%d matches", len(matches))
	return nil
}

func (s *GlobalSearchState) focusedMatch() *GlobalSearchMatch {
	item := s.Picker.State.GetFocused()
	if item == nil {
		return nil
	}
	m, ok := s.byID[item.ID]
	if !ok {
		return nil
	}
	return &m
}

func (s *GlobalSearchState) searchWorkspace(query string) ([]GlobalSearchMatch, error) {
	cmd := exec.Command("rg", "-n", "--no-heading", "-i", "-m", "10", "-F", "-e", query, ".")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	results := make([]GlobalSearchMatch, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		file, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		lineStr, text, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		n, convErr := strconv.Atoi(lineStr)
		if convErr != nil {
			continue
		}
		results = append(results, GlobalSearchMatch{
			File: strings.TrimPrefix(file, "./"),
			Line: n,
			Text: strings.TrimSpace(text),
		})
		if len(results) >= 500 {
			break
		}
	}
	return results, nil
}
