package tests

import (
	"testing"

	"claude-code-go/internal/tool/bash"
)

func TestSecurityValidatorDetectsDangerousPatterns(t *testing.T) {
	t.Parallel()

	validator := bash.CreateSecurityValidator(bash.SecurityValidatorOptions{
		ShellType: "bash",
	})

	cases := []struct {
		name    string
		command string
		level   bash.SecurityLevel
		checkID string
	}{
		{
			name:    "destructive rm",
			command: "rm -rf /tmp/demo",
			level:   bash.SecurityLevelAsk,
			checkID: bash.CheckIDDestructiveCommand,
		},
		{
			name:    "network curl",
			command: "curl https://example.com",
			level:   bash.SecurityLevelWarning,
			checkID: bash.CheckIDDestructiveCommand,
		},
		{
			name:    "command substitution",
			command: "echo $(whoami)",
			level:   bash.SecurityLevelWarning,
			checkID: bash.CheckIDMalformedTokens,
		},
		{
			name:    "unbalanced quotes",
			command: `echo "hello`,
			level:   bash.SecurityLevelAsk,
			checkID: bash.CheckIDUnbalancedQuotes,
		},
		{
			name:    "tab fragment",
			command: "\t--help",
			level:   bash.SecurityLevelAsk,
			checkID: bash.CheckIDIncompleteCommand,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := validator.Validate(tc.command)
			if result.Level != tc.level {
				t.Fatalf("unexpected level for %q: got %v want %v", tc.command, result.Level, tc.level)
			}
			if result.CheckID != tc.checkID {
				t.Fatalf("unexpected check id for %q: got %s want %s", tc.command, result.CheckID, tc.checkID)
			}
		})
	}
}

func TestSecurityValidatorAllowsConfiguredCommands(t *testing.T) {
	t.Parallel()

	validator := bash.CreateSecurityValidator(bash.SecurityValidatorOptions{
		AllowDestructive: true,
		AllowNetwork:     true,
		AllowSystem:      true,
		ShellType:        "bash",
	})

	for _, command := range []string{
		"rm -rf /tmp/demo",
		"curl https://example.com",
		"systemctl status ssh",
	} {
		result := validator.Validate(command)
		if result.Level != bash.SecurityLevelSafe {
			t.Fatalf("expected command %q to be safe, got %#v", command, result)
		}
	}
}

func TestSecurityValidatorZshDangerousCommandDenied(t *testing.T) {
	t.Parallel()

	validator := bash.CreateSecurityValidator(bash.SecurityValidatorOptions{
		ShellType: "zsh",
	})
	result := validator.Validate("zmodload zsh/system")
	if result.Level != bash.SecurityLevelDeny {
		t.Fatalf("expected deny for dangerous zsh command, got %#v", result)
	}
	if result.CheckID != bash.CheckIDZshExtension {
		t.Fatalf("unexpected check id: %s", result.CheckID)
	}
}

func TestValidateCommandSecurityHelpers(t *testing.T) {
	t.Parallel()

	if !bash.IsCommandSafe("echo hello") {
		t.Fatalf("expected echo to be safe")
	}
	if bash.IsCommandSafe("rm -rf /tmp/demo") {
		t.Fatalf("expected rm to be unsafe by default")
	}
	suggestions := bash.GetSecuritySuggestions(`echo "unterminated`)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions for unsafe command")
	}
}
