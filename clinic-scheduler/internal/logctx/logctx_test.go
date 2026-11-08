package logctx_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"github.com/WillerTravassos/showcase/clinic-scheduler/internal/logctx"
)

// newLogger returns a logger writing JSON through a logctx handler into the returned buffer.
func newLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(logctx.NewHandler(slog.NewJSONHandler(&buf, nil))), &buf
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	return record
}

func TestContextAttributesAppearOnEveryRecord(t *testing.T) {
	t.Parallel()
	logger, buf := newLogger()
	ctx := logctx.With(t.Context(), slog.String("clinic.clinic.id", "c1"))
	ctx = logctx.With(ctx, slog.String("clinic.appointment.id", "a1"))

	logger.InfoContext(ctx, "appointment booked")

	record := decode(t, buf)
	assert.Equal(t, "c1", record["clinic.clinic.id"])
	assert.Equal(t, "a1", record["clinic.appointment.id"])
}

func TestWithLeavesTheParentContextUnchanged(t *testing.T) {
	t.Parallel()
	logger, buf := newLogger()
	parent := logctx.With(t.Context(), slog.String("clinic.clinic.id", "c1"))
	_ = logctx.With(parent, slog.String("clinic.appointment.id", "a1")) // only the parent is logged

	logger.InfoContext(parent, "clinic opened")

	assert.NotContains(t, decode(t, buf), "clinic.appointment.id")
}

func TestRecordInsideASpanCarriesItsIDs(t *testing.T) {
	t.Parallel()
	logger, buf := newLogger()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x0a, 0x0b},
		SpanID:     trace.SpanID{0x0c},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(t.Context(), sc)

	logger.InfoContext(ctx, "appointment booked")

	record := decode(t, buf)
	assert.Equal(t, sc.TraceID().String(), record["trace_id"])
	assert.Equal(t, sc.SpanID().String(), record["span_id"])
}

func TestRecordOutsideASpanHasNoSpanIDs(t *testing.T) {
	t.Parallel()
	logger, buf := newLogger()

	logger.InfoContext(context.Background(), "service started")

	record := decode(t, buf)
	assert.NotContains(t, record, "trace_id")
	assert.NotContains(t, record, "span_id")
}

func TestDerivedLoggerKeepsContextAttributes(t *testing.T) {
	t.Parallel()
	logger, buf := newLogger()
	ctx := logctx.With(t.Context(), slog.String("clinic.appointment.id", "a1"))

	logger.With(slog.String("component", "booking")).InfoContext(ctx, "appointment booked")

	record := decode(t, buf)
	assert.Equal(t, "a1", record["clinic.appointment.id"])
	assert.Equal(t, "booking", record["component"])
}
