# Session 07 — Audit Log

## Claude Code Delegation Prompt

```
You are implementing Session 07 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, dependency flow
  4. specs/conventions/database.md — audit schema, partition strategy
  5. specs/conventions/tgv.md — TGV audit chaining requirement, 10-year retention,
     cross-clinic access flag, CSV/JSON export for regulatory submission

Session 01 is complete. The AuditEvent type is referenced throughout the codebase
but not yet fully implemented.

Your job is to implement the complete audit subsystem: append-only write path with
dedicated DB user, SHA-256 event chaining for tamper-evidence (TGV requirement),
archival, and the admin query/export API.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Session 01

---

## Package Location

```
pkg/audit/
├── audit.go      # Event type, Action type, Writer interface
├── writer.go     # DBWriter implementation
├── chainer.go    # TGV SHA-256 event chaining
├── archiver.go   # Monthly S3 archival job
└── testing.go    # NoopWriter and SpyWriter (test helpers, build tag: testing)
```

Audit admin query and export handlers live in `internal/api/app/httpingress/`.

**Audit writes belong at the service layer.** When domain services access patient data
on behalf of non-patient staff, they call `audit.Writer.Write(...)` before returning
the resource. If the write fails, the service returns the error — the caller receives
a 500. This is not middleware: middleware runs post-response and cannot fail an
already-sent response.

---

## Deliverables

### 1. Migration (`migrations/000008_audit.up.sql`)

```sql
CREATE SCHEMA IF NOT EXISTS audit;

CREATE TABLE audit.events (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    actor_uuid      UUID NOT NULL,
    actor_role      TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     UUID,
    action          TEXT NOT NULL CHECK (action IN ('READ','CREATE','UPDATE','DELETE')),
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address      INET,
    user_agent      TEXT,
    clinic_id       UUID,
    request_id      TEXT,
    cross_clinic    BOOLEAN NOT NULL DEFAULT false, -- TGV: flag cross-clinic access
    -- TGV: SHA-256 chain for tamper-evidence. Each event hashes its predecessor.
    -- Chain is per resource_id per day. Verifiable by MSSS auditor.
    previous_hash   TEXT,     -- SHA-256 of previous event in chain (null for first)
    event_hash      TEXT NOT NULL, -- SHA-256 of this event's canonical fields
    PRIMARY KEY (id, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Partition creation: one table per month.
-- New partitions must be created at least 1 month ahead by a scheduled job (see archiver.go).
-- Seed the first few months in this migration:
CREATE TABLE audit.events_y2024m01 PARTITION OF audit.events
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
-- ... add additional months as needed at initial deploy time.

-- Separate table for patient migration audit trail (cross-jurisdiction, permanent)
CREATE TABLE audit.patient_migrations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_uuid        UUID NOT NULL,
    from_jurisdiction   jurisdiction NOT NULL,
    to_jurisdiction     jurisdiction NOT NULL,
    migrated_by         UUID NOT NULL,
    occurred_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    source_row_count    INT NOT NULL,
    dest_row_count      INT NOT NULL,
    checksum_match      BOOLEAN NOT NULL
);

-- DB role grants (apply in migration or CNPG cluster bootstrap):
-- GRANT USAGE ON SCHEMA audit TO audit_writer;
-- GRANT INSERT ON ALL TABLES IN SCHEMA audit TO audit_writer;
-- REVOKE UPDATE, DELETE ON ALL TABLES IN SCHEMA audit FROM audit_writer;
-- REVOKE UPDATE, DELETE ON ALL TABLES IN SCHEMA audit FROM app_user;
```

### 2. Event Model (`pkg/audit/audit.go`)

Named types for all bounded-value fields.

```go
package audit

type Action string

const (
    ActionRead   Action = "READ"
    ActionCreate Action = "CREATE"
    ActionUpdate Action = "UPDATE"
    ActionDelete Action = "DELETE"
)

type Event struct {
    ActorUUID    uuid.UUID
    ActorRole    auth.Role
    ResourceType string
    ResourceID   *uuid.UUID
    Action       Action
    IPAddress    string
    UserAgent    string
    ClinicID     *uuid.UUID
    RequestID    string
    // CrossClinic is true if the actor's clinic_ids do not include the resource's clinic.
    // TGV: cross-clinic access must always be flagged for auditor review.
    CrossClinic  bool
}

// Writer is the port for audit event persistence.
// Implementations must never discard writes silently.
// A Write failure must be returned to the caller, resulting in a 500.
type Writer interface {
    Write(ctx context.Context, e Event) error
}
```

### 3. SHA-256 Event Chain (`pkg/audit/chainer.go`)

TGV requires tamper-evident audit logs. Each event hashes its predecessor.

```go
// ComputeEventHash produces a deterministic SHA-256 hash of an event's canonical fields.
// Canonical fields (in order): actor_uuid, resource_type, resource_id, action, occurred_at (RFC3339 UTC), previous_hash.
func ComputeEventHash(e Event, occurredAt time.Time, previousHash string) string

// previousHash queries for the most recent event hash for the same resource_id
// using SELECT ... FOR UPDATE to prevent race conditions in concurrent writes.
// Returns "" if this is the first event for this resource.
func previousHash(ctx context.Context, tx pgx.Tx, resourceID uuid.UUID) (string, error)
```

`previousHash` is called within the same transaction as the INSERT, with `SELECT ... FOR UPDATE`
on the latest event row for the given `resource_id` to prevent two concurrent writes from
computing the same predecessor hash.

### 4. Audit Writer (`pkg/audit/writer.go`)

```go
// DBWriter uses the audit_writer DB pool (INSERT-only DB user).
type DBWriter struct {
    pool *pgxpool.Pool
    log  log.Logger
}

func NewDBWriter(pool *pgxpool.Pool, log log.Logger) *DBWriter
```

`DBWriter.Write` must within a single transaction:
1. `SELECT ... FOR UPDATE` on the latest event row for `resource_id` to get chain predecessor
2. Call `ComputeEventHash` to produce this event's hash
3. INSERT into `audit.events` including `previous_hash`, `event_hash`, `cross_clinic`
4. Return any error — never swallow

### 5. Test Helpers (`pkg/audit/testing.go`)

```go
// NoopWriter for unit tests where audit writes are not under test.
// T.Fatal is called if Write is called unexpectedly.
type NoopWriter struct{ T *testing.T }

// SpyWriter for tests that assert specific audit events are written.
type SpyWriter struct {
    Events []Event
    mu     sync.Mutex
}

func (s *SpyWriter) Write(ctx context.Context, e Event) error
```

### 6. Archival Job (`pkg/audit/archiver.go`)

Runs nightly via a scheduled job in `cmd/api` or a dedicated worker.

```go
type ObjectStorage interface {
    Put(ctx context.Context, key string, data []byte) error
}

type Archiver struct {
    pool    *pgxpool.Pool
    storage ObjectStorage
    log     log.Logger
}

// ArchiveMonth exports all events for year/month to:
// s3://stdclinic-audit-archive-{jurisdiction}/{year}/{month:02d}/events.ndjson.gz
// Verifies: exported row count == DB row count for the partition.
// CreateNextPartition creates the next month's partition if it does not exist.
// TGV retention: 10 years. Object storage bucket has 10-year lifecycle policy.
func (a *Archiver) ArchiveMonth(ctx context.Context, jurisdiction string, year, month int) error
func (a *Archiver) CreateNextPartition(ctx context.Context, year, month int) error
```

### 7. Admin Query & Export (`internal/api/app/httpingress/`)

Audit query endpoints are added to the existing `Handler` struct. The `Handler`
receives an `auditQueryService` interface (defined locally in httpingress) for
querying audit events.

**GET /api/v1/audit** — role: admin only

Query params: `actor_uuid`, `resource_type`, `action`, `from` (RFC3339), `to` (RFC3339),
`cross_clinic` (bool), `page`, `per_page` (max 100)

**GET /api/v1/audit/export** — role: admin only
- TGV: produces downloadable file for regulatory submission
- Query params: same filters as above
- `?format=json` → NDJSON, `?format=csv` → CSV
- Response: `Content-Disposition: attachment; filename=audit-export-{timestamp}.{ext}`

---

## Tests Required

- `TestComputeEventHash_deterministic` — same inputs always produce same hash
- `TestComputeEventHash_differentForDifferentEvents`
- `TestDBWriter_insertsEvent` (integration: `*_integration_test.go`, calls `test.RequireTiltCI(t)`)
- `TestDBWriter_setsEventHash`
- `TestDBWriter_chainsPreviousHash` — second event for same resource references first's hash
- `TestDBWriter_concurrentWrites_chainIntact` — two goroutines; chain is consistent (SELECT FOR UPDATE)
- `TestDBWriter_failurePropagates` — INSERT error returned, not swallowed
- `TestArchiveMonth_rowCountMatches` (mock storage)
- `TestExportHandler_csvFormat_correctHeaders`

---

## Done Criteria

- Every non-patient data access causes an audit write at the service layer
- Failed audit write returns error to caller (never swallowed); caller returns 500
- SHA-256 event chain is computed within a transaction with SELECT FOR UPDATE
- Cross-clinic access correctly flagged in the event
- Admin export endpoint produces CSV and JSON formats for regulatory submission
- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` pass
- `go test -mod=vendor ./...` passes
