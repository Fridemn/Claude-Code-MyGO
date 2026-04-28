//go:build darwin

package cli

import (
	"syscall"
	"unsafe"
)

// On macOS, use TIOCGETA/TIOCSETA instead of TCGETS/TCSETS
const (
	TIOCGETA = 0x40487413
	TIOCSETA = 0x80487414
)

func getTermios(fd int) (*syscall.Termios, error) {
	var termios syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(TIOCGETA), uintptr(unsafe.Pointer(&termios)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return &termios, nil
}

func setTermios(fd int, termios *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), uintptr(TIOCSETA), uintptr(unsafe.Pointer(termios)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
