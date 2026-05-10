package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/WillerTravassos/showcase/internal/random-logger/app"
	"github.com/WillerTravassos/showcase/pkg/log"
)

func main() {
	osSignal := make(chan os.Signal, 1)

	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGTERM)
	defer stop()

	application, err := app.New()
	if err != nil {
		log.WithError(err).Fatal("failed to configure application")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), application.ShutdownGracePeriod())
	defer shutdownCancel()

	go watchSigTermination(shutdownCtx, application, osSignal)

	if err := application.Start(ctx); err != nil {
		log.WithError(err).Fatal("failed to start application")

		os.Exit(1)
	}

	<-shutdownCtx.Done()

	log.Info("application shutdown")
}

func watchSigTermination(ctx context.Context, app *app.App, osSignal <-chan os.Signal) {
	terminalSignal := <-osSignal

	log.Infof("application shutdown signal received: %v", terminalSignal)
	log.Infof("application starting %s shutdown grace period...", app.ShutdownGracePeriod())

	if err := app.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("tilt-tui: shutdown error")

		os.Exit(1)
	}
}
