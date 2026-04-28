package skills

import (
	"context"

	"claude-go/internal/command"
)

func registerSkills(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "skills",
		Description: "show available skills",
		Load:        loadSkillsModel,
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return renderSkillsOverview(runtime), nil
		},
	})
}
