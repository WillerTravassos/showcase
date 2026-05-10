# Tooling Conventions

---

## Go

- **Version:** Always the latest stable release. Check `go.mod` for the current pinned version.
- **Module path:** `github.com/WillerTravassos/showcase` — set in `go.mod`, used as prefix for all internal imports.
- Use `any` not `interface{}`. Use `min()`/`max()` builtins. Use `clear()` for map/slice clearing.
- `//go:build` tags only — never legacy `// +build` tags.

### Required dependencies (anchor versions in go.mod)

| Package | Purpose |
|---------|---------|
| `github.com/go-chi/chi/v5` | HTTP router |
| `github.com/google/uuid` | UUID generation (UUIDv5) |
| `github.com/jackc/pgx/v5` | PostgreSQL driver + pool |
| `github.com/redis/go-redis/v9` | Redis client |
| `github.com/nats-io/nats.go` | NATS JetStream client |
| `github.com/golang-jwt/jwt/v5` | JWT issuance and validation |
| `github.com/caarlos0/env/v11` | Config loading from env vars |
| `github.com/golang-migrate/migrate/v4` | DB migrations |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/rs/zerolog` | Logging backend (used only via `github.com/WillerTravassos/showcase/pkg/log`) |
| `github.com/sqlc-dev/sqlc` | SQL query generation (dev tool, not a runtime dep) |
| `github.com/Masterminds/squirrel` | Dynamic SQL query builder (repository layer only) |
| `github.com/ogen-go/ogen` | Schema-first OpenAPI code generation — generates server interface from `openapi.yaml` |
| `github.com/testcontainers/testcontainers-go` | Spin up real dependencies in isolated integration tests |

---

## Logging — `github.com/WillerTravassos/showcase/pkg/log`

The project uses a custom wrapper package at `github.com/WillerTravassos/showcase/pkg/log` that exposes a `Logger` interface matching the stdlib `log/slog` style but backed by zerolog.

**Rules:**
- **Never** import `log` (stdlib), `fmt` for logging, or `github.com/rs/zerolog` directly in any file outside `pkg/log/`.
- Always import `github.com/WillerTravassos/showcase/pkg/log` and use the `log.Logger` interface.
- Obtain a logger from context in handlers: `logger := log.FromCtx(r.Context())`.
- Pass logger through constructors for services and repositories — never use a package-level logger.
- Every structured log event must include `request_id` (from context) when inside an HTTP request scope.

**Log levels:**
| Level | When to use |
|-------|-------------|
| `Debug` | Internal state transitions, cache hits/misses — dev only |
| `Info` | Significant lifecycle events: server started, migration ran, worker subscribed |
| `Warn` | Recoverable unexpected conditions: retry triggered, config defaulted |
| `Error` | Failures that affect a request or job: DB error, NATS publish failed |
| `Fatal` | Startup failures only (config missing, DB unreachable at boot) — calls `os.Exit(1)` |

**What must never appear in logs:**
- Patient name, DOB, email, phone, address
- Any field from a FHIR Patient resource
- JWT tokens or refresh tokens
- Encryption keys

---

## Error Handling — `internal/<service>/pkg/<domain>/errors.go`

Full detail in `specs/conventions/errors.md`. Summary:

- Sentinel errors are defined per domain package in `errors.go`.
- All errors are wrapped with context: `fmt.Errorf("scheduling slot %s: %w", slotID, err)`.
- HTTP handlers map domain errors to HTTP status codes using a central `pkg/http.WriteError(w, r, err)` function that understands sentinel types.
- Never expose internal error messages to HTTP clients. Always use the error code envelope.

---

## Database — CloudNativePG (CNPG)

Full detail in `specs/conventions/database.md`. Summary:

- CNPG operator manages all PostgreSQL clusters. No generic `postgres:` Docker images in production.
- Three CNPG clusters: `stdclinic-ca`, `stdclinic-us`, `stdclinic-br`.
- Local dev uses a CNPG cluster inside kind (managed by Tilt) — not a plain Docker postgres container.
- Connection strings come from CNPG-generated Kubernetes Secrets, injected as env vars.
- Migrations run via `golang-migrate` using the `pgx/v5` driver.

---

## Message Queue — NATS JetStream

- NATS JetStream for all async messaging.
- Client: `github.com/nats-io/nats.go`
- All NATS interaction is encapsulated in `pkg/queue/`. Domain code calls a `Publisher` interface — never calls NATS directly.
- Streams are pre-created by the infrastructure layer (Helm/Tofu) — the application asserts stream existence at startup but does not create streams.
- Every published message includes `trace_id`, `occurred_at` (RFC3339), `schema_version`, `payload`.
- Consumer groups use durable consumer names. Never use ephemeral consumers in production.
- At-least-once delivery. All consumers are idempotent — processing the same message twice must have the same result as processing it once.

---

## Migrations — golang-migrate

- Migration files: `migrations/NNNNNN_description.up.sql` and `migrations/NNNNNN_description.down.sql`
- Sequential 6-digit numbering: `000001`, `000002`, etc.
- **Never modify an existing migration file.** All changes are new migrations.
- Down migrations must be correct and tested. Every `up` has a working `down`.
- Run at binary startup via `internal/db.RunMigrations(dsn)` before the server begins accepting requests.
- `Makefile` target `make migrate-up` and `make migrate-down` for manual use.

---

## Testing — stdlib + testify

- Test files: `{file}_test.go` alongside the file under test.
- Use `github.com/stretchr/testify/assert` for non-fatal assertions and `require` for fatal ones.
- Use `require.*` for setup steps (DB connection, file load). Use `assert.*` for per-case checks.
- **Never** use `t.Fatal` or `t.FailNow` directly — always go through `require`.
- Every exported function must have at least one success-path test and one failure-path test. Functions with more than two logical branches use table-driven tests:
  ```go
  tests := []struct {
      name    string
      input   X
      want    Y
      wantErr bool
  }{ ... }
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) { ... })
  }
  ```
- Do not write tests purely for coverage. Tests must verify logic correctness of self-contained behaviour.

### Integration tests

Integration tests run in two modes:

| Mode | When | Tool |
|---|---|---|
| Isolated | Always (no cluster needed) | `testcontainers-go` spins up real dependencies per test |
| Cluster | When `TILT_CI=1` env var is set | Tests run against the live Tilt-managed kind cluster |

- Use `test.RequireTiltCI(t)` at the top of any test that requires the live cluster. This helper skips the test if `TILT_CI` is not set.
- Integration test files are named `*_integration_test.go`.
- Code is instrumented with OpenTelemetry tracing. Cluster-mode integration tests may use the `otel/tracetest` library to select spans and assert on their attributes.
- No `//go:build integration` tags. Use the `TILT_CI` env var to control test selection.

### Mocking strategy

| What | How |
|---|---|
| External HTTP APIs (fax services, DSP, etc.) | WireMock container run via Tiltfile — JSON mapping files define stub responses |
| Internal domain interfaces | Hand-written structs implementing the interface — kept in `*_test.go` files |

No mocking frameworks. No mockery-generated files.

---

## Local Development — Tilt + kind

Full detail in `specs/conventions/tilt.md`. Summary:

- `kind` creates a local Kubernetes cluster named `stdclinic-dev`.
- `Tilt` orchestrates all services via the `Tiltfile` at repo root.
- CNPG operator is installed in the kind cluster — dev uses real CNPG, not a plain postgres container.
- `tilt up` starts everything. `tilt down` tears it down.
- Hot reload: Go services rebuild and restart on source change via Tilt's `local_resource` + `live_update`.
- Never use `docker-compose` for the primary dev workflow. `docker-compose.dev.yml` may exist for CI pipeline use only.

---

## Linting — golangci-lint

Config in `.golangci.yml` at repo root. Required linters:

```yaml
linters:
  enable:
    - errcheck        # all errors must be checked
    - govet           # go vet checks
    - staticcheck     # SA and ST checks
    - revive          # style and idiom
    - exhaustive      # switch exhaustiveness on enums
    - forbidigo       # forbid direct log/zerolog imports
    - noctx           # HTTP requests must use context
    - wrapcheck       # errors from external packages must be wrapped
    - godot           # comment sentences end with period
```

`forbidigo` patterns to block direct logging imports:
```yaml
- p: '^log\.'
  msg: "Use github.com/WillerTravassos/showcase/pkg/log instead of stdlib log"
- p: '^zerolog\.'
  msg: "Use github.com/WillerTravassos/showcase/pkg/log instead of zerolog directly"
- p: '^fmt\.Print'
  msg: "Use github.com/WillerTravassos/showcase/pkg/log for output"
```

CI must pass `golangci-lint run ./...` with zero warnings before merge.

---

## Code Generation — sqlc + squirrel

- SQL queries are written in `internal/<service>/pkg/<domain>/queries.sql`.
- `sqlc` generates Go code into `internal/<service>/pkg/<domain>/db.go` (gitignored, regenerated on change).
- Run: `make sqlc` (calls `sqlc generate` for each package).
- For complex dynamic queries (multi-condition `WHERE`, optional filters, dynamic `JOIN`s) that sqlc cannot express statically, use `github.com/Masterminds/squirrel` to build the query programmatically.
- Squirrel-built queries must never leave repository files. No squirrel imports in services or handlers.

## Vendoring

- All dependencies are vendored via `go mod vendor`.
- Run `go mod vendor` after any change to `go.mod` or `go.sum`.
- The `vendor/` directory is committed to the repository — this guarantees reproducible builds in CI without network access and protects against upstream package disappearance.
- CI must build from `vendor/` using `-mod=vendor`.
