package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"claude-go/internal/agent"
	"claude-go/internal/api"
	"claude-go/internal/bootstrap"
	"claude-go/internal/command"
	"claude-go/internal/config"
	"claude-go/internal/constants"
	"claude-go/internal/engine"
	"claude-go/internal/services"
	"claude-go/internal/settings"

	bashperm "claude-go/internal/tool/bash"
)

type App struct {
	cfg      config.Config
	state    *bootstrap.Store
	services *services.Container
	input    io.Reader
}

func Create(ctx context.Context, sessionID string) (*App, error) {
	cfg, err := config.Load(".env")
	if err != nil {
		return nil, err
	}

	state, err := bootstrap.CreateStore(cfg)
	if err != nil {
		return nil, err
	}

	container, err := services.Create(ctx, cfg, state, sessionID)
	if err != nil {
		return nil, err
	}

	// Initialize settings directory and load persisted permission rules
	settings.InitSettingsDirectory("")
	bashperm.LoadPersistedPermissionRules()

	return &App{
		cfg:      cfg,
		state:    state,
		services: container,
		input:    os.Stdin,
	}, nil
}

func (a *App) Engine() *engine.Engine { return a.services.Engine() }

func (a *App) Commands() *command.Registry { return a.services.Commands() }

func (a *App) Agents() *agent.Manager { return a.services.Agents() }

func (a *App) Provider() *api.OpenAICompatibleClient { return a.services.Provider() }

func (a *App) Config() config.Config { return a.cfg }

func (a *App) ApplyConfig(cfg config.Config) {
	a.cfg = cfg
	if a.services != nil {
		a.services.ApplyConfig(cfg)
	}
}

func (a *App) State() *bootstrap.Store { return a.state }

func (a *App) Services() *services.Container { return a.services }

func (a *App) Input() io.Reader { return a.input }

func (a *App) Version() string { return constants.Version }

func (a *App) Banner() string {
	return fmt.Sprintf("%s %s", a.cfg.AppName, constants.Version)
}
