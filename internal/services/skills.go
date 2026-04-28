package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-go/internal/command"
	"claude-go/internal/tool/skill"
)

// Skill type is defined in tool/skill package to avoid import cycles

type SkillsService struct {
	cwd           string
	status        string
	lastLoadedAt  time.Time
	lastLoadError string
	userSkills    []skill.Skill
	localSkills   []skill.Skill
	pluginSkills  []skill.Skill
	bundledSkills []skill.Skill
}

func CreateSkillsService(cwd string) *SkillsService {
	service := &SkillsService{
		cwd:    cwd,
		status: "active",
	}
	service.bundledSkills = bundledSkillEntries()
	service.Reload(nil)
	return service
}

func (s *SkillsService) Reload(pluginSkills []skill.Skill) {
	var userSkills []skill.Skill
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) != "" {
		userSkillsDir := filepath.Join(home, ".claude-go", "skills")
		userLoaded, err := loadSkillEntriesFromDir(userSkillsDir, "userSettings", "skills", "")
		if err == nil {
			userSkills = userLoaded
		} else if !os.IsNotExist(err) {
			s.lastLoadError = err.Error()
		}
	}
	skillsDir := filepath.Join(s.cwd, ".claude-go", "skills")
	localSkills, err := loadSkillEntriesFromDir(skillsDir, "projectSettings", "skills", "")
	s.lastLoadedAt = time.Now()
	s.userSkills = userSkills
	s.pluginSkills = append([]skill.Skill(nil), pluginSkills...)
	if err != nil {
		s.lastLoadError = err.Error()
		s.localSkills = nil
		return
	}
	s.lastLoadError = ""
	s.localSkills = localSkills
}

func (s *SkillsService) List() []skill.Skill {
	out := make([]skill.Skill, 0, len(s.bundledSkills)+len(s.userSkills)+len(s.localSkills)+len(s.pluginSkills))
	out = append(out, s.bundledSkills...)
	out = append(out, s.userSkills...)
	out = append(out, s.localSkills...)
	out = append(out, s.pluginSkills...)
	return out
}

func (s *SkillsService) Commands() []command.Command {
	return buildCommandsFromSkills(s.List())
}

func (s *SkillsService) Status() string {
	skills := s.List()
	lines := []string{
		"skills=" + s.status,
		fmt.Sprintf("registered=%d", len(skills)),
		fmt.Sprintf("user_dir=%s", filepath.Join(userHomeDir(), ".claude-go", "skills")),
		fmt.Sprintf("project_dir=%s", filepath.Join(s.cwd, ".claude-go", "skills")),
		fmt.Sprintf("last_loaded=%s", formatLoadTime(s.lastLoadedAt)),
	}
	if strings.TrimSpace(s.lastLoadError) != "" {
		lines = append(lines, "last_error="+s.lastLoadError)
	}
	if len(skills) > 0 {
		lines = append(lines, "", "skills:")
		for _, skill := range skills {
			line := "- " + skill.Name + " [" + skill.Source + "]"
			if strings.TrimSpace(skill.WhenToUse) != "" {
				line += " when_to_use=" + skill.WhenToUse
			}
			lines = append(lines, line)
			if strings.TrimSpace(skill.Description) != "" {
				lines = append(lines, "  "+skill.Description)
			}
			meta := []string{}
			if strings.TrimSpace(skill.Path) != "" {
				meta = append(meta, "path="+skill.Path)
			}
			if strings.TrimSpace(skill.ArgumentHint) != "" {
				meta = append(meta, "argument_hint="+skill.ArgumentHint)
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+strings.Join(meta, "  "))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func userHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
