package cli

import (
	"strings"
	"testing"
	"time"

	"claude-go/internal/bootstrap"
	"claude-go/internal/command"
	modelcmd "claude-go/internal/command/model"
	"claude-go/internal/ui"
)

func TestBuildSlashSuggestionsIncludesModelUsageDetails(t *testing.T) {
	registry := command.EmptyRegistry()
	modelcmd.Register(registry)

	suggestions := buildSlashSuggestions("/model", registry)
	if len(suggestions) != 1 {
		t.Fatalf("expected one /model suggestion, got %#v", suggestions)
	}
	got := strings.Join(suggestions[0].Details, "\n")
	for _, want := range []string{
		"guided API profile setup",
		"/model list",
		"/model use <name>",
		"/model add <name>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected details to contain %q, got:\n%s", want, got)
		}
	}
}

func TestBuildSlashSuggestionsCompletesModelSubcommands(t *testing.T) {
	registry := command.EmptyRegistry()
	modelcmd.Register(registry)

	suggestions := buildSlashSuggestions("/model li", registry)
	if len(suggestions) != 1 {
		t.Fatalf("expected one /model list suggestion, got %#v", suggestions)
	}
	if suggestions[0].Command != "/model list" {
		t.Fatalf("expected /model list suggestion, got %q", suggestions[0].Command)
	}
	if got := applySlashSuggestion("/model li", suggestions[0].Command); got != "/model list" {
		t.Fatalf("expected tab completion to replace subcommand, got %q", got)
	}
}

func TestApplySlashSuggestionPreservesSubcommandArguments(t *testing.T) {
	got := applySlashSuggestion("/model add default key=abc", "/model add")
	want := "/model add default key=abc"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderInputPanelShowsSuggestionDetails(t *testing.T) {
	panel := ui.RenderInputPanel(
		100,
		bootstrap.State{},
		"/model",
		len("/model"),
		-1,
		-1,
		[]ui.SlashSuggestion{{
			Command:     "/model",
			Description: "Manage models and API profiles",
			Details:     []string{"/model list · /model current", "/model add <name> key=<api-key> url=<base-url> model=<model>"},
		}},
		-1,
		false,
		0,
		"",
		"",
		time.Time{},
		"",
		0,
		0,
		nil,
		"",
		0,
		"",
		"",
	)
	if !strings.Contains(panel, "/model list") || !strings.Contains(panel, "/model add <name>") {
		t.Fatalf("expected suggestion details in panel, got:\n%s", panel)
	}
}
