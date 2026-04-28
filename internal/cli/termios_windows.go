//go:build windows

package cli

import (
	"fmt"
)

// Windows does not support termios; provide stub implementations.
// Interactive terminal raw mode is not available on Windows.

func getTermios(fd int) error {
	return fmt.Errorf("termios not supported on windows")
}

func setTermios(fd int, termios interface{}) error {
	return fmt.Errorf("termios not supported on windows")
}
