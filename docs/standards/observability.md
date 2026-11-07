# Observability standard

Logs, traces, and metrics use [OpenTelemetry](https://opentelemetry.io/docs/) and its
[semantic conventions](https://opentelemetry.io/docs/specs/semconv/) (the standard
attribute and metric names). Everything is exported over OTLP to the Grafana LGTM stack;
locally that is the shared `otel-lgtm` service in Tilt.

## Resource and export

- Every service sets the resource attributes `service.name`, `service.version`, and
  `deployment.environment.name`.
- Configuration uses the standard environment variables (`OTEL_EXPORTER_OTLP_ENDPOINT`,
  `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`) set in
  the kustomize manifests, never hard-coded.
- Locally: `OTEL_EXPORTER_OTLP_ENDPOINT=http://lgtm.observability:4318` with
  `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`. Grafana is at <http://localhost:3000>.

## Attributes

- Use a semantic convention attribute whenever one exists (`http.request.method`,
  `db.system.name`, `error.type`, `user.id`).
- Custom attributes are lowercase, dot-separated, and namespaced by project and domain:
  `clinic.appointment.id`, `pong.match.id`. The same key means the same thing across
  logs, spans, and metrics.
- Never record secrets or personal data (names, emails, health details) as attribute
  values; record identifiers instead.

## Logs

JSON to stdout (for `kubectl logs`) and OTLP to Loki, correlated by `trace_id` and
`span_id`.

| Level | Use                                                                          |
|-------|------------------------------------------------------------------------------|
| ERROR | The main diagnostic tool: a failure that was handled at a boundary           |
| WARN  | Degraded but handled (a retry succeeded, a fallback served the request)      |
| INFO  | A workflow step completed ("appointment booked"). One line per step, at most |
| DEBUG | Rarely; never committed in a hot path                                        |

- **Log or return, never both.** Errors are wrapped with context on the way up and
  logged once at the boundary that handles them (ingress handler, consumer, `main`).
- An error log carries `error.type`, `exception.message`, the operation that failed, and
  the domain identifiers involved.
- **Context attributes**: attributes that apply to a whole execution path (a request ID,
  `clinic.appointment.id`) are attached once to the context or span and appear on every
  log line below it. Never repeat them on individual log calls.

### Go

- `log/slog` with a JSON handler for stdout and the
  `go.opentelemetry.io/contrib/bridges/otelslog` handler for OTLP, combined in the
  project's `internal/telemetry` package.
- The logger is passed in through constructors; there is no global logger. Log calls use
  the context variants (`logger.InfoContext(ctx, ...)`) with typed attributes
  (`slog.String`, `slog.Int`).
- `internal/logctx` holds context attributes: `ctx = logctx.With(ctx, slog.String(...))`.
  Its handler adds those attributes, plus `trace_id` and `span_id` from the active span,
  to every record.

### Rust

- `tracing` for instrumentation, with `tracing-subscriber` (JSON formatter, current span
  fields included) for stdout, `tracing-opentelemetry` for spans, and
  `opentelemetry-appender-tracing` for OTLP logs.
- Context attributes are span fields: `#[instrument(fields(pong.match.id = %id))]` or
  `Span::current().record(...)`. Events inside the span inherit them.

### TypeScript

- Server-side TypeScript (if any) uses `pino`, passing child loggers
  (`logger.child({ ... })`) down the call path. Browser code follows `typescript.md`.

## Traces

- Instrumentation libraries create the protocol spans (HTTP server and client, database,
  messaging); add spans for business operations worth seeing in a trace, named in domain
  language (`appointment.book`).
- Record errors on the span at the same boundary that logs them: Go `span.RecordError`
  plus `span.SetStatus(codes.Error, ...)`; Rust the `otel.status_code = "error"` and
  `otel.status_message` span fields read by `tracing-opentelemetry`.
- Propagate context with W3C `traceparent` across every HTTP call and message.

### Traces in tests

- **In-process** (unit and integration): Go `go.opentelemetry.io/otel/sdk/trace/tracetest`
  (`SpanRecorder`); Rust `opentelemetry_sdk::trace::InMemorySpanExporter` (feature
  `testing`). Assert span names, key attributes, and status for the workflow.
- **Across services** (acceptance): the test sends a `traceparent` header with a known
  trace ID, then the project's Tempo helper polls `GET /api/traces/<trace-id>` on Tempo
  (port 3200) until the trace is complete and asserts the span tree.

## Metrics

- Use semantic convention instruments from instrumentation libraries (for example
  `http.server.request.duration`) before defining your own.
- Custom metrics follow the attribute naming above (`clinic.appointment.booked`), declare
  a unit in [UCUM](https://ucum.org) form (`{appointment}`, `ms`, `By`), and keep
  attribute values low in cardinality (no IDs on metrics).
