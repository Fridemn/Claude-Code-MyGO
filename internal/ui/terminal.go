package ui

import (
	"os"
	"syscall"
	"unsafe"
)

type terminalSize struct {
	Width  int
	Height int
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// GetTerminalSize returns the current terminal dimensions.
// This is the exported version of getTerminalSize for use by other packages.
// It matches the original TS behavior where process.stdout.columns is used.
func GetTerminalSize() terminalSize {
	return getTerminalSize()
}

func getTerminalSize() terminalSize {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return terminalSize{Width: 100, Height: 32}
	}
	return terminalSize{Width: int(ws.Col), Height: int(ws.Row)}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
