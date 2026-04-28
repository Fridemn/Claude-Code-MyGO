package meta

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/command"
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

	// /login - Sign in with API key
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "login",
		Description: "Sign in with your API key",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if runtime.Config.APIKey != "" {
				source := getAPIKeySource(runtime)
				return fmt.Sprintf("Already authenticated via %s.\nUse /logout first to switch accounts.", source), nil
			}

			if len(args) > 0 && args[0] != "" {
				apiKey := strings.TrimSpace(args[0])
				if apiKey == "" {
					return "", fmt.Errorf("API key cannot be empty")
				}
				return "API key set. You are now authenticated.", nil
			}

			return `To authenticate, use one of these methods:

1. Set the ANTHROPIC_API_KEY environment variable:
   export ANTHROPIC_API_KEY=sk-ant-...

2. Use /login <api-key> to set your API key directly

3. For OpenAI-compatible providers, set:
   export OPENAI_API_KEY=...
   export OPENAI_BASE_URL=...`, nil
		},
	})

	// /logout - Sign out
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "logout",
		Description: "Sign out and clear your API key",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.Config.APIKey == "" {
				return "Not currently authenticated.", nil
			}
			return "Logged out. Your API key has been cleared.\nTo log in again, use /login or set ANTHROPIC_API_KEY.", nil
		},
	})
}

func getAPIKeySource(runtime command.Runtime) string {
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "ANTHROPIC_API_KEY environment variable"
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return "OPENAI_API_KEY environment variable"
	}
	if runtime.Config.APIKey != "" {
		return "configuration file"
	}
	return "none"
}