package tests

import (
	"fmt"
	"strings"
	"testing"

	"claude-go/internal/ui/components"
)

type testVirtualItem struct {
	id     string
	text   string
	height int
}

func (i testVirtualItem) Key() string { return i.id }
func (i testVirtualItem) Height() int { return i.height }
func (i testVirtualItem) Render(_ int, focused bool) string {
	if focused {
		return "❯ " + i.text
	}
	return "  " + i.text
}

func buildItems(n int, h int) []components.VirtualListItem {
	out := make([]components.VirtualListItem, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, testVirtualItem{
			id:     fmt.Sprintf("id-%d", i),
			text:   fmt.Sprintf("item-%d", i),
			height: h,
		})
	}
	return out
}

func TestVirtualListStateNavigation(t *testing.T) {
	state := components.VirtualListStateFor(buildItems(100, 1), 80, 10)

	start, end := state.GetVisibleRange()
	if start != 0 {
		t.Fatalf("expected initial start=0, got %d", start)
	}
	if end <= 0 {
		t.Fatalf("expected non-empty visible range, got [%d,%d)", start, end)
	}

	for i := 0; i < 20; i++ {
		state.MoveDown()
	}
	if state.FocusedIdx != 20 {
		t.Fatalf("expected focused index 20, got %d", state.FocusedIdx)
	}
	if state.ScrollOffset <= 0 {
		t.Fatalf("expected scroll offset > 0 after moving down, got %d", state.ScrollOffset)
	}

	state.ScrollToItem(99, 3)
	if state.ScrollOffset == 0 {
		t.Fatalf("expected non-zero scroll when jumping to bottom item")
	}
}

func TestVirtualListStateHandleKey(t *testing.T) {
	state := components.VirtualListStateFor(buildItems(30, 1), 80, 8)

	if !state.HandleKey("down") || state.FocusedIdx != 1 {
		t.Fatalf("expected down key to move focus to 1, got %d", state.FocusedIdx)
	}
	if !state.HandleKey("pgdown") {
		t.Fatalf("expected pgdown to be handled")
	}
	if state.ScrollOffset <= 0 {
		t.Fatalf("expected pgdown to scroll")
	}
	if !state.HandleKey("home") || state.FocusedIdx != 0 {
		t.Fatalf("expected home to reset focus to 0, got %d", state.FocusedIdx)
	}
	if !state.HandleKey("end") || state.FocusedIdx != 29 {
		t.Fatalf("expected end to focus last item, got %d", state.FocusedIdx)
	}
}

func TestRenderFullscreenLayoutCore(t *testing.T) {
	state := components.FullscreenLayoutStateFor(60, 20)
	state.SetContentHeight(200)
	state.StickyPrompt = &components.StickyPrompt{Text: "show sticky"}
	state.IsStickyScroll = false
	state.NewMessageCount = 3
	state.DividerY = 150
	state.ScrollOffset = 10

	rendered := components.RenderFullscreenLayout(state, components.FullscreenLayoutRegions{
		Scrollable: "line1\nline2\nline3",
		Bottom:     "prompt>",
	})

	if !strings.Contains(rendered, "show sticky") {
		t.Fatalf("expected sticky prompt text in layout")
	}
	if !strings.Contains(rendered, "3 new messages") {
		t.Fatalf("expected new-message pill in layout")
	}
	if !strings.Contains(rendered, "prompt>") {
		t.Fatalf("expected bottom region in layout")
	}
}

func TestRenderFullscreenLayoutModal(t *testing.T) {
	state := components.FullscreenLayoutStateFor(60, 20)
	state.ShowModal = true
	state.SetContentHeight(80)

	rendered := components.RenderFullscreenLayout(state, components.FullscreenLayoutRegions{
		Modal: "modal-content",
	})

	if !strings.Contains(rendered, "modal-content") {
		t.Fatalf("expected modal content in modal layout")
	}
	if !strings.Contains(rendered, "▔") {
		t.Fatalf("expected modal divider in modal layout")
	}
}
