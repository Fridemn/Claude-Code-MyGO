package tests

import (
	"context"
	"testing"

	"claude-go/internal/command"
)

func TestSlashCommandExecuteModel(t *testing.T) {
	t.Parallel()

	reg := command.EmptyRegistry()
	reg.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:        "model",
			Description: "show model",
		},
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (command.CommandResult, error) {
			return command.CommandResult{Type: command.ResultTypeText, Value: "model=test\nbase_url=http://x"}, nil
		},
	})

	out, handled, err := reg.Execute(context.Background(), "/model", command.Runtime{})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !handled {
		t.Fatal("expected command handled")
	}
	if out.Value == "" {
		t.Fatal("expected non-empty output for /model")
	}
}

