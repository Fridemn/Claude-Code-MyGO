package services

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	"claude-code-go/internal/engine"
)

type Hook struct {
	Event       string `json:"event"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	Matcher     string `json:"matcher,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
	Blocking    bool   `json:"blocking,omitempty"`
	Shell       string `json:"shell,omitempty"`
	RunCount    int    `json:"run_count,omitempty"`
	LastRunAt   string `json:"last_run_at,omitempty"`
	LastResult  string `json:"last_result,omitempty"`
	LastOutput  string `json:"last_output,omitempty"`
	LastError   string `json:"last_error,omitempty"`
}

type HooksService struct {
	enabled       bool
	status        string
	hooks         []Hook
	path          string
	lastLoadedAt  time.Time
	lastLoadError string
	lastEvent     engine.HookExecution
}

func CreateHooksService(path string) *HooksService {
	service := &HooksService{enabled: true, status: "not_yet_ported", path: path}
	service.load(path)
	return service
}

func defaultHooks() []Hook {
	return []Hook{
		{Event: "pre_tool", Source: "core", Status: "placeholder", Command: "echo pre_tool", Description: "Default placeholder hook before tool execution.", Enabled: true, Matcher: "*", TimeoutMs: 1000},
		{Event: "post_tool", Source: "core", Status: "placeholder", Command: "echo post_tool", Description: "Default placeholder hook after tool execution.", Enabled: true, Matcher: "*", TimeoutMs: 1000},
		{Event: "pre_turn", Source: "core", Status: "placeholder", Command: "echo pre_turn", Description: "Default placeholder hook before model turn execution.", Enabled: true, Matcher: "*", TimeoutMs: 1000},
		{Event: "post_turn", Source: "core", Status: "placeholder", Command: "echo post_turn", Description: "Default placeholder hook after model turn execution.", Enabled: true, Matcher: "*", TimeoutMs: 1000},
	}
}

func (s *HooksService) Register(hook Hook) {
	s.hooks = append(s.hooks, hook)
}

func (s *HooksService) Add(hook Hook) {
	for i := range s.hooks {
		if s.hooks[i].Event != hook.Event {
			continue
		}
		s.hooks[i] = hook
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.persist()
		return
	}
	s.hooks = append(s.hooks, hook)
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
}

func (s *HooksService) Remove(event string) bool {
	for i := range s.hooks {
		if s.hooks[i].Event != event {
			continue
		}
		s.hooks = append(s.hooks[:i], s.hooks[i+1:]...)
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.persist()
		return true
	}
	return false
}

func (s *HooksService) List() []Hook {
	return append([]Hook(nil), s.hooks...)
}

func (s *HooksService) LoadPluginHooks(hooks []Hook) {
	base := make([]Hook, 0, len(s.hooks))
	for _, hook := range s.hooks {
		if hook.Source == "plugin" {
			continue
		}
		base = append(base, hook)
	}
	for _, hook := range hooks {
		if hook.Source == "" {
			hook.Source = "plugin"
		}
		if hook.Status == "" {
			hook.Status = "loaded"
		}
		base = append(base, hook)
	}
	s.hooks = base
	s.lastLoadedAt = time.Now()
}

func (s *HooksService) LastEvent() engine.HookExecution {
	return s.lastEvent
}

func (s *HooksService) Reload() string {
	s.enabled = true
	s.status = "not_yet_ported"
	s.hooks = nil
	s.lastLoadedAt = time.Time{}
	s.lastLoadError = ""
	s.load(s.path)
	return fmt.Sprintf("hook registry reloaded\nregistered=%d", len(s.hooks))
}

func (s *HooksService) Reset() string {
	s.enabled = true
	s.status = "not_yet_ported"
	s.hooks = defaultHooks()
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.lastEvent = engine.HookExecution{}
	s.persist()
	return fmt.Sprintf("hook registry reset\nregistered=%d", len(s.hooks))
}

func (s *HooksService) Trigger(ctx context.Context, event engine.HookEvent) ([]engine.HookExecution, error) {
	if !s.enabled {
		return nil, nil
	}
	var reports []engine.HookExecution
	for i := range s.hooks {
		hook := &s.hooks[i]
		if !hook.Enabled || hook.Event != event.Name || !hookMatches(*hook, event.Target) {
			continue
		}
		report := s.executeHook(ctx, hook, event)
		reports = append(reports, report)
		s.lastEvent = report
		if hook.Blocking && report.Error != "" {
			s.status = "execution_error"
			s.lastLoadedAt = time.Now()
			s.persist()
			return reports, fmt.Errorf("hook %s failed: %s", hook.Event, report.Error)
		}
	}
	if len(reports) > 0 && s.status == "not_yet_ported" {
		s.status = "active"
		s.lastLoadedAt = time.Now()
		s.persist()
	}
	return reports, nil
}

func (s *HooksService) SetEnabledAll(enabled bool) {
	s.enabled = enabled
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
}

func (s *HooksService) SetServiceStatus(status string) {
	s.status = status
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
}

func (s *HooksService) SetEnabled(event string, enabled bool) bool {
	for i := range s.hooks {
		if s.hooks[i].Event != event {
			continue
		}
		s.hooks[i].Enabled = enabled
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.persist()
		return true
	}
	return false
}

func (s *HooksService) SetStatus(event, status string) bool {
	for i := range s.hooks {
		if s.hooks[i].Event != event {
			continue
		}
		s.hooks[i].Status = status
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.persist()
		return true
	}
	return false
}

func (s *HooksService) load(path string) {
	var payload struct {
		Enabled *bool  `json:"enabled"`
		Status  string `json:"status"`
		Hooks   []Hook `json:"hooks"`
	}
	err := loadJSONFile(path, &payload)
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
	}
	if err == nil && (len(payload.Hooks) > 0 || payload.Status != "" || payload.Enabled != nil) {
		if payload.Enabled != nil {
			s.enabled = *payload.Enabled
		}
		if payload.Status != "" {
			s.status = payload.Status
		}
		s.hooks = append([]Hook(nil), payload.Hooks...)
	}
	if len(s.hooks) == 0 {
		s.hooks = defaultHooks()
	}
}

func (s *HooksService) persist() {
	payload := struct {
		Enabled bool   `json:"enabled"`
		Status  string `json:"status"`
		Hooks   []Hook `json:"hooks"`
	}{
		Enabled: s.enabled,
		Status:  s.status,
		Hooks:   append([]Hook(nil), s.hooks...),
	}
	if err := saveJSONFile(s.path, payload); err != nil {
		s.lastLoadError = err.Error()
		return
	}
	s.lastLoadError = ""
}

func (s *HooksService) Status() string {
	lines := []string{
		"hooks=" + s.status,
		fmt.Sprintf("enabled=%t", s.enabled),
		fmt.Sprintf("registered=%d", len(s.hooks)),
		fmt.Sprintf("config_path=%s", s.path),
		fmt.Sprintf("last_loaded=%s", formatLoadTime(s.lastLoadedAt)),
	}
	if strings.TrimSpace(s.lastLoadError) != "" {
		lines = append(lines, "last_error="+s.lastLoadError)
	}
	if strings.TrimSpace(s.lastEvent.Event) != "" {
		lines = append(lines,
			fmt.Sprintf("last_event=%s", s.lastEvent.Event),
			fmt.Sprintf("last_hook=%s", s.lastEvent.Hook),
			fmt.Sprintf("last_result=%s", s.lastEvent.Result),
		)
		if strings.TrimSpace(s.lastEvent.Target) != "" {
			lines = append(lines, "last_target="+s.lastEvent.Target)
		}
		if strings.TrimSpace(s.lastEvent.Timestamp) != "" {
			lines = append(lines, "last_event_at="+s.lastEvent.Timestamp)
		}
		if strings.TrimSpace(s.lastEvent.Error) != "" {
			lines = append(lines, "last_event_error="+s.lastEvent.Error)
		}
	}
	if len(s.hooks) > 0 {
		lines = append(lines, "", "hooks:")
		for _, hook := range s.hooks {
			line := fmt.Sprintf("- %s [%s] %s", hook.Event, hook.Source, hook.Status)
			if hook.Enabled {
				line += " enabled=true"
			}
			lines = append(lines, line)
			if strings.TrimSpace(hook.Command) != "" {
				lines = append(lines, "  command="+hook.Command)
			}
			meta := []string{}
			if strings.TrimSpace(hook.Matcher) != "" {
				meta = append(meta, "matcher="+hook.Matcher)
			}
			if hook.TimeoutMs > 0 {
				meta = append(meta, fmt.Sprintf("timeout_ms=%d", hook.TimeoutMs))
			}
			if hook.Blocking {
				meta = append(meta, "blocking=true")
			}
			if strings.TrimSpace(hook.Shell) != "" {
				meta = append(meta, "shell="+hook.Shell)
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+strings.Join(meta, "  "))
			}
			runtimeMeta := []string{}
			if hook.RunCount > 0 {
				runtimeMeta = append(runtimeMeta, fmt.Sprintf("run_count=%d", hook.RunCount))
			}
			if strings.TrimSpace(hook.LastRunAt) != "" {
				runtimeMeta = append(runtimeMeta, "last_run_at="+hook.LastRunAt)
			}
			if strings.TrimSpace(hook.LastResult) != "" {
				runtimeMeta = append(runtimeMeta, "last_result="+hook.LastResult)
			}
			if len(runtimeMeta) > 0 {
				lines = append(lines, "  "+strings.Join(runtimeMeta, "  "))
			}
			if strings.TrimSpace(hook.LastError) != "" {
				lines = append(lines, "  last_error="+hook.LastError)
			}
			if strings.TrimSpace(hook.LastOutput) != "" {
				lines = append(lines, "  last_output="+hook.LastOutput)
			}
			if strings.TrimSpace(hook.Description) != "" {
				lines = append(lines, "  "+hook.Description)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func hookMatches(hook Hook, target string) bool {
	matcher := strings.TrimSpace(hook.Matcher)
	if matcher == "" || matcher == "*" {
		return true
	}
	if target == "" {
		return false
	}
	ok, err := path.Match(matcher, target)
	if err == nil && ok {
		return true
	}
	return matcher == target
}

func (s *HooksService) executeHook(ctx context.Context, hook *Hook, event engine.HookEvent) engine.HookExecution {
	timeout := time.Duration(hook.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := runHookCommand(runCtx, *hook, event)
	now := time.Now().Format(time.RFC3339)
	hook.RunCount++
	hook.LastRunAt = now
	hook.LastOutput = strings.TrimSpace(output)
	hook.LastError = ""
	hook.LastResult = "ok"

	report := engine.HookExecution{
		Event:     event.Name,
		Target:    event.Target,
		Hook:      hook.Event,
		Blocking:  hook.Blocking,
		Result:    "ok",
		Output:    strings.TrimSpace(output),
		Timestamp: now,
		Payload:   event.Payload,
	}
	if err != nil {
		hook.LastResult = "error"
		hook.LastError = err.Error()
		report.Result = "error"
		report.Error = err.Error()
	}
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
	return report
}

func runHookCommand(ctx context.Context, hook Hook, event engine.HookEvent) (string, error) {
	command := strings.TrimSpace(hook.Command)
	if command == "" {
		return "", fmt.Errorf("empty hook command")
	}
	shell := strings.TrimSpace(hook.Shell)
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.CommandContext(ctx, shell, "-lc", command)
	cmd.Env = append(cmd.Environ(),
		"CLAUDE_CODE_HOOK_EVENT="+event.Name,
		"CLAUDE_CODE_HOOK_TARGET="+event.Target,
	)
	data, err := cmd.CombinedOutput()
	return string(data), err
}
