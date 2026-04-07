package plan

import (
	"claude-code-go/internal/tool"
)

// RegisterPlanTools registers plan mode tools to the given registry
func RegisterPlanTools(r *tool.Registry) {
	r.Register(CreateEnterPlanModeTool())
	r.Register(CreateExitPlanModeTool())
}

// PlanModeState tracks plan mode state for the session
type PlanModeState struct {
	// HasExitedPlanMode tracks if user has exited plan mode in this session
	HasExitedPlanMode bool
	// NeedsPlanModeExitAttachment tracks if we need to show the plan mode exit attachment
	NeedsPlanModeExitAttachment bool
	// NeedsAutoModeExitAttachment tracks if we need to show the auto mode exit attachment
	NeedsAutoModeExitAttachment bool
	// CurrentPlanSlug is the slug for the current plan file
	CurrentPlanSlug string
}

// HandlePlanModeTransition handles transitions between plan mode and other modes
func HandlePlanModeTransition(fromMode, toMode string, state *PlanModeState) {
	// If switching TO plan mode, clear any pending exit attachment
	// This prevents sending both plan_mode and plan_mode_exit when user toggles quickly
	if toMode == "plan" && fromMode != "plan" {
		state.NeedsPlanModeExitAttachment = false
	}

	// If switching out of plan mode, trigger the plan_mode_exit attachment
	if fromMode == "plan" && toMode != "plan" {
		state.NeedsPlanModeExitAttachment = true
		state.HasExitedPlanMode = true
	}
}

// HandleAutoModeTransition handles transitions between auto mode and other modes
func HandleAutoModeTransition(fromMode, toMode string, state *PlanModeState) {
	// Auto↔plan transitions are handled by HandlePlanModeTransition
	// Skip both directions so this function only handles direct auto transitions
	if (fromMode == "auto" && toMode == "plan") || (fromMode == "plan" && toMode == "auto") {
		return
	}

	fromIsAuto := fromMode == "auto"
	toIsAuto := toMode == "auto"

	// If switching TO auto mode, clear any pending exit attachment
	// This prevents sending both auto_mode and auto_mode_exit when user toggles quickly
	if toIsAuto && !fromIsAuto {
		state.NeedsAutoModeExitAttachment = false
	}

	// If switching out of auto mode, trigger the auto_mode_exit attachment
	if fromIsAuto && !toIsAuto {
		state.NeedsAutoModeExitAttachment = true
	}
}
