package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/command"
)

type markdownFrontmatter map[string]string

func parseMarkdownDocument(raw string) (markdownFrontmatter, string) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return markdownFrontmatter{}, strings.TrimSpace(raw)
	}
	rest := strings.TrimPrefix(raw, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return markdownFrontmatter{}, strings.TrimSpace(raw)
	}
	head := rest[:idx]
	body := rest[idx+5:]
	fm := markdownFrontmatter{}
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fm[strings.TrimSpace(strings.ToLower(key))] = strings.TrimSpace(value)
	}
	return fm, strings.TrimSpace(body)
}

func frontmatterString(fm markdownFrontmatter, key string) string {
	return strings.TrimSpace(fm[strings.ToLower(key)])
}

func frontmatterBool(fm markdownFrontmatter, key string, fallback bool) bool {
	value := strings.ToLower(frontmatterString(fm, key))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func frontmatterInt(fm markdownFrontmatter, key string) int {
	value := frontmatterString(fm, key)
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func frontmatterList(fm markdownFrontmatter, key string) []string {
	value := frontmatterString(fm, key)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstMarkdownDescription(body string, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#-*> "))
		if line != "" {
			return line
		}
	}
	return fallback
}

func walkMarkdownFiles(root string, stopAtSkillDir bool) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if d.IsDir() {
			if strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			if stopAtSkillDir {
				entries, err := os.ReadDir(path)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() {
							continue
						}
						if isSkillMarkdown(entry.Name()) {
							files = append(files, filepath.Join(path, entry.Name()))
							return filepath.SkipDir
						}
					}
				}
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isSkillMarkdown(name string) bool {
	return strings.EqualFold(name, "skill.md") || strings.EqualFold(name, "SKILL.md")
}

func buildSkillName(root, file string) string {
	dir := filepath.Dir(file)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if isSkillMarkdown(filepath.Base(file)) {
			return filepath.Base(dir)
		}
		return base
	}
	segments := strings.Split(filepath.ToSlash(rel), "/")
	if isSkillMarkdown(filepath.Base(file)) {
		return strings.Join(segments, ":")
	}
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	return strings.Join(append(segments, base), ":")
}

func buildPluginCommandName(pluginName, root, file string) string {
	name := buildSkillName(root, file)
	if name == "" {
		return pluginName
	}
	return pluginName + ":" + name
}

func makePromptHandler(body, baseDir string) command.Handler {
	return func(_ context.Context, _ command.Runtime, args []string) (string, error) {
		var lines []string
		if strings.TrimSpace(baseDir) != "" {
			lines = append(lines, "Base directory for this skill: "+baseDir, "")
		}
		lines = append(lines, body)
		if joined := strings.TrimSpace(strings.Join(args, " ")); joined != "" {
			lines = append(lines, "", "Arguments:", joined)
		}
		return strings.TrimSpace(strings.Join(lines, "\n")), nil
	}
}

func loadSkillEntriesFromDir(root, source, loadedFrom, pluginName string) ([]Skill, error) {
	files, err := walkMarkdownFiles(root, true)
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		fm, body := parseMarkdownDocument(string(raw))
		name := buildSkillName(root, file)
		if pluginName != "" {
			name = pluginName + ":" + name
		}
		description := frontmatterString(fm, "description")
		if description == "" {
			description = firstMarkdownDescription(body, "Skill")
		}
		displayName := frontmatterString(fm, "name")
		aliases := frontmatterList(fm, "aliases")
		skills = append(skills, Skill{
			Name:                   name,
			DisplayName:            displayName,
			Aliases:                aliases,
			Description:            description,
			WhenToUse:              frontmatterString(fm, "when_to_use"),
			ArgumentHint:           frontmatterString(fm, "argument-hint"),
			AllowedTools:           frontmatterList(fm, "allowed-tools"),
			Version:                frontmatterString(fm, "version"),
			Model:                  frontmatterString(fm, "model"),
			Context:                frontmatterString(fm, "context"),
			Agent:                  frontmatterString(fm, "agent"),
			Source:                 source,
			LoadedFrom:             loadedFrom,
			Path:                   file,
			BaseDir:                filepath.Dir(file),
			Prompt:                 body,
			UserInvocable:          frontmatterBool(fm, "user-invocable", true),
			DisableModelInvocation: frontmatterBool(fm, "disable-model-invocation", false),
		})
	}
	return skills, nil
}

func buildCommandsFromSkills(skills []Skill) []command.Command {
	out := make([]command.Command, 0, len(skills))
	for _, skill := range skills {
		if !skill.UserInvocable {
			continue
		}
		handler := makePromptHandler(skill.Prompt, skill.BaseDir)
		out = append(out, command.LegacyCommand{
			Type:         command.KindPrompt,
			Name:         skill.Name,
			DisplayName:  skill.DisplayName,
			Aliases:      append([]string(nil), skill.Aliases...),
			Description:  skill.Description,
			ArgumentHint: skill.ArgumentHint,
			WhenToUse:    skill.WhenToUse,
			AllowedTools: append([]string(nil), skill.AllowedTools...),
			Model:        skill.Model,
			DisableModelInvocation: skill.DisableModelInvocation,
			BaseDir:      skill.BaseDir,
			Hidden:       !skill.UserInvocable,
			Handler:      handler,
		})
	}
	return out
}

func loadPluginCommandsFromDir(plugin Plugin, dir string) ([]command.Command, error) {
	files, err := walkMarkdownFiles(dir, true)
	if err != nil {
		return nil, err
	}
	var commandsOut []command.Command
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		fm, body := parseMarkdownDocument(string(raw))
		cmdType := strings.ToLower(frontmatterString(fm, "type"))
		kind := command.KindPrompt
		switch cmdType {
		case "local-jsx":
			kind = command.KindLocalJSX
		case "local":
			kind = command.KindLocal
		}
		name := buildPluginCommandName(plugin.Name, dir, file)
		if override := frontmatterString(fm, "command"); override != "" {
			name = plugin.Name + ":" + override
		}
		description := frontmatterString(fm, "description")
		if description == "" {
			description = firstMarkdownDescription(body, "Plugin command")
		}
		userInvocable := frontmatterBool(fm, "user-invocable", true)
		aliases := frontmatterList(fm, "aliases")
		displayName := frontmatterString(fm, "name")
		whenToUse := frontmatterString(fm, "when_to_use")
		allowedTools := frontmatterList(fm, "allowed-tools")
		model := frontmatterString(fm, "model")
		disableModelInvocation := frontmatterBool(fm, "disable-model-invocation", false)
		baseDir := filepath.Dir(file)
		render := strings.TrimSpace(body)
		commandsOut = append(commandsOut, command.LegacyCommand{
			Type:         kind,
			Name:         name,
			DisplayName:  displayName,
			Aliases:      aliases,
			Description:  description,
			ArgumentHint: frontmatterString(fm, "argument-hint"),
			WhenToUse:    whenToUse,
			AllowedTools: allowedTools,
			Model:        model,
			DisableModelInvocation: disableModelInvocation,
			BaseDir:      baseDir,
			Hidden:       !userInvocable,
			Handler: func(kind command.Kind, body, baseDir string) command.Handler {
				return func(_ context.Context, _ command.Runtime, args []string) (string, error) {
					if kind == command.KindPrompt {
						return strings.TrimSpace(buildPromptBody(body, baseDir, args)), nil
					}
					if joined := strings.TrimSpace(strings.Join(args, " ")); joined != "" {
						return strings.TrimSpace(body) + "\n\nargs=" + joined, nil
					}
					return strings.TrimSpace(body), nil
				}
			}(kind, render, baseDir),
		})
	}
	return commandsOut, nil
}

func buildPromptBody(body, baseDir string, args []string) string {
	lines := []string{}
	if strings.TrimSpace(baseDir) != "" {
		lines = append(lines, "Base directory for this skill: "+baseDir, "")
	}
	lines = append(lines, strings.TrimSpace(body))
	if joined := strings.TrimSpace(strings.Join(args, " ")); joined != "" {
		lines = append(lines, "", "Arguments:", joined)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func loadPluginAgentsFromDir(plugin Plugin, dir string) ([]agent.Definition, error) {
	files, err := walkMarkdownFiles(dir, false)
	if err != nil {
		return nil, err
	}
	var defs []agent.Definition
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		fm, body := parseMarkdownDocument(string(raw))
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			rel = filepath.Base(file)
		}
		name := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		name = strings.ReplaceAll(name, "/", ":")
		if explicit := frontmatterString(fm, "name"); explicit != "" {
			name = explicit
		}
		description := frontmatterString(fm, "description")
		if description == "" {
			description = firstMarkdownDescription(body, "Agent from plugin")
		}
		defs = append(defs, agent.Definition{
			AgentType:    plugin.Name + ":" + name,
			WhenToUse:    description,
			Tools:        frontmatterList(fm, "tools"),
			Disallowed:   frontmatterList(fm, "disallowedtools"),
			Source:       "plugin",
			BaseDir:      filepath.Dir(file),
			Model:        frontmatterString(fm, "model"),
			Color:        frontmatterString(fm, "color"),
			Background:   frontmatterBool(fm, "background", false),
			SystemPrompt: body,
			ReadOnly:     frontmatterBool(fm, "read-only", false),
			OmitClaudeMd: frontmatterBool(fm, "omit-claude-md", false),
			MaxTurns:     frontmatterInt(fm, "maxturns"),
			InitialPrompt: frontmatterString(fm, "initial-prompt"),
		})
	}
	return defs, nil
}
