package services

// Terminal notification service.
// Ported from src/services/notifier.ts

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
)

// NotificationOptions contains options for a terminal notification.
// Ported from src/services/notifier.ts:NotificationOptions
type NotificationOptions struct {
	Message          string
	Title            string
	NotificationType string
}

// NotificationChannel represents available notification methods.
type NotificationChannel string

const (
	ChannelAuto             NotificationChannel = "auto"
	ChannelITerm2           NotificationChannel = "iterm2"
	ChannelITerm2WithBell   NotificationChannel = "iterm2_with_bell"
	ChannelKitty            NotificationChannel = "kitty"
	ChannelGhostty          NotificationChannel = "ghostty"
	ChannelTerminalBell     NotificationChannel = "terminal_bell"
	ChannelDisabled         NotificationChannel = "notifications_disabled"
)

// TerminalNotifier handles terminal-specific notifications.
// Ported from src/services/notifier.ts
type TerminalNotifier struct {
	preferredChannel NotificationChannel
	terminal         string // Terminal app name from env
}

// NewTerminalNotifier creates a new terminal notifier.
func NewTerminalNotifier(channel NotificationChannel, terminal string) *TerminalNotifier {
	return &TerminalNotifier{
		preferredChannel: channel,
		terminal:         terminal,
	}
}

// DefaultTerminalNotifier creates a notifier with auto-detection.
func DefaultTerminalNotifier() *TerminalNotifier {
	terminal := detectTerminal()
	return NewTerminalNotifier(ChannelAuto, terminal)
}

// detectTerminal detects the current terminal application.
func detectTerminal() string {
	// Check TERM_PROGRAM environment variable (common on macOS)
	termProgram := os.Getenv("TERM_PROGRAM")
	if termProgram != "" {
		return termProgram
	}

	// Check TERM variable for terminal type
	term := os.Getenv("TERM")
	if strings.Contains(term, "xterm") || strings.Contains(term, "screen") {
		return "generic"
	}

	return "unknown"
}

// SendNotification sends a notification using the configured channel.
// Ported from src/services/notifier.ts:sendNotification
func (n *TerminalNotifier) SendNotification(opts NotificationOptions) error {
	return n.sendToChannel(n.preferredChannel, opts)
}

// sendToChannel sends notification to a specific channel.
// Ported from src/services/notifier.ts:sendToChannel
func (n *TerminalNotifier) sendToChannel(channel NotificationChannel, opts NotificationOptions) error {
	title := opts.Title
	if title == "" {
		title = "Claude Code"
	}

	switch channel {
	case ChannelAuto:
		return n.sendAuto(opts)
	case ChannelITerm2:
		return n.notifyITerm2(opts)
	case ChannelITerm2WithBell:
		if err := n.notifyITerm2(opts); err != nil {
			return err
		}
		return n.notifyBell()
	case ChannelKitty:
		return n.notifyKitty(opts, title)
	case ChannelGhostty:
		return n.notifyGhostty(opts, title)
	case ChannelTerminalBell:
		return n.notifyBell()
	case ChannelDisabled:
		return nil // Silently skip
	default:
		return nil // Unknown channel, skip
	}
}

// sendAuto automatically selects the best notification method for the terminal.
// Ported from src/services/notifier.ts:sendAuto
func (n *TerminalNotifier) sendAuto(opts NotificationOptions) error {
	title := opts.Title
	if title == "" {
		title = "Claude Code"
	}

	switch n.terminal {
	case "Apple_Terminal":
		// Apple Terminal might have bell disabled, so we just use bell
		return n.notifyBell()

	case "iTerm.app":
		return n.notifyITerm2(opts)

	case "kitty":
		return n.notifyKitty(opts, title)

	case "ghostty":
		return n.notifyGhostty(opts, title)

	default:
		// No specific method available for this terminal
		return n.notifyBell()
	}
}

// notifyITerm2 sends an iTerm2-specific notification.
// Uses iTerm2's proprietary escape sequence for notifications.
// Ported from src/ink/useTerminalNotification.ts (implied)
func (n *TerminalNotifier) notifyITerm2(opts NotificationOptions) error {
	// iTerm2 notification escape sequence: ESC ] 997 ; message ST
	// This triggers a system notification in iTerm2
	OSCSequence := fmt.Sprintf("\x1b]997;%s\x07", opts.Message)
	fmt.Fprint(os.Stderr, OSCSequence)
	return nil
}

// notifyKitty sends a Kitty-specific notification.
// Uses Kitty's OSC 99 notification protocol.
// Ported from src/ink/useTerminalNotification.ts (implied)
func (n *TerminalNotifier) notifyKitty(opts NotificationOptions, title string) error {
	// Kitty notification escape sequence: ESC ] 99 ; i:id ; p:prio ; d:title ; a:action ; body ST
	// We use a simple form with just title and body
	id := generateKittyId()
	OSCSequence := fmt.Sprintf("\x1b]99;i=%d;d=%s;%s\x1b\\", id, title, opts.Message)
	fmt.Fprint(os.Stderr, OSCSequence)
	return nil
}

// notifyGhostty sends a Ghostty-specific notification.
// Uses Ghostty's OSC 99 notification protocol.
// Ported from src/ink/useTerminalNotification.ts (implied)
func (n *TerminalNotifier) notifyGhostty(opts NotificationOptions, title string) error {
	// Ghostty uses OSC 99 with title and body
	OSCSequence := fmt.Sprintf("\x1b]99;d=%s;%s\x1b\\", title, opts.Message)
	fmt.Fprint(os.Stderr, OSCSequence)
	return nil
}

// notifyBell sends a terminal bell notification.
// This works on most terminals as a fallback.
func (n *TerminalNotifier) notifyBell() error {
	// Terminal bell: ASCII BEL character (0x07)
	fmt.Fprint(os.Stderr, "\x07")
	return nil
}

// generateKittyId generates a random ID for Kitty notifications.
// Ported from src/services/notifier.ts:generateKittyId
func generateKittyId() int {
	return rand.Intn(10000)
}

// SetChannel sets the preferred notification channel.
func (n *TerminalNotifier) SetChannel(channel NotificationChannel) {
	n.preferredChannel = channel
}

// GetChannel returns the current preferred notification channel.
func (n *TerminalNotifier) GetChannel() NotificationChannel {
	return n.preferredChannel
}