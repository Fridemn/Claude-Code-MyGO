package tests

import (
	"context"
	"testing"

	"claude-go/internal/command"
	tea "github.com/charmbracelet/bubbletea"
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

func TestRegisterLegacyPreservesPromptAndLocalJSXKinds(t *testing.T) {
	t.Parallel()

	r := command.EmptyRegistry()
	r.RegisterLegacy(command.LegacyCommand{
		Type: command.KindPrompt,
		Name: "legacy-prompt",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "prompt-body", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type: command.KindLocalJSX,
		Name: "legacy-panel",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "panel-body", nil
		},
	})

	promptCmd, ok := r.Lookup("/legacy-prompt")
	if !ok {
		t.Fatal("expected /legacy-prompt lookup to succeed")
	}
	if promptCmd.GetKind() != command.KindPrompt {
		t.Fatalf("expected /legacy-prompt kind=%q, got %q", command.KindPrompt, promptCmd.GetKind())
	}

	panelCmd, ok := r.Lookup("/legacy-panel")
	if !ok {
		t.Fatal("expected /legacy-panel lookup to succeed")
	}
	if panelCmd.GetKind() != command.KindLocalJSX {
		t.Fatalf("expected /legacy-panel kind=%q, got %q", command.KindLocalJSX, panelCmd.GetKind())
	}

	out, handled, err := r.Execute(context.Background(), "/legacy-prompt", command.Runtime{})
	if err != nil || !handled || out.Value != "prompt-body" {
		t.Fatalf("expected /legacy-prompt execute to return prompt body, handled=%t err=%v out=%q", handled, err, out.Value)
	}

	out, handled, err = r.Execute(context.Background(), "/legacy-panel", command.Runtime{})
	if err != nil || !handled || out.Value != "panel-body" {
		t.Fatalf("expected /legacy-panel execute to return panel body, handled=%t err=%v out=%q", handled, err, out.Value)
	}
}

func TestLoadModelBridgesRegisterLegacyLocalJSX(t *testing.T) {
	t.Parallel()

	r := command.EmptyRegistry()
	r.RegisterLegacy(command.LegacyCommand{
		Type: command.KindLocalJSX,
		Name: "legacy-panel",
		Handler: func(_ context.Context, _ command.Runtime, args []string) (string, error) {
			if len(args) == 0 {
				return "panel-body", nil
			}
			return "panel-body args=" + args[0], nil
		},
	})

	closed := false
	model, _, handled, err := r.LoadModel(context.Background(), "/legacy-panel foo", command.Runtime{
		OnExit: func() {
			closed = true
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected legacy local-jsx model bridge, handled=%t model=%T", handled, model)
	}
	if got := model.View(); got != "panel-body args=foo" {
		t.Fatalf("unexpected bridged model view: %q", got)
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from bridged model")
	}
	if !closed {
		t.Fatal("expected OnExit callback to be triggered by esc")
	}
}

func TestLoadModelBridgesDirectLegacyCommandLocalJSX(t *testing.T) {
	t.Parallel()

	r := command.EmptyRegistry()
	r.Register(command.LegacyCommand{
		Type: command.KindLocalJSX,
		Name: "plugin:panel",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "plugin-panel", nil
		},
	})

	model, _, handled, err := r.LoadModel(context.Background(), "/plugin:panel", command.Runtime{})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected legacy command local-jsx model bridge, handled=%t model=%T", handled, model)
	}
	if got := model.View(); got != "plugin-panel" {
		t.Fatalf("unexpected bridged model view: %q", got)
	}
}
