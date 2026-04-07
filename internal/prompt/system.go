package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-code-go/internal/config"
	"claude-code-go/internal/tool"
)

func System(cfg config.Config) string {
	if strings.TrimSpace(cfg.SystemPrompt) != "" {
		return strings.TrimSpace(cfg.SystemPrompt)
	}

	cwd, err := os.Getwd()
	if err != nil || strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	if abs, absErr := filepath.Abs(cwd); absErr == nil {
		cwd = abs
	}

	return fmt.Sprintf(
		"You are Claude-Code-Go, a pragmatic coding assistant running in a local CLI.\n\n"+
			"Environment:\n"+
			"- Primary working directory: %s\n"+
			"- Treat relative paths as relative to this directory.\n"+
			"- When the user mentions sibling directories or repositories, resolve them from this directory before calling tools.\n"+
			"- If the current repository name is mentioned again, do not prepend it to paths. Use '.' for the current repository root, and use '../name' for sibling repositories.\n"+
			"- For repository structure discovery, prefer list_files or Glob instead of Bash ls/find.\n"+
			"- For reading files, prefer Read instead of Bash cat/head/tail.",
		cwd,
	)
}

func WithTools(base string, definitions []tool.Definition) string {
	if len(definitions) == 0 {
		return strings.TrimSpace(base)
	}
	fragment := tool.SystemPromptFragment(definitions)
	if strings.TrimSpace(fragment) == "" {
		return strings.TrimSpace(base)
	}
	if strings.TrimSpace(base) == "" {
		return fragment
	}
	return strings.TrimSpace(base) + "\n\n" + fragment
}
