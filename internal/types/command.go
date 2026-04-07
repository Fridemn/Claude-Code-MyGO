package types

type CommandType string

const (
	CommandTypeLocal  CommandType = "local"
	CommandTypePrompt CommandType = "prompt"
)

type CommandSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        CommandType `json:"type"`
}
