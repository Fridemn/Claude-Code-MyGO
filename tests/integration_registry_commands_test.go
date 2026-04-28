package tests

import (
	mcptool "claude-go/internal/tool/mcp"

	"claude-go/internal/tool/file"

	"context"
	"strings"
	"testing"

	"claude-go/internal/command"
	cmdintegration "claude-go/internal/command/integration"
	"claude-go/internal/tool"
	bashtool "claude-go/internal/tool/bash"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPluginRegistryCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	plugins := []command.PluginInfo{
		{
			Name:         "demo-plugin",
			Source:       "local",
			Status:       "configured",
			SourceType:   "directory",
			Description:  "demo plugin",
			Enabled:      true,
			Category:     "utility",
			CommandCount: 1,
		},
		{
			Name:       "builtin-plugin",
			Source:     "builtin",
			Status:     "loaded",
			SourceType: "builtin",
			Enabled:    true,
			SkillCount: 1,
		},
		{
			Name:        "market-plugin",
			Source:      "remote",
			Status:      "installed",
			SourceType:  "marketplace",
			Enabled:     false,
			Marketplace: "official",
		},
	}
	reloadCalled := false
	resetCalled := false
	runtime := command.Runtime{
		PluginStatus: func() string { return "plugins=configured" },
		PluginList: func() []command.PluginInfo {
			return append([]command.PluginInfo(nil), plugins...)
		},
		ReloadPlugins: func() string {
			reloadCalled = true
			return "plugin registry reloaded"
		},
		ResetPlugins: func() string {
			resetCalled = true
			return "plugin registry reset"
		},
		SetPluginsEnabledAll:    func(enabled bool) {},
		SetPluginsServiceStatus: func(status string) {},
		SetPluginEnabled: func(name string, enabled bool) bool {
			for i := range plugins {
				if plugins[i].Name == name {
					plugins[i].Enabled = enabled
					return true
				}
			}
			return false
		},
		SetPluginStatus: func(name, status string) bool {
			for i := range plugins {
				if plugins[i].Name == name {
					plugins[i].Status = status
					return true
				}
			}
			return false
		},
		AddPlugin: func(plugin command.PluginInfo) {
			for i := range plugins {
				if plugins[i].Name == plugin.Name {
					plugins[i] = plugin
					return
				}
			}
			plugins = append(plugins, plugin)
		},
		RemovePlugin: func(name string) bool {
			for i := range plugins {
				if plugins[i].Name == name {
					plugins = append(plugins[:i], plugins[i+1:]...)
					return true
				}
			}
			return false
		},
	}

	checkCommandContains(t, registry, runtime, "/plugins", "registry=plugins")
	checkCommandContains(t, registry, runtime, "/plugins", "builtin_entries=1")
	checkCommandContains(t, registry, runtime, "/plugins", "directory_entries=1")
	checkCommandContains(t, registry, runtime, "/plugins", "marketplace_entries=1")
	checkCommandContains(t, registry, runtime, "/plugins", "builtin:")
	checkCommandContains(t, registry, runtime, "/plugins", "directory:")
	checkCommandContains(t, registry, runtime, "/plugins", "marketplace:")
	checkCommandContains(t, registry, runtime, "/plugins", "commands=1")
	checkCommandContains(t, registry, runtime, "/plugins-summary", "entries=3")
	checkCommandContains(t, registry, runtime, "/plugins-validate", "validation:")
	checkCommandContains(t, registry, runtime, "/plugins-json", `"registry": "plugins"`)
	checkCommandContains(t, registry, runtime, "/plugins-service-show", "plugins=configured")
	checkCommandContains(t, registry, runtime, "/plugin-fields", "marketplace")
	checkCommandContains(t, registry, runtime, "/plugin-template", "name=my-plugin")
	checkCommandContains(t, registry, runtime, "/plugin-show demo-plugin", "name=demo-plugin")
	checkCommandContains(t, registry, runtime, "/plugin-set demo-plugin version 1.2.3", "version=1.2.3")
	checkCommandContains(t, registry, runtime, "/plugin-enable demo-plugin off", "enabled=false")
	checkCommandContains(t, registry, runtime, "/plugin-status demo-plugin loaded", "status=loaded")
	checkCommandContains(t, registry, runtime, "/plugin-add extra-plugin remote active", "name=extra-plugin")
	checkCommandContains(t, registry, runtime, "/plugin-remove extra-plugin", "plugin removed")
	checkCommandContains(t, registry, runtime, "/reload-plugins", "plugin registry reloaded")
	checkCommandContains(t, registry, runtime, "/reset-plugins", "plugin registry reset")

	if !reloadCalled || !resetCalled {
		t.Fatalf("expected reload and reset plugin callbacks to run")
	}
}

func TestHookRegistryCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	hooks := []command.HookInfo{
		{
			Event:       "pre_tool",
			Source:      "local",
			Status:      "configured",
			Command:     "echo pre_tool",
			Description: "demo hook",
			Enabled:     true,
			Matcher:     "*",
			TimeoutMs:   1000,
			Shell:       "bash",
		},
	}
	reloadCalled := false
	resetCalled := false
	runtime := command.Runtime{
		HookStatus: func() string { return "hooks=configured" },
		HookList: func() []command.HookInfo {
			return append([]command.HookInfo(nil), hooks...)
		},
		ReloadHooks: func() string {
			reloadCalled = true
			return "hook registry reloaded"
		},
		ResetHooks: func() string {
			resetCalled = true
			return "hook registry reset"
		},
		SetHooksEnabledAll:    func(enabled bool) {},
		SetHooksServiceStatus: func(status string) {},
		SetHookEnabled: func(event string, enabled bool) bool {
			for i := range hooks {
				if hooks[i].Event == event {
					hooks[i].Enabled = enabled
					return true
				}
			}
			return false
		},
		SetHookStatus: func(event, status string) bool {
			for i := range hooks {
				if hooks[i].Event == event {
					hooks[i].Status = status
					return true
				}
			}
			return false
		},
		AddHook: func(h command.HookInfo) {
			for i := range hooks {
				if hooks[i].Event == h.Event {
					hooks[i] = h
					return
				}
			}
			hooks = append(hooks, h)
		},
		RemoveHook: func(event string) bool {
			for i := range hooks {
				if hooks[i].Event == event {
					hooks = append(hooks[:i], hooks[i+1:]...)
					return true
				}
			}
			return false
		},
	}

	checkCommandContains(t, registry, runtime, "/hooks", "registry=hooks")
	checkCommandContains(t, registry, runtime, "/hooks-summary", "entries=1")
	checkCommandContains(t, registry, runtime, "/hooks-validate", "validation:")
	checkCommandContains(t, registry, runtime, "/hooks-json", `"registry": "hooks"`)
	checkCommandContains(t, registry, runtime, "/hooks-service-show", "hooks=configured")
	checkCommandContains(t, registry, runtime, "/hook-fields", "timeout_ms")
	checkCommandContains(t, registry, runtime, "/hook-template", "event=pre_tool")
	checkCommandContains(t, registry, runtime, "/hook-show pre_tool", "event=pre_tool")
	checkCommandContains(t, registry, runtime, "/hook-set pre_tool shell zsh", "shell=zsh")
	checkCommandContains(t, registry, runtime, "/hook-enable pre_tool off", "enabled=false")
	checkCommandContains(t, registry, runtime, "/hook-status pre_tool active", "status=active")
	checkCommandContains(t, registry, runtime, "/hook-add post_tool 'echo post_tool' configured", "event=post_tool")
	checkCommandContains(t, registry, runtime, "/hook-remove post_tool", "hook removed")
	checkCommandContains(t, registry, runtime, "/reload-hooks", "hook registry reloaded")
	checkCommandContains(t, registry, runtime, "/reset-hooks", "hook registry reset")

	if !reloadCalled || !resetCalled {
		t.Fatalf("expected reload and reset hook callbacks to run")
	}
}

func TestThemeCommandLoadModelAppliesThemeAndOnDone(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	var changedTheme string
	var doneResult string
	var doneDisplay string
	runtime := command.Runtime{
		OnThemeChange: func(theme string) {
			changedTheme = theme
		},
		OnLocalJSXDone: func(result string, options command.LocalJSXDoneOptions) {
			doneResult = result
			doneDisplay = options.Display
		},
	}

	model, _, handled, err := registry.LoadModel(context.Background(), "/theme", runtime)
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /theme load model handled, handled=%t model=%T", handled, model)
	}
	if !strings.Contains(model.View(), "Theme") {
		t.Fatalf("expected theme picker view, got %q", model.View())
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected enter to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from theme picker")
	}
	if changedTheme == "" {
		t.Fatal("expected OnThemeChange callback")
	}
	if !strings.HasPrefix(doneResult, "Theme set to ") {
		t.Fatalf("expected onDone success message, got %q", doneResult)
	}
	if doneDisplay != "" {
		t.Fatalf("expected default display for success, got %q", doneDisplay)
	}
}

func TestPermissionsCommandLoadModelSupportsModeSwitching(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	checker := bashtool.GetPermissionChecker()
	origMode := checker.GetMode()
	origRules := checker.RulesSnapshot()
	checker.ClearRules()
	checker.SetMode(bashtool.PermissionModeAsk)
	checker.AddRule(bashtool.RuleFromPattern("git *", bashtool.BehaviorAllow))
	checker.AddRule(bashtool.RuleFromPattern("rm *", bashtool.BehaviorDeny))
	defer func() {
		checker.ClearRules()
		checker.AddRules(origRules)
		checker.SetMode(origMode)
	}()

	model, _, handled, err := registry.LoadModel(context.Background(), "/permissions", command.Runtime{})
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /permissions load model handled, handled=%t model=%T", handled, model)
	}
	view := model.View()
	if !strings.Contains(view, "Permissions") || !strings.Contains(view, "git *") {
		t.Fatalf("unexpected permissions model view: %q", view)
	}

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	model = next
	if checker.GetMode() != bashtool.PermissionModeAcceptEdits {
		t.Fatalf("expected mode switch to acceptEdits, got %s", checker.GetMode())
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from permissions model")
	}
}

func TestMCPCommandLoadModelRendersOverview(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	closed := false
	runtime := command.Runtime{
		MCPStatus: func() string { return "mcp=active" },
		MCPServers: func() []command.MCPServerInfo {
			return []command.MCPServerInfo{
				{
					Name:      "demo",
					Transport: "stdio",
					Status:    "configured",
					Enabled:   true,
					ToolCount: 2,
				},
			}
		},
		OnExit: func() {
			closed = true
		},
	}

	model, _, handled, err := registry.LoadModel(context.Background(), "/mcp", runtime)
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /mcp load model handled, handled=%t model=%T", handled, model)
	}
	view := model.View()
	if !strings.Contains(view, "registry=mcp") || !strings.Contains(view, "demo") {
		t.Fatalf("unexpected mcp model view: %q", view)
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from mcp model")
	}
	if !closed {
		t.Fatal("expected mcp model to trigger OnExit")
	}
}

func TestPluginsCommandLoadModelRendersOverview(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	runtime := command.Runtime{
		PluginStatus: func() string { return "plugins=active" },
		PluginList: func() []command.PluginInfo {
			return []command.PluginInfo{
				{
					Name:       "demo-plugin",
					Source:     "local",
					SourceType: "directory",
					Status:     "configured",
					Enabled:    true,
				},
			}
		},
	}

	model, _, handled, err := registry.LoadModel(context.Background(), "/plugins", runtime)
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /plugins load model handled, handled=%t model=%T", handled, model)
	}
	view := model.View()
	if !strings.Contains(view, "registry=plugins") || !strings.Contains(view, "demo-plugin") {
		t.Fatalf("unexpected plugins model view: %q", view)
	}
}

func TestHooksCommandLoadModelRendersOverview(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	closed := false
	runtime := command.Runtime{
		HookStatus: func() string { return "hooks=active" },
		HookList: func() []command.HookInfo {
			return []command.HookInfo{
				{
					Event:   "pre_tool",
					Command: "echo pre",
					Status:  "configured",
					Source:  "local",
					Enabled: true,
				},
			}
		},
		OnExit: func() {
			closed = true
		},
	}

	model, _, handled, err := registry.LoadModel(context.Background(), "/hooks", runtime)
	if err != nil {
		t.Fatalf("load model failed: %v", err)
	}
	if !handled || model == nil {
		t.Fatalf("expected /hooks load model handled, handled=%t model=%T", handled, model)
	}
	view := model.View()
	if !strings.Contains(view, "registry=hooks") || !strings.Contains(view, "pre_tool") {
		t.Fatalf("unexpected hooks model view: %q", view)
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to emit quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from hooks model")
	}
	if !closed {
		t.Fatal("expected hooks model to trigger OnExit")
	}
}

func TestDynamicMCPDispatchViaLookupDefinition(t *testing.T) {
	t.Parallel()

	runtime := tool.Runtime{
		MCP: stubMCPRuntime{
			dynamic: []tool.MCPDynamicToolInfo{
				{
					Name:        "mcp__demo__workspace_echo",
					Server:      "demo",
					Tool:        "workspace.echo",
					Description: "dynamic echo",
					ReadOnly:    true,
				},
			},
		},
	}

	registry := tool.EmptyRegistry()
	file.RegisterFileTools(registry)

	def, ok := mcptool.LookupDefinition(registry, runtime, "mcp__demo__workspace_echo")
	if !ok {
		t.Fatalf("expected dynamic mcp definition lookup to succeed")
	}
	result, err := def.Call(context.Background(), tool.Input{"value": "hello"}, runtime)
	if err != nil {
		t.Fatalf("dynamic mcp tool call failed: %v", err)
	}
	if result.Content != "dynamic:demo:workspace.echo" {
		t.Fatalf("unexpected dynamic dispatch result: %#v", result.Content)
	}
	if result.Meta["mcp"] != true {
		t.Fatalf("expected mcp metadata, got %#v", result.Meta)
	}
	if !strings.Contains(def.Description(), "dynamic echo") {
		t.Fatalf("unexpected dynamic tool description: %q", def.Description())
	}
}
