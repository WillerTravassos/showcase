# Overview: Core Backend API

> Reference this file for all backend sessions (01–07, 17).

---

## Binary Entrypoint

`cmd/api/main.go` — wires together router, middleware chain, DB pool, Redis client, NATS connection, and starts HTTP server on `:8080`.

---

## Router Structure (chi)

```
/health                         GET  — liveness probe (no auth)
/ready                          GET  — readiness probe (no auth)

/api/v1/
  /auth/
    /login                      POST
    /refresh                    POST
    /logout                     POST

  /patients/                    GET (staff only), POST
    /{uuid}                     GET, PATCH
    /{uuid}/appointments        GET

  /appointments/                GET (staff), POST (patient)
    /{id}                       GET, PATCH
    /{id}/events                GET  — DAG event log
    /{id}/advance               POST — trigger DAG transition
    /{id}/notes                 GET, POST
    /{id}/tasks                 GET, POST
    /{id}/tasks/{tid}           PATCH
    /{id}/results               GET
    /{id}/results/release       POST — staff triggers result release
    /{id}/upload-result         POST — multipart PDF upload

  /slots/                       GET  — ?clinic_id=&date=&type=
  /schedules/                   GET, POST (admin)
    /{id}                       GET, PATCH, DELETE

  /clinics/                     GET, POST (admin)
    /{id}                       GET, PATCH
    /{id}/staff                 GET, POST

  /prescriptions/               GET (pharmacist queue)
    /{id}                       GET
    /{id}/fulfil                POST

  /staff/                       GET, POST (admin)
    /{uuid}                     GET, PATCH
    /{uuid}/schedule            GET
    /{uuid}/absences            GET, POST

  /audit/                       GET (admin only)

  /admin/
    /config                     GET, PATCH
    /test-panels                GET, POST, PATCH
```

---

## Middleware Chain

Applied in this order on all `/api/v1/` routes:

```go
r.Use(middleware.RequestID)    // inject X-Request-ID
r.Use(middleware.RealIP)
r.Use(middleware.Logger)       // structured JSON log, no PII
r.Use(middleware.Recoverer)
r.Use(RateLimiter)             // Redis-backed, per-IP + per-user
r.Use(Authenticate)            // validate JWT, inject claims into ctx
r.Use(AuditMiddleware)         // emit AuditEvent for non-patient access
```

`Authenticate` is skipped for `/auth/*` and health endpoints.

---

## Auth Middleware Pattern

```go
// Claims injected into context
type Claims struct {
    Subject    uuid.UUID  // UUIDv5 of the user
    Role       Role
    ClinicIDs  []uuid.UUID
    Issued     time.Time
    Expires    time.Time
}

func ClaimsFromCtx(ctx context.Context) (Claims, bool)

// Role guard — use in handlers or as sub-middleware
func RequireRole(roles ...Role) func(http.Handler) http.Handler
```

---

## Audit Middleware Pattern

The `AuditMiddleware` intercepts responses. After the handler runs, if the request was made by a non-patient user and touched a patient resource, it writes an `AuditEvent`:

```go
type AuditEvent struct {
    ID           uuid.UUID
    ActorUUID    uuid.UUID
    ActorRole    string
    ResourceType string
    ResourceID   uuid.UUID
    Action       string    // READ | CREATE | UPDATE | DELETE
    Timestamp    time.Time
    IPAddress    string
    UserAgent    string
    ClinicID     *uuid.UUID
}
```

The middleware infers `ResourceType` and `ResourceID` from route parameters. Action is inferred from HTTP method (GET→READ, POST→CREATE, PATCH→UPDATE, DELETE→DELETE).

Audit writes go to a **dedicated DB connection** with INSERT-only privileges. They must never fail silently — if the audit write fails, the entire request returns 500.

---

## Database Conventions

- All DB access via `pgx/v5` with a `pgxpool.Pool`
- Migrations use sequential numbered SQL files in `/migrations/`
- Migration tool: `golang-migrate/migrate`
- Column-level encryption via `pgcrypto`: `pgp_sym_encrypt(value::text, $key)` / `pgp_sym_decrypt`
- PII columns in the `patients` table are encrypted; their type is `bytea`
- Use `sqlc` for query generation where possible; raw pgx for complex queries

---

## Message Queue Conventions (NATS JetStream)

Streams and their subjects:

| Stream | Subject Pattern | Published By | Consumed By |
|--------|----------------|--------------|-------------|
| `appointments` | `appointments.>` | Backend API | Admin portal (live updates), Notification worker |
| `test-results` | `results.>` | Backend API | Notification worker |
| `parser-inbound` | `parser.inbound` | Backend API (on PDF upload) | Test parser |
| `notifications` | `notifications.>` | Backend API | Notification worker |

All messages are JSON with envelope:
```json
{
  "trace_id": "uuid",
  "occurred_at": "RFC3339",
  "schema_version": "1",
  "payload": { ... }
}
```

---

## Response Envelope

Success responses:
```json
{ "data": { ... } }          // single resource
{ "data": [ ... ], "meta": { "total": 100, "page": 1, "per_page": 20 } }  // list
```

Error responses: see `overview/01-system.md`.
