package ingress_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/internal/ingress"
	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web"
)

func newHandler(t *testing.T) (http.Handler, *tracetest.SpanRecorder) {
	t.Helper()

	catalogue, err := web.Catalogue()
	require.NoError(t, err)

	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	logger := slog.New(slog.DiscardHandler)

	return ingress.NewHandler(logger, catalogue, "1.2.3", provider), spans
}

func get(t *testing.T, handler http.Handler, target string) *http.Response {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

	return rec.Result()
}

func TestHomeShowsThePortalNameInEnglish(t *testing.T) {
	t.Parallel()
	handler, _ := newHandler(t)

	resp := get(t, handler, "/")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `<html lang="en">`)
	assert.Contains(t, string(body), "<h1>Patient portal</h1>")
}

func TestHomeShowsTheApplicationVersion(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "lang=fr"})

	assert.Contains(t, body, "<footer><p>Version 1.2.3</p></footer>")
}

func TestEveryResponseForbidsFramingAndSniffing(t *testing.T) {
	t.Parallel()
	handler, _ := newHandler(t)

	for _, target := range []string{"/", "/missing"} {
		resp := get(t, handler, target)
		require.NoError(t, resp.Body.Close())

		assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'", target)
		assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"), target)
		assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"), target)
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	t.Parallel()
	handler, _ := newHandler(t)

	resp := get(t, handler, "/missing")
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnknownPathRecordsAServerSpanWithoutRoute(t *testing.T) {
	t.Parallel()
	handler, spans := newHandler(t)

	resp := get(t, handler, "/missing")
	require.NoError(t, resp.Body.Close())

	ended := spans.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "GET", ended[0].Name())
	attrs := attribute.NewSet(ended[0].Attributes()...)
	assert.False(t, attrs.HasValue("http.route"), "an unmatched path has no route")
}

func TestHomeRecordsAServerSpanWithItsRoute(t *testing.T) {
	t.Parallel()
	handler, spans := newHandler(t)

	resp := get(t, handler, "/")
	require.NoError(t, resp.Body.Close())

	ended := spans.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "GET /", ended[0].Name())
	attrs := attribute.NewSet(ended[0].Attributes()...)
	route, found := attrs.Value("http.route")
	require.True(t, found, "the span should carry http.route")
	assert.Equal(t, "/", route.AsString())
}

func TestStylesheetIsServed(t *testing.T) {
	t.Parallel()
	handler, _ := newHandler(t)

	resp := get(t, handler, "/static/app.css")
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/css")
}
