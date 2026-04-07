package services

type BundledSkillDefinition struct {
	Name                   string
	Description            string
	Aliases                []string
	WhenToUse              string
	ArgumentHint           string
	AllowedTools           []string
	Model                  string
	DisableModelInvocation bool
	UserInvocable          bool
	Prompt                 string
}

var bundledSkillRegistry []BundledSkillDefinition

func registerBundledSkill(def BundledSkillDefinition) {
	bundledSkillRegistry = append(bundledSkillRegistry, def)
}

func initBundledSkills() {
	if len(bundledSkillRegistry) > 0 {
		return
	}
	registerBundledSkill(BundledSkillDefinition{
		Name:         "debug",
		Description:  "Debug an issue by narrowing scope and testing assumptions.",
		Aliases:      []string{"dbg"},
		WhenToUse:    "unexpected behavior or failing tests",
		ArgumentHint: "<symptom>",
		AllowedTools: []string{"read_file", "grep", "exec_command"},
		Prompt: "Investigate the issue methodically. Reproduce it, narrow the scope, identify the most likely cause, and propose the smallest correct fix.",
		UserInvocable: true,
	})
	registerBundledSkill(BundledSkillDefinition{
		Name:         "verify",
		Description:  "Verify a change with targeted checks and note residual risk.",
		WhenToUse:    "after implementing or editing code",
		ArgumentHint: "<change>",
		AllowedTools: []string{"read_file", "grep", "exec_command"},
		Prompt: "Verify the change with focused checks. Identify regressions, missing tests, and residual risk. Prefer concrete validation over broad claims.",
		UserInvocable: true,
	})
}

func bundledSkillEntries() []Skill {
	initBundledSkills()
	out := make([]Skill, 0, len(bundledSkillRegistry))
	for _, def := range bundledSkillRegistry {
		out = append(out, Skill{
			Name:                   def.Name,
			DisplayName:            def.Name,
			Aliases:                append([]string(nil), def.Aliases...),
			Description:            def.Description,
			WhenToUse:              def.WhenToUse,
			ArgumentHint:           def.ArgumentHint,
			AllowedTools:           append([]string(nil), def.AllowedTools...),
			Model:                  def.Model,
			Source:                 "bundled",
			LoadedFrom:             "bundled",
			BaseDir:                "bundled",
			Prompt:                 def.Prompt,
			UserInvocable:          def.UserInvocable,
			DisableModelInvocation: def.DisableModelInvocation,
		})
	}
	return out
}
