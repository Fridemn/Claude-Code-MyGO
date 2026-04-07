package cli

import (
	"testing"
	"time"
)

func TestListenForStreamUpdatesBlocksUntilUpdate(t *testing.T) {
	t.Parallel()

	runner := &ChatRunner{
		streamChan: make(chan streamUpdate),
	}

	cmd := listenForStreamUpdates(runner)
	if cmd == nil {
		t.Fatal("listenForStreamUpdates returned nil command")
	}

	done := make(chan streamUpdateMsg, 1)
	go func() {
		msg, _ := cmd().(streamUpdateMsg)
		done <- msg
	}()

	select {
	case <-done:
		t.Fatal("listener returned before any stream update was sent")
	case <-time.After(40 * time.Millisecond):
		// Expected: command blocks until an update is available.
	}

	want := streamUpdate{
		text:     "chunk",
		toolName: "Bash",
		status:   "Running tool: Bash",
		refresh:  true,
	}
	runner.streamChan <- want

	select {
	case got := <-done:
		if got.text != want.text {
			t.Fatalf("text = %q, want %q", got.text, want.text)
		}
		if got.toolName != want.toolName {
			t.Fatalf("toolName = %q, want %q", got.toolName, want.toolName)
		}
		if got.status != want.status {
			t.Fatalf("status = %q, want %q", got.status, want.status)
		}
		if got.refresh != want.refresh {
			t.Fatalf("refresh = %v, want %v", got.refresh, want.refresh)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for listener to receive stream update")
	}
}
