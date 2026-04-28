package config

import "claude-go/internal/command"

func Register(r *command.Registry) {
	registerVim(r)
	registerColor(r)
	registerKeybindings(r)
}