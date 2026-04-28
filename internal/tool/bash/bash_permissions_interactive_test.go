package bash

import (
	"fmt"
	"strings"
	"testing"

	"claude-go/internal/tool/interaction"
)

type chooseOptionHandler struct {
	index int
	calls int
}

func (h *chooseOptionHandler) AskQuestion(q interaction.Question) (string, error) {
	h.calls++
	if len(q.Options) == 0 {
		return "", fmt.Errorf("no options available")
	}
	if h.index < 0 || h.index >= len(q.Options) {
		return "", fmt.Errorf("option index out of range: %d", h.index)
	}
	return q.Options[h.index].Label, nil
}

func (h *chooseOptionHandler) AskMultiSelect(q interaction.Question) ([]string, error) {
	h.calls++
	if len(q.Options) == 0 {
		return nil, fmt.Errorf("no options available")
	}
	if h.index < 0 || h.index >= len(q.Options) {
		return nil, fmt.Errorf("option index out of range: %d", h.index)
	}
	return []string{q.Options[h.index].Label}, nil
}

func preparePermissionPromptTest(t *testing.T) *PermissionChecker {
	t.Helper()

	checker := GetPermissionChecker()
	originalMode := checker.GetMode()
	originalRules := checker.RulesSnapshot()
	interaction.SetUserInputHandler(nil)
	checker.ClearRules()
	checker.SetMode(PermissionModeAsk)

	t.Cleanup(func() {
		interaction.SetUserInputHandler(nil)
		checker.ClearRules()
		checker.AddRules(originalRules)
		checker.SetMode(originalMode)
	})

	return checker
}

func TestCheckGlobalPermissionPromptAllowOnce(t *testing.T) {
	checker := preparePermissionPromptTest(t)
	handler := &chooseOptionHandler{index: 0}
	interaction.SetUserInputHandler(handler)

	result := CheckGlobalPermission("rm /tmp/test-file", "remove test file")
	if !result.Allowed {
		t.Fatalf("expected allow-once decision to allow command, got %#v", result)
	}
	if handler.calls != 1 {
		t.Fatalf("expected one interactive prompt, got %d", handler.calls)
	}
	if rules := checker.RulesSnapshot(); len(rules) != 0 {
		t.Fatalf("allow-once should not persist a rule, got %d rules", len(rules))
	}
}

func TestCheckGlobalPermissionPromptAlwaysAllowPersistsSessionRule(t *testing.T) {
	checker := preparePermissionPromptTest(t)
	handler := &chooseOptionHandler{index: 1}
	interaction.SetUserInputHandler(handler)

	command := "rm /tmp/test-file"
	first := CheckGlobalPermission(command, "remove test file")
	if !first.Allowed {
		t.Fatalf("expected always-allow decision to allow command, got %#v", first)
	}
	if handler.calls != 1 {
		t.Fatalf("expected one interactive prompt, got %d", handler.calls)
	}

	rules := checker.RulesSnapshot()
	if len(rules) != 1 {
		t.Fatalf("expected one persisted session rule, got %d", len(rules))
	}
	if rules[0].Pattern != command || rules[0].Behavior != BehaviorAllow {
		t.Fatalf("unexpected persisted rule: %#v", rules[0])
	}

	interaction.SetUserInputHandler(nil)
	second := CheckGlobalPermission(command, "remove test file")
	if !second.Allowed {
		t.Fatalf("expected second check to be auto-allowed by session rule, got %#v", second)
	}
}

func TestCheckGlobalPermissionPromptDeny(t *testing.T) {
	preparePermissionPromptTest(t)
	handler := &chooseOptionHandler{index: 2}
	interaction.SetUserInputHandler(handler)

	result := CheckGlobalPermission("rm /tmp/test-file", "remove test file")
	if result.Allowed {
		t.Fatalf("expected deny decision to reject command, got %#v", result)
	}
	if !strings.Contains(strings.ToLower(result.Reason), "denied") {
		t.Fatalf("expected denial reason, got %q", result.Reason)
	}
	if handler.calls != 1 {
		t.Fatalf("expected one interactive prompt, got %d", handler.calls)
	}
}

func TestCheckGlobalPermissionWithPromptOverridePromptsWhenBaseWouldAllow(t *testing.T) {
	checker := preparePermissionPromptTest(t)
	checker.SetMode(PermissionModeBypassPermissions)
	handler := &chooseOptionHandler{index: 0}
	interaction.SetUserInputHandler(handler)

	result := CheckGlobalPermissionWithPromptOverride("ls -la", "list files", &PermissionPromptOverride{
		Reason: "Command requires explicit approval",
	})
	if !result.Allowed {
		t.Fatalf("expected override prompt allow-once to allow command, got %#v", result)
	}
	if handler.calls != 1 {
		t.Fatalf("expected one interactive prompt, got %d", handler.calls)
	}
}

func TestCheckGlobalPermissionWithPromptOverrideFailsClosedWithoutHandler(t *testing.T) {
	checker := preparePermissionPromptTest(t)
	checker.SetMode(PermissionModeBypassPermissions)
	interaction.SetUserInputHandler(nil)

	result := CheckGlobalPermissionWithPromptOverride("ls -la", "list files", &PermissionPromptOverride{
		Reason: "Command requires explicit approval",
	})
	if result.Allowed {
		t.Fatalf("expected override prompt to deny when no handler is configured, got %#v", result)
	}
	if !strings.Contains(strings.ToLower(result.Reason), "explicit approval") {
		t.Fatalf("expected override reason in denial, got %q", result.Reason)
	}
}
