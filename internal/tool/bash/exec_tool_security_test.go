package bash

import (
	"strings"
	"testing"
)

func TestValidateCommandSecurityReturnsPromptOverrideForAskLevelChecks(t *testing.T) {
	override, err := validateCommandSecurity("rm /tmp/demo-file")
	if err != nil {
		t.Fatalf("expected destructive command to require approval, got error: %v", err)
	}
	if override == nil {
		t.Fatalf("expected prompt override for destructive command")
	}
	if !strings.Contains(strings.ToLower(override.Reason), "destructive") {
		t.Fatalf("expected destructive reason, got %q", override.Reason)
	}
}

func TestValidateCommandSecurityAllowsSafeCommandWithoutOverride(t *testing.T) {
	override, err := validateCommandSecurity("ls -la")
	if err != nil {
		t.Fatalf("expected safe command to pass security validation, got %v", err)
	}
	if override != nil {
		t.Fatalf("did not expect prompt override for safe command, got %#v", override)
	}
}
