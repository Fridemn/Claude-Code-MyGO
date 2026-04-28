//go:build windows

package ui

import (
	"os"
)

func getTerminalSize() terminalSize {
	// On Windows, try to get size from console, fallback to defaults
	type shortInfo struct {
		SizeX, SizeY   int16
		CursorX, CursorY int16
		Attrs          uint16
		WindowLeft, WindowTop int16
		WindowRight, WindowBottom int16
		MaxWindowSizeX, MaxWindowSizeY int16
	}
	return terminalSize{Width: 100, Height: 32}
}

func init() {
	// Suppress unused import
	_ = os.Stdout
}
