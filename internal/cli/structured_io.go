//go:build !windows

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"
)

type StructuredIO struct {
	scanner *bufio.Scanner
	stdout  io.Writer
	stderr  io.Writer
	file    *os.File

	lastRenderLines int
	hasRendered     bool
}

var (
	rawTerminalMu     sync.Mutex
	rawTerminalState  = map[int]*termState{}
	signalRestoreOnce sync.Once
)

func CreateStructuredIO(input io.Reader, stdout, stderr io.Writer) *StructuredIO {
	file, _ := input.(*os.File)
	return &StructuredIO{
		scanner: bufio.NewScanner(input),
		stdout:  stdout,
		stderr:  stderr,
		file:    file,
	}
}

func (s *StructuredIO) Render(screen string) error {
	if s.file != nil {
		if s.hasRendered {
			if _, err := fmt.Fprint(s.stdout, renderRewritePrefix(s.lastRenderLines)); err != nil {
				return err
			}
		}
		s.hasRendered = true
		s.lastRenderLines = renderedLineCount(screen)
	}
	_, err := fmt.Fprint(s.stdout, screen)
	return err
}

func (s *StructuredIO) Prompt(label string) error {
	_, err := fmt.Fprint(s.stdout, label)
	return err
}

func (s *StructuredIO) ReadLine() (string, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", err
		}
		return "", io.EOF
	}
	return s.scanner.Text(), nil
}

func (s *StructuredIO) Stderr() io.Writer {
	return s.stderr
}

func (s *StructuredIO) ReadLineInteractive(render func(string) string) (string, error) {
	if s.file == nil {
		if err := s.Render(render("")); err != nil {
			return "", err
		}
		return s.ReadLine()
	}

	state, err := makeRaw(int(s.file.Fd()))
	if err != nil {
		if err := s.Render(render("")); err != nil {
			return "", err
		}
		return s.ReadLine()
	}
	installTerminalRestoreOnSignal()
	registerRawTerminal(int(s.file.Fd()), state)
	defer unregisterRawTerminal(int(s.file.Fd()))
	defer restoreTerminal(int(s.file.Fd()), state)

	current := ""
	pending := make([]byte, 0, utf8.UTFMax)
	if _, err := fmt.Fprint(s.stdout, render(current)); err != nil {
		return "", err
	}

	var one [1]byte
	for {
		n, err := s.file.Read(one[:])
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}
		switch b := one[0]; b {
		case '\r', '\n':
			return current, nil
		case 3:
			return "", io.EOF
		case 127, 8:
			pending = pending[:0]
			current = dropLastRune(current)
		case 27:
			pending = pending[:0]
			continue
		default:
			current, pending = appendUTF8Byte(current, pending, b)
		}
		if _, err := fmt.Fprint(s.stdout, render(current)); err != nil {
			return "", err
		}
	}
}

type termState struct {
	state syscall.Termios
}

func makeRaw(fd int) (*termState, error) {
	termios, err := getTermios(fd)
	if err != nil {
		return nil, err
	}
	old := *termios
	raw := old
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	return &termState{state: old}, nil
}

func restoreTerminal(fd int, state *termState) {
	if state == nil {
		return
	}
	_ = setTermios(fd, &state.state)
}

func installTerminalRestoreOnSignal() {
	signalRestoreOnce.Do(func() {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		go func() {
			sig := <-signals
			restoreAllRawTerminals()
			signal.Stop(signals)
			if unixSig, ok := sig.(syscall.Signal); ok {
				os.Exit(128 + int(unixSig))
			}
			os.Exit(1)
		}()
	})
}

func registerRawTerminal(fd int, state *termState) {
	if state == nil {
		return
	}
	rawTerminalMu.Lock()
	defer rawTerminalMu.Unlock()
	rawTerminalState[fd] = state
}

func unregisterRawTerminal(fd int) {
	rawTerminalMu.Lock()
	defer rawTerminalMu.Unlock()
	delete(rawTerminalState, fd)
}

func restoreAllRawTerminals() {
	rawTerminalMu.Lock()
	defer rawTerminalMu.Unlock()
	for fd, state := range rawTerminalState {
		restoreTerminal(fd, state)
		delete(rawTerminalState, fd)
	}
}

func appendUTF8Byte(current string, pending []byte, b byte) (string, []byte) {
	if len(pending) == 0 && b < 32 {
		return current, pending
	}
	if b == 255 {
		return current, pending
	}
	pending = append(pending, b)
	if !utf8.FullRune(pending) {
		return current, pending
	}
	r, size := utf8.DecodeRune(pending)
	if r == utf8.RuneError && size == 1 {
		if len(pending) >= utf8.UTFMax {
			return current, pending[:0]
		}
		return current, pending
	}
	current += string(r)
	pending = pending[size:]
	if len(pending) == 0 {
		pending = pending[:0]
	}
	return current, pending
}

func dropLastRune(value string) string {
	if value == "" {
		return value
	}
	_, size := utf8.DecodeLastRuneInString(value)
	if size <= 0 || size > len(value) {
		return ""
	}
	return value[:len(value)-size]
}

func renderRewritePrefix(lines int) string {
	if lines <= 1 {
		return "\r\033[J"
	}
	return fmt.Sprintf("\r\033[%dA\033[J", lines-1)
}

func renderedLineCount(screen string) int {
	if screen == "" {
		return 1
	}
	return strings.Count(screen, "\n") + 1
}
