package meta

import (
	"context"
	"strings"

	"claude-code-go/internal/command"
)

func registerMetaCommands(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "help",
		Description: "show built-in commands",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			commands := r.List()
			lines := make([]string, 0, len(commands))
			for _, cmd := range commands {
				base := cmd.GetBase()
				if base.Hidden {
					continue
				}
				line := "/" + base.Name
				if base.ArgumentHint != "" {
					line += " " + base.ArgumentHint
				}
				line += "  [" + string(cmd.GetKind()) + "]  " + base.Description
				lines = append(lines, line)
			}
			return strings.Join(lines, "\n"), nil
		},
	})
}