package ui

type terminalSize struct {
	Width  int
	Height int
}

// GetTerminalSize returns the current terminal dimensions.
// This is the exported version of getTerminalSize for use by other packages.
// It matches the original TS behavior where process.stdout.columns is used.
func GetTerminalSize() terminalSize {
	return getTerminalSize()
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
