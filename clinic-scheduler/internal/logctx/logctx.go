// Package logctx attaches log attributes to a context once, so that every log record
// written with that context carries them.
//
// Use [With] where an execution path starts (a request, a message) and wrap the
// service's [slog.Handler] with [NewHandler]:
//
//	ctx = logctx.With(ctx, slog.String("clinic.appointment.id", id))
//	logger.InfoContext(ctx, "appointment booked") // carries clinic.appointment.id
package logctx

import (
	"context"
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/trace"
)

type attrsKey struct{}

// With returns a copy of ctx that carries attrs after any attributes already on ctx.
func With(ctx context.Context, attrs ...slog.Attr) context.Context {
	return context.WithValue(ctx, attrsKey{}, slices.Concat(attrsFrom(ctx), attrs))
}

func attrsFrom(ctx context.Context) []slog.Attr {
	attrs, _ := ctx.Value(attrsKey{}).([]slog.Attr) // a missing value leaves attrs nil
	return attrs
}

// Handler is a [slog.Handler] that adds the context's attributes, and the trace_id and
// span_id of the context's active span, to every record before passing it on. It is
// safe for concurrent use when the wrapped handler is.
type Handler struct {
	next slog.Handler
}

// NewHandler returns a [Handler] that passes records on to next.
func NewHandler(next slog.Handler) *Handler {
	return &Handler{next: next}
}

// Enabled reports whether the wrapped handler handles records at level.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle adds the context attributes and span IDs to r and passes it on.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(attrsFrom(ctx)...)
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	//nolint:wrapcheck // a decorator returns the wrapped handler's error unchanged
	return h.next.Handle(ctx, r)
}

// WithAttrs returns a [Handler] whose wrapped handler has attrs added.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewHandler(h.next.WithAttrs(attrs))
}

// WithGroup returns a [Handler] whose wrapped handler opens the group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	return NewHandler(h.next.WithGroup(name))
}
