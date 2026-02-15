package app

import (
	"context"
	"fmt"
	"os"
	"time"

	log "github.com/WillerTravassos/showcase/pkg/log"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	ui       *tview.Application
	Settings Settings
}

func NewTiltTUI() (*App, error) {
	settings, err := NewSettings()
	if err != nil {
		return nil, err
	}

	return &App{
		ui:       tview.NewApplication(),
		Settings: *settings,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	logFile, err := os.OpenFile(a.Settings.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, DefaultFileMode)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	log.SetLogger(log.NewLogger(log.Settings{
		LogOrigin:          true,
		LogWriter:          logFile,
		MinLogLevel:        log.InfoLevel,
		TimestampFieldName: log.Timestamp,
		TimestampFormat:    log.DefaultLogEntryTimeFormat,
	}))

	log.Info("tilt-tui: starting")

	box := tview.NewBox().SetBorder(true).SetTitle("Hello")

	a.ui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			// NOTE: use os.Signal to quit app
			proc, _ := os.FindProcess(os.Getpid())
			_ = proc.Signal(os.Interrupt)

			return nil
		}

		return event
	})

	if err := a.ui.SetRoot(box, true).Run(); err != nil {
		log.WithError(err).Fatalf("tilt-tui: failed to render screen")

		return err
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	a.ui.Stop()

	return nil
}

func (a *App) ShutdownGracePeriod() time.Duration {
	return a.Settings.ShutdownGracePerion
}
