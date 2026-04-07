package tests

import (
	mcptool "claude-code-go/internal/tool/mcp"

	"claude-code-go/internal/tool/file"

	"context"
	"strings"
	"testing"

	"claude-code-go/internal/command"
	cmdintegration "claude-code-go/internal/command/integration"
	"claude-code-go/internal/tool"
)

func TestPluginRegistryCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdintegration.Register(registry)

	plugins := []command.PluginInfo{
		{
			Name:        "demo-plugin",
			Source:      "local",
			Status:      "configured",
			SourceType:  "directory",
			Description: "demo plugin",
			Enabled:     true,
			Category:    "utility",
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
		SetPluginsEnabledAll: func(enabled bool) {},
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
		SetHooksEnabledAll: func(enabled bool) {},
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
