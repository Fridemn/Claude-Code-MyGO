package main

import (
	"context"
	"fmt"
	"os"

	"claude-go/cmd"
)

// Build-time version information (set via ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	ctx := context.Background()
	err := cmd.Execute(ctx, os.Stdout, os.Stderr, os.Args[1:])
	cmd.Exit(err)
}

// GetVersionInfo returns formatted version information
func GetVersionInfo() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
