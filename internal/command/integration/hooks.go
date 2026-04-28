package integration

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"claude-go/internal/command"
)

func registerHooksCommands(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "hooks",
		Description: "show current hook system status",
		Load:        loadHooksModel,
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return renderHooksOverview(runtime), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hooks-mode",
		Description:  "set hooks service enabled state",
		ArgumentHint: "<on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hooks-mode", "<on|off>"))
			}
			if runtime.SetHooksEnabledAll == nil {
				return "", fmt.Errorf("hooks service mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[0])
			if err != nil {
				return "", err
			}
			runtime.SetHooksEnabledAll(enabled)
			return fmt.Sprintf("hooks service updated\nenabled=%t", enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hooks-service-status",
		Description:  "set hooks service status",
		ArgumentHint: "<status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hooks-service-status", "<status>"))
			}
			if runtime.SetHooksServiceStatus == nil {
				return "", fmt.Errorf("hooks service mutation is not configured")
			}
			status := strings.Join(args, " ")
			runtime.SetHooksServiceStatus(status)
			return fmt.Sprintf("hooks service updated\nstatus=%s", status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hooks-summary",
		Description: "show hooks registry summary",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			hooks := []command.HookInfo(nil)
			if runtime.HookList != nil {
				hooks = runtime.HookList()
			}
			return strings.Join([]string{
				"summary:",
				"registry=hooks",
				fmt.Sprintf("entries=%d", len(hooks)),
				fmt.Sprintf("enabled_entries=%d", countEnabledHooks(hooks)),
				fmt.Sprintf("disabled_entries=%d", len(hooks)-countEnabledHooks(hooks)),
				fmt.Sprintf("blocking_entries=%d", countBlockingHooks(hooks)),
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hooks-validate",
		Description: "validate hook registry entries",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			hooks := []command.HookInfo(nil)
			if runtime.HookList != nil {
				hooks = runtime.HookList()
			}
			return validateHooks(hooks), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hooks-json",
		Description: "show normalized hooks registry JSON",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			hooks := []command.HookInfo(nil)
			if runtime.HookList != nil {
				hooks = runtime.HookList()
			}
			return marshalPanelJSON(map[string]any{
				"registry": "hooks",
				"hooks":    hooks,
			})
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hooks-service-show",
		Description: "show hooks service metadata",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.HookStatus == nil {
				return "", fmt.Errorf("hooks service is not configured")
			}
			return runtime.HookStatus(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hook-fields",
		Description: "show editable hook fields",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"fields:",
				"- source",
				"- status",
				"- command",
				"- description",
				"- enabled",
				"- matcher",
				"- timeout_ms",
				"- blocking",
				"- shell",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hook-template",
		Description: "show a minimal hook template",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"template:",
				"event=pre_tool",
				"source=local",
				"status=configured",
				"command=echo pre_tool",
				"enabled=true",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reload-hooks",
		Description: "reload hook registry from config",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ReloadHooks != nil {
				return runtime.ReloadHooks(), nil
			}
			return "hook registry reloaded (placeholder)", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reset-hooks",
		Description: "reset hook registry to defaults",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ResetHooks == nil {
				return "", fmt.Errorf("hook reset is not configured")
			}
			return runtime.ResetHooks(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-enable",
		Description:  "set hook enabled state",
		ArgumentHint: "<event> <on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-enable", "<event> <on|off>"))
			}
			if runtime.SetHookEnabled == nil {
				return "", fmt.Errorf("hook mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[1])
			if err != nil {
				return "", err
			}
			if !runtime.SetHookEnabled(args[0], enabled) {
				return "", fmt.Errorf("hook not found: %s", args[0])
			}
			return fmt.Sprintf("hook updated\nevent=%s\nenabled=%t", args[0], enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-status",
		Description:  "set hook status",
		ArgumentHint: "<event> <status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-status", "<event> <status>"))
			}
			if runtime.SetHookStatus == nil {
				return "", fmt.Errorf("hook mutation is not configured")
			}
			status := strings.Join(args[1:], " ")
			if !runtime.SetHookStatus(args[0], status) {
				return "", fmt.Errorf("hook not found: %s", args[0])
			}
			return fmt.Sprintf("hook updated\nevent=%s\nstatus=%s", args[0], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-add",
		Description:  "add or replace a hook entry",
		ArgumentHint: "<event> <command> [status]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-add", "<event> <command> [status]"))
			}
			if runtime.AddHook == nil {
				return "", fmt.Errorf("hook registry mutation is not configured")
			}
			status := "configured"
			if len(args) > 2 {
				status = strings.Join(args[2:], " ")
			}
			runtime.AddHook(command.HookInfo{
				Event:     args[0],
				Command:   args[1],
				Status:    status,
				Source:    "local",
				Enabled:   true,
				Matcher:   "*",
				TimeoutMs: 1000,
				Shell:     "bash",
			})
			return fmt.Sprintf("hook saved\nevent=%s\ncommand=%s\nstatus=%s", args[0], args[1], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-show",
		Description:  "show a single hook entry",
		ArgumentHint: "<event>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-show", "<event>"))
			}
			if runtime.HookList == nil {
				return "", fmt.Errorf("hook registry is not configured")
			}
			hook, ok := findHook(runtime.HookList(), args[0])
			if !ok {
				return "", fmt.Errorf("hook not found: %s", args[0])
			}
			return renderHook(hook), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-set",
		Description:  "set a field on a hook entry",
		ArgumentHint: "<event> <field> <value>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 3 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-set", "<event> <field> <value>"))
			}
			if runtime.HookList == nil || runtime.AddHook == nil {
				return "", fmt.Errorf("hook registry mutation is not configured")
			}
			hook, ok := findHook(runtime.HookList(), args[0])
			if !ok {
				return "", fmt.Errorf("hook not found: %s", args[0])
			}
			updated, err := applyHookField(hook, args[1], strings.Join(args[2:], " "))
			if err != nil {
				return "", err
			}
			runtime.AddHook(updated)
			return renderHook(updated), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "hook-remove",
		Description:  "remove a hook entry",
		ArgumentHint: "<event>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("hook-remove", "<event>"))
			}
			if runtime.RemoveHook == nil {
				return "", fmt.Errorf("hook registry mutation is not configured")
			}
			if !runtime.RemoveHook(args[0]) {
				return "", fmt.Errorf("hook not found: %s", args[0])
			}
			return fmt.Sprintf("hook removed\nevent=%s", args[0]), nil
		},
	})
}

// Hook helper functions

func renderHooksOverview(runtime command.Runtime) string {
	lines := []string{
		"overview:",
		"registry=hooks",
	}
	hooks := []command.HookInfo(nil)
	if runtime.HookList != nil {
		hooks = runtime.HookList()
		lines = append(lines,
			fmt.Sprintf("entries=%d", len(hooks)),
			fmt.Sprintf("enabled_entries=%d", countEnabledHooks(hooks)),
			fmt.Sprintf("disabled_entries=%d", len(hooks)-countEnabledHooks(hooks)),
		)
	}
	lines = append(lines, "mutable_config=true", "")
	if runtime.HookStatus != nil {
		lines = append(lines, runtime.HookStatus())
	} else {
		lines = append(lines, "hooks=not_yet_ported")
	}
	if len(hooks) > 0 {
		lines = append(lines, "", "entries:")
		for _, entry := range hooks {
			line := fmt.Sprintf("- %s [%s] %s", entry.Event, entry.Source, entry.Status)
			if entry.Enabled {
				line += " enabled=true"
			}
			lines = append(lines, line)
			if strings.TrimSpace(entry.Command) != "" {
				lines = append(lines, "  command="+entry.Command)
			}
			meta := []string{}
			if strings.TrimSpace(entry.Matcher) != "" {
				meta = append(meta, "matcher="+entry.Matcher)
			}
			if entry.TimeoutMs > 0 {
				meta = append(meta, fmt.Sprintf("timeout_ms=%d", entry.TimeoutMs))
			}
			if entry.Blocking {
				meta = append(meta, "blocking=true")
			}
			if strings.TrimSpace(entry.Shell) != "" {
				meta = append(meta, "shell="+entry.Shell)
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+strings.Join(meta, "  "))
			}
			runtimeMeta := []string{}
			if entry.RunCount > 0 {
				runtimeMeta = append(runtimeMeta, fmt.Sprintf("run_count=%d", entry.RunCount))
			}
			if strings.TrimSpace(entry.LastRunAt) != "" {
				runtimeMeta = append(runtimeMeta, "last_run_at="+entry.LastRunAt)
			}
			if strings.TrimSpace(entry.LastResult) != "" {
				runtimeMeta = append(runtimeMeta, "last_result="+entry.LastResult)
			}
			if len(runtimeMeta) > 0 {
				lines = append(lines, "  "+strings.Join(runtimeMeta, "  "))
			}
			if strings.TrimSpace(entry.LastError) != "" {
				lines = append(lines, "  last_error="+entry.LastError)
			}
			if strings.TrimSpace(entry.LastOutput) != "" {
				lines = append(lines, "  last_output="+entry.LastOutput)
			}
			if strings.TrimSpace(entry.Description) != "" {
				lines = append(lines, "  "+entry.Description)
			}
		}
	}
	if runtime.State != nil {
		state := runtime.State.Snapshot()
		lines = append(lines, fmt.Sprintf("session=%s", command.EmptyDash(state.SessionID)))
	}
	lines = append(lines,
		"",
		"actions:",
		"- /hooks-mode <on|off>",
		"- /hooks-summary",
		"- /hooks-validate",
		"- /hooks-json",
		"- /hooks-service-show",
		"- /hooks-service-status <status>",
		"- /hook-fields",
		"- /hook-template",
		"- /hook-add <event> <command> [status]",
		"- /hook-show <event>",
		"- /hook-set <event> <field> <value>",
		"- /hook-enable <event> <on|off>",
		"- /hook-status <event> <status>",
		"- /hook-remove <event>",
		"- /reset-hooks",
		"- /reload-hooks",
	)
	return strings.Join(lines, "\n")
}

func findHook(hooks []command.HookInfo, event string) (command.HookInfo, bool) {
	for _, hook := range hooks {
		if hook.Event == event {
			return hook, true
		}
	}
	return command.HookInfo{}, false
}

func renderHook(hook command.HookInfo) string {
	lines := []string{
		"event=" + hook.Event,
		"source=" + hook.Source,
		"status=" + hook.Status,
		fmt.Sprintf("enabled=%t", hook.Enabled),
	}
	if strings.TrimSpace(hook.Command) != "" {
		lines = append(lines, "command="+hook.Command)
	}
	if strings.TrimSpace(hook.Description) != "" {
		lines = append(lines, "description="+hook.Description)
	}
	if strings.TrimSpace(hook.Matcher) != "" {
		lines = append(lines, "matcher="+hook.Matcher)
	}
	if hook.TimeoutMs > 0 {
		lines = append(lines, fmt.Sprintf("timeout_ms=%d", hook.TimeoutMs))
	}
	if hook.Blocking {
		lines = append(lines, "blocking=true")
	}
	if strings.TrimSpace(hook.Shell) != "" {
		lines = append(lines, "shell="+hook.Shell)
	}
	if hook.RunCount > 0 {
		lines = append(lines, fmt.Sprintf("run_count=%d", hook.RunCount))
	}
	if strings.TrimSpace(hook.LastRunAt) != "" {
		lines = append(lines, "last_run_at="+hook.LastRunAt)
	}
	if strings.TrimSpace(hook.LastResult) != "" {
		lines = append(lines, "last_result="+hook.LastResult)
	}
	if strings.TrimSpace(hook.LastError) != "" {
		lines = append(lines, "last_error="+hook.LastError)
	}
	if strings.TrimSpace(hook.LastOutput) != "" {
		lines = append(lines, "last_output="+hook.LastOutput)
	}
	return strings.Join(lines, "\n")
}

func applyHookField(hook command.HookInfo, field, value string) (command.HookInfo, error) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "source":
		hook.Source = value
	case "status":
		hook.Status = value
	case "command":
		hook.Command = value
	case "description":
		hook.Description = value
	case "enabled":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return hook, err
		}
		hook.Enabled = enabled
	case "matcher":
		hook.Matcher = value
	case "timeout_ms":
		timeout, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return hook, fmt.Errorf("invalid timeout_ms: %s", value)
		}
		hook.TimeoutMs = timeout
	case "blocking":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return hook, err
		}
		hook.Blocking = enabled
	case "shell":
		hook.Shell = value
	default:
		return hook, fmt.Errorf("unsupported hook field: %s", field)
	}
	return hook, nil
}

func countEnabledHooks(hooks []command.HookInfo) int {
	count := 0
	for _, hook := range hooks {
		if hook.Enabled {
			count++
		}
	}
	return count
}

func countBlockingHooks(hooks []command.HookInfo) int {
	count := 0
	for _, hook := range hooks {
		if hook.Blocking {
			count++
		}
	}
	return count
}

func validateHooks(hooks []command.HookInfo) string {
	lines := []string{
		"validation:",
		"registry=hooks",
	}
	seen := map[string]int{}
	issues := 0
	for _, hook := range hooks {
		seen[hook.Event]++
	}
	for _, hook := range hooks {
		prefix := "- " + hook.Event + ": "
		switch {
		case strings.TrimSpace(hook.Event) == "":
			lines = append(lines, prefix+"missing event")
			issues++
		case seen[hook.Event] > 1:
			lines = append(lines, prefix+"duplicate event")
			issues++
		case strings.TrimSpace(hook.Status) == "":
			lines = append(lines, prefix+"missing status")
			issues++
		case strings.TrimSpace(hook.Command) == "":
			lines = append(lines, prefix+"missing command")
			issues++
		case hook.TimeoutMs < 0:
			lines = append(lines, prefix+"negative timeout")
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
