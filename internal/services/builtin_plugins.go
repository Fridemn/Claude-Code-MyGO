package services

type BuiltinPluginDefinition struct {
	Name           string
	Description    string
	Version        string
	DefaultEnabled bool
	Skills         []BundledSkillDefinition
	Hooks          []Hook
}

var builtinPluginRegistry = map[string]BuiltinPluginDefinition{}

func registerBuiltinPlugin(def BuiltinPluginDefinition) {
	builtinPluginRegistry[def.Name] = def
}

func initBuiltinPlugins() {
	if len(builtinPluginRegistry) > 0 {
		return
	}
	registerBuiltinPlugin(BuiltinPluginDefinition{
		Name:           "workspace-defaults",
		Description:    "Built-in workspace helpers that ship with Claude-Go.",
		Version:        "0.1.0",
		DefaultEnabled: true,
		Skills: []BundledSkillDefinition{
			{
				Name:         "workspace-defaults:triage",
				Description:  "Triage a repository before making code changes.",
				WhenToUse:    "starting work in an unfamiliar codebase",
				ArgumentHint: "[goal]",
				AllowedTools: []string{"read_file", "grep", "list_files"},
				Prompt:       "Inspect the repository structure, identify the main modules, and suggest the next engineering step that unblocks progress.",
				UserInvocable: true,
			},
		},
		Hooks: []Hook{
			{
				Event:       "pre_turn",
				Source:      "plugin",
				Status:      "builtin",
				Description: "Builtin plugin lifecycle placeholder.",
				Enabled:     true,
				Matcher:     "*",
				Command:     "echo builtin_pre_turn",
				TimeoutMs:   500,
				Shell:       "bash",
			},
		},
	})
}

func getBuiltinPluginDefinition(name string) (BuiltinPluginDefinition, bool) {
	initBuiltinPlugins()
	def, ok := builtinPluginRegistry[name]
	return def, ok
}
