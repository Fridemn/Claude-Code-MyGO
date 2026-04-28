package tests

import (
	"context"
	"testing"
	"time"

	"claude-go/internal/provider"
	"claude-go/internal/query"
)

func TestQueryLoopCreation(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	if loop == nil {
		t.Fatal("NewQueryLoop returned nil")
	}

	if loop.SessionID() != "test-session" {
		t.Errorf("SessionID mismatch: got %s", loop.SessionID())
	}

	if loop.GetTurnCount() != 0 {
		t.Errorf("Initial turn count should be 0, got %d", loop.GetTurnCount())
	}
}

func TestQueryLoopSystemMessage(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.AddSystemMessage("You are a helpful assistant.")

	messages := loop.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("Role should be 'system', got %s", messages[0].Role)
	}

	if messages[0].Content != "You are a helpful assistant." {
		t.Errorf("Content mismatch: got %s", messages[0].Content)
	}
}

func TestQueryLoopUserMessage(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.AddUserMessage("Hello, how are you?")

	messages := loop.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}

	if messages[0].Role != "user" {
		t.Errorf("Role should be 'user', got %s", messages[0].Role)
	}
}

func TestQueryLoopQuery(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	// Set up a simple provider
	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	ctx := context.Background()
	result, err := loop.Query(ctx, "Hello!")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result == nil {
		t.Fatal("Query result is nil")
	}

	if result.Response == "" {
		t.Error("Response should not be empty")
	}

	if result.Turn != 1 {
		t.Errorf("Turn should be 1, got %d", result.Turn)
	}
}

func TestQueryLoopQueryStream(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	ctx := context.Background()
	var streamedContent string

	result, err := loop.QueryStream(ctx, "Hello!", func(content string) error {
		streamedContent += content
		return nil
	})

	if err != nil {
		t.Fatalf("QueryStream failed: %v", err)
	}

	if result == nil {
		t.Fatal("QueryStream result is nil")
	}

	if streamedContent == "" {
		t.Error("Streamed content should not be empty")
	}
}

func TestQueryLoopMaxTurns(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  2,
		SessionID: "test-session",
	})

	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	ctx := context.Background()

	// First query should succeed
	result1, err := loop.Query(ctx, "Turn 1")
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}
	if result1.Turn != 1 {
		t.Errorf("Turn should be 1, got %d", result1.Turn)
	}

	// Second query should succeed
	result2, err := loop.Query(ctx, "Turn 2")
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}
	if result2.Turn != 2 {
		t.Errorf("Turn should be 2, got %d", result2.Turn)
	}

	// Third query should hit max turns
	result3, err := loop.Query(ctx, "Turn 3")
	if err != nil {
		t.Fatalf("Turn 3 failed: %v", err)
	}
	if result3.StopReason != "max_turns" {
		t.Errorf("Stop reason should be 'max_turns', got %s", result3.StopReason)
	}
}

func TestQueryLoopReset(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.AddSystemMessage("System prompt")
	loop.AddUserMessage("Hello")
	loop.AddAssistantMessage("Hi there!")

	if loop.GetMessageCount() != 3 {
		t.Errorf("Message count should be 3, got %d", loop.GetMessageCount())
	}

	loop.Reset()

	if loop.GetMessageCount() != 0 {
		t.Errorf("Message count after reset should be 0, got %d", loop.GetMessageCount())
	}

	if loop.GetTurnCount() != 0 {
		t.Errorf("Turn count after reset should be 0, got %d", loop.GetTurnCount())
	}
}

func TestQueryLoopResetExceptSystem(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.AddSystemMessage("System prompt")
	loop.AddUserMessage("Hello")
	loop.AddAssistantMessage("Hi there!")

	loop.ResetExceptSystem()

	if loop.GetMessageCount() != 1 {
		t.Errorf("Message count after reset should be 1, got %d", loop.GetMessageCount())
	}

	messages := loop.GetMessages()
	if messages[0].Role != "system" {
		t.Errorf("Only system message should remain, got role: %s", messages[0].Role)
	}
}

func TestQueryLoopEstimateTokenCount(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	// Empty loop
	if loop.EstimateTokenCount() != 0 {
		t.Errorf("Token count should be 0 for empty loop")
	}

	// Add some content
	loop.AddUserMessage("This is a test message for token estimation.")

	tokens := loop.EstimateTokenCount()
	if tokens <= 0 {
		t.Errorf("Token count should be positive, got %d", tokens)
	}
}

func TestQueryLoopHistory(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.AddUserMessage("Hello")
	loop.AddAssistantMessage("Hi there!")

	history := loop.History()
	if len(history) != 2 {
		t.Errorf("History should have 2 entries, got %d", len(history))
	}

	if history[0].Role != "user" {
		t.Errorf("First history entry should be user, got %s", history[0].Role)
	}

	if history[1].Role != "assistant" {
		t.Errorf("Second history entry should be assistant, got %s", history[1].Role)
	}
}

func TestSimpleProvider(t *testing.T) {
	t.Parallel()

	p := provider.NewSimpleProvider("test", "test-model")

	if p.Name() != "test" {
		t.Errorf("Name mismatch: got %s", p.Name())
	}

	if p.Model() != "test-model" {
		t.Errorf("Model mismatch: got %s", p.Model())
	}

	ctx := context.Background()
	resp, err := p.Complete(ctx, []provider.Message{
		{Role: "user", Content: "Hello!"},
	})

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if resp.Content == "" {
		t.Error("Response content should not be empty")
	}
}

func TestQueryLoopNoProvider(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	// Don't set a provider
	ctx := context.Background()
	_, err := loop.Query(ctx, "Hello!")

	if err == nil {
		t.Error("Expected error when no provider is set")
	}
}

func TestQueryLoopToolCalls(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  10,
		SessionID: "test-session",
	})

	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	// Simulate a response with a tool call
	loop.AddUserMessage("Read the file")
	loop.AddAssistantMessage("Tool: Read\nArgs: {\"file_path\": \"test.txt\"}")

	ctx := context.Background()
	result, err := loop.Query(ctx, "test")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// The mock response may or may not contain tool calls depending on content
	// Just verify the structure is correct
	if result.Turn < 1 {
		t.Error("Turn should be at least 1")
	}
}

func TestQueryLoopMultipleTurns(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  100,
		SessionID: "test-session",
	})

	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	ctx := context.Background()

	// Simulate multiple turns
	for i := 1; i <= 5; i++ {
		_, err := loop.Query(ctx, "Turn "+string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Turn %d failed: %v", i, err)
		}
	}

	if loop.GetTurnCount() != 5 {
		t.Errorf("Turn count should be 5, got %d", loop.GetTurnCount())
	}

	if loop.GetMessageCount() != 10 { // 5 user + 5 assistant
		t.Errorf("Message count should be 10, got %d", loop.GetMessageCount())
	}
}

func TestQueryLoopMessageAccumulation(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  100,
		SessionID: "test-session",
	})

	loop.AddSystemMessage("System")

	// Simulate conversation
	turns := 3
	for i := 0; i < turns; i++ {
		loop.AddUserMessage("Message from user")
		loop.AddAssistantMessage("Response from assistant")
	}

	messages := loop.GetMessages()
	if len(messages) != 1+turns*2 { // system + (user+assistant) * turns
		t.Errorf("Expected %d messages, got %d", 1+turns*2, len(messages))
	}
}

func TestQueryLoopConcurrency(t *testing.T) {
	t.Parallel()

	loop := query.NewQueryLoop(query.QueryConfig{
		MaxTurns:  100,
		SessionID: "test-session",
	})

	loop.SetProvider(provider.NewSimpleProvider("test", "test-model"))

	// Run concurrent queries
	done := make(chan bool)
	ctx := context.Background()

	// Start multiple goroutines
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				loop.AddUserMessage("concurrent message")
				loop.Query(ctx, "test")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for concurrent operations")
		}
	}
}
