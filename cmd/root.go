package cmd

import (
	"context"
	"fmt"
	"io"

	"claude-code-go/internal/app"
	"claude-code-go/internal/cli"
)

func Execute(ctx context.Context, stdout, stderr io.Writer, args []string) error {
	if len(args) == 0 {
		application, err := app.Create(ctx)
		if err != nil {
			return err
		}
		return RunChat(ctx, application, stdout, stderr)
	}

	switch args[0] {
	case "chat":
		application, err := app.Create(ctx)
		if err != nil {
			return err
		}
		return RunChat(ctx, application, stdout, stderr)
	case "config":
		application, err := app.Create(ctx)
		if err != nil {
			return err
		}
		return RunConfig(ctx, application, stdout)
	case "test":
		return RunTest(ctx, stdout, stderr, args[1:])
	case "version", "--version", "-v":
		application, err := app.Create(ctx)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, application.Version())
		return err
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func Exit(err error) {
	cli.Exit(err)
}
