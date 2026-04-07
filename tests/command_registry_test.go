package tests

import (
	"context"
	"testing"

	"claude-code-go/internal/command"
)

func TestCommandRegistryAliasLookupAndListDedup(t *testing.T) {
	t.Parallel()

	r := command.EmptyRegistry()
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:    "primary",
			Aliases: []string{"p"},
			Source:  "builtin",
		},
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (command.CommandResult, error) {
			return command.CommandResult{Type: command.ResultTypeText, Value: "ok"}, nil
		},
	})

	out, ok, err := r.Execute(context.Background(), "/p one two", command.Runtime{})
	if err != nil || !ok {
		t.Fatalf("execute alias failed: ok=%t err=%v", ok, err)
	}
	if out.Value != "ok" {
		t.Fatalf("unexpected output: %s", out.Value)
	}

	cmd, ok := r.Lookup("/p")
	if !ok || cmd.GetBase().Name != "primary" {
		t.Fatalf("lookup alias failed: %#v", cmd)
	}

	list := r.List()
	if len(list) != 1 || list[0].GetBase().Name != "primary" {
		t.Fatalf("expected deduped list, got %#v", list)
	}
}
