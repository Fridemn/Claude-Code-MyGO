package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func RunTest(_ context.Context, stdout, stderr io.Writer, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	pattern := "./..."
	if len(args) > 0 {
		pattern = args[0]
	}

	buildCache := filepath.Join(cwd, ".cache", "go-build")
	modCache := filepath.Join(cwd, ".cache", "go-mod")
	if err := os.MkdirAll(buildCache, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(modCache, 0o755); err != nil {
		return err
	}

	cmd := exec.Command("go", "test", pattern)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(),
		"GOCACHE="+buildCache,
		"GOMODCACHE="+modCache,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run tests: %w", err)
	}
	return nil
}
