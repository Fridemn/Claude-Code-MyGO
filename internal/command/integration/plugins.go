package integration

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/command"
)

func registerPluginsCommands(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugins-mode",
		Description:  "set plugins service enabled state",
		ArgumentHint: "<on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugins-mode", "<on|off>"))
			}
			if runtime.SetPluginsEnabledAll == nil {
				return "", fmt.Errorf("plugins service mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[0])
			if err != nil {
				return "", err
			}
			runtime.SetPluginsEnabledAll(enabled)
			return fmt.Sprintf("plugins service updated\nenabled=%t", enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugins-service-status",
		Description:  "set plugins service status",
		ArgumentHint: "<status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugins-service-status", "<status>"))
			}
			if runtime.SetPluginsServiceStatus == nil {
				return "", fmt.Errorf("plugins service mutation is not configured")
			}
			status := strings.Join(args, " ")
			runtime.SetPluginsServiceStatus(status)
			return fmt.Sprintf("plugins service updated\nstatus=%s", status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugins-summary",
		Description: "show plugins registry summary",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			plugins := []command.PluginInfo(nil)
			if runtime.PluginList != nil {
				plugins = runtime.PluginList()
			}
			return strings.Join([]string{
				"summary:",
				"registry=plugins",
				fmt.Sprintf("entries=%d", len(plugins)),
				fmt.Sprintf("enabled_entries=%d", countEnabledPlugins(plugins)),
				fmt.Sprintf("disabled_entries=%d", len(plugins)-countEnabledPlugins(plugins)),
				fmt.Sprintf("dev_entries=%d", countDevPlugins(plugins)),
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugins-validate",
		Description: "validate plugin registry entries",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			plugins := []command.PluginInfo(nil)
			if runtime.PluginList != nil {
				plugins = runtime.PluginList()
			}
			return validatePlugins(plugins), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugins-json",
		Description: "show normalized plugins registry JSON",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			plugins := []command.PluginInfo(nil)
			if runtime.PluginList != nil {
				plugins = runtime.PluginList()
			}
			return marshalPanelJSON(map[string]any{
				"registry": "plugins",
				"plugins":  plugins,
			})
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugins-service-show",
		Description: "show plugins service metadata",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.PluginStatus == nil {
				return "", fmt.Errorf("plugins service is not configured")
			}
			return runtime.PluginStatus(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugin-fields",
		Description: "show editable plugin fields",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"fields:",
				"- source",
				"- status",
				"- version",
				"- description",
				"- enabled",
				"- marketplace",
				"- category",
				"- path",
				"- dev",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "plugin-template",
		Description: "show a minimal plugin template",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"template:",
				"name=my-plugin",
				"source=local",
				"status=configured",
				"enabled=true",
				"category=workspace",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "plugins",
		Description: "show plugin subsystem status",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return renderPluginsOverview(runtime), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reload-plugins",
		Description: "reload plugin registry and dynamic plugin components",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ReloadPlugins != nil {
				return runtime.ReloadPlugins(), nil
			}
			return "plugin registry reloaded", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reset-plugins",
		Description: "reset plugin registry to defaults",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ResetPlugins == nil {
				return "", fmt.Errorf("plugin reset is not configured")
			}
			return runtime.ResetPlugins(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-enable",
		Description:  "set plugin enabled state",
		ArgumentHint: "<name> <on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-enable", "<name> <on|off>"))
			}
			if runtime.SetPluginEnabled == nil {
				return "", fmt.Errorf("plugin mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[1])
			if err != nil {
				return "", err
			}
			if !runtime.SetPluginEnabled(args[0], enabled) {
				return "", fmt.Errorf("plugin not found: %s", args[0])
			}
			return fmt.Sprintf("plugin updated\nname=%s\nenabled=%t", args[0], enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-status",
		Description:  "set plugin status",
		ArgumentHint: "<name> <status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-status", "<name> <status>"))
			}
			if runtime.SetPluginStatus == nil {
				return "", fmt.Errorf("plugin mutation is not configured")
			}
			status := strings.Join(args[1:], " ")
			if !runtime.SetPluginStatus(args[0], status) {
				return "", fmt.Errorf("plugin not found: %s", args[0])
			}
			return fmt.Sprintf("plugin updated\nname=%s\nstatus=%s", args[0], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-add",
		Description:  "add or replace a plugin entry",
		ArgumentHint: "<name> <source> [status]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-add", "<name> <source> [status]"))
			}
			if runtime.AddPlugin == nil {
				return "", fmt.Errorf("plugin registry mutation is not configured")
			}
			status := "configured"
			if len(args) > 2 {
				status = strings.Join(args[2:], " ")
			}
			runtime.AddPlugin(command.PluginInfo{
				Name:    args[0],
				Source:  args[1],
				Status:  status,
				Enabled: true,
			})
			return fmt.Sprintf("plugin saved\nname=%s\nsource=%s\nstatus=%s", args[0], args[1], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-show",
		Description:  "show a single plugin entry",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-show", "<name>"))
			}
			if runtime.PluginList == nil {
				return "", fmt.Errorf("plugin registry is not configured")
			}
			plugin, ok := findPlugin(runtime.PluginList(), args[0])
			if !ok {
				return "", fmt.Errorf("plugin not found: %s", args[0])
			}
			return renderPlugin(plugin), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-set",
		Description:  "set a field on a plugin entry",
		ArgumentHint: "<name> <field> <value>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 3 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-set", "<name> <field> <value>"))
			}
			if runtime.PluginList == nil || runtime.AddPlugin == nil {
				return "", fmt.Errorf("plugin registry mutation is not configured")
			}
			plugin, ok := findPlugin(runtime.PluginList(), args[0])
			if !ok {
				return "", fmt.Errorf("plugin not found: %s", args[0])
			}
			updated, err := applyPluginField(plugin, args[1], strings.Join(args[2:], " "))
			if err != nil {
				return "", err
			}
			runtime.AddPlugin(updated)
			return renderPlugin(updated), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "plugin-remove",
		Description:  "remove a plugin entry",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("plugin-remove", "<name>"))
			}
			if runtime.RemovePlugin == nil {
				return "", fmt.Errorf("plugin registry mutation is not configured")
			}
			if !runtime.RemovePlugin(args[0]) {
				return "", fmt.Errorf("plugin not found: %s", args[0])
			}
			return fmt.Sprintf("plugin removed\nname=%s", args[0]), nil
		},
	})
}

// Plugin helper functions

func renderPluginsOverview(runtime command.Runtime) string {
	lines := []string{
		"overview:",
		"registry=plugins",
	}
	plugins := []command.PluginInfo(nil)
	if runtime.PluginList != nil {
		plugins = runtime.PluginList()
		lines = append(lines,
			fmt.Sprintf("entries=%d", len(plugins)),
			fmt.Sprintf("enabled_entries=%d", countEnabledPlugins(plugins)),
			fmt.Sprintf("disabled_entries=%d", len(plugins)-countEnabledPlugins(plugins)),
			fmt.Sprintf("dev_entries=%d", countDevPlugins(plugins)),
		)
		sourceCounts := summarizePluginSources(plugins)
		lines = append(lines,
			fmt.Sprintf("builtin_entries=%d", sourceCounts["builtin"]),
			fmt.Sprintf("directory_entries=%d", sourceCounts["directory"]),
			fmt.Sprintf("marketplace_entries=%d", sourceCounts["marketplace"]),
			fmt.Sprintf("registry_entries=%d", sourceCounts["registry"]),
		)
	}
	lines = append(lines, "mutable_config=true", "")
	if runtime.PluginStatus != nil {
		lines = append(lines, runtime.PluginStatus())
	} else {
		lines = append(lines, "plugins=not_yet_ported")
	}
	if len(plugins) > 0 {
		lines = append(lines, "", "entries:")
		lines = appendPluginSections(lines, plugins)
	}
	lines = append(lines,
		"",
		"actions:",
		"- /plugins-mode <on|off>",
		"- /plugins-summary",
		"- /plugins-validate",
		"- /plugins-json",
		"- /plugins-service-show",
		"- /plugins-service-status <status>",
		"- /plugin-fields",
		"- /plugin-template",
		"- /plugin-add <name> <source> [status]",
		"- /plugin-show <name>",
		"- /plugin-set <name> <field> <value>",
		"- /plugin-enable <name> <on|off>",
		"- /plugin-status <name> <status>",
		"- /plugin-remove <name>",
		"- /reset-plugins",
		"- /reload-plugins",
	)
	return strings.Join(lines, "\n")
}

func findPlugin(plugins []command.PluginInfo, name string) (command.PluginInfo, bool) {
	for _, plugin := range plugins {
		if plugin.Name == name {
			return plugin, true
		}
	}
	return command.PluginInfo{}, false
}

func renderPlugin(plugin command.PluginInfo) string {
	lines := []string{
		"name=" + plugin.Name,
		"source=" + plugin.Source,
		"status=" + plugin.Status,
		fmt.Sprintf("enabled=%t", plugin.Enabled),
	}
	if strings.TrimSpace(plugin.SourceType) != "" {
		lines = append(lines, "source_type="+plugin.SourceType)
	}
	if strings.TrimSpace(plugin.Version) != "" {
		lines = append(lines, "version="+plugin.Version)
	}
	if strings.TrimSpace(plugin.Description) != "" {
		lines = append(lines, "description="+plugin.Description)
	}
	if strings.TrimSpace(plugin.Marketplace) != "" {
		lines = append(lines, "marketplace="+plugin.Marketplace)
	}
	if strings.TrimSpace(plugin.Category) != "" {
		lines = append(lines, "category="+plugin.Category)
	}
	if strings.TrimSpace(plugin.Path) != "" {
		lines = append(lines, "path="+plugin.Path)
	}
	if plugin.CommandCount > 0 || plugin.AgentCount > 0 || plugin.SkillCount > 0 || plugin.HookCount > 0 {
		lines = append(lines,
			fmt.Sprintf("command_count=%d", plugin.CommandCount),
			fmt.Sprintf("agent_count=%d", plugin.AgentCount),
			fmt.Sprintf("skill_count=%d", plugin.SkillCount),
			fmt.Sprintf("hook_count=%d", plugin.HookCount),
		)
	}
	if plugin.Dev {
		lines = append(lines, "dev=true")
	}
	return strings.Join(lines, "\n")
}

func appendPluginSections(lines []string, plugins []command.PluginInfo) []string {
	order := []string{"builtin", "directory", "marketplace", "registry", "other"}
	grouped := map[string][]command.PluginInfo{}
	for _, plugin := range plugins {
		key := strings.TrimSpace(plugin.SourceType)
		if key == "" {
			key = "registry"
		}
		switch key {
		case "builtin", "directory", "marketplace", "registry":
		default:
			key = "other"
		}
		grouped[key] = append(grouped[key], plugin)
	}
	for _, group := range order {
		items := grouped[group]
		if len(items) == 0 {
			continue
		}
		lines = append(lines, group+":")
		for _, plugin := range items {
			line := fmt.Sprintf("- %s [%s] %s", plugin.Name, plugin.Source, plugin.Status)
			if strings.TrimSpace(plugin.Version) != "" {
				line += " version=" + plugin.Version
			}
			if plugin.Enabled {
				line += " enabled=true"
			}
			lines = append(lines, line)
			meta := []string{"source_type=" + group}
			if strings.TrimSpace(plugin.Category) != "" {
				meta = append(meta, "category="+plugin.Category)
			}
			if strings.TrimSpace(plugin.Marketplace) != "" {
				meta = append(meta, "marketplace="+plugin.Marketplace)
			}
			if plugin.Dev {
				meta = append(meta, "dev=true")
			}
			if strings.TrimSpace(plugin.Path) != "" {
				meta = append(meta, "path="+plugin.Path)
			}
			if plugin.CommandCount > 0 || plugin.AgentCount > 0 || plugin.SkillCount > 0 || plugin.HookCount > 0 {
				meta = append(meta,
					fmt.Sprintf("commands=%d", plugin.CommandCount),
					fmt.Sprintf("agents=%d", plugin.AgentCount),
					fmt.Sprintf("skills=%d", plugin.SkillCount),
					fmt.Sprintf("hooks=%d", plugin.HookCount),
				)
			}
			lines = append(lines, "  "+strings.Join(meta, "  "))
			if strings.TrimSpace(plugin.Description) != "" {
				lines = append(lines, "  "+plugin.Description)
			}
		}
	}
	return lines
}

func summarizePluginSources(plugins []command.PluginInfo) map[string]int {
	out := map[string]int{
		"builtin":     0,
		"directory":   0,
		"marketplace": 0,
		"registry":    0,
	}
	for _, plugin := range plugins {
		key := strings.TrimSpace(plugin.SourceType)
		if key == "" {
			key = "registry"
		}
		if _, ok := out[key]; !ok {
			key = "registry"
		}
		out[key]++
	}
	return out
}

func countEnabledPlugins(plugins []command.PluginInfo) int {
	count := 0
	for _, plugin := range plugins {
		if plugin.Enabled {
			count++
		}
	}
	return count
}

func countDevPlugins(plugins []command.PluginInfo) int {
	count := 0
	for _, plugin := range plugins {
		if plugin.Dev {
			count++
		}
	}
	return count
}

func validatePlugins(plugins []command.PluginInfo) string {
	lines := []string{
		"validation:",
		"registry=plugins",
	}
	seen := map[string]int{}
	issues := 0
	for _, plugin := range plugins {
		seen[plugin.Name]++
	}
	for _, plugin := range plugins {
		prefix := "- " + plugin.Name + ": "
		switch {
		case strings.TrimSpace(plugin.Name) == "":
			lines = append(lines, prefix+"missing name")
			issues++
		case seen[plugin.Name] > 1:
			lines = append(lines, prefix+"duplicate name")
			issues++
		case strings.TrimSpace(plugin.Source) == "":
			lines = append(lines, prefix+"missing source")
			issues++
		case strings.TrimSpace(plugin.Status) == "":
			lines = append(lines, prefix+"missing status")
			issues++
		}
	}
	if issues == 0 {
		lines = append(lines, "result=ok", "issues=0")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "result=issues", fmt.Sprintf("issues=%d", issues))
	return strings.Join(lines, "\n")
}

func applyPluginField(plugin command.PluginInfo, field, value string) (command.PluginInfo, error) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "source":
		plugin.Source = value
	case "status":
		plugin.Status = value
	case "version":
		plugin.Version = value
	case "description":
		plugin.Description = value
	case "enabled":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return plugin, err
		}
		plugin.Enabled = enabled
	case "marketplace":
		plugin.Marketplace = value
	case "category":
		plugin.Category = value
	case "path":
		plugin.Path = value
	case "dev":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return plugin, err
		}
		plugin.Dev = enabled
	default:
		return plugin, fmt.Errorf("unsupported plugin field: %s", field)
	}
	return plugin, nil
}