package cli

import (
	"testing"

	"claude-go/internal/tool/interaction"
)

func TestIsPermissionQuestion(t *testing.T) {
	if !isPermissionQuestion(interaction.Question{
		Header: "Permission",
		Options: []interaction.QuestionOption{
			{Label: "Allow once (Recommended)"},
			{Label: "Always allow this command"},
			{Label: "Deny"},
		},
	}) {
		t.Fatalf("expected Permission header to be recognized")
	}

	if !isPermissionQuestion(interaction.Question{
		Header: "Action",
		Options: []interaction.QuestionOption{
			{Label: "Allow this"},
			{Label: "Deny"},
		},
	}) {
		t.Fatalf("expected allow/deny options to be recognized as permission prompt")
	}
}

func TestParsePermissionQuestion(t *testing.T) {
	title, command, reason, hint, extras := parsePermissionQuestion("Bash needs permission to run:\nrm /tmp/test.json\nReason: command may modify files\nHint: Review before approving")
	if title != "Bash Permission" {
		t.Fatalf("unexpected title: %q", title)
	}
	if command != "rm /tmp/test.json" {
		t.Fatalf("unexpected command: %q", command)
	}
	if reason != "command may modify files" {
		t.Fatalf("unexpected reason: %q", reason)
	}
	if hint != "Review before approving" {
		t.Fatalf("unexpected hint: %q", hint)
	}
	if len(extras) != 0 {
		t.Fatalf("expected no extra lines, got %#v", extras)
	}
}

func TestFindQuestionOptionIndex(t *testing.T) {
	options := []interaction.QuestionOption{
		{Label: "Allow once (Recommended)"},
		{Label: "Always allow this command"},
		{Label: "Deny"},
	}

	if got := findQuestionOptionIndex(options, "allow once", "allow"); got != 0 {
		t.Fatalf("expected allow once index 0, got %d", got)
	}
	if got := findQuestionOptionIndex(options, "always allow"); got != 1 {
		t.Fatalf("expected always allow index 1, got %d", got)
	}
	if got := findQuestionOptionIndex(options, "deny"); got != 2 {
		t.Fatalf("expected deny index 2, got %d", got)
	}
}

func TestIsPermissionPotentiallyDangerous(t *testing.T) {
	if !isPermissionPotentiallyDangerous("rm /tmp/demo", "") {
		t.Fatalf("expected rm command to be marked dangerous")
	}
	if isPermissionPotentiallyDangerous("ls -la", "read-only command") {
		t.Fatalf("did not expect ls command to be marked dangerous")
	}
}
