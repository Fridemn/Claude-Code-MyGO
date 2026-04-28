package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/command"
	cmdskills "claude-go/internal/command/skills"
	"claude-go/internal/services"
	"claude-go/internal/tool/skill"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSkillsServiceLoadsProjectSkills(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	skillsDir := filepath.Join(root, ".claude", "skills", "refactor")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}
	raw := `---
description: Refactor a module safely
argument-hint: <target>
when_to_use: large refactors
aliases: rf, ref
allowed-tools: read_file, grep
model: test-model
context: fork
agent: planner
---
Inspect the target, identify coupling, and propose a staged refactor plan.
`
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	service := services.CreateSkillsService(root)
	skills := service.List()
	if len(skills) < 3 {
		t.Fatalf("expected bundled + project skills, got %d", len(skills))
	}
	var projectSkill skill.Skill
	found := false
	for _, skill := range skills {
		if skill.Name == "refactor" {
			projectSkill = skill
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("project skill not found in %#v", skills)
	}
	if len(projectSkill.Aliases) != 2 || projectSkill.Aliases[0] != "rf" {
		t.Fatalf("unexpected skill aliases: %#v", projectSkill.Aliases)
	}
	if projectSkill.Context != "fork" || projectSkill.Agent != "planner" {
		t.Fatalf("unexpected skill execution metadata: %#v", projectSkill)
	}
	if !strings.Contains(service.Status(), "registered=") {
		t.Fatalf("unexpected skills status: %s", service.Status())
	}

	commands := service.Commands()
	if len(commands) < 3 {
		t.Fatalf("expected bundled + project skill commands, got %d", len(commands))
	}
	var projectCommand command.Command
	found = false
	for _, cmd := range commands {
		if cmd.GetBase().Name == "refactor" {
			projectCommand = cmd
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("project skill command not found in %#v", commands)
	}
	base := projectCommand.GetBase()
	if len(base.Aliases) != 2 || base.Aliases[1] != "ref" {
		t.Fatalf("unexpected command aliases: %#v", base.Aliases)
	}
}

func TestSkillsServiceMergesUserAndProjectSkills(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	userSkillDir := filepath.Join(home, ".claude", "skills", "home-skill")
	projectSkillDir := filepath.Join(root, ".claude", "skills", "project-skill")
	if err := os.MkdirAll(userSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir user skill dir: %v", err)
	}
	if err := os.MkdirAll(projectSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir project skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkillDir, "SKILL.md"), []byte("---\ndescription: User skill\n---\nUser prompt"), 0o644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte("---\ndescription: Project skill\n---\nProject prompt"), 0o644); err != nil {
		t.Fatalf("write project skill: %v", err)
	}

	service := services.CreateSkillsService(root)
	skills := service.List()
	if len(skills) < 4 {
		t.Fatalf("expected bundled + user + project skills, got %d", len(skills))
	}
	if !strings.Contains(service.Status(), filepath.Join(home, ".claude", "skills")) {
		t.Fatalf("expected user skills dir in status: %s", service.Status())
	}
}

func TestBundledSkillsArePresent(t *testing.T) {
	service := services.CreateSkillsService(t.TempDir())
	skills := service.List()
	names := make(map[string]bool, len(skills))
	for _, skill := range skills {
		names[skill.Name] = true
	}
	if !names["debug"] || !names["verify"] {
		t.Fatalf("expected bundled skills in registry, got %#v", names)
	}
}

func TestPluginsServiceLoadsCommandsAgentsSkillsAndHooks(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "demo")
	if err := os.MkdirAll(filepath.Join(pluginRoot, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "agents"), 0o755); err != nil {
		t.Fatalf("mkdir agents dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "skills", "audit"), 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pluginRoot, "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir hooks dir: %v", err)
	}

	manifest := `{
  "name": "demo-plugin",
  "description": "Demo plugin",
  "version": "1.2.3",
  "marketplace": "custom-market",
  "category": "delivery",
  "dev": true,
  "commands": "commands",
  "agents": "agents",
  "skills": "skills",
  "hooks": "hooks/hooks.json"
}`
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "commands", "ship.md"), []byte(`---
description: Ship the project
aliases: deploy, release
allowed-tools: read_file, exec_command
model: deploy-model
when_to_use: shipping a release
disable-model-invocation: true
---
Prepare a release checklist.
`), 0o644); err != nil {
		t.Fatalf("write command markdown: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "agents", "reviewer.md"), []byte(`---
description: Review risky changes
tools: read_file,grep
background: true
maxTurns: 3
read-only: true
omit-claude-md: true
initial-prompt: Review this carefully first.
---
Review the supplied changes and focus on regressions.
`), 0o644); err != nil {
		t.Fatalf("write agent markdown: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "skills", "audit", "SKILL.md"), []byte(`---
description: Audit a subsystem
---
Inspect the subsystem and summarize risks.
`), 0o644); err != nil {
		t.Fatalf("write plugin skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "hooks", "hooks.json"), []byte(`{
  "hooks": [
    {
      "event": "pre_turn",
      "command": "echo plugin_pre_turn",
      "description": "Plugin hook",
      "enabled": true,
      "matcher": "*",
      "timeout_ms": 500
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write plugin hooks: %v", err)
	}

	configPath := filepath.Join(root, "plugins.json")
	if err := os.WriteFile(configPath, []byte(`{
  "enabled": true,
  "status": "active",
  "plugins": [
    {
      "name": "demo-plugin",
      "source": "local",
      "status": "registered",
      "enabled": true,
      "path": "`+strings.ReplaceAll(pluginRoot, `\`, `\\`)+`"
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write plugins config: %v", err)
	}

	service := services.CreatePluginsService(configPath)

	plugins := service.List()
	if len(plugins) != 1 {
		t.Fatalf("expected one plugin, got %d", len(plugins))
	}
	if plugins[0].Status != "loaded" {
		t.Fatalf("expected loaded plugin status, got %s", plugins[0].Status)
	}
	if plugins[0].Marketplace != "custom-market" || plugins[0].Category != "delivery" || !plugins[0].Dev {
		t.Fatalf("unexpected manifest metadata on plugin: %#v", plugins[0])
	}
	if plugins[0].SourceType != "directory" {
		t.Fatalf("expected directory source type, got %#v", plugins[0])
	}
	if len(service.Commands()) != 1 || service.Commands()[0].GetBase().Name != "demo-plugin:ship" {
		t.Fatalf("unexpected plugin commands: %#v", service.Commands())
	}
	cmdBase := service.Commands()[0].GetBase()
	if len(cmdBase.Aliases) != 2 || cmdBase.Aliases[0] != "deploy" {
		t.Fatalf("unexpected plugin command aliases: %#v", cmdBase.Aliases)
	}
	if len(service.Agents()) != 1 || service.Agents()[0].AgentType != "demo-plugin:reviewer" {
		t.Fatalf("unexpected plugin agents: %#v", service.Agents())
	}
	if !service.Agents()[0].ReadOnly || !service.Agents()[0].OmitClaudeMd || service.Agents()[0].InitialPrompt != "Review this carefully first." {
		t.Fatalf("unexpected plugin agent metadata: %#v", service.Agents()[0])
	}
	if len(service.Skills()) != 1 || service.Skills()[0].Name != "demo-plugin:audit" {
		t.Fatalf("unexpected plugin skills: %#v", service.Skills())
	}
	if len(service.PluginHooks()) != 1 || service.PluginHooks()[0].Source != "plugin" {
		t.Fatalf("unexpected plugin hooks: %#v", service.PluginHooks())
	}
	if plugins[0].CommandCount != 1 || plugins[0].AgentCount != 1 || plugins[0].SkillCount != 1 || plugins[0].HookCount != 1 {
		t.Fatalf("unexpected plugin component counts: %#v", plugins[0])
	}
	if !strings.Contains(service.Status(), "loaded_commands=1") || !strings.Contains(service.Status(), "loaded_skills=1") {
		t.Fatalf("unexpected plugin status output: %s", service.Status())
	}
}

func TestPluginManifestCommandMetadataOverridesMarkdown(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "meta")
	if err := os.MkdirAll(filepath.Join(pluginRoot, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}

	manifest := `{
  "name": "meta-plugin",
  "version": "0.2.0",
  "commands": "commands",
  "commands_metadata": {
    "ship": {
      "description": "Overridden description",
      "display_name": "Ship Release",
      "argument_hint": "<target>",
      "when_to_use": "release day",
      "allowed_tools": ["exec_command"],
      "model": "override-model",
      "aliases": ["shipit"],
      "type": "local-jsx",
      "hidden": true,
      "disable_model_invocation": true
    }
  }
}`
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "commands", "ship.md"), []byte(`---
description: Markdown description
aliases: deploy
allowed-tools: read_file
model: markdown-model
---
Release flow
`), 0o644); err != nil {
		t.Fatalf("write markdown command: %v", err)
	}
	configPath := filepath.Join(root, "plugins.json")
	if err := os.WriteFile(configPath, []byte(`{
  "enabled": true,
  "plugins": [
    {
      "name": "meta-plugin",
      "source": "local",
      "enabled": true,
      "path": "`+strings.ReplaceAll(pluginRoot, `\`, `\\`)+`"
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	service := services.CreatePluginsService(configPath)
	commands := service.Commands()
	if len(commands) != 1 {
		t.Fatalf("expected one command, got %d", len(commands))
	}
	cmd := commands[0]
	base := cmd.GetBase()
	if base.Description != "Overridden description" || base.DisplayName != "Ship Release" {
		t.Fatalf("expected manifest description/display name override, got %#v", base)
	}
	if cmd.GetKind() != command.KindLocalJSX || !base.Hidden || !base.DisableModelInvocation {
		t.Fatalf("expected manifest type/hidden/model-invocation override, got %#v", base)
	}
	if len(base.Aliases) != 1 || base.Aliases[0] != "shipit" {
		t.Fatalf("expected manifest alias override, got %#v", base.Aliases)
	}
}

func TestBuiltinPluginLoadsBuiltinComponents(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "plugins.json")
	service := services.CreatePluginsService(configPath)

	plugins := service.List()
	if len(plugins) == 0 || plugins[0].Source != "builtin" {
		t.Fatalf("expected builtin plugin defaults, got %#v", plugins)
	}
	if plugins[0].SourceType != "builtin" || plugins[0].SkillCount == 0 || plugins[0].HookCount == 0 {
		t.Fatalf("expected builtin plugin runtime counts/source type, got %#v", plugins[0])
	}
	if len(service.Skills()) == 0 {
		t.Fatalf("expected builtin plugin skills")
	}
	if len(service.PluginHooks()) == 0 {
		t.Fatalf("expected builtin plugin hooks")
	}
}

func TestPluginsReloadRecomputesComponentCounts(t *testing.T) {
	root := t.TempDir()
	pluginRoot := filepath.Join(root, "plugins", "reloadable")
	if err := os.MkdirAll(filepath.Join(pluginRoot, "commands"), 0o755); err != nil {
		t.Fatalf("mkdir commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), []byte(`{
  "name": "reloadable",
  "commands": "commands"
}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "commands", "one.md"), []byte("---\ndescription: one\n---\nOne"), 0o644); err != nil {
		t.Fatalf("write first command: %v", err)
	}
	configPath := filepath.Join(root, "plugins.json")
	if err := os.WriteFile(configPath, []byte(`{
  "enabled": true,
  "plugins": [
    {
      "name": "reloadable",
      "source": "local",
      "enabled": true,
      "path": "`+strings.ReplaceAll(pluginRoot, `\`, `\\`)+`"
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	service := services.CreatePluginsService(configPath)
	if got := service.List()[0].CommandCount; got != 1 {
		t.Fatalf("expected initial command count 1, got %d", got)
	}

	if err := os.WriteFile(filepath.Join(pluginRoot, "commands", "two.md"), []byte("---\ndescription: two\n---\nTwo"), 0o644); err != nil {
		t.Fatalf("write second command: %v", err)
	}
	out := service.Reload()
	if !strings.Contains(out, "loaded=1") {
		t.Fatalf("unexpected reload output: %s", out)
	}
	if got := service.List()[0].CommandCount; got != 2 {
		t.Fatalf("expected recomputed command count 2, got %d", got)
	}
	status := service.Status()
	if !strings.Contains(status, "last_event=plugins_reloaded") {
		t.Fatalf("expected reload event in status, got %s", status)
	}
}

func TestPluginsStatusTracksLifecycleEvents(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "plugins.json")
	service := services.CreatePluginsService(configPath)

	service.Add(services.Plugin{
		Name:    "demo",
		Source:  "local",
		Enabled: true,
		Status:  "registered",
	})
	if !strings.Contains(service.Status(), "last_event=plugin_added") || !strings.Contains(service.Status(), "last_target=demo") {
		t.Fatalf("expected add event in status: %s", service.Status())
	}
	notices := service.DrainNotices()
	if len(notices) == 0 || notices[len(notices)-1].Kind != "plugin_added" {
		t.Fatalf("expected plugin add notice, got %#v", notices)
	}

	if ok := service.SetEnabled("demo", false); !ok {
		t.Fatalf("expected enable mutation to succeed")
	}
	if !strings.Contains(service.Status(), "last_event=plugin_enabled_changed") {
		t.Fatalf("expected enable event in status: %s", service.Status())
	}
	notices = service.DrainNotices()
	if len(notices) == 0 || notices[len(notices)-1].Kind != "plugin_enabled_changed" {
		t.Fatalf("expected plugin enable notice, got %#v", notices)
	}

	if ok := service.Remove("demo"); !ok {
		t.Fatalf("expected remove mutation to succeed")
	}
	if !strings.Contains(service.Status(), "last_event=plugin_removed") || !strings.Contains(service.Status(), "last_target=demo") {
		t.Fatalf("expected remove event in status: %s", service.Status())
	}
	notices = service.DrainNotices()
	if len(notices) == 0 || notices[len(notices)-1].Kind != "plugin_removed" {
		t.Fatalf("expected plugin remove notice, got %#v", notices)
	}
}

func TestSkillsCommandRendersPanel(t *testing.T) {
	registry := command.EmptyRegistry()
	cmdskills.Register(registry)

	out, ok, err := registry.Execute(context.Background(), "/skills", command.Runtime{
		SkillList: func() []command.SkillInfo {
			return []command.SkillInfo{{
				Name:          "refactor",
				DisplayName:   "Refactor Module",
				Aliases:       []string{"rf"},
				Description:   "Refactor a subsystem safely",
				WhenToUse:     "large changes",
				Source:        "projectSettings",
				LoadedFrom:    "skills",
				Path:          ".claude/skills/refactor/SKILL.md",
				UserInvocable: true,
			}}
		},
		SkillStatus: func() string {
			return "skills=active\nregistered=1"
		},
	})
	if err != nil || !ok {
		t.Fatalf("/skills failed: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "registry=skills") || !strings.Contains(out.Value, "refactor") {
		t.Fatalf("unexpected /skills output: %s", out.Value)
	}
	if !strings.Contains(out.Value, "aliases=rf") || !strings.Contains(out.Value, "display_name=Refactor Module") {
		t.Fatalf("expected skill metadata in /skills output: %s", out.Value)
	}
}

func TestSkillsCommandLoadModel(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdskills.Register(registry)

	var doneResult string
	var doneDisplay string
	model, _, handled, err := registry.LoadModel(context.Background(), "/skills", command.Runtime{
		SkillList: func() []command.SkillInfo {
			return []command.SkillInfo{
				{
					Name:          "refactor",
					DisplayName:   "Refactor Module",
					Aliases:       []string{"rf"},
					Description:   "Refactor a subsystem safely",
					WhenToUse:     "large changes",
					Source:        "projectSettings",
					Path:          ".claude/skills/refactor/SKILL.md",
					UserInvocable: true,
				},
				{
					Name:          "demo-plugin:audit",
					DisplayName:   "Audit Plugin",
					Description:   "Audit plugin state",
					Source:        "plugin",
					UserInvocable: false,
				},
			}
		},
		SkillStatus: func() string {
			return "skills=active\nregistered=2"
		},
		OnLocalJSXDone: func(result string, options command.LocalJSXDoneOptions) {
			doneResult = result
			doneDisplay = options.Display
		},
	})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /skills load model handled, handled=%t model=%T", handled, model)
	}
	view := model.View()
	if !strings.Contains(view, "Project skills") || !strings.Contains(view, "Plugin skills") {
		t.Fatalf("unexpected /skills model view: %q", view)
	}
	if !strings.Contains(view, "Refactor Module") || !strings.Contains(view, "hidden=true") {
		t.Fatalf("expected grouped skill metadata in view: %q", view)
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from skills model")
	}
	if doneResult != "Skills dialog dismissed" {
		t.Fatalf("unexpected skills onDone result: %q", doneResult)
	}
	if doneDisplay != "system" {
		t.Fatalf("expected system display for skills onDone, got %q", doneDisplay)
	}
}

func TestDynamicSkillAndPluginAliasesResolveInRegistry(t *testing.T) {
	registry := command.EmptyRegistry()
	cmdskills.Register(registry)
	for _, cmd := range []command.LocalCommand{
		{
			CommandBase: command.CommandBase{
				Name:        "refactor",
				Aliases:     []string{"rf"},
				Description: "Refactor safely",
				Source:      "builtin",
			},
			Handler: func(_ context.Context, _ command.Runtime, _ []string) (command.CommandResult, error) {
				return command.CommandResult{Type: command.ResultTypeText, Value: "skill prompt"}, nil
			},
		},
		{
			CommandBase: command.CommandBase{
				Name:        "demo-plugin:ship",
				Aliases:     []string{"deploy"},
				Description: "Ship release",
				Source:      "builtin",
			},
			Handler: func(_ context.Context, _ command.Runtime, _ []string) (command.CommandResult, error) {
				return command.CommandResult{Type: command.ResultTypeText, Value: "plugin prompt"}, nil
			},
		},
	} {
		registry.Register(cmd)
	}

	out, ok, err := registry.Execute(context.Background(), "/rf", command.Runtime{})
	if err != nil || !ok || out.Value != "skill prompt" {
		t.Fatalf("skill alias failed: ok=%v err=%v out=%q", ok, err, out.Value)
	}
	out, ok, err = registry.Execute(context.Background(), "/deploy", command.Runtime{})
	if err != nil || !ok || out.Value != "plugin prompt" {
		t.Fatalf("plugin alias failed: ok=%v err=%v out=%q", ok, err, out.Value)
	}
}
