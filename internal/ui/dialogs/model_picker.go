package dialogs

import (
	"fmt"

	"claude-go/internal/ui/components"
)

type ModelPickerAction struct {
	Done     bool
	Selected string
}

type ModelPickerState struct {
	Picker *components.FuzzyPickerModel
}

func ModelPickerStateFor(currentModel string) *ModelPickerState {
	models := []string{
		"gpt-4.1",
		"claude-sonnet-4-5",
		"claude-opus-4-5",
		"claude-haiku-4-5",
	}
	items := make([]components.FuzzyPickerItem, 0, len(models))
	currentIdx := 0
	for i, model := range models {
		items = append(items, components.FuzzyPickerItem{
			ID:          fmt.Sprintf("model-%d", i),
			Label:       model,
			Description: "",
		})
		if model == currentModel {
			currentIdx = i
		}
	}
	picker := components.FuzzyPickerModelFor(components.FuzzyPickerConfig{
		Title:        "Select model",
		Placeholder:  "Type to filter models…",
		Items:        items,
		VisibleCount: 8,
		Direction:    "up",
		EmptyMessage: "No matching models",
		MatchLabel:   fmt.Sprintf("%d models", len(items)),
		SelectAction: "select",
	})
	picker.State.FocusedIndex = currentIdx
	return &ModelPickerState{Picker: picker}
}

func (s *ModelPickerState) View() string {
	return s.Picker.View()
}

func (s *ModelPickerState) HandleKey(key string) ModelPickerAction {
	switch key {
	case "esc", "ctrl+c":
		return ModelPickerAction{Done: true}
	case "enter", "tab":
		item := s.Picker.State.GetFocused()
		if item == nil {
			return ModelPickerAction{Done: true}
		}
		return ModelPickerAction{Done: true, Selected: item.Label}
	}
	_ = s.Picker.HandleKey(key)
	return ModelPickerAction{}
}
