// Package ingress holds the patient portal's HTTP entry points.
package ingress

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// maxBodyBytes bounds every request body; the portal has no uploads.
const maxBodyBytes = 64 << 10

// contentSecurityPolicy allows only same-origin resources and forbids every frame
// ancestor, so no other site can embed the portal.
const contentSecurityPolicy = "default-src 'self'; base-uri 'none'; form-action 'self'; " +
	"frame-ancestors 'none'; object-src 'none'"

// Handler serves the patient portal pages.
type Handler struct {
	logger    *slog.Logger
	catalogue *i18n.Bundle
}

// NewHandler returns the patient portal's root [http.Handler]. Every response carries the
// security headers, every request body is limited, and every request gets a server span
// from tracerProvider.
func NewHandler(logger *slog.Logger, catalogue *i18n.Bundle, tracerProvider trace.TracerProvider) http.Handler {
	h := &Handler{logger: logger, catalogue: catalogue}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.home)
	mux.HandleFunc("POST /preferences", h.savePreferences)

	// CrossOriginProtection rejects state-changing requests that a browser sends from
	// another site (CSRF, cross-site request forgery).
	csrf := http.NewCrossOriginProtection().Handler(routeAttribute(mux))

	// otelhttp reads the route from the request the mux matched, so nothing between them
	// may copy the request (http.MaxBytesHandler does).
	traced := otelhttp.NewHandler(
		securityHeaders(csrf),
		"patient-web",
		otelhttp.WithTracerProvider(tracerProvider),
		otelhttp.WithSpanNameFormatter(spanName),
	)

	return http.MaxBytesHandler(traced, maxBodyBytes)
}

// spanName names a server span "{method} {route}" as the HTTP semantic conventions ask
// ("GET /"). Before routing, and for unmatched paths, it is the method alone.
func spanName(_ string, r *http.Request) string {
	if route := route(r); route != "" {
		return r.Method + " " + route
	}

	return r.Method
}

// routeAttribute sets http.route on the server span once the mux has matched the request;
// otelhttp starts the span before routing, so it cannot.
func routeAttribute(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)

		if route := route(r); route != "" {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(route))
		}
	})
}

// route is the path part of the pattern the mux matched r with, without the ServeMux
// exact-match marker {$} ("/", not "/{$}"). It is empty when no pattern matched.
func route(r *http.Request) string {
	_, path, _ := strings.Cut(r.Pattern, " ")
	return strings.TrimSuffix(path, "{$}")
}

// securityHeaders sets the response headers that every page needs (docs/standards/security.md).
// Only the gateway's HTTPS listener routes here, so Strict-Transport-Security is safe.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(w, r)
	})
}
