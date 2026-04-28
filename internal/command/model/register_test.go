package model

import (
	"context"
	"strings"
	"testing"

	"claude-go/internal/command"
	cmdstats "claude-go/internal/command/stats"
	"claude-go/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelAddUseListProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	applied := config.Config{}
	rt := command.Runtime{
		Config: config.Config{
			APIKey:  "env-key",
			BaseURL: "https://env.example/v1/chat/completions",
			Model:   "env-model",
		},
		OnConfigChange: func(cfg config.Config) error {
			applied = cfg
			return nil
		},
	}

	out, err := handleModelCommand(context.Background(), rt, []string{
		"add",
		"coolyeah",
		"key=secret-key",
		"url=https://code.coolyeah.net/v1/chat/completions",
		"model=glm-5",
		"summary=glm-4.7",
	})
	if err != nil {
		t.Fatalf("add profile: %v", err)
	}
	if !strings.Contains(out, "coolyeah") || !strings.Contains(out, "glm-5") {
		t.Fatalf("unexpected add output: %s", out)
	}
	if applied.APIKey != "secret-key" || applied.BaseURL != "https://code.coolyeah.net/v1/chat/completions" || applied.Model != "glm-5" || applied.SummaryModel != "glm-4.7" {
		t.Fatalf("unexpected applied config: %#v", applied)
	}

	list, err := handleModelCommand(context.Background(), rt, []string{"list"})
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if !strings.Contains(list, "* coolyeah") {
		t.Fatalf("expected active profile in list, got:\n%s", list)
	}
	if strings.Contains(list, "secret-key") {
		t.Fatalf("list leaked full API key:\n%s", list)
	}

	applied = config.Config{}
	_, err = handleModelCommand(context.Background(), rt, []string{"use", "coolyeah"})
	if err != nil {
		t.Fatalf("use profile: %v", err)
	}
	if applied.Model != "glm-5" {
		t.Fatalf("expected profile model applied, got %#v", applied)
	}
}

func TestModelAddSanitizesControlCharacters(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	applied := config.Config{}
	rt := command.Runtime{
		OnConfigChange: func(cfg config.Config) error {
			applied = cfg
			return nil
		},
	}

	_, err := handleModelCommand(context.Background(), rt, []string{
		"add",
		"default\x00",
		"key=\x00secret-key\r",
		"url=\x00https://api.example.com/v1/chat/completions\r",
		"model=\x00glm-5\n",
		"summary=\x00glm-5\t",
	})
	if err != nil {
		t.Fatalf("add sanitized profile: %v", err)
	}
	if applied.APIKey != "secret-key" || applied.BaseURL != "https://api.example.com/v1/chat/completions" || applied.Model != "glm-5" || applied.SummaryModel != "glm-5" {
		t.Fatalf("expected sanitized applied config, got %#v", applied)
	}

	store, err := config.LoadAPIProfiles()
	if err != nil {
		t.Fatalf("load profiles: %v", err)
	}
	profile, ok := store.Profiles["default"]
	if !ok {
		t.Fatalf("expected sanitized profile name, got %#v", store.Profiles)
	}
	if strings.ContainsRune(profile.BaseURL, '\x00') || strings.ContainsRune(profile.BaseURL, '\r') {
		t.Fatalf("base URL was not sanitized: %q", profile.BaseURL)
	}
}

func TestModelRemoveProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	rt := command.Runtime{Config: config.Config{APIKey: "k", BaseURL: "https://api.example.com/v1/chat/completions", Model: "m"}}
	if _, err := handleModelCommand(context.Background(), rt, []string{"add", "one", "key=k", "url=https://api.example.com/v1/chat/completions", "model=m"}); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	out, err := handleModelCommand(context.Background(), rt, []string{"remove", "one"})
	if err != nil {
		t.Fatalf("remove profile: %v", err)
	}
	if !strings.Contains(out, "Removed") {
		t.Fatalf("unexpected remove output: %s", out)
	}
}

func TestModelNameSwitchAppliesRuntimeConfig(t *testing.T) {
	applied := config.Config{}
	rt := command.Runtime{
		Config: config.Config{APIKey: "k", BaseURL: "u", Model: "old-model"},
		OnConfigChange: func(cfg config.Config) error {
			applied = cfg
			return nil
		},
	}

	out, err := handleModelCommand(context.Background(), rt, []string{"gpt-4o"})
	if err != nil {
		t.Fatalf("switch model: %v", err)
	}
	if !strings.Contains(out, "gpt-4o") {
		t.Fatalf("unexpected output: %s", out)
	}
	if applied.Model != "gpt-4o" {
		t.Fatalf("expected runtime config model to change, got %#v", applied)
	}
}

func TestModelWizardPromptsOneFieldAtATime(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	applied := config.Config{}
	done := ""
	rt := command.Runtime{
		Config: config.Config{Model: "default-model"},
		OnConfigChange: func(cfg config.Config) error {
			applied = cfg
			return nil
		},
		OnLocalJSXDone: func(result string, _ command.LocalJSXDoneOptions) {
			done = result
		},
	}

	modelAny, err := loadModelCommandModel(context.Background(), rt, nil)
	if err != nil {
		t.Fatalf("load wizard: %v", err)
	}
	view := modelAny.View()
	if !strings.Contains(view, "Step 1 of 5") || !strings.Contains(view, "Profile name") {
		t.Fatalf("expected first prompt, got:\n%s", view)
	}
	if !strings.Contains(view, "Input") || !strings.Contains(view, "╭") || !strings.Contains(view, "|") {
		t.Fatalf("expected visible input box and cursor, got:\n%s", view)
	}

	modelAny = sendText(modelAny, "work")
	modelAny, _ = modelAny.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = modelAny.View()
	if !strings.Contains(view, "Step 2 of 5") || !strings.Contains(view, "API key") {
		t.Fatalf("expected API key prompt, got:\n%s", view)
	}

	modelAny = sendText(modelAny, "secret-key")
	modelAny, _ = modelAny.Update(tea.KeyMsg{Type: tea.KeyEnter})
	modelAny = sendText(modelAny, "https://api.example.com/v1/chat/completions")
	modelAny, _ = modelAny.Update(tea.KeyMsg{Type: tea.KeyEnter})
	modelAny = sendText(modelAny, "example-model")
	modelAny, _ = modelAny.Update(tea.KeyMsg{Type: tea.KeyEnter})
	modelAny, _ = modelAny.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if applied.APIKey != "secret-key" || applied.BaseURL != "https://api.example.com/v1/chat/completions" || applied.Model != "example-model" {
		t.Fatalf("wizard did not apply profile config: %#v", applied)
	}
	if !strings.Contains(done, "work") || !strings.Contains(done, "example-model") {
		t.Fatalf("unexpected completion result: %q", done)
	}
}

func TestRenderWizardInputBoxShowsCursorAndMasksSecrets(t *testing.T) {
	box := renderWizardInputBox("secret", true, 80)
	if strings.Contains(box, "secret") {
		t.Fatalf("secret input leaked:\n%s", box)
	}
	if !strings.Contains(box, "******") || !strings.Contains(box, "|") {
		t.Fatalf("expected masked input with cursor, got:\n%s", box)
	}

	empty := renderWizardInputBox("", false, 40)
	if !strings.Contains(empty, "|") || !strings.Contains(empty, "╭") {
		t.Fatalf("expected empty box with cursor, got:\n%s", empty)
	}
}

func TestRegisterModelCommandIsInteractive(t *testing.T) {
	registry := command.EmptyRegistry()
	Register(registry)
	cmdstats.Register(registry)

	cmd, ok := registry.Lookup("/model")
	if !ok {
		t.Fatal("expected /model to be registered")
	}
	if cmd.GetKind() != command.KindLocalJSX {
		t.Fatalf("expected /model to be interactive, got %s", cmd.GetKind())
	}
}

func sendText(model tea.Model, text string) tea.Model {
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
	return updated
}
