package ui

func PromptLabel() string {
	return style(&dark.userLabel, nil, "⏵", true) + " "
}

func ClearScreen() string {
	return "\033[2J\033[H"
}
