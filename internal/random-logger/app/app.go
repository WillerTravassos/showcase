package app

import (
	"context"
	"fmt"
	"time"

	"github.com/WillerTravassos/showcase/pkg/log"
	"github.com/brianvoe/gofakeit/v7"
)

const logGenerationTick = 1 * time.Second

type App struct {
	faker    *gofakeit.Faker
	Settings Settings
}

func New() (*App, error) {
	settings, err := NewSettings()
	if err != nil {
		return nil, fmt.Errorf("app configuration failed: %w", err)
	}

	return &App{
		faker:    gofakeit.New(0),
		Settings: *settings,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	logger := log.WithContext(ctx)
	ticker := time.NewTicker(logGenerationTick)

	for {
		select {
		case <-ctx.Done():
			logger.WithError(ctx.Err()).Info("log generation stopped: context cancelled")

			return nil
		case <-ticker.C:
			logger.Info(a.faker.Phrase())
		}
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	return nil
}

func (a *App) ShutdownGracePeriod() time.Duration {
	return a.Settings.ShutdownGracePeriod
}
