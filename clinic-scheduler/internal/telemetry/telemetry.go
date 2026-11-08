// Package telemetry starts OpenTelemetry traces, metrics, and logs for a service and
// builds its logger (docs/standards/observability.md).
//
// Exporters read the standard OTEL_EXPORTER_OTLP_* variables; the resource reads
// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES (which sets
// deployment.environment.name). Call [Start] once in main and defer [Telemetry.Shutdown].
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/WillerTravassos/showcase/clinic-scheduler/internal/logctx"
)

// scope is the instrumentation scope name of log records bridged from slog.
const scope = "github.com/WillerTravassos/showcase/clinic-scheduler/internal/telemetry"

// Telemetry holds a started service's logger and the providers to flush on shutdown.
type Telemetry struct {
	// Logger writes JSON to stdout and OTLP log records, each carrying the logctx
	// context attributes.
	Logger *slog.Logger

	shutdowns []func(context.Context) error
}

// Start creates the OTLP/HTTP exporters and installs the global tracer, meter, and
// propagator (W3C traceparent and baggage). version becomes service.version.
//
// shortcut: OTEL_EXPORTER_OTLP_PROTOCOL is ignored and every signal uses http/protobuf;
// add the gRPC exporters when a deployment needs grpc.
func Start(ctx context.Context, version string) (*Telemetry, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceVersion(version)),
	)
	if err != nil {
		return nil, fmt.Errorf("build telemetry resource: %w", err)
	}

	spans, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create span exporter: %w", err)
	}
	metrics, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}
	logs, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create log exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(spans))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metrics)))
	lp := sdklog.NewLoggerProvider(sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logs)))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))

	handler := slog.NewMultiHandler(
		slog.NewJSONHandler(os.Stdout, nil),
		otelslog.NewHandler(scope, otelslog.WithLoggerProvider(lp)),
	)
	return &Telemetry{
		Logger:    slog.New(logctx.NewHandler(handler)),
		shutdowns: []func(context.Context) error{tp.Shutdown, mp.Shutdown, lp.Shutdown},
	}, nil
}

// Shutdown flushes buffered spans, metrics, and log records and stops the exporters. It
// tries every provider and returns their errors joined. Call it once, before exit.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, shutdown := range t.shutdowns {
		if err := shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shut down telemetry: %w", err))
		}
	}
	return errors.Join(errs...)
}
