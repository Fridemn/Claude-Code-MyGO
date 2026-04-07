package tests

import (
	"context"
	"strings"
	"testing"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/command"
	cmdagent "claude-code-go/internal/command/agent"
	cmdmeta "claude-code-go/internal/command/meta"
	cmdprompt "claude-code-go/internal/command/prompt"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/session"
	"claude-code-go/internal/tool"
)

func TestPromptCommands(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdprompt.Register(registry)
	cmdmeta.Register(registry)

	out, ok, err := registry.Execute(context.Background(), "/review src/module", command.Runtime{})
	if err != nil || !ok {
		t.Fatalf("/review failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "code review mindset") || !strings.Contains(out.Value, "src/module") {
		t.Fatalf("unexpected /review output: %s", out.Value)
	}

	out, ok, err = registry.Execute(context.Background(), "/init", command.Runtime{})
	if err != nil || !ok {
		t.Fatalf("/init failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "Understand this repository") {
		t.Fatalf("unexpected /init output: %s", out.Value)
	}
}

func TestAgentCommandsLifecycle(t *testing.T) {
	t.Parallel()

	registry := command.EmptyRegistry()
	cmdagent.Register(registry)
	cmdmeta.Register(registry)

	sessions, err := session.CreateManager(t.TempDir())
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	manager := agent.CreateManager(
		config.Config{
			Model:      "test-model",
			MaxTurns:   8,
			SessionDir: t.TempDir(),
		},
		&scriptedProvider{
			responses: toEngineResponses([]string{
				"agent one",
				"agent two",
			}),
		},
		tool.EmptyRegistry(),
		sessions,
		nil,
		nil,
	)

	runtime := command.Runtime{
		Agents: manager,
	}

	out, ok, err := registry.Execute(context.Background(), "/agents", runtime)
	if err != nil || !ok {
		t.Fatalf("/agents failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "general-purpose") {
		t.Fatalf("unexpected /agents output: %s", out.Value)
	}

	out, ok, err = registry.Execute(context.Background(), "/agent general-purpose hello world", runtime)
	if err != nil || !ok {
		t.Fatalf("/agent failed: ok=%t err=%v", ok, err)
	}
	if !strings.Contains(out.Value, "agent completed") || !strings.Contains(out.Value, "agent one") {
		t.Fatalf("unexpected /agent output: %s", out.Value)
	}

	tasks := manager.Tasks().List()
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %d", len(tasks))
	}
	taskID := tasks[0].ID

	checkCommandContains(t, registry, runtime, "/tasks", taskID)
	checkCommandContains(t, registry, runtime, "/task "+taskID, "status=")
	checkCommandContains(t, registry, runtime, "/tasklog "+taskID, "ASSISTANT")
	checkCommandContains(t, registry, runtime, "/send "+taskID+" follow up", "agent continued")
	checkCommandContains(t, registry, runtime, "/resume "+taskID+" final follow up", "agent continued")
	checkCommandContains(t, registry, runtime, "/wait "+taskID, "agent finished")

}

func toEngineResponses(values []string) []engine.Response {
	out := make([]engine.Response, 0, len(values))
	for _, value := range values {
		out = append(out, engine.Response{Text: value})
	}
	return out
}
