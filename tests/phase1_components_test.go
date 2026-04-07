package tests

import (
	"strings"
	"testing"

	"claude-code-go/internal/ui/components"
)

func TestListItem(t *testing.T) {
	t.Run("renders focused item with pointer", func(t *testing.T) {
		cfg := components.ListItemConfig{
			Content:   "Test Item",
			IsFocused: true,
			Styled:    true,
		}
		result := components.RenderListItem(cfg)

		// Should contain pointer indicator
		if !strings.Contains(result, "❯") {
			t.Errorf("Expected pointer indicator in focused item, got: %s", result)
		}
		// Should contain content
		if !strings.Contains(result, "Test Item") {
			t.Errorf("Expected content in item, got: %s", result)
		}
	})

	t.Run("renders selected item with checkmark", func(t *testing.T) {
		cfg := components.ListItemConfig{
			Content:    "Selected Item",
			IsFocused:  false,
			IsSelected: true,
			Styled:     true,
		}
		result := components.RenderListItem(cfg)

		// Should contain checkmark
		if !strings.Contains(result, "✓") {
			t.Errorf("Expected checkmark in selected item, got: %s", result)
		}
	})

	t.Run("renders disabled item without indicators", func(t *testing.T) {
		cfg := components.ListItemConfig{
			Content:  "Disabled Item",
			Disabled: true,
			Styled:   true,
		}
		result := components.RenderListItem(cfg)

		// Should not contain pointer or checkmark
		if strings.Contains(result, "❯") || strings.Contains(result, "✓") {
			t.Errorf("Expected no indicators in disabled item, got: %s", result)
		}
	})

	t.Run("renders description below content", func(t *testing.T) {
		cfg := components.ListItemConfig{
			Content:     "Main Content",
			Description: "Description text",
			Styled:      true,
		}
		result := components.RenderListItem(cfg)

		// Should contain both content and description
		if !strings.Contains(result, "Main Content") {
			t.Errorf("Expected main content, got: %s", result)
		}
		if !strings.Contains(result, "Description text") {
			t.Errorf("Expected description, got: %s", result)
		}
	})

	t.Run("renders scroll indicators", func(t *testing.T) {
		cfgUp := components.ListItemConfig{
			Content:      "Item with up scroll",
			ShowScrollUp: true,
			Styled:       true,
		}
		resultUp := components.RenderListItem(cfgUp)
		if !strings.Contains(resultUp, "↑") {
			t.Errorf("Expected up arrow, got: %s", resultUp)
		}

		cfgDown := components.ListItemConfig{
			Content:        "Item with down scroll",
			ShowScrollDown: true,
			Styled:         true,
		}
		resultDown := components.RenderListItem(cfgDown)
		if !strings.Contains(resultDown, "↓") {
			t.Errorf("Expected down arrow, got: %s", resultDown)
		}
	})
}

func TestSearchBox(t *testing.T) {
	t.Run("renders with placeholder when empty", func(t *testing.T) {
		cfg := components.SearchBoxConfig{
			Query:             "",
			Placeholder:       "Type to search...",
			IsFocused:         true,
			IsTerminalFocused: true,
		}
		result := components.RenderSearchBox(cfg)

		if !strings.Contains(result, "⌕") {
			t.Errorf("Expected search prefix, got: %s", result)
		}
	})

	t.Run("renders query text", func(t *testing.T) {
		cfg := components.SearchBoxConfig{
			Query:             "test query",
			IsFocused:         true,
			IsTerminalFocused: false,
		}
		result := components.RenderSearchBox(cfg)

		if !strings.Contains(result, "test query") {
			t.Errorf("Expected query in result, got: %s", result)
		}
	})

	t.Run("renders borderless mode", func(t *testing.T) {
		cfg := components.SearchBoxConfig{
			Query:      "test",
			IsFocused:  true,
			Borderless: true,
		}
		result := components.RenderSearchBox(cfg)

		// Should not contain border characters
		if strings.Contains(result, "╭") || strings.Contains(result, "╰") {
			t.Errorf("Expected no border in borderless mode, got: %s", result)
		}
	})
}

func TestSearchBoxState(t *testing.T) {
	t.Run("Insert adds character at cursor", func(t *testing.T) {
		state := components.SearchBoxStateFor()
		state.Insert("a")
		state.Insert("b")
		state.Insert("c")

		if state.Query != "abc" {
			t.Errorf("Expected 'abc', got: %s", state.Query)
		}
		if state.CursorOffset != 3 {
			t.Errorf("Expected cursor at 3, got: %d", state.CursorOffset)
		}
	})

	t.Run("Backspace removes character before cursor", func(t *testing.T) {
		state := components.SearchBoxStateFor()
		state.Set("hello")
		state.Backspace()

		if state.Query != "hell" {
			t.Errorf("Expected 'hell', got: %s", state.Query)
		}
	})

	t.Run("MoveLeft/MoveRight navigates cursor", func(t *testing.T) {
		state := components.SearchBoxStateFor()
		state.Set("test")

		state.MoveLeft()
		if state.CursorOffset != 3 {
			t.Errorf("Expected cursor at 3 after MoveLeft, got: %d", state.CursorOffset)
		}

		state.MoveRight()
		if state.CursorOffset != 4 {
			t.Errorf("Expected cursor at 4 after MoveRight, got: %d", state.CursorOffset)
		}
	})

	t.Run("Clear resets state", func(t *testing.T) {
		state := components.SearchBoxStateFor()
		state.Set("test")
		state.Clear()

		if state.Query != "" {
			t.Errorf("Expected empty query, got: %s", state.Query)
		}
		if state.CursorOffset != 0 {
			t.Errorf("Expected cursor at 0, got: %d", state.CursorOffset)
		}
	})
}

func TestFuzzyPicker(t *testing.T) {
	items := []components.FuzzyPickerItem{
		{ID: "1", Label: "Apple"},
		{ID: "2", Label: "Banana"},
		{ID: "3", Label: "Cherry"},
		{ID: "4", Label: "Date"},
		{ID: "5", Label: "Elderberry"},
	}

	t.Run("creates state with all items", func(t *testing.T) {
		state := components.FuzzyPickerStateFor(items)

		if len(state.FilteredItems) != 5 {
			t.Errorf("Expected 5 items, got: %d", len(state.FilteredItems))
		}
		if state.FocusedIndex != 0 {
			t.Errorf("Expected focus at 0, got: %d", state.FocusedIndex)
		}
	})

	t.Run("filters items on query update", func(t *testing.T) {
		state := components.FuzzyPickerStateFor(items)
		state.UpdateQuery("ban")

		// Should only match "Banana"
		if len(state.FilteredItems) != 1 {
			t.Errorf("Expected 1 filtered item, got: %d", len(state.FilteredItems))
		}
		if state.FilteredItems[0].Label != "Banana" {
			t.Errorf("Expected 'Banana', got: %s", state.FilteredItems[0].Label)
		}
	})

	t.Run("MoveDown/MoveUp navigates focus", func(t *testing.T) {
		state := components.FuzzyPickerStateFor(items)

		state.MoveDown("down")
		if state.FocusedIndex != 1 {
			t.Errorf("Expected focus at 1, got: %d", state.FocusedIndex)
		}

		state.MoveUp("down")
		if state.FocusedIndex != 0 {
			t.Errorf("Expected focus at 0, got: %d", state.FocusedIndex)
		}
	})

	t.Run("GetFocused returns current item", func(t *testing.T) {
		state := components.FuzzyPickerStateFor(items)
		state.MoveDown("down")

		focused := state.GetFocused()
		if focused == nil || focused.Label != "Banana" {
			t.Errorf("Expected 'Banana', got: %v", focused)
		}
	})

	t.Run("clamps focus within bounds", func(t *testing.T) {
		state := components.FuzzyPickerStateFor(items)

		// Try to go above first item
		state.MoveUp("down")
		if state.FocusedIndex != 0 {
			t.Errorf("Expected focus clamped at 0, got: %d", state.FocusedIndex)
		}

		// Move to last and try to go beyond
		for i := 0; i < 10; i++ {
			state.MoveDown("down")
		}
		if state.FocusedIndex != 4 {
			t.Errorf("Expected focus clamped at 4, got: %d", state.FocusedIndex)
		}
	})
}

func TestFuzzyPickerModel(t *testing.T) {
	items := []components.FuzzyPickerItem{
		{ID: "1", Label: "Option 1"},
		{ID: "2", Label: "Option 2"},
	}

	t.Run("HandleKey navigates with arrow keys", func(t *testing.T) {
		cfg := components.FuzzyPickerConfig{
			Title: "Test",
			Items: items,
		}
		model := components.FuzzyPickerModelFor(cfg)

		model.HandleKey("down")
		if model.State.FocusedIndex != 1 {
			t.Errorf("Expected focus at 1, got: %d", model.State.FocusedIndex)
		}

		model.HandleKey("up")
		if model.State.FocusedIndex != 0 {
			t.Errorf("Expected focus at 0, got: %d", model.State.FocusedIndex)
		}
	})

	t.Run("HandleKey calls OnSelect on enter", func(t *testing.T) {
		cfg := components.FuzzyPickerConfig{
			Title: "Test",
			Items: items,
		}
		model := components.FuzzyPickerModelFor(cfg)

		selected := false
		model.OnSelect = func(item *components.FuzzyPickerItem) {
			selected = true
		}

		closed := model.HandleKey("enter")
		if !selected {
			t.Error("Expected OnSelect to be called")
		}
		if !closed {
			t.Error("Expected HandleKey to return true on enter")
		}
	})

	t.Run("HandleKey filters with character input", func(t *testing.T) {
		cfg := components.FuzzyPickerConfig{
			Title: "Test",
			Items: items,
		}
		model := components.FuzzyPickerModelFor(cfg)

		model.HandleKey("2")
		if model.State.Query != "2" {
			t.Errorf("Expected query '2', got: %s", model.State.Query)
		}
	})
}

func TestTabBarState(t *testing.T) {
	tabs := []string{"Tab 1", "Tab 2", "Tab 3"}

	t.Run("creates state with initial active", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 1)

		if state.ActiveIdx != 1 {
			t.Errorf("Expected active at 1, got: %d", state.ActiveIdx)
		}
	})

	t.Run("Next/Prev wraps around", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 2)

		state.Next()
		if state.ActiveIdx != 0 {
			t.Errorf("Expected wrap to 0, got: %d", state.ActiveIdx)
		}

		state.Prev()
		if state.ActiveIdx != 2 {
			t.Errorf("Expected wrap to 2, got: %d", state.ActiveIdx)
		}
	})

	t.Run("HandleKey handles arrow keys", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 0)

		handled := state.HandleKey("right")
		if !handled || state.ActiveIdx != 1 {
			t.Errorf("Expected right arrow to move to 1, got: %d", state.ActiveIdx)
		}

		handled = state.HandleKey("left")
		if !handled || state.ActiveIdx != 0 {
			t.Errorf("Expected left arrow to move to 0, got: %d", state.ActiveIdx)
		}
	})

	t.Run("HandleKey handles number keys", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 0)

		handled := state.HandleKey("2")
		if !handled || state.ActiveIdx != 1 {
			t.Errorf("Expected '2' to select tab 1, got: %d", state.ActiveIdx)
		}
	})

	t.Run("SetActiveByName finds correct tab", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 0)

		state.SetActiveByName("Tab 3")
		if state.ActiveIdx != 2 {
			t.Errorf("Expected active at 2, got: %d", state.ActiveIdx)
		}
	})

	t.Run("GetActive returns current tab name", func(t *testing.T) {
		state := components.TabBarStateFor(tabs, 1)

		if state.GetActive() != "Tab 2" {
			t.Errorf("Expected 'Tab 2', got: %s", state.GetActive())
		}
	})
}
