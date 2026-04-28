//go:build !windows

package ui

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTerminalSize() terminalSize {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return terminalSize{Width: 100, Height: 32}
	}
	return terminalSize{Width: int(ws.Col), Height: int(ws.Row)}
}
