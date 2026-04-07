package agent

import (
	"fmt"
	"strings"
	"time"

	"claude-code-go/internal/command"
	"claude-code-go/internal/task"
)

func formatTaskList(tasks []*task.AgentTask) string {
	if len(tasks) == 0 {
		return "no tasks"
	}

	lines := make([]string, 0, len(tasks)*2)
	for _, t := range tasks {
		lines = append(lines, fmt.Sprintf("%s  %s  %s", t.ID, t.AgentType, t.Status))
		lines = append(lines, fmt.Sprintf("  model=%s  bg=%t  turns=%d  age=%s  desc=%s", command.EmptyDash(t.Model), t.Background, t.TurnCount, formatTaskAge(t), t.Description))
		if strings.TrimSpace(t.Summary) != "" {
			lines = append(lines, fmt.Sprintf("  summary=%s", t.Summary))
		}
	}
	return strings.Join(lines, "\n")
}

func formatTaskDetail(t *task.AgentTask) string {
	lines := []string{
		fmt.Sprintf("id=%s", t.ID),
		fmt.Sprintf("type=%s", t.Type),
		fmt.Sprintf("status=%s", t.Status),
		fmt.Sprintf("agent=%s", t.AgentType),
		fmt.Sprintf("model=%s", command.EmptyDash(t.Model)),
		fmt.Sprintf("background=%t", t.Background),
		fmt.Sprintf("turns=%d", t.TurnCount),
		fmt.Sprintf("started=%s", t.StartTime.Format(time.RFC3339)),
		fmt.Sprintf("ended=%s", formatEndTime(t.EndTime)),
		fmt.Sprintf("updated=%s", formatEndTime(t.UpdatedAt)),
		fmt.Sprintf("duration=%s", formatDuration(t)),
		fmt.Sprintf("description=%s", t.Description),
	}

	if strings.TrimSpace(t.Summary) != "" {
		lines = append(lines, fmt.Sprintf("summary=%s", t.Summary))
	}

	lines = append(lines, "", "prompt:", t.Prompt)

	if strings.TrimSpace(t.LastUserPrompt) != "" {
		lines = append(lines, "", "last_user_prompt:", t.LastUserPrompt)
	}
	if strings.TrimSpace(t.LastAssistantReply) != "" {
		lines = append(lines, "", "last_assistant_reply:", t.LastAssistantReply)
	}

	if strings.TrimSpace(t.Output) != "" {
		lines = append(lines, "", "output:", t.Output)
	}
	if strings.TrimSpace(t.Error) != "" {
		lines = append(lines, "", "error:", t.Error)
	}
	if len(t.Messages) > 0 {
		lines = append(lines, "", fmt.Sprintf("messages=%d", len(t.Messages)))
	}
	return strings.Join(lines, "\n")
}

func formatTaskTranscript(t *task.AgentTask) string {
	if len(t.Messages) == 0 {
		return "no transcript recorded"
	}
	lines := make([]string, 0, len(t.Messages)*3)
	for i, msg := range t.Messages {
		lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, strings.ToUpper(msg.Role)))
		lines = append(lines, msg.Content)
		if i < len(t.Messages)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func formatEndTime(v time.Time) string {
	if v.IsZero() {
		return "-"
	}
	return v.Format(time.RFC3339)
}

func formatDuration(t *task.AgentTask) string {
	if t.EndTime.IsZero() {
		return time.Since(t.StartTime).Round(time.Second).String()
	}
	return t.EndTime.Sub(t.StartTime).Round(time.Millisecond).String()
}

func formatTaskAge(t *task.AgentTask) string {
	return time.Since(t.StartTime).Round(time.Second).String()
}