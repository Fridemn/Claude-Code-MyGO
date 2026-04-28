package cmd

import (
	"context"
	"encoding/json"
	"io"

	"claude-go/internal/app"
)

func RunConfig(_ context.Context, application *app.App, stdout io.Writer) error {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(application.Config())
}
