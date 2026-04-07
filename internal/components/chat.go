package components

import (
	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/config"
	"claude-code-go/internal/ui"
	"time"
)

type TranscriptEntry = ui.TranscriptEntry
type SlashSuggestion = ui.SlashSuggestion
type ViewMode = ui.ViewMode

const (
	ViewModeNormal     = ui.ViewModeNormal
	ViewModeVerbose    = ui.ViewModeVerbose
	ViewModeTranscript = ui.ViewModeTranscript
)

type ChatProps struct {
	Version              string
	Config               config.Config
	Width                int
	Height               int
	State                bootstrap.State
	Entries              []TranscriptEntry
	CurrentInput         string
	Suggestions          []SlashSuggestion
	SelectedSuggestion   int
	Busy                 bool
	SpinnerTick          int
	StreamingText        string
	ToolName             string
	ToolCallID           string
	StatusText           string
	StartedAt            time.Time
	Verb                 string   // Randomly selected verb, constant during request
	TokenCount           int      // Current token count for display
	Mode                 ViewMode // Current view mode
	TranscriptScroll     int      // Scroll offset from the bottom of transcript, in lines
	LastThinkingBlockID  string   // ID of last thinking block for visibility
	LatestBashOutputUUID string   // UUID of most recent bash output for auto-expand
	Teammates            []ui.TeammateSpinnerNode
	TeammateLeaderVerb   string
	TeammateLeaderTokens int
	// InProgressToolIDs tracks tool_use IDs that are currently executing
	InProgressToolIDs map[string]bool
}

type ChatApp struct{}

func ChatAppFor() *ChatApp {
	return &ChatApp{}
}

func (a *ChatApp) Render(props ChatProps) string {
	return ui.RenderScreen(ui.ScreenState{
		Version:              props.Version,
		Config:               props.Config,
		Width:                props.Width,
		Height:               props.Height,
		State:                props.State,
		SessionID:            props.State.SessionID,
		Turn:                 props.State.TurnCount,
		Entries:              props.Entries,
		CurrentInput:         props.CurrentInput,
		Suggestions:          props.Suggestions,
		SelectedSuggestion:   props.SelectedSuggestion,
		Busy:                 props.Busy,
		SpinnerTick:          props.SpinnerTick,
		StreamingText:        props.StreamingText,
		ToolName:             props.ToolName,
		ToolCallID:           props.ToolCallID,
		StatusText:           props.StatusText,
		StartedAt:            props.StartedAt,
		Verb:                 props.Verb,
		TokenCount:           props.TokenCount,
		Mode:                 props.Mode,
		TranscriptScroll:     props.TranscriptScroll,
		LastThinkingBlockID:  props.LastThinkingBlockID,
		LatestBashOutputUUID: props.LatestBashOutputUUID,
		Teammates:            props.Teammates,
		TeammateLeaderVerb:   props.TeammateLeaderVerb,
		TeammateLeaderTokens: props.TeammateLeaderTokens,
		InProgressToolIDs:    props.InProgressToolIDs,
	})
}

func (a *ChatApp) PromptLabel() string {
	return ui.PromptLabel()
}
