package lifecycle_test

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/internal/lifecycle"
)

// running is a [lifecycle.Run] call in progress.
type running struct {
	appURL   string
	probeURL string
	stop     context.CancelFunc
	done     chan error
}

// start runs handler under [lifecycle.Run] on loopback listeners until stop is called.
func start(t *testing.T, handler http.Handler) *running {
	t.Helper()

	appListener := listen(t)
	probeListener := listen(t)
	ctx, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)

	r := &running{
		appURL:   "http://" + appListener.Addr().String(),
		probeURL: "http://" + probeListener.Addr().String(),
		stop:     stop,
		done:     make(chan error, 1),
	}
	app := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}

	go func() {
		r.done <- lifecycle.Run(ctx, slog.New(slog.DiscardHandler), app, appListener, probeListener)
	}()

	return r
}

func listen(t *testing.T) net.Listener {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return listener
}

// status sends a GET to url and returns the response status, or 0 when the request fails.
// It is safe to call from any goroutine.
func status(t *testing.T, url string) int {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		return 0
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	return resp.StatusCode
}

func TestRunReportsAliveAndReadyWhileServing(t *testing.T) {
	t.Parallel()

	r := start(t, http.NotFoundHandler())

	require.Eventually(
		t,
		func() bool { return status(t, r.probeURL+"/readyz") == http.StatusOK },
		5*time.Second,
		10*time.Millisecond,
		"the service should report ready once it serves",
	)
	assert.Equal(t, http.StatusOK, status(t, r.probeURL+"/livez"), "the service should report alive")

	r.stop()

	require.NoError(t, <-r.done)
}

func TestRunDrainsAnInFlightRequestAfterTheStopSignal(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{})
	release := make(chan struct{})
	r := start(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)

		select {
		case <-req.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		case <-release:
			w.WriteHeader(http.StatusOK)
		}
	}))
	response := make(chan int, 1)

	go func() { response <- status(t, r.appURL) }()

	<-entered
	r.stop()

	require.Eventually(
		t,
		func() bool { return status(t, r.probeURL+"/readyz") == http.StatusServiceUnavailable },
		5*time.Second,
		10*time.Millisecond,
		"the service should report not ready while it drains",
	)
	assert.Equal(t, http.StatusOK, status(t, r.probeURL+"/livez"), "the service should stay alive while it drains")

	close(release)

	assert.Equal(t, http.StatusOK, <-response, "the in-flight request should finish, not be cancelled")
	require.NoError(t, <-r.done)
}

func TestRunReturnsTheServeErrorWhenAListenerFails(t *testing.T) {
	t.Parallel()

	appListener := listen(t)
	probeListener := listen(t)
	require.NoError(t, probeListener.Close())

	app := &http.Server{Handler: http.NotFoundHandler(), ReadHeaderTimeout: time.Second}
	err := lifecycle.Run(t.Context(), slog.New(slog.DiscardHandler), app, appListener, probeListener)

	require.ErrorIs(t, err, net.ErrClosed)
	assert.NotErrorIs(t, err, lifecycle.ErrShutdown, "a serve failure is not a shutdown failure")
}
