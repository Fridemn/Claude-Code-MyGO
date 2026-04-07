package tool

import (
	"os/exec"
	"path/filepath"
)

func cleanToolPath(path string) string {
	if path == "" {
		return "."
	}
	return filepath.Clean(path)
}

func defaultShellCommand(command string) *exec.Cmd {
	return exec.Command("bash", "-lc", command)
}
