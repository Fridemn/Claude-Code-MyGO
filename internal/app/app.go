package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/command"
	"claude-code-go/internal/config"
	"claude-code-go/internal/constants"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/services"
)

type App struct {
	cfg      config.Config
	state    *bootstrap.Store
	services *services.Container
	input    io.Reader
}

func Create(ctx context.Context) (*App, error) {
	cfg, err := config.Load(".env")
	if err != nil {
		return nil, err
	}

	state, err := bootstrap.CreateStore(cfg)
	if err != nil {
		return nil, err
	}

	container, err := services.Create(ctx, cfg, state)
	if err != nil {
		return nil, err
	}

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

func (a *App) Config() config.Config { return a.cfg }

func (a *App) State() *bootstrap.Store { return a.state }

func (a *App) Services() *services.Container { return a.services }

func (a *App) Input() io.Reader { return a.input }

func (a *App) Version() string { return constants.Version }

func (a *App) Banner() string {
	return fmt.Sprintf("%s %s", a.cfg.AppName, constants.Version)
}
