package tests

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"claude-go/cmd"
	"claude-go/internal/provider"
	"claude-go/internal/query"
)

func TestCLIVersion(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{"--version"})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Error("Version output should not be empty")
	}
}

func TestCLIHelp(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{"--help"})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Usage:")) {
		t.Error("Help output should contain 'Usage:'")
	}

	if !bytes.Contains([]byte(output), []byte("version")) {
		t.Error("Help output should contain 'version'")
	}
}

func TestCLIShortFlags(t *testing.T) {
	t.Parallel()

	// Test -v short flag
	var stdout1, stderr1 bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout1, &stderr1, []string{"-v"})
	if err != nil {
		t.Fatalf("RunCLI with -v failed: %v", err)
	}
	if stdout1.String() == "" {
		t.Error("-v should output version")
	}

	// Test -h short flag
	var stdout2, stderr2 bytes.Buffer
	err = cmd.RunCLI(context.Background(), &stdout2, &stderr2, []string{"-h"})
	if err != nil {
		t.Fatalf("RunCLI with -h failed: %v", err)
	}
	if !bytes.Contains(stdout2.Bytes(), []byte("Usage:")) {
		t.Error("-h should output help")
	}
}

func TestCLIPrintModeNoAPIKey(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{"-p", "hello"})

	if err == nil {
		t.Error("Should fail without API key")
	}
}

func TestCLIPrintModeSimpleProvider(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p",
		"hello",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	output := stdout.String()
	if output == "" {
		t.Error("Output should not be empty")
	}
}

func TestCLIPrintModeJSON(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p",
		"test message",
		"--json",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte(`"response":`)) {
		t.Errorf("JSON output should contain 'response', got: %s", output)
	}

	if !bytes.Contains([]byte(output), []byte(`"stop_reason":`)) {
		t.Errorf("JSON output should contain 'stop_reason', got: %s", output)
	}
}

func TestCLIParseArgsModel(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p", "test",
		"--model", "gpt-4o",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}
}

func TestCLIParseArgsBaseURL(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p", "test",
		"--base-url", "https://custom.api.com/v1",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}
}

func TestCLIParseArgsMaxTurns(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p", "test",
		"--max-turns", "50",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}
}

func TestCLISystemPrompt(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p", "test",
		"--system", "You are a helpful coding assistant.",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Fatalf("RunCLI failed: %v", err)
	}
}

func TestCLIProviderSimple(t *testing.T) {
	t.Parallel()

	// Only test simple provider as it doesn't require real API calls
	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p", "test",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	if err != nil {
		t.Errorf("Simple provider failed: %v", err)
	}
}

func TestCLIEmptyPrompt(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	err := cmd.RunCLI(context.Background(), &stdout, &stderr, []string{
		"-p",
		"--provider", "simple",
		"--api-key", "test-key",
	})

	// Should fail because prompt is empty in print mode
	if err == nil {
		t.Error("Should fail with empty prompt in print mode")
	}
}

// TestStreamingQuery tests the streaming query functionality
func TestStreamingQuery(t *testing.T) {
	t.Parallel()

	// Create a query loop with simple provider
	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-streaming",
	})
	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	// Test streaming with handler
	var received strings.Builder
	handler := func(content string) error {
		received.WriteString(content)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := loop.QueryStream(ctx, "hello", handler)
	if err != nil {
		t.Fatalf("QueryStream failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Response == "" {
		t.Error("Response should not be empty")
	}

	// Verify streaming handler received content
	streamedContent := received.String()
	if streamedContent == "" {
		t.Error("Streaming handler should have received content")
	}

	// Streaming content should match final response
	if streamedContent != result.Response {
		t.Errorf("Streamed content mismatch: got %q, want %q", streamedContent, result.Response)
	}
}

// TestStreamingQueryCancellation tests that streaming can be cancelled
func TestStreamingQueryCancellation(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-cancel",
	})
	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var received strings.Builder
	handler := func(content string) error {
		received.WriteString(content)
		return nil
	}

	_, err := loop.QueryStream(ctx, "hello", handler)
	if err == nil {
		t.Error("Should fail with cancelled context")
	}
}

// TestStreamingQueryMultipleChunks tests streaming with multiple chunks
func TestStreamingQueryMultipleChunks(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-multi-chunk",
	})
	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	var chunkCount int
	handler := func(content string) error {
		if content != "" {
			chunkCount++
		}
		return nil
	}

	ctx := context.Background()
	result, err := loop.QueryStream(ctx, "tell me a story", handler)
	if err != nil {
		t.Fatalf("QueryStream failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Simple provider should produce multiple chunks
	if chunkCount == 0 {
		t.Error("Should have received at least one chunk")
	}
}
