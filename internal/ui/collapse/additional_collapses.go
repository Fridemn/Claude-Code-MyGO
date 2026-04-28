package collapse

import (
	"encoding/json"
	"fmt"
	"strings"

	"claude-go/internal/types"
	"claude-go/internal/ui"
)

const (
	teammateShutdownBatchType    = "teammate_shutdown_batch"
	taskStatusType               = "task_status"
	inProcessTeammateTaskType    = "in_process_teammate"
	completedStatus              = "completed"
	taskNotificationTag          = "task-notification"
	statusTag                    = "status"
	summaryTag                   = "summary"
	backgroundBashSummaryPrefix  = "Background command "
	backgroundBashCollapsedLabel = "background commands completed"
)

// TeammateShutdowns collapses consecutive completed in-process teammate
// task_status attachments into one teammate_shutdown_batch attachment.
func TeammateShutdowns(entries []ui.TranscriptEntry) []ui.TranscriptEntry {
	result := make([]ui.TranscriptEntry, 0, len(entries))

	for i := 0; i < len(entries); {
		entry := entries[i]
		if !isTeammateShutdownAttachment(entry) {
			result = append(result, entry)
			i++
			continue
		}

		first := entry
		count := 0
		for i < len(entries) && isTeammateShutdownAttachment(entries[i]) {
			count++
			i++
		}

		if count == 1 {
			result = append(result, first)
			continue
		}

		payload := map[string]any{
			"type":  teammateShutdownBatchType,
			"count": count,
		}
		data := marshalJSON(payload)

		batch := first
		batch.Subtype = teammateShutdownBatchType
		batch.Data = data
		batch.Content = fmt.Sprintf("%d teammate tasks completed", count)
		result = append(result, batch)
	}

	return result
}

func isTeammateShutdownAttachment(entry ui.TranscriptEntry) bool {
	if entry.Kind != "attachment" {
		return false
	}

	payload, ok := parseEntryPayload(entry)
	if !ok {
		return false
	}

	attachmentType := getPayloadString(payload, "type", "attachmentType")
	if !strings.EqualFold(strings.TrimSpace(attachmentType), taskStatusType) {
		return false
	}

	taskType := getPayloadString(payload, "taskType", "task_type")
	if !strings.EqualFold(strings.TrimSpace(taskType), inProcessTeammateTaskType) {
		return false
	}

	status := getPayloadString(payload, "status")
	return strings.EqualFold(strings.TrimSpace(status), completedStatus)
}

// HookSummaries collapses consecutive stop_hook_summary system entries
// with the same hookLabel into one merged summary.
func HookSummaries(entries []ui.TranscriptEntry) []ui.TranscriptEntry {
	result := make([]ui.TranscriptEntry, 0, len(entries))

	for i := 0; i < len(entries); {
		entry := entries[i]
		label, ok := hookSummaryLabel(entry)
		if !ok {
			result = append(result, entry)
			i++
			continue
		}

		group := []ui.TranscriptEntry{entry}
		i++
		for i < len(entries) {
			nextLabel, nextOK := hookSummaryLabel(entries[i])
			if !nextOK || nextLabel != label {
				break
			}
			group = append(group, entries[i])
			i++
		}

		if len(group) == 1 {
			result = append(result, group[0])
			continue
		}

		result = append(result, mergeHookSummaryGroup(group))
	}

	return result
}

func hookSummaryLabel(entry ui.TranscriptEntry) (string, bool) {
	if entry.Kind != "system" || strings.TrimSpace(entry.Subtype) != "stop_hook_summary" {
		return "", false
	}
	payload, ok := parseEntryPayload(entry)
	if !ok {
		return "", false
	}
	label := strings.TrimSpace(getPayloadString(payload, "hookLabel"))
	if label == "" {
		return "", false
	}
	return label, true
}

func mergeHookSummaryGroup(group []ui.TranscriptEntry) ui.TranscriptEntry {
	merged := group[0]
	basePayload, _ := parseEntryPayload(merged)
	if basePayload == nil {
		basePayload = map[string]any{}
	}

	hookCount := 0
	hookInfos := make([]any, 0)
	hookErrors := make([]any, 0)
	preventedContinuation := false
	hasOutput := false
	maxTotalDurationMs := 0

	for _, item := range group {
		payload, ok := parseEntryPayload(item)
		if !ok {
			continue
		}

		hookCount += getIntFromAny(payload["hookCount"])
		if infos, ok := payload["hookInfos"].([]any); ok {
			hookInfos = append(hookInfos, infos...)
		}
		if errors, ok := payload["hookErrors"].([]any); ok {
			hookErrors = append(hookErrors, errors...)
		}
		preventedContinuation = preventedContinuation || getBoolFromAny(payload["preventedContinuation"])
		hasOutput = hasOutput || getBoolFromAny(payload["hasOutput"])
		totalDurationMs := getIntFromAny(payload["totalDurationMs"])
		if totalDurationMs > maxTotalDurationMs {
			maxTotalDurationMs = totalDurationMs
		}
	}

	basePayload["hookCount"] = hookCount
	basePayload["hookInfos"] = hookInfos
	basePayload["hookErrors"] = hookErrors
	basePayload["preventedContinuation"] = preventedContinuation
	basePayload["hasOutput"] = hasOutput
	basePayload["totalDurationMs"] = maxTotalDurationMs

	merged.Data = marshalJSON(basePayload)
	return merged
}

// BackgroundBashNotifications collapses consecutive successful background bash
// task notifications into one synthetic summary notification.
func BackgroundBashNotifications(entries []ui.TranscriptEntry) []ui.TranscriptEntry {
	result := make([]ui.TranscriptEntry, 0, len(entries))

	for i := 0; i < len(entries); {
		entry := entries[i]
		if !isCompletedBackgroundBashNotification(entry) {
			result = append(result, entry)
			i++
			continue
		}

		first := entry
		count := 0
		for i < len(entries) && isCompletedBackgroundBashNotification(entries[i]) {
			count++
			i++
		}

		if count == 1 {
			result = append(result, first)
			continue
		}

		synthesized := first
		synthesized.Content = fmt.Sprintf(
			"<%s><%s>%s</%s><%s>%d %s</%s></%s>",
			taskNotificationTag,
			statusTag, completedStatus, statusTag,
			summaryTag, count, backgroundBashCollapsedLabel, summaryTag,
			taskNotificationTag,
		)
		result = append(result, synthesized)
	}

	return result
}

func isCompletedBackgroundBashNotification(entry ui.TranscriptEntry) bool {
	if entry.Kind != "user" {
		return false
	}

	content := entry.Content
	if !strings.Contains(content, "<"+taskNotificationTag) {
		return false
	}

	if strings.TrimSpace(types.ExtractTag(content, statusTag)) != completedStatus {
		return false
	}

	summary := types.ExtractTag(content, summaryTag)
	return strings.HasPrefix(summary, backgroundBashSummaryPrefix)
}

func parseEntryPayload(entry ui.TranscriptEntry) (map[string]any, bool) {
	raw := strings.TrimSpace(entry.Data)
	if raw == "" {
		raw = strings.TrimSpace(entry.Content)
	}
	if raw == "" {
		return nil, false
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func getPayloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if value, ok := payload[key]; ok {
			if s := strings.TrimSpace(getStringFromAny(value)); s != "" {
				return s
			}
		}
	}
	return ""
}

func getBoolFromAny(value any) bool {
	b, ok := value.(bool)
	return ok && b
}

func marshalJSON(payload map[string]any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}
