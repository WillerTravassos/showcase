// Package lifecycle runs an HTTP service under the Kubernetes pod lifecycle
// (docs/standards/kubernetes.md). It answers the kubelet's liveness and readiness probes
// on a listener of their own and, when the stop signal arrives, stops reporting ready
// and drains in-flight requests before it returns.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// DrainTimeout bounds how long in-flight requests may take to finish once the stop signal
// arrives. With the 5 second preStop wait and the telemetry flush, it fits inside the
// default 30 second grace period (the time Kubernetes allows before it kills the pod).
const DrainTimeout = 15 * time.Second

// Probe server timeouts (docs/standards/security.md). Probes are small and frequent.
const (
	probeReadTimeout  = 5 * time.Second
	probeWriteTimeout = 5 * time.Second
	probeIdleTimeout  = 30 * time.Second
)

// servers is the number of HTTP servers Run starts: the app and the probes.
const servers = 2

// ErrShutdown marks an error returned while stopping, as opposed to while serving.
var ErrShutdown = errors.New("shut down")

// Run serves app on appListener and the probes on probeListener until ctx ends or a
// server fails. It reports ready once both serve. When ctx ends, it reports not ready,
// waits up to [DrainTimeout] for in-flight requests to finish, and returns nil; errors
// from that stage wrap [ErrShutdown].
//
// Run replaces app.BaseContext so that requests keep ctx's values but never its
// cancellation: a request stopped by the signal would fail instead of draining. Run
// takes ownership of both listeners and closes them.
func Run(ctx context.Context, logger *slog.Logger, app *http.Server, appListener, probeListener net.Listener) error {
	var ready atomic.Bool

	app.BaseContext = func(net.Listener) context.Context { return context.WithoutCancel(ctx) }
	probe := &http.Server{
		Handler:           probes(&ready),
		ReadHeaderTimeout: probeReadTimeout,
		ReadTimeout:       probeReadTimeout,
		WriteTimeout:      probeWriteTimeout,
		IdleTimeout:       probeIdleTimeout,
	}
	served := make(chan error, servers)

	go func() { served <- app.Serve(appListener) }()
	go func() { served <- probe.Serve(probeListener) }()

	ready.Store(true)

	select {
	case err := <-served:
		// Close errors only report listeners that are already closed.
		_ = app.Close()
		_ = probe.Close()

		return fmt.Errorf("serve: %w", errors.Join(err, wait(served, servers-1)))
	case <-ctx.Done():
	}

	ready.Store(false)
	logger.InfoContext(ctx, "server stopping")

	started := time.Now()

	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), DrainTimeout)
	defer cancel()

	drainErr := app.Shutdown(drainCtx)
	if drainErr != nil {
		// The drain timed out; closing cuts the requests still open.
		_ = app.Close()
	}

	// Probes have no in-flight work worth waiting for.
	_ = probe.Close()

	if err := errors.Join(drainErr, wait(served, servers)); err != nil {
		return fmt.Errorf("%w: drain in-flight requests: %w", ErrShutdown, err)
	}

	logger.InfoContext(ctx, "server stopped", slog.Duration("clinic.server.drain.duration", time.Since(started)))

	return nil
}

// wait receives n results from served and joins those other than [http.ErrServerClosed].
func wait(served <-chan error, n int) error {
	var errs []error

	for range n {
		if err := <-served; !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// probes answers liveness (the process can serve) and readiness (it should receive
// traffic now). Neither checks dependencies, and neither is traced.
func probes(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) { answer(w, http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready.Load() {
			answer(w, http.StatusOK)

			return
		}

		answer(w, http.StatusServiceUnavailable)
	})

	return mux
}

// answer writes a plain, uncacheable probe response with status and its text.
func answer(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	// A failed write means the kubelet hung up; it counts the probe as failed on its own.
	_, _ = io.WriteString(w, http.StatusText(status)+"\n")
}
