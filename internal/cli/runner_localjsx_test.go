package cli

import (
	"fmt"
	"strings"
	"testing"

	"claude-go/internal/command"
	"claude-go/internal/tool/interaction"

	tea "github.com/charmbracelet/bubbletea"
)

type testLocalJSXModel struct {
	width  int
	height int
}

func (m testLocalJSXModel) Init() tea.Cmd { return nil }

func (m testLocalJSXModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
		m.height = size.Height
	}
	return m, nil
}

func (m testLocalJSXModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	return fmt.Sprintf("size=%dx%d", m.width, m.height)
}

type localJSXCustomMsg struct{}

type escHandledMsg struct{}

type interactiveLocalJSXModel struct {
	width       int
	height      int
	lastKey     string
	customCount int
}

func (m interactiveLocalJSXModel) Init() tea.Cmd { return nil }

func (m interactiveLocalJSXModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			m.lastKey = string(msg.Runes)
		}
	case localJSXCustomMsg:
		m.customCount++
	}
	return m, nil
}

func (m interactiveLocalJSXModel) View() string {
	return fmt.Sprintf("interactive:%dx%d:%s:%d", m.width, m.height, m.lastKey, m.customCount)
}

type escHandlingLocalJSXModel struct {
	escCount    int
	customCount int
}

func (m escHandlingLocalJSXModel) Init() tea.Cmd { return nil }

func (m escHandlingLocalJSXModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			m.escCount++
			return m, func() tea.Msg { return escHandledMsg{} }
		}
	case escHandledMsg:
		m.customCount++
	}
	return m, nil
}

func (m escHandlingLocalJSXModel) View() string {
	return fmt.Sprintf("esc:%d:%d", m.escCount, m.customCount)
}

func TestRenderLocalJSXModelSnapshotUsesWindowSize(t *testing.T) {
	t.Parallel()

	content := renderLocalJSXModelSnapshot(testLocalJSXModel{}, 88, 23)
	if !strings.Contains(content, "size=88x23") {
		t.Fatalf("expected rendered snapshot to include size=88x23, got %q", content)
	}
}

func TestChatModelLocalJSXSubUIHandlesInputAndResize(t *testing.T) {
	t.Parallel()

	m := chatModel{
		width:        101,
		height:       37,
		currentInput: "should-not-change",
	}
	if cmd := (&m).activateLocalJSX(interactiveLocalJSXModel{}, "help", nil); cmd != nil {
		_ = cmd()
	}

	im, ok := m.activeLocalJSXModel.(interactiveLocalJSXModel)
	if !ok {
		t.Fatalf("expected active LocalJSX model type")
	}
	wantW, wantH := localJSXWindowSize(101, 37)
	if im.width != wantW || im.height != wantH {
		t.Fatalf("expected localjsx model size=%dx%d, got %dx%d", wantW, wantH, im.width, im.height)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := next.(chatModel)
	im, ok = updated.activeLocalJSXModel.(interactiveLocalJSXModel)
	if !ok {
		t.Fatalf("expected active LocalJSX model type after key update")
	}
	if im.lastKey != "j" {
		t.Fatalf("expected LocalJSX model to receive key 'j', got %q", im.lastKey)
	}
	if updated.currentInput != "should-not-change" {
		t.Fatalf("expected main input to stay unchanged, got %q", updated.currentInput)
	}

	next, _ = updated.Update(localJSXCustomMsg{})
	updated = next.(chatModel)
	im = updated.activeLocalJSXModel.(interactiveLocalJSXModel)
	if im.customCount != 1 {
		t.Fatalf("expected custom LocalJSX message to be forwarded once, got %d", im.customCount)
	}
}

func TestChatModelLocalJSXEscClosesSubUI(t *testing.T) {
	t.Parallel()

	m := chatModel{}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "help", nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(chatModel)
	if cmd == nil {
		t.Fatal("expected esc to emit close command")
	}
	closeMsg := cmd()
	next, _ = updated.Update(closeMsg)
	updated = next.(chatModel)
	if updated.activeLocalJSXModel != nil {
		t.Fatalf("expected esc to close active LocalJSX model")
	}
}

func TestChatModelLocalJSXOnExitLifecycleClosesSubUI(t *testing.T) {
	t.Parallel()

	life := newLocalJSXLifecycle()
	m := chatModel{}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "help", life)

	life.RequestClose()

	next, cmd := m.Update(localJSXCustomMsg{})
	updated := next.(chatModel)
	if cmd == nil {
		t.Fatal("expected lifecycle close command")
	}
	closeMsg := cmd()
	next, _ = updated.Update(closeMsg)
	updated = next.(chatModel)
	if updated.activeLocalJSXModel != nil {
		t.Fatalf("expected lifecycle close to deactivate localjsx model")
	}
}

func TestChatModelLocalJSXEscPrefersChildHandlingBeforeFallbackClose(t *testing.T) {
	t.Parallel()

	m := chatModel{}
	(&m).activateLocalJSX(escHandlingLocalJSXModel{}, "help", nil)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to run child command")
	}
	updated := next.(chatModel)
	msg := cmd()
	next, _ = updated.Update(msg)
	updated = next.(chatModel)
	im, ok := updated.activeLocalJSXModel.(escHandlingLocalJSXModel)
	if !ok {
		t.Fatalf("expected active LocalJSX esc model type")
	}
	if im.escCount != 1 || im.customCount != 1 {
		t.Fatalf("expected esc to be handled by child model once, got esc=%d custom=%d", im.escCount, im.customCount)
	}
	if updated.activeLocalJSXModel == nil {
		t.Fatalf("expected LocalJSX model to remain active after child-handled esc")
	}
}

func TestChatModelLocalJSXEscDoesNotForceCloseWhenLifecyclePresent(t *testing.T) {
	t.Parallel()

	life := newLocalJSXLifecycle()
	m := chatModel{}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "help", life)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(chatModel)
	if cmd != nil {
		t.Fatal("expected esc to not force close when lifecycle is active and child did not close")
	}
	if updated.activeLocalJSXModel == nil {
		t.Fatal("expected LocalJSX model to remain active")
	}

	life.RequestClose()
	next, cmd = updated.Update(localJSXCustomMsg{})
	updated = next.(chatModel)
	if cmd == nil {
		t.Fatal("expected lifecycle close command after request")
	}
	closeMsg := cmd()
	next, _ = updated.Update(closeMsg)
	updated = next.(chatModel)
	if updated.activeLocalJSXModel != nil {
		t.Fatal("expected lifecycle close to deactivate model")
	}
}

func TestChatModelLocalJSXOnDoneLifecycleClosesWithResultAndNextInput(t *testing.T) {
	t.Parallel()

	life := newLocalJSXLifecycle()
	m := chatModel{
		runner: &ChatRunner{},
	}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "help", life)

	life.Done("Help dismissed", command.LocalJSXDoneOptions{
		Display:   "system",
		NextInput: "/help",
	})

	next, cmd := m.Update(localJSXCustomMsg{})
	if cmd == nil {
		t.Fatal("expected onDone lifecycle command")
	}
	updated := next.(chatModel)
	doneMsg := cmd()
	next, _ = updated.Update(doneMsg)
	updated = next.(chatModel)
	if updated.activeLocalJSXModel != nil {
		t.Fatalf("expected onDone to deactivate localjsx model")
	}
	if updated.currentInput != "/help" {
		t.Fatalf("expected nextInput to be restored, got %q", updated.currentInput)
	}
	if len(updated.runner.entries) != 1 {
		t.Fatalf("expected one command entry from onDone, got %d", len(updated.runner.entries))
	}
	entry := updated.runner.entries[0]
	if entry.Title != "Command /help" || entry.Content != "Help dismissed" {
		t.Fatalf("unexpected onDone entry: title=%q content=%q", entry.Title, entry.Content)
	}
}

func TestChatModelLocalJSXActivationFromSlashCommandClearsBusyState(t *testing.T) {
	t.Parallel()

	m := chatModel{
		runner: &ChatRunner{},
		state: renderState{
			busy:       true,
			statusText: "Waiting for model response",
			verb:       "Working",
		},
	}

	next, _ := m.Update(localJSXActivatedMsg{
		commandName: "model",
		model:       interactiveLocalJSXModel{},
		lifecycle:   newLocalJSXLifecycle(),
		resetBusy:   true,
	})
	updated := next.(chatModel)
	if updated.state.busy {
		t.Fatal("expected LocalJSX activation from slash command to clear busy state")
	}
	if updated.state.statusText != "" || updated.state.verb != "" {
		t.Fatalf("expected busy labels to be cleared, got status=%q verb=%q", updated.state.statusText, updated.state.verb)
	}
	if updated.activeLocalJSXModel == nil {
		t.Fatal("expected LocalJSX model to remain active")
	}
}

func TestChatModelLocalJSXOnDoneDisplaySkipSuppressesTranscriptEntry(t *testing.T) {
	t.Parallel()

	life := newLocalJSXLifecycle()
	m := chatModel{
		runner: &ChatRunner{},
	}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "permissions", life)

	life.Done("Permission panel closed", command.LocalJSXDoneOptions{
		Display: "skip",
	})

	next, cmd := m.Update(localJSXCustomMsg{})
	if cmd == nil {
		t.Fatal("expected onDone lifecycle command")
	}
	updated := next.(chatModel)
	doneMsg := cmd()
	next, _ = updated.Update(doneMsg)
	updated = next.(chatModel)
	if updated.activeLocalJSXModel != nil {
		t.Fatalf("expected onDone to deactivate localjsx model")
	}
	if len(updated.runner.entries) != 0 {
		t.Fatalf("expected display=skip to suppress transcript entry, got %d entries", len(updated.runner.entries))
	}
}

func TestChatModelLocalJSXOnDoneShouldQueryNoRuntimeDoesNotForceSyntheticSubmit(t *testing.T) {
	t.Parallel()

	life := newLocalJSXLifecycle()
	m := chatModel{
		runner: &ChatRunner{},
	}
	(&m).activateLocalJSX(interactiveLocalJSXModel{}, "brief", life)

	life.Done("refresh quick status", command.LocalJSXDoneOptions{
		ShouldQuery: true,
	})

	next, cmd := m.Update(localJSXCustomMsg{})
	if cmd == nil {
		t.Fatal("expected onDone lifecycle command")
	}
	updated := next.(chatModel)
	doneMsg := cmd()
	next, cmd = updated.Update(doneMsg)
	updated = next.(chatModel)
	if cmd != nil {
		t.Fatalf("expected no synthetic submit command without runner app runtime, got %#v", cmd())
	}
	if updated.currentInput != "" {
		t.Fatalf("expected input to remain unchanged without runtime, got %q", updated.currentInput)
	}
}

func TestBuildLocalJSXModelContextMetaMessagesHonorsDisplayMode(t *testing.T) {
	t.Parallel()

	if got := buildLocalJSXModelContextMetaMessages("help", "done", "skip"); len(got) != 0 {
		t.Fatalf("expected display=skip to suppress model context messages, got %#v", got)
	}
	if got := buildLocalJSXModelContextMetaMessages("help", "done", "system"); len(got) != 0 {
		t.Fatalf("expected display=system to suppress model context messages, got %#v", got)
	}
	if got := buildLocalJSXModelContextMetaMessages("help", "done", "user"); len(got) != 0 {
		t.Fatalf("expected display=user to avoid duplicate meta context messages, got %#v", got)
	}
}

func TestBuildLocalJSXStdoutPayloadUsesNoContentFallback(t *testing.T) {
	t.Parallel()

	if got := buildLocalJSXStdoutPayload(""); got != "<local-command-stdout>(no content)</local-command-stdout>" {
		t.Fatalf("expected no-content fallback payload, got %q", got)
	}
}

func TestBuildLocalCommandOutputContentCombinesStdoutAndStderr(t *testing.T) {
	t.Parallel()

	if got := buildLocalCommandOutputContent("hello", "warn"); got != "hello\nwarn" {
		t.Fatalf("expected combined stdout/stderr, got %q", got)
	}
	if got := buildLocalCommandOutputContent("", "warn"); got != "warn" {
		t.Fatalf("expected stderr-only output, got %q", got)
	}
	if got := buildLocalCommandOutputContent("hello", ""); got != "hello" {
		t.Fatalf("expected stdout-only output, got %q", got)
	}
}

func TestBuildLocalJSXSystemLocalCommandMessageMarksTranscriptOnly(t *testing.T) {
	t.Parallel()

	msg := buildLocalJSXSystemLocalCommandMessage("help", "--verbose", "done")
	if msg.Role != "system" {
		t.Fatalf("expected system role, got %q", msg.Role)
	}
	if msg.Type != "local_command" {
		t.Fatalf("expected local_command subtype, got %q", msg.Type)
	}
	if !msg.IsVisibleInTranscriptOnly {
		t.Fatal("expected transcript-only flag to be true")
	}
	if !strings.Contains(msg.Content, "<command-name>/help</command-name>") {
		t.Fatalf("expected command breadcrumb tags, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "<command-args>--verbose</command-args>") {
		t.Fatalf("expected command args tags, got %q", msg.Content)
	}
	if !strings.Contains(msg.Content, "<local-command-stdout>done</local-command-stdout>") {
		t.Fatalf("expected stdout payload tag, got %q", msg.Content)
	}
}

func TestBuildLocalJSXUserCommandMessagesCreatesUserVisibleMessages(t *testing.T) {
	t.Parallel()

	msgs := buildLocalJSXUserCommandMessages("help", "--verbose", "done")
	if len(msgs) != 2 {
		t.Fatalf("expected two user messages, got %#v", msgs)
	}
	if msgs[0].Role != "user" || !strings.Contains(msgs[0].Content, "<command-name>/help</command-name>") {
		t.Fatalf("unexpected breadcrumb message: %#v", msgs[0])
	}
	if !strings.Contains(msgs[0].Content, "<command-args>--verbose</command-args>") {
		t.Fatalf("unexpected breadcrumb args: %#v", msgs[0])
	}
	if msgs[1].Role != "user" || msgs[1].Content != "<local-command-stdout>done</local-command-stdout>" {
		t.Fatalf("unexpected stdout message: %#v", msgs[1])
	}
}

func TestWrapLocalJSXCmdMapsQuitToCloseMsg(t *testing.T) {
	t.Parallel()

	cmd := wrapLocalJSXCmd(tea.Quit)
	if cmd == nil {
		t.Fatal("expected wrapped command")
	}
	msg := cmd()
	if _, ok := msg.(localJSXCloseMsg); !ok {
		t.Fatalf("expected localJSXCloseMsg, got %T", msg)
	}
}

func TestChatModelViewPrefersLocalJSXSubUI(t *testing.T) {
	t.Parallel()

	m := chatModel{
		width:  100,
		height: 30,
		activeLocalJSXModel: interactiveLocalJSXModel{
			width:   80,
			height:  24,
			lastKey: "x",
		},
		activeLocalJSXName: "help",
	}
	view := m.View()
	if !strings.Contains(view, "/help · Press Esc to close") {
		t.Fatalf("expected modal header, got %q", view)
	}
	if !strings.Contains(view, "interactive:80x24:x:0") {
		t.Fatalf("expected LocalJSX view, got %q", view)
	}
}

func TestChatModelViewOverlayWhenQuestionPromptAndLocalJSXActive(t *testing.T) {
	t.Parallel()

	// Create model with both question prompt and LocalJSX active
	m := chatModel{
		width:  100,
		height: 30,
		activeLocalJSXModel: interactiveLocalJSXModel{
			width:   80,
			height:  24,
			lastKey: "y",
		},
		activeLocalJSXName: "btw",
		questionPrompt: &activeQuestionPrompt{
			request: userQuestionRequest{
				question: interaction.Question{
					Question: "Permission required: Allow command?",
					Options: []interaction.QuestionOption{
						{Label: "Allow"},
						{Label: "Deny"},
					},
				},
			},
			selected: 0,
		},
	}

	view := m.View()

	// Should contain the LocalJSX overlay header
	if !strings.Contains(view, "/btw · Press Esc to close") {
		t.Fatalf("expected overlay header for btw, got %q", view)
	}

	// Should contain LocalJSX view content
	if !strings.Contains(view, "interactive:80x24:y:0") {
		t.Fatalf("expected LocalJSX view content, got %q", view)
	}

	// The view should NOT be the fullscreen modal (which would hide the question prompt)
	// In overlay mode, the question prompt content should still be visible
	// Since we don't have a real runner, the main view may be empty, but the structure should be overlay
	if strings.Contains(view, "▔▔▔▔") {
		// Fullscreen modal divider - should NOT appear in overlay mode
		t.Fatalf("expected overlay mode, but got fullscreen modal divider")
	}
}

func TestSpinnerTickUpdatesLocalJSXWhenQuestionPromptActive(t *testing.T) {
	t.Parallel()

	// Create model with both question prompt and LocalJSX active
	m := chatModel{
		width:  100,
		height: 30,
		activeLocalJSXModel: interactiveLocalJSXModel{
			width:   80,
			height:  24,
			lastKey: "y",
		},
		activeLocalJSXName: "btw",
		activeLocalJSXLife: newLocalJSXLifecycle(),
		questionPrompt: &activeQuestionPrompt{
			request: userQuestionRequest{
				question: interaction.Question{
					Question: "Permission required?",
					Options: []interaction.QuestionOption{
						{Label: "Allow"},
						{Label: "Deny"},
					},
				},
			},
			selected: 0,
		},
		state: renderState{
			busy: true,
		},
		runner: &ChatRunner{},
	}

	// Send a custom message that LocalJSX should receive
	// This simulates btwTickMsg being forwarded to the LocalJSX model
	next, _ := m.Update(localJSXCustomMsg{})
	updated := next.(chatModel)

	// The LocalJSX model should have received the custom message
	im, ok := updated.activeLocalJSXModel.(interactiveLocalJSXModel)
	if !ok {
		t.Fatalf("expected LocalJSX model to remain interactive type")
	}
	if im.customCount != 1 {
		t.Fatalf("expected LocalJSX to receive custom message (customCount=1), got %d", im.customCount)
	}

	// The question prompt should still be active
	if updated.questionPrompt == nil {
		t.Fatal("expected question prompt to remain active")
	}

	// LocalJSX should still be active
	if updated.activeLocalJSXModel == nil {
		t.Fatal("expected LocalJSX model to remain active")
	}
}
