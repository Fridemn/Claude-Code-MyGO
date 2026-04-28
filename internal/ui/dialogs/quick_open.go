package dialogs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/ui/components"
)

type QuickOpenAction struct {
	Done   bool
	Open   string
	Insert string
	Err    error
}

type QuickOpenState struct {
	Picker *components.FuzzyPickerModel
	files  []string
	byID   map[string]string

	ListFiles func() ([]string, error)
}

func QuickOpenStateFor(width, height int) *QuickOpenState {
	s := &QuickOpenState{
		byID: map[string]string{},
	}
	s.ListFiles = s.listWorkspaceFiles
	s.Picker = components.FuzzyPickerModelFor(components.FuzzyPickerConfig{
		Title:        "Quick Open",
		Placeholder:  "Type to search files…",
		VisibleCount: 8,
		Direction:    "up",
		EmptyMessage: "Start typing to search…",
		MatchLabel:   " ",
		SelectAction: "open",
		OnTab: &components.FuzzyPickerAction{
			Action: "mention",
		},
		OnShiftTab: &components.FuzzyPickerAction{
			Action: "insert path",
		},
	})
	_ = s.reloadFiles()
	return s
}

func (s *QuickOpenState) View() string {
	return s.Picker.View()
}

func (s *QuickOpenState) HandleKey(key string) QuickOpenAction {
	switch key {
	case "esc", "ctrl+c":
		return QuickOpenAction{Done: true}
	case "tab":
		if f := s.focusedFile(); f != "" {
			return QuickOpenAction{Done: true, Insert: "@" + f + " "}
		}
		return QuickOpenAction{Done: true}
	case "shift+tab":
		if f := s.focusedFile(); f != "" {
			return QuickOpenAction{Done: true, Insert: f + " "}
		}
		return QuickOpenAction{Done: true}
	case "enter":
		if f := s.focusedFile(); f != "" {
			return QuickOpenAction{Done: true, Open: f}
		}
		return QuickOpenAction{Done: true}
	}

	closed := s.Picker.HandleKey(key)
	if closed {
		return QuickOpenAction{Done: true}
	}
	return QuickOpenAction{}
}

func (s *QuickOpenState) focusedFile() string {
	item := s.Picker.State.GetFocused()
	if item == nil {
		return ""
	}
	return s.byID[item.ID]
}

func (s *QuickOpenState) reloadFiles() error {
	files, err := s.ListFiles()
	if err != nil {
		s.Picker.Config.EmptyMessage = "Failed to list files"
		return err
	}
	s.files = files
	items := make([]components.FuzzyPickerItem, 0, len(files))
	s.byID = make(map[string]string, len(files))
	for i, file := range files {
		id := fmt.Sprintf("file-%d", i)
		s.byID[id] = file
		items = append(items, components.FuzzyPickerItem{
			ID:    id,
			Label: file,
		})
	}
	s.Picker.State.AllItems = items
	s.Picker.State.UpdateQuery("")
	s.Picker.Config.MatchLabel = fmt.Sprintf("%d files", len(files))
	return nil
}

func (s *QuickOpenState) ReloadForTest() error {
	return s.reloadFiles()
}

func (s *QuickOpenState) listWorkspaceFiles() ([]string, error) {
	const maxFiles = 3000
	var files []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".git") || name == "node_modules" || name == ".claude-go" {
				return filepath.SkipDir
			}
			return nil
		}
		clean := strings.TrimPrefix(filepath.ToSlash(path), "./")
		files = append(files, clean)
		if len(files) >= maxFiles {
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return nil, err
	}
	return files, nil
}
