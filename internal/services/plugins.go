package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/command"
)

type Plugin struct {
	Name         string   `json:"name"`
	Source       string   `json:"source"`
	Status       string   `json:"status"`
	SourceType   string   `json:"source_type,omitempty"`
	Version      string   `json:"version,omitempty"`
	Description  string   `json:"description,omitempty"`
	Enabled      bool     `json:"enabled"`
	Marketplace  string   `json:"marketplace,omitempty"`
	Category     string   `json:"category,omitempty"`
	Path         string   `json:"path,omitempty"`
	Dev          bool     `json:"dev,omitempty"`
	DefaultEnabled *bool  `json:"default_enabled,omitempty"`
	ManifestPath string   `json:"manifest_path,omitempty"`
	CommandsPath string   `json:"commands_path,omitempty"`
	Commands     []string `json:"commands,omitempty"`
	CommandMetadata map[string]PluginCommandMetadata `json:"command_metadata,omitempty"`
	AgentsPath   string   `json:"agents_path,omitempty"`
	Agents       []string `json:"agents,omitempty"`
	SkillsPath   string   `json:"skills_path,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	HooksPath    string   `json:"hooks_path,omitempty"`
	CommandCount int      `json:"command_count,omitempty"`
	AgentCount   int      `json:"agent_count,omitempty"`
	SkillCount   int      `json:"skill_count,omitempty"`
	HookCount    int      `json:"hook_count,omitempty"`
}

type PluginCommandMetadata struct {
	Description            string   `json:"description,omitempty"`
	DisplayName            string   `json:"display_name,omitempty"`
	ArgumentHint           string   `json:"argument_hint,omitempty"`
	WhenToUse              string   `json:"when_to_use,omitempty"`
	AllowedTools           []string `json:"allowed_tools,omitempty"`
	Model                  string   `json:"model,omitempty"`
	Aliases                []string `json:"aliases,omitempty"`
	Type                   string   `json:"type,omitempty"`
	Hidden                 *bool    `json:"hidden,omitempty"`
	DisableModelInvocation *bool    `json:"disable_model_invocation,omitempty"`
}

type PluginsService struct {
	enabled       bool
	inlineCount   int
	status        string
	plugins       []Plugin
	commands      []command.Command
	agents        []agent.Definition
	skills        []Skill
	pluginHooks   []Hook
	path          string
	lastLoadedAt  time.Time
	lastLoadError string
	lastEvent     string
	lastEventAt   time.Time
	lastTarget    string
	notices       []PluginNotice
}

type PluginNotice struct {
	Kind      string
	Target    string
	Message   string
	Timestamp string
}

type pluginManifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Marketplace string   `json:"marketplace"`
	Category    string   `json:"category"`
	Dev         bool     `json:"dev"`
	DefaultEnabled *bool `json:"default_enabled"`
	Commands    string   `json:"commands"`
	CommandsArr []string `json:"commands_paths"`
	CommandMetadata map[string]PluginCommandMetadata `json:"commands_metadata"`
	Agents      string   `json:"agents"`
	AgentsArr   []string `json:"agents_paths"`
	Skills      string   `json:"skills"`
	SkillsArr   []string `json:"skills_paths"`
	Hooks       string   `json:"hooks"`
}

func CreatePluginsService(path string) *PluginsService {
	initBuiltinPlugins()
	service := &PluginsService{
		enabled: true,
		status:  "active",
		path:    path,
	}
	service.load(path)
	return service
}

func defaultPlugins() []Plugin {
	return []Plugin{
		{
			Name:        "workspace-defaults",
			Source:      "builtin",
			SourceType:  "builtin",
			Status:      "builtin",
			Version:     "0.1.0",
			Description: "Built-in workspace helpers that ship with Claude-Code-Go.",
			Enabled:     true,
			Category:    "core",
			Dev:         true,
		},
	}
}

func (s *PluginsService) Register(plugin Plugin) {
	s.plugins = append(s.plugins, plugin)
}

func (s *PluginsService) Add(plugin Plugin) {
	for i := range s.plugins {
		if s.plugins[i].Name != plugin.Name {
			continue
		}
		s.plugins[i] = plugin
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.recordEvent("plugin_updated", plugin.Name)
		s.persist()
		s.load(s.path)
		return
	}
	s.plugins = append(s.plugins, plugin)
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.recordEvent("plugin_added", plugin.Name)
	s.persist()
	s.load(s.path)
}

func (s *PluginsService) Remove(name string) bool {
	for i := range s.plugins {
		if s.plugins[i].Name != name {
			continue
		}
		s.plugins = append(s.plugins[:i], s.plugins[i+1:]...)
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.recordEvent("plugin_removed", name)
		s.persist()
		s.load(s.path)
		return true
	}
	return false
}

func (s *PluginsService) List() []Plugin {
	return append([]Plugin(nil), s.plugins...)
}

func (s *PluginsService) Commands() []command.Command {
	return append([]command.Command(nil), s.commands...)
}

func (s *PluginsService) Agents() []agent.Definition {
	return append([]agent.Definition(nil), s.agents...)
}

func (s *PluginsService) Skills() []Skill {
	return append([]Skill(nil), s.skills...)
}

func (s *PluginsService) PluginHooks() []Hook {
	return append([]Hook(nil), s.pluginHooks...)
}

func (s *PluginsService) DrainNotices() []PluginNotice {
	out := append([]PluginNotice(nil), s.notices...)
	s.notices = nil
	return out
}

func (s *PluginsService) Reload() string {
	s.enabled = true
	s.inlineCount = 0
	s.status = "active"
	s.plugins = nil
	s.commands = nil
	s.agents = nil
	s.skills = nil
	s.pluginHooks = nil
	s.lastLoadedAt = time.Time{}
	s.lastLoadError = ""
	s.load(s.path)
	s.recordEvent("plugins_reloaded", "")
	return fmt.Sprintf("plugin registry reloaded\nloaded=%d", len(s.plugins))
}

func (s *PluginsService) Reset() string {
	s.enabled = true
	s.inlineCount = 0
	s.status = "active"
	s.plugins = defaultPlugins()
	s.commands = nil
	s.agents = nil
	s.skills = nil
	s.pluginHooks = nil
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.recordEvent("plugins_reset", "")
	s.persist()
	return fmt.Sprintf("plugin registry reset\nloaded=%d", len(s.plugins))
}

func (s *PluginsService) SetEnabledAll(enabled bool) {
	s.enabled = enabled
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.recordEvent("plugins_mode_changed", fmt.Sprintf("enabled=%t", enabled))
	s.persist()
}

func (s *PluginsService) SetServiceStatus(status string) {
	s.status = status
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.recordEvent("plugins_status_changed", status)
	s.persist()
}

func (s *PluginsService) SetEnabled(name string, enabled bool) bool {
	for i := range s.plugins {
		if s.plugins[i].Name != name {
			continue
		}
		s.plugins[i].Enabled = enabled
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.recordEvent("plugin_enabled_changed", name)
		s.persist()
		s.load(s.path)
		return true
	}
	return false
}

func (s *PluginsService) SetStatus(name, status string) bool {
	for i := range s.plugins {
		if s.plugins[i].Name != name {
			continue
		}
		s.plugins[i].Status = status
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.recordEvent("plugin_status_changed", name)
		s.persist()
		s.load(s.path)
		return true
	}
	return false
}

func (s *PluginsService) load(path string) {
	var payload struct {
		Enabled     *bool    `json:"enabled"`
		Status      string   `json:"status"`
		InlineCount int      `json:"inline_count"`
		Plugins     []Plugin `json:"plugins"`
	}
	err := loadJSONFile(path, &payload)
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
	}
	if err == nil && (len(payload.Plugins) > 0 || payload.Status != "" || payload.InlineCount > 0 || payload.Enabled != nil) {
		if payload.Enabled != nil {
			s.enabled = *payload.Enabled
		}
		if payload.Status != "" {
			s.status = payload.Status
		}
		s.inlineCount = payload.InlineCount
		s.plugins = append([]Plugin(nil), payload.Plugins...)
	}
	if len(s.plugins) == 0 {
		s.plugins = defaultPlugins()
	}
	s.loadComponents()
}

func (s *PluginsService) loadComponents() {
	s.commands = nil
	s.agents = nil
	s.skills = nil
	s.pluginHooks = nil

	for i := range s.plugins {
		plugin := &s.plugins[i]
		if !plugin.Enabled {
			continue
		}
		if builtin, ok := getBuiltinPluginDefinition(plugin.Name); ok && (plugin.Source == "builtin" || plugin.Source == "built-in") {
			plugin.SourceType = "builtin"
			if strings.TrimSpace(plugin.Description) == "" {
				plugin.Description = builtin.Description
			}
			if strings.TrimSpace(plugin.Version) == "" {
				plugin.Version = builtin.Version
			}
			for _, skillDef := range builtin.Skills {
				s.skills = append(s.skills, Skill{
					Name:                   skillDef.Name,
					DisplayName:            skillDef.Name,
					Aliases:                append([]string(nil), skillDef.Aliases...),
					Description:            skillDef.Description,
					WhenToUse:              skillDef.WhenToUse,
					ArgumentHint:           skillDef.ArgumentHint,
					AllowedTools:           append([]string(nil), skillDef.AllowedTools...),
					Model:                  skillDef.Model,
					Source:                 "builtin-plugin",
					LoadedFrom:             "plugin",
					BaseDir:                "builtin",
					Prompt:                 skillDef.Prompt,
					UserInvocable:          skillDef.UserInvocable,
					DisableModelInvocation: skillDef.DisableModelInvocation,
				})
			}
			plugin.SkillCount = len(builtin.Skills)
			for _, hook := range builtin.Hooks {
				s.pluginHooks = append(s.pluginHooks, hook)
			}
			plugin.HookCount = len(builtin.Hooks)
			plugin.Status = "loaded"
			continue
		}
		if plugin.SourceType == "" {
			plugin.SourceType = classifyPluginSourceType(*plugin)
		}
		s.loadPluginManifest(plugin)

		plugin.CommandCount = 0
		for _, dir := range s.commandDirs(*plugin) {
			cmds, err := loadPluginCommandsFromDir(*plugin, dir)
			if err != nil {
				s.lastLoadError = err.Error()
				continue
			}
			cmds = applyPluginCommandMetadata(*plugin, cmds)
			s.commands = append(s.commands, cmds...)
			plugin.CommandCount += len(cmds)
		}
		plugin.SkillCount = 0
		for _, dir := range s.skillDirs(*plugin) {
			skills, err := loadSkillEntriesFromDir(dir, plugin.Source, "plugin", plugin.Name)
			if err != nil {
				s.lastLoadError = err.Error()
				continue
			}
			s.skills = append(s.skills, skills...)
			plugin.SkillCount += len(skills)
		}
		plugin.AgentCount = 0
		for _, dir := range s.agentDirs(*plugin) {
			defs, err := loadPluginAgentsFromDir(*plugin, dir)
			if err != nil {
				s.lastLoadError = err.Error()
				continue
			}
			s.agents = append(s.agents, defs...)
			plugin.AgentCount += len(defs)
		}
		if hooks, err := s.loadPluginHooks(*plugin); err == nil {
			s.pluginHooks = append(s.pluginHooks, hooks...)
			plugin.HookCount = len(hooks)
		} else if err != nil {
			s.lastLoadError = err.Error()
		}

		switch {
		case plugin.CommandCount+plugin.SkillCount+plugin.AgentCount+plugin.HookCount > 0:
			plugin.Status = "loaded"
		case strings.TrimSpace(plugin.Status) == "":
			plugin.Status = "registered"
		}
	}
}

func (s *PluginsService) loadPluginManifest(plugin *Plugin) {
	root := strings.TrimSpace(plugin.Path)
	if root == "" {
		return
	}
	manifestPath := filepath.Join(root, "plugin.json")
	if !isFile(manifestPath) {
		return
	}
	var manifest pluginManifest
	if err := loadJSONFile(manifestPath, &manifest); err != nil {
		s.lastLoadError = err.Error()
		return
	}
	plugin.ManifestPath = manifestPath
	if strings.TrimSpace(manifest.Name) != "" {
		plugin.Name = manifest.Name
	}
	if strings.TrimSpace(manifest.Description) != "" {
		plugin.Description = manifest.Description
	}
	if plugin.SourceType == "" {
		plugin.SourceType = classifyPluginSourceType(*plugin)
	}
	if strings.TrimSpace(manifest.Version) != "" {
		plugin.Version = manifest.Version
	}
	if strings.TrimSpace(manifest.Marketplace) != "" {
		plugin.Marketplace = manifest.Marketplace
	}
	if strings.TrimSpace(manifest.Category) != "" {
		plugin.Category = manifest.Category
	}
	if manifest.Dev {
		plugin.Dev = true
	}
	if manifest.DefaultEnabled != nil {
		plugin.DefaultEnabled = manifest.DefaultEnabled
		if plugin.Status == "" {
			plugin.Status = "configured"
		}
	}
	if strings.TrimSpace(manifest.Commands) != "" {
		plugin.CommandsPath = filepath.Join(root, manifest.Commands)
	}
	if len(manifest.CommandsArr) > 0 {
		plugin.Commands = resolveRelativeSlice(root, manifest.CommandsArr)
	}
	if len(manifest.CommandMetadata) > 0 {
		plugin.CommandMetadata = manifest.CommandMetadata
	}
	if strings.TrimSpace(manifest.Agents) != "" {
		plugin.AgentsPath = filepath.Join(root, manifest.Agents)
	}
	if len(manifest.AgentsArr) > 0 {
		plugin.Agents = resolveRelativeSlice(root, manifest.AgentsArr)
	}
	if strings.TrimSpace(manifest.Skills) != "" {
		plugin.SkillsPath = filepath.Join(root, manifest.Skills)
	}
	if len(manifest.SkillsArr) > 0 {
		plugin.Skills = resolveRelativeSlice(root, manifest.SkillsArr)
	}
	if strings.TrimSpace(manifest.Hooks) != "" {
		plugin.HooksPath = filepath.Join(root, manifest.Hooks)
	}
}

func (s *PluginsService) commandDirs(plugin Plugin) []string {
	return pluginComponentDirs(plugin.Path, plugin.CommandsPath, plugin.Commands, "commands")
}

func (s *PluginsService) skillDirs(plugin Plugin) []string {
	return pluginComponentDirs(plugin.Path, plugin.SkillsPath, plugin.Skills, "skills")
}

func (s *PluginsService) agentDirs(plugin Plugin) []string {
	return pluginComponentDirs(plugin.Path, plugin.AgentsPath, plugin.Agents, "agents")
}

func (s *PluginsService) loadPluginHooks(plugin Plugin) ([]Hook, error) {
	hooksPath := strings.TrimSpace(plugin.HooksPath)
	if hooksPath == "" && strings.TrimSpace(plugin.Path) != "" {
		for _, candidate := range []string{
			filepath.Join(plugin.Path, "hooks.json"),
			filepath.Join(plugin.Path, "hooks", "hooks.json"),
		} {
			if isFile(candidate) {
				hooksPath = candidate
				break
			}
		}
	}
	if hooksPath == "" || !isFile(hooksPath) {
		return nil, nil
	}
	var payload struct {
		Hooks []Hook `json:"hooks"`
	}
	if err := loadJSONFile(hooksPath, &payload); err != nil {
		return nil, err
	}
	for i := range payload.Hooks {
		payload.Hooks[i].Source = "plugin"
		if payload.Hooks[i].Status == "" {
			payload.Hooks[i].Status = "loaded"
		}
		if payload.Hooks[i].Shell == "" {
			payload.Hooks[i].Shell = "bash"
		}
	}
	return payload.Hooks, nil
}

func (s *PluginsService) persist() {
	payload := struct {
		Enabled     bool     `json:"enabled"`
		Status      string   `json:"status"`
		InlineCount int      `json:"inline_count"`
		Plugins     []Plugin `json:"plugins"`
	}{
		Enabled:     s.enabled,
		Status:      s.status,
		InlineCount: s.inlineCount,
		Plugins:     append([]Plugin(nil), s.plugins...),
	}
	if err := saveJSONFile(s.path, payload); err != nil {
		s.lastLoadError = err.Error()
		return
	}
	s.lastLoadError = ""
}

func (s *PluginsService) Status() string {
	lines := []string{
		fmt.Sprintf("plugins=%s", s.status),
		fmt.Sprintf("enabled=%t", s.enabled),
		fmt.Sprintf("loaded=%d", len(s.plugins)),
		fmt.Sprintf("inline_plugins=%d", s.inlineCount),
		fmt.Sprintf("loaded_commands=%d", len(s.commands)),
		fmt.Sprintf("loaded_agents=%d", len(s.agents)),
		fmt.Sprintf("loaded_skills=%d", len(s.skills)),
		fmt.Sprintf("loaded_hooks=%d", len(s.pluginHooks)),
		fmt.Sprintf("config_path=%s", s.path),
		fmt.Sprintf("last_loaded=%s", formatLoadTime(s.lastLoadedAt)),
		"reload_supported=full",
	}
	if strings.TrimSpace(s.lastEvent) != "" {
		lines = append(lines, "last_event="+s.lastEvent)
		lines = append(lines, "last_event_at="+formatLoadTime(s.lastEventAt))
		if strings.TrimSpace(s.lastTarget) != "" {
			lines = append(lines, "last_target="+s.lastTarget)
		}
	}
	if strings.TrimSpace(s.lastLoadError) != "" {
		lines = append(lines, "last_error="+s.lastLoadError)
	}
	if len(s.plugins) > 0 {
		lines = append(lines, "", "registered:")
		for _, plugin := range s.plugins {
			line := fmt.Sprintf("- %s [%s] %s", plugin.Name, plugin.Source, plugin.Status)
			if strings.TrimSpace(plugin.Version) != "" {
				line += " version=" + plugin.Version
			}
			if plugin.Enabled {
				line += " enabled=true"
			}
			lines = append(lines, line)
			meta := []string{}
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
			if strings.TrimSpace(plugin.ManifestPath) != "" {
				meta = append(meta, "manifest="+plugin.ManifestPath)
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+strings.Join(meta, "  "))
			}
			if strings.TrimSpace(plugin.Description) != "" {
				lines = append(lines, "  "+plugin.Description)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func resolveRelativeSlice(root string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, filepath.Join(root, value))
	}
	return out
}

func pluginComponentDirs(root, singular string, multi []string, conventional string) []string {
	seen := map[string]bool{}
	out := []string{}
	push := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) && root != "" {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		if !isDir(path) || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	push(singular)
	for _, item := range multi {
		push(item)
	}
	if root != "" {
		push(filepath.Join(root, conventional))
	}
	return out
}

func applyPluginCommandMetadata(plugin Plugin, commandsIn []command.Command) []command.Command {
	if len(plugin.CommandMetadata) == 0 {
		return commandsIn
	}
	out := make([]command.Command, 0, len(commandsIn))
	prefix := plugin.Name + ":"
	for _, cmd := range commandsIn {
		base := cmd.GetBase()
		meta, ok := plugin.CommandMetadata[base.Name]
		if !ok {
			short := strings.TrimPrefix(base.Name, prefix)
			meta, ok = plugin.CommandMetadata[short]
		}
		if ok {
			// If it's a LegacyCommand, we can modify it directly
			if legacyCmd, ok := cmd.(command.LegacyCommand); ok {
				if strings.TrimSpace(meta.Description) != "" {
					legacyCmd.Description = meta.Description
				}
				if strings.TrimSpace(meta.DisplayName) != "" {
					legacyCmd.DisplayName = meta.DisplayName
				}
				if strings.TrimSpace(meta.ArgumentHint) != "" {
					legacyCmd.ArgumentHint = meta.ArgumentHint
				}
				if strings.TrimSpace(meta.WhenToUse) != "" {
					legacyCmd.WhenToUse = meta.WhenToUse
				}
				if len(meta.AllowedTools) > 0 {
					legacyCmd.AllowedTools = append([]string(nil), meta.AllowedTools...)
				}
				if strings.TrimSpace(meta.Model) != "" {
					legacyCmd.Model = meta.Model
				}
				if len(meta.Aliases) > 0 {
					legacyCmd.Aliases = append([]string(nil), meta.Aliases...)
				}
				if meta.DisableModelInvocation != nil {
					legacyCmd.DisableModelInvocation = *meta.DisableModelInvocation
				}
				if meta.Hidden != nil {
					legacyCmd.Hidden = *meta.Hidden
				}
				switch strings.TrimSpace(meta.Type) {
				case string(command.KindPrompt):
					legacyCmd.Type = command.KindPrompt
				case string(command.KindLocal):
					legacyCmd.Type = command.KindLocal
				case string(command.KindLocalJSX):
					legacyCmd.Type = command.KindLocalJSX
				}
				cmd = legacyCmd
			}
		}
		out = append(out, cmd)
	}
	return out
}

func classifyPluginSourceType(plugin Plugin) string {
	switch {
	case plugin.Source == "builtin" || plugin.Source == "built-in":
		return "builtin"
	case strings.TrimSpace(plugin.Path) != "":
		return "directory"
	case strings.TrimSpace(plugin.Marketplace) != "":
		return "marketplace"
	default:
		return "registry"
	}
}

func (s *PluginsService) recordEvent(event, target string) {
	s.lastEvent = event
	s.lastEventAt = time.Now()
	s.lastTarget = target
	s.notices = append(s.notices, PluginNotice{
		Kind:      event,
		Target:    target,
		Message:   formatPluginEvent(event, target),
		Timestamp: s.lastEventAt.Format(time.RFC3339),
	})
}

func formatPluginEvent(event, target string) string {
	switch event {
	case "plugin_added":
		return "plugin added"
	case "plugin_updated":
		return "plugin updated"
	case "plugin_removed":
		return "plugin removed"
	case "plugins_reloaded":
		return "plugin registry reloaded"
	case "plugins_reset":
		return "plugin registry reset"
	case "plugins_mode_changed":
		return "plugin service mode updated"
	case "plugins_status_changed":
		return "plugin service status updated"
	case "plugin_enabled_changed":
		return "plugin enabled state changed"
	case "plugin_status_changed":
		return "plugin status changed"
	default:
		return event
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *PluginsService) ExportJSON() string {
	payload := map[string]any{
		"enabled":      s.enabled,
		"status":       s.status,
		"inline_count": s.inlineCount,
		"plugins":      s.plugins,
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return string(data)
}
