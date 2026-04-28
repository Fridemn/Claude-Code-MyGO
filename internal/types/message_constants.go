package types

// Message constants.
// Ported from src/utils/messages.ts

const (
	// Interrupt messages
	InterruptMessage           = "[Request interrupted by user]"
	InterruptMessageForToolUse = "[Request interrupted by user for tool use]"

	// Cancel/reject messages
	CancelMessage = "The user doesn't want to take this action right now. STOP what you are doing and wait for the user to tell you how to proceed."
	RejectMessage = "The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). STOP what you are doing and wait for the user to tell you how to proceed."

	// Reject message with reason prefix
	RejectMessageWithReasonPrefix = "The user doesn't want to proceed with this tool use. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). To tell you how to proceed, the user said:\n"

	// Subagent reject messages
	SubagentRejectMessage                 = "Permission for this tool use was denied. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). Try a different approach or report the limitation to complete your task."
	SubagentRejectMessageWithReasonPrefix = "Permission for this tool use was denied. The tool use was rejected (eg. if it was a file edit, the new_string was NOT written to the file). The user said:\n"

	// Plan rejection prefix
	PlanRejectionPrefix = "The agent proposed a plan that was rejected by the user. The user chose to stay in plan mode rather than proceed with implementation.\n\nRejected plan:\n"

	// Denial workaround guidance
	DenialWorkaroundGuidance = "IMPORTANT: You *may* attempt to accomplish this action using other tools that might naturally be used to accomplish this goal, " +
		"e.g. using head instead of cat. But you *should not* attempt to work around this denial in malicious ways, " +
		"e.g. do not use your ability to run tests to execute non-test actions. " +
		"You should only try to work around this restriction in reasonable ways that do not attempt to bypass the intent behind this denial. " +
		"If you believe this capability is essential to complete the user's request, STOP and explain to the user " +
		"what you were trying to do and why you need this permission. Let the user decide how to proceed."

	// No response message
	NoResponseRequested = "No response requested."

	// Synthetic tool result placeholder
	SyntheticToolResultPlaceholder = "[Tool result missing due to internal error]"

	// No content message
	NoContentMessage = "<system_instruction>Content blocked</system_instruction>"

	// Synthetic model marker
	SyntheticModel = "<synthetic>"

	// Auto mode rejection prefix
	AutoModeRejectionPrefix = "Permission for this action has been denied. Reason: "

	// Local command tags
	LocalCommandCaveatTag = "local-caveat"
	LocalCommandStdoutTag = "local-command-stdout"
	LocalCommandStderrTag = "local-command-stderr"
	CommandNameTag        = "command-name"
	CommandMessageTag     = "command-message"
	CommandArgsTag        = "command-args"
)

// AutoRejectMessage builds a rejection message for auto mode denials.
func AutoRejectMessage(toolName string) string {
	return "Permission to use " + toolName + " has been denied. " + DenialWorkaroundGuidance
}

// DontAskRejectMessage builds a rejection message for don't ask mode.
func DontAskRejectMessage(toolName string) string {
	return "Permission to use " + toolName + " has been denied because Claude Code is running in don't ask mode. " + DenialWorkaroundGuidance
}

// BuildYoloRejectionMessage builds a rejection message for auto mode classifier denials.
func BuildYoloRejectionMessage(reason string) string {
	return AutoModeRejectionPrefix + reason + ". " +
		"If you have other tasks that don't depend on this action, continue working on those. " +
		DenialWorkaroundGuidance + " " +
		"To allow this type of action in the future, the user can add a permission rule to their settings."
}

// BuildClassifierUnavailableMessage builds a message for when the classifier is unavailable.
func BuildClassifierUnavailableMessage(toolName, classifierModel string) string {
	return classifierModel + " is temporarily unavailable, so auto mode cannot determine the safety of " + toolName + " right now. " +
		"Wait briefly and then try this action again. " +
		"If it keeps failing, continue with other tasks that don't require this action and come back to it later. " +
		"Note: reading files, searching code, and other read-only operations do not require the classifier and can still be used."
}

// IsSyntheticMessage checks if a message content is synthetic.
func IsSyntheticMessage(content string) bool {
	switch content {
	case InterruptMessage, InterruptMessageForToolUse, CancelMessage, RejectMessage, NoResponseRequested:
		return true
	default:
		return false
	}
}

// IsClassifierDenial checks if content is a classifier denial.
func IsClassifierDenial(content string) bool {
	return len(content) >= len(AutoModeRejectionPrefix) &&
		content[:len(AutoModeRejectionPrefix)] == AutoModeRejectionPrefix
}
