package main

import (
	"context"
	"os"

	"claude-code-go/cmd"
)

func main() {
	ctx := context.Background()
	err := cmd.Execute(ctx, os.Stdout, os.Stderr, os.Args[1:])
	cmd.Exit(err)
}
