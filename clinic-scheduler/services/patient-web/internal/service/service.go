// Package service wires patient-web together and runs it until its context ends.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/WillerTravassos/showcase/clinic-scheduler/internal/telemetry"
	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/internal/config"
	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/internal/ingress"
	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web"
)

// Server timeouts (docs/standards/security.md).
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

// Run starts telemetry and the HTTP server, and serves until ctx ends; then it drains
// open requests and flushes telemetry. version becomes service.version.
func Run(ctx context.Context, version string) error {
	tel, err := telemetry.Start(ctx, version)
	if err != nil {
		return fmt.Errorf("start telemetry: %w", err)
	}

	defer func() {
		// The run context has ended by now; shutdown gets its own deadline.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if shutdownErr := tel.Shutdown(shutdownCtx); shutdownErr != nil {
			tel.Logger.ErrorContext(
				shutdownCtx,
				"telemetry shutdown failed",
				slog.String("exception.message", shutdownErr.Error()),
			)
		}
	}()

	if err = serve(ctx, tel.Logger, config.Load(), version); err != nil {
		tel.Logger.ErrorContext(
			ctx,
			"server failed",
			slog.String("error.type", "startup"),
			slog.String("exception.message", err.Error()),
		)
	}

	return err
}

func serve(ctx context.Context, logger *slog.Logger, cfg config.Config, version string) error {
	catalogue, err := web.Catalogue()
	if err != nil {
		return fmt.Errorf("load message catalogue: %w", err)
	}

	listener, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Addr, err)
	}

	server := &http.Server{
		Handler:           ingress.NewHandler(logger, catalogue, otel.GetTracerProvider()),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	served := make(chan error, 1)

	go func() { served <- server.Serve(listener) }()

	logger.InfoContext(
		ctx,
		"server started",
		slog.String(string(semconv.ServiceVersionKey), version),
		slog.String(string(attribute.Key("server.address")), cfg.Addr),
	)

	select {
	case err = <-served:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err = server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}

	if err = <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}
