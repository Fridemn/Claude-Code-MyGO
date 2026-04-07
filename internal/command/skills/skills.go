package skills

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/command"
)

func registerSkills(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "skills",
		Description: "show available skills",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			lines := []string{
				"overview:",
				"registry=skills",
			}
			if runtime.SkillList != nil {
				skills := runtime.SkillList()
				lines = append(lines, fmt.Sprintf("entries=%d", len(skills)))
				lines = append(lines, "", "entries:")
				for _, skill := range skills {
					line := "- " + skill.Name + " [" + skill.Source + "]"
					if !skill.UserInvocable {
						line += " hidden=true"
					}
					lines = append(lines, line)
					if strings.TrimSpace(skill.DisplayName) != "" {
						lines = append(lines, "  display_name="+skill.DisplayName)
					}
					if strings.TrimSpace(skill.Description) != "" {
						lines = append(lines, "  "+skill.Description)
					}
					meta := []string{}
					if strings.TrimSpace(skill.WhenToUse) != "" {
						meta = append(meta, "when_to_use="+skill.WhenToUse)
					}
					if strings.TrimSpace(skill.Path) != "" {
						meta = append(meta, "path="+skill.Path)
					}
					if len(skill.Aliases) > 0 {
						meta = append(meta, "aliases="+strings.Join(skill.Aliases, ","))
					}
					if len(meta) > 0 {
						lines = append(lines, "  "+strings.Join(meta, "  "))
					}
				}
			}
			if runtime.SkillStatus != nil {
				lines = append(lines, "", "status:", runtime.SkillStatus())
			}
			lines = append(lines, "", "actions:", "- /skills", "- /reload-plugins")
			return strings.Join(lines, "\n"), nil
		},
	})
}