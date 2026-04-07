package tests

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"claude-code-go/internal/cli"
)

func TestStructuredIORenderPromptAndReadLine(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("first line\nsecond line\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	ioLayer := cli.CreateStructuredIO(input, &stdout, &stderr)
	if err := ioLayer.Render("screen"); err != nil {
		t.Fatalf("render: %v", err)
	}
	if err := ioLayer.Prompt("prompt> "); err != nil {
		t.Fatalf("prompt: %v", err)
	}

	line, err := ioLayer.ReadLine()
	if err != nil {
		t.Fatalf("read first line: %v", err)
	}
	if line != "first line" {
		t.Fatalf("unexpected first line: %q", line)
	}

	line, err = ioLayer.ReadLine()
	if err != nil {
		t.Fatalf("read second line: %v", err)
	}
	if line != "second line" {
		t.Fatalf("unexpected second line: %q", line)
	}

	_, err = ioLayer.ReadLine()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}

	if stdout.String() != "screenprompt> " {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if ioLayer.Stderr() != &stderr {
		t.Fatalf("unexpected stderr writer")
	}
}

