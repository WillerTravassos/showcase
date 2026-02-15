package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/WillerTravassos/showcase/internal/tilt-tui/app"
	log "github.com/WillerTravassos/showcase/pkg/log"
)

func main() {
	osSignal := make(chan os.Signal, 1)

	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGTERM)
	defer stop()

	// Run app here
	tiltTuiApp, err := app.NewTiltTUI()
	if err != nil {
		log.WithError(err).Fatal("tilt-tui: startup error")

		os.Exit(1)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), tiltTuiApp.ShutdownGracePeriod())
	defer shutdownCancel()

	go watchSigTermination(shutdownCtx, tiltTuiApp, osSignal)

	if err := tiltTuiApp.Start(ctx); err != nil {
		log.WithError(err).Fatal("tilt-tui: runtime error")

		os.Exit(1)
	}

	<-shutdownCtx.Done()

	log.Info("tilt-tui: shutdown")
}

func watchSigTermination(ctx context.Context, tiltTuiApp *app.App, osSignal <-chan os.Signal) {
	terminalSignal := <-osSignal

	log.Infof("tilt-tui: shutdown signal received: %v", terminalSignal)
	log.Infof("tilt-tui: starting %s shutdown grace period...", tiltTuiApp.ShutdownGracePeriod())

	// Run shutdown logic here
	if err := tiltTuiApp.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("tilt-tui: shutdown error")

		os.Exit(1)
	}
}
