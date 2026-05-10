# Architecture Conventions

---

## Go Standard Project Layout

The project follows the [Go Standard Project Layout](https://github.com/golang-standards/project-layout).
Every directory listed below has a fixed purpose. Do not invent new top-level directories.
For full file-level detail see `go_package_structure.md` at the repo root.

```
github.com/WillerTravassos/showcase/
├── cmd/                    # Binary entrypoints — one subdirectory per binary
│   ├── api/                # Core backend API
│   ├── parser/             # Medical test parser worker
│   ├── patient/            # Patient web app
│   ├── admin/              # Admin portal
│   └── notifier/           # Notification worker
│
├── internal/               # Private application code — not importable externally
│   ├── api/                # Core backend API
│   │   ├── app/            # Bootstrapping, composition root, and ingress entry points
│   │   │   ├── service.go          # package app — composition root (DI wiring)
│   │   │   ├── settings.go         # Env-var config structs for this service
│   │   │   ├── httpingress/        # HTTP handlers and page data structs
│   │   │   └── messageingress/     # NATS consumer workflows
│   │   └── pkg/            # Domain packages — importable only within internal/api
│   │       ├── patient/
│   │       ├── appointment/
│   │       ├── scheduling/
│   │       ├── prescription/
│   │       ├── staff/
│   │       └── clinic/
│   ├── parser/             # Medical test parser worker
│   │   ├── app/
│   │   └── pkg/
│   ├── patient/            # Patient web app (SSR)
│   │   ├── app/
│   │   │   ├── service.go
│   │   │   ├── settings.go
│   │   │   └── httpingress/        # HTTP handlers + pages.go (typed page data structs)
│   │   └── pkg/
│   │       ├── appointment/
│   │       └── appointmentdag/
│   ├── admin/              # Admin portal (SSR)
│   │   ├── app/
│   │   │   ├── service.go
│   │   │   ├── settings.go
│   │   │   └── httpingress/        # HTTP handlers + pages.go (typed page data structs)
│   │   └── pkg/
│   └── notifier/           # Notification worker
│       ├── app/
│       │   ├── service.go
│       │   ├── settings.go
│       │   └── messageingress/
│       └── pkg/
│
├── pkg/                    # Shared packages — importable by more than one service
│   ├── auth/               # Authentication, JWT, RBAC, UUIDv5 identity
│   ├── audit/              # Audit event writing and middleware
│   ├── fhir/               # FHIR R4 resource types and mapping helpers
│   ├── db/                 # DB pool, connection helpers, migration runner
│   ├── queue/              # NATS JetStream client, publisher, consumer helpers
│   ├── middleware/         # HTTP middleware (rate limit, CSRF, request ID)
│   ├── http/               # Shared HTTP helpers (write error, render JSON, etc.)
│   └── log/                # Logger wrapper (zerolog underneath, stdlib interface)
│
├── web/                    # Frontend assets
│   ├── patient/
│   │   ├── templates/      # html/template files
│   │   ├── static/         # Compiled CSS and JS (gitignored outputs)
│   │   └── ts/             # TypeScript source (Alpine.js components)
│   └── admin/
│       ├── templates/
│       ├── static/
│       └── ts/
│
├── migrations/             # SQL migration files (golang-migrate format)
├── build/                  # Dockerfiles and build scripts
├── deploy/                 # Kubernetes manifests per service, with base + overlays
├── infra/                  # OpenTofu IaC (see conventions/iac.md)
├── vendor/                 # Vendored dependencies — guarantees builds without network access
├── scripts/
├── docs/
├── specs/
├── Tiltfile
├── TiltfileCI
├── go.mod / go.sum
├── Makefile
└── CLAUDE.md
```

---

## Domain Layer (Pragmatic DDD)

Domain logic lives in `internal/<service>/pkg/<domain>/`. Each domain package is a Go package.
The file layout within each package follows the conventions in `go_package_structure.md`:

```
internal/api/pkg/patient/
├── patient.go       # Aggregate root struct + port interfaces (e.g. PatientRepository)
├── registration.go  # Business logic — named after the functionality, not "service"
├── repository.go    # Storage adapter implementing PatientRepository
└── errors.go        # Sentinel errors for this domain
```

### Interface naming

Interfaces are defined in `<package_name>.go` alongside the types they operate on.
The concrete implementation lives in the same package (`repository.go`) but is a distinct type:

- **Interface:** `PatientRepository` — the port. Defined in `patient.go`. Everything depends on this.
- **Implementation:** `Repository` — the Postgres adapter. Defined in `repository.go`. Never referenced directly outside the package.

```go
// patient/patient.go
type PatientRepository interface {
    GetByID(ctx context.Context, id string) (*Patient, error)
    Save(ctx context.Context, p *Patient) error
}

// patient/repository.go
type Repository struct { pool *pgxpool.Pool }  // implements PatientRepository
```

### What "Pragmatic DDD" means here

- **Domain types** are plain Go structs. No full aggregate root pattern, no domain event bus.
- **Port interfaces** (`PatientRepository`, etc.) are defined in the package that **uses** them, not the package that implements them.
- **Business logic** lives in named files (`registration.go`, `booking.go`) — never in a generic `service.go`.
- **No anemic models.** Structs may have methods for validation, derived values, and state checks. Example: `func (a *Appointment) CanTransitionTo(s State) bool`.
- **Value types** for domain concepts that have identity rules — e.g. `type Jurisdiction string` with a `func (j Jurisdiction) Validate() error`.
- **Errors are domain-owned.** `internal/<service>/pkg/<domain>/errors.go` defines `ErrPatientNotFound`, `ErrEmailImmutable`, etc. Never define sentinel errors inline.

### What is explicitly NOT done

- No CQRS, no event sourcing bus, no separate read/write models (the DAG uses event sourcing for its own state only — that is a domain-specific choice, not a system-wide pattern).
- No `AggregateRoot` base type.
- No domain event dispatcher.
- No handlers in domain packages — HTTP ingress lives in `app/httpingress/`.

---

## Frontend Architecture — Server-Side MVC

Both the patient app and admin portal follow server-side MVC:

| MVC Role | Go Equivalent | Location |
|----------|--------------|----------|
| Model | Domain types | `internal/<service>/pkg/<domain>/` |
| View | `html/template` files + typed page data structs | `web/{app}/templates/` + `internal/<service>/app/httpingress/pages.go` |
| Controller | HTTP handler functions | `internal/<service>/app/httpingress/handler.go` |

### Controller rules
- Handlers receive an HTTP request, call one or more service methods, and render a template or return JSON.
- **No business logic in handlers.** Validation of HTTP input shape (field present, correct type) is acceptable. Business rule validation belongs in the domain package.
- Every handler that renders a full page uses the `base.html` layout. Every handler that serves an HTMX partial skips the layout (detected via `HX-Request` header).
- Handlers are methods on a `Handler` struct that holds injected dependencies (service interfaces, renderer, logger).

```go
// Correct handler shape
type Handler struct {
    svc    patient.RegistrationService
    log    log.Logger
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
    // 1. Parse and validate input
    // 2. Call service
    // 3. Handle error → render error or redirect
    // 4. Render template with typed page struct
}
```

### View rules
- Template files live in `web/{app}/templates/`. Partials live in `web/{app}/templates/partials/`.
- No logic in templates beyond iteration and conditional display. No computed values.
- All data passed to templates is a typed Go struct — **never `map[string]any`**.
- Page data structs are defined in `internal/<service>/app/httpingress/pages.go`, named `{Page}Data`. Example: `AppointmentDetailData`.
- Page data structs are presentation layer — they are not domain types and must not be imported by domain packages.

### Alpine.js (TypeScript)
- TypeScript source in `web/{app}/ts/`. Compiled to `web/{app}/static/js/app.js` via esbuild.
- Alpine components are registered in `web/{app}/ts/components/{name}.ts`.
- No global state stores unless strictly necessary. Component state is local.
- Never manipulate the DOM directly — only Alpine reactive state.

---

## Cross-Jurisdiction Identity & Data Routing

### Single Global Identity
Every patient has one UUIDv5 derived from a stable internal seed generated at registration.
This UUID is the same across all regional clusters — it is the global identity anchor.
Email is a lookup key only — it is **not** the UUIDv5 seed and must never be logged.
UUIDv5 namespace constants live in `pkg/auth/`.

### Regional Cluster Routing
The backend maintains a `JurisdictionRouter` in `pkg/db/` that selects the correct CNPG cluster
connection pool based on `patient.Jurisdiction`:

```
Jurisdiction "ca" → CNPG cluster: stdclinic-ca (AWS ca-central-1)
Jurisdiction "us" → CNPG cluster: stdclinic-us (AWS us-east-1)
Jurisdiction "br" → CNPG cluster: stdclinic-br (AWS sa-east-1)
```

All DB writes for patient PII go to the patient's home cluster. Cross-cluster reads are not
permitted except during the migration flow.

### Cross-Jurisdiction Patient Migration
When a patient changes jurisdiction:

1. **Patient initiates** a migration request via the patient app
2. **Admin reviews and approves** — triggers the migration workflow
3. Migration steps (handled within `internal/api/`):
   - Validate patient identity exists in source cluster
   - Copy full patient record (FHIR Patient + all Appointments, DiagnosticReports, Observations, Prescriptions, AppointmentEvents) to destination cluster
   - Verify row counts and checksums in destination before proceeding
   - Update `patient.jurisdiction` in the global identity store (a small cross-region table in the `auth` schema that holds UUID → jurisdiction mapping only — no PII)
   - **Hard delete all PII** from source cluster: Patient FHIR data, appointments, results, prescriptions (in dependency order)
   - **Retain in source cluster:** audit events (operational records, not PII), the migration event itself, a tombstone record (`patient_migrations` table: uuid, from_jurisdiction, to_jurisdiction, migrated_at, migrated_by)
   - Emit `patient.migrated` event to NATS
4. If any step fails after the copy: rollback by deleting the destination copy and leaving source intact. Never leave data in both clusters.

**Legal basis for deletion from origin:** PIPEDA s.4.5 (retention only as long as necessary), LGPD Art.16 (data must be deleted after fulfilment of purpose). Retaining PII in the origin jurisdiction after migration has no legal basis and creates regulatory exposure.

### Data Retention Per Jurisdiction
Each jurisdiction has a maximum retention period for patient health records. After this period, records are scheduled for deletion even without a migration event:

| Jurisdiction | Retention Period | Legal Basis |
|---|---|---|
| Canada (QC) | 10 years from last encounter | Loi sur les services de santé et services sociaux |
| United States | 7 years (or age of majority + 3) | HIPAA + state law minimum |
| Brazil | 20 years | CFM Resolution 1.821/2007 |

A background job (`cmd/retention-worker` — to be specced) checks for expired records monthly and schedules deletion with a 30-day admin review window before hard delete.

---

## Dependency Injection

Services and repositories are wired in `internal/<service>/app/service.go` using plain
constructor functions — no DI framework. `cmd/{binary}/main.go` calls `app.Start()` only.

```go
// internal/api/app/service.go
package app

func Start(ctx context.Context, cfg *Settings) error {
    // Wire order:
    // 1. Connect to DB pool(s)
    // 2. Connect to NATS
    // 3. Construct repositories (pass DB pool)
    // 4. Construct domain services (pass repositories + other dependencies)
    // 5. Construct httpingress handler (pass services)
    // 6. Construct messageingress consumers (pass services)
    // 7. Build router and start server
}
```

---

## Package Naming Rules

- Package names are singular, lowercase, no underscores: `patient`, `appointment`, `scheduling`
- No `util`, `helper`, `common`, or `shared` packages — name packages by what they do
- Shared HTTP helpers live in `pkg/http/` — the package name is `http` and is aliased on import where it conflicts with stdlib: `pkghttp "github.com/WillerTravassos/showcase/pkg/http"`

---

## Type Safety Rules

**Use custom types for any domain value with a bounded or meaningful set of values — never plain `string`.**

If a field can only hold specific values, or if the value has semantic meaning beyond its string representation, it must be a named type with defined constants. This prevents invalid values from being passed silently and makes intent explicit at the call site.

```go
// Wrong — compiler cannot help you here
type Patient struct {
    Gender       string
    Jurisdiction string
}

// Correct — the type IS the documentation and the constraint
type Gender string

const (
    GenderMale     Gender = "male"
    GenderFemale   Gender = "female"
    GenderOther    Gender = "other"
    GenderUnknown  Gender = "unknown"
)

type Jurisdiction string

const (
    JurisdictionCA Jurisdiction = "ca"
    JurisdictionUS Jurisdiction = "us"
    JurisdictionBR Jurisdiction = "br"
)
```

**Rules:**
- All enumerations are named string types with `const` blocks. No bare string literals for enum values outside their defining file.
- Named types belong in `<package_name>.go` alongside the structs that use them.
- Validation methods (`func (g Gender) Valid() bool`) may be added to the type to check membership. Use them at system boundaries (HTTP ingress, NATS consumers) — not inside domain logic where the type guarantee is already in effect.
