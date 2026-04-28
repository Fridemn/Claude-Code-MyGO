package cmd

import (
	"context"
	"io"

	"claude-go/internal/app"
	"claude-go/internal/cli"
)

func RunChat(ctx context.Context, application *app.App, stdout, stderr io.Writer) error {
	return cli.CreateChatRunner(application, stdout, stderr).Run(ctx)
}
