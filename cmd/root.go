package cmd

import (
	"context"
	"io"

	"claude-go/internal/cli"
)

// Execute is the main entry point for the CLI
// Delegates to RunCLI which handles all argument parsing and execution
func Execute(ctx context.Context, stdout, stderr io.Writer, args []string) error {
	return RunCLI(ctx, stdout, stderr, args)
}

func Exit(err error) {
	cli.Exit(err)
}
