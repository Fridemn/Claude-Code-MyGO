package cmd

import (
	"context"
	"io"

	"claude-code-go/internal/app"
	"claude-code-go/internal/cli"
)

func RunChat(ctx context.Context, application *app.App, stdout, stderr io.Writer) error {
	return cli.CreateChatRunner(application, stdout, stderr).Run(ctx)
}
