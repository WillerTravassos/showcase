# Go Package & File Structure

---

## Guiding Principles

**Common Closure Principle.** Group things that change together into the same
package. A domain package contains its entity, ports, business logic, and
adapters — because a change to a business rule typically touches the entity, its
service, and possibly its repository in one PR.

**High Cohesion / Low Coupling.** Each package owns a single responsibility.
Dependencies flow inward: handlers → domain packages → shared libraries. Domain
packages should not import each other unless there is a genuine aggregate
relationship.

**Pragmatic DDD.** We borrow useful DDD concepts (aggregates, ports, adapters)
without the full ceremony. No `domain/`, `application/`, `infrastructure/`
wrapper folders. The Go package **is** the boundary.

**Flat by default.** Avoid nesting beyond `internal/<service-name>/pkg/<package>/`. A package
should have a few files. If it grows beyond that, split by sub-domain, not by
layer.

---

## Package Structure

```text
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
│   │   ├── app/            # Bootstrapping (app start, settings, etc), and entry points of application (HTTP handlers, Message Consumers — gRPC reserved for future)
│   │   │   ├── service.go
│   │   │   ├── settings.go
│   │   │   ├── httpingress/
│   │   │   │   ├── handler.go
│   │   │   │   └── openapi.yaml
│   │   │   └── messageingress/
│   │   └── pkg/            # Domain packages — importable only within internal/api
│   │       ├── patient/
│   │       ├── appointment/
│   │       ├── scheduling/
│   │       ├── prescription/
│   │       ├── staff/
│   │       └── clinic/
│   ├── parser/             # Medical test parser worker
│   │   ├── app/
│   │   │   ├── service.go
│   │   │   └── settings.go
│   │   └── pkg/
│   ├── patient/            # Patient web app (SSR)
│   │   ├── app/            # Bootstrapping (app start, settings, etc), and entry points of application (HTTP handlers — gRPC reserved for future)
│   │   │   ├── service.go
│   │   │   ├── settings.go
│   │   │   └── httpingress/
│   │   │       ├── handler.go
│   │   │       └── pages.go        # Typed page data structs (view models passed to html/template)
│   │   └── pkg/            # Domain packages — importable only within internal/patient
│   │       ├── appointment/
│   │       └── appointmentdag/
│   ├── admin/              # Admin portal (SSR)
│   │   ├── app/            # Bootstrapping (app start, settings, etc), and entry points of application (HTTP handlers — gRPC reserved for future)
│   │   │   ├── service.go
│   │   │   ├── settings.go
│   │   │   └── httpingress/
│   │   │       ├── handler.go
│   │   │       └── pages.go        # Typed page data structs (view models passed to html/template)
│   │   └── pkg/            # Domain packages — importable only within internal/admin
│   └── notifier/           # Notification worker
│       ├── app/            # Bootstrapping (app start, settings, etc), and entry points of application (Message Consumers — gRPC reserved for future)
│       │   ├── service.go
│       │   ├── settings.go
│       │   └── messageingress/
│       └── pkg/
├── pkg/                    # Shared packages — importable by more than one service
│   ├── auth/               # Authentication, JWT, RBAC, UUIDv5 identity
│   ├── audit/              # Audit event writing and middleware
│   ├── fhir/               # FHIR R4 resource types and mapping helpers
│   ├── db/                 # DB pool, connection helpers, migration runner
│   ├── queue/              # NATS JetStream client, publisher, consumer helpers
│   ├── middleware/         # HTTP middleware (rate limit, CSRF, request ID)
│   ├── http/               # Shared HTTP helpers (render JSON, write error, etc.)
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
│   └── NNNNNN_description.{up,down}.sql
│
├── build/                  # Dockerfiles and build scripts
├── deploy/                 # Kubernetes manifests (pure kubernetes manifests loaded via kustomize)
│   ├── api/
│   │   ├── base/               # Default state of API kubernetes manifests
│   │   └── overlays/           # Overlays to change configuration of app in different environments
│   │       ├── ci              # CI layers should always refer to local overlay. Only used when we need CI to behave slightly different from local
│   │       ├── local
│   │       ├── dev
│   │       ├── stg
│   │       └── prod
│   ├── parser/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── ci
│   │       ├── local
│   │       ├── dev
│   │       ├── stg
│   │       └── prod
│   ├── patient/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── ci
│   │       ├── local
│   │       ├── dev
│   │       ├── stg
│   │       └── prod
│   ├── admin/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── ci
│   │       ├── local
│   │       ├── dev
│   │       ├── stg
│   │       └── prod
│   └── notifier/
│       ├── base/
│       └── overlays/
│           ├── ci
│           ├── local
│           ├── dev
│           ├── stg
│           └── prod
├── infra/                  # OpenTofu IaC (see conventions/iac.md)
├── scripts/                # Dev utility scripts (seed data, local helpers)
├── docs/                   # Architecture decision records (ADRs), diagrams
├── specs/                  # This directory — all spec and convention files
├── Tiltfile                # Tilt entry point — dynamically loads tilt/tilt.d/*.tiltfile
├── TiltfileCI              # CI Tilt entry point — includes Tiltfile, disables non-CI services
├── tilt/                   # Tilt configuration, local service definitions, and kustomize overlays
│   ├── services/           # Local-only services (WireMock, Jaeger, local SMTP, etc.) — never in production
│   ├── deploy/             # Tilt-managed kustomize overlays (local and CI only)
│   │   ├── api/
│   │   │   ├── base/
│   │   │   └── overlays/
│   │   │       ├── local/
│   │   │       └── ci/
│   │   ├── parser/
│   │   ├── patient/
│   │   ├── admin/
│   │   └── notifier/
│   └── tilt.d/             # Per-service tiltfiles — included dynamically by Tiltfile
│       ├── api.tiltfile
│       ├── parser.tiltfile
│       ├── patient.tiltfile
│       ├── admin.tiltfile
│       └── notifier.tiltfile
├── vendor/                 # Vendored dependencies (go mod vendor) — guarantees builds without network access
├── go.mod / go.sum
├── Makefile
└── CLAUDE.md               # Read by Claude Code on every session
```

## File Naming Conventions

**Goal:** The suggested file names are conventions and suggestions designed to
make it clear to developers what they are currently working on. It is not
intended to enforce rigid file names, but rather to establish a common
vocabulary so any developer can easily navigate any domain.

**Prefixing files:** Single files like `mapper.go`, `repository.go`,
`client.go`, and `handler.go` can be prefixed by their specific functionality
(e.g., `account_repository.go`, `connection_event_repository.go`,
`risk_client.go`) to differentiate between multiple mappers, repositories,
clients, and handlers that a package may contain.

Files are `lower_snake_case.go` and **singular** (except `mappers.go`). Package
names are **singular**, concise, lowercase, and have no underscores or hyphens.

### Core Conventions (all tiers)

| File                          | Purpose                                                                                                                                                                                                                                                                    | When to Use                                                                                  |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `<package_name>.go`           | **Package entry point.** Defines the aggregate root (structs) and ports (interfaces) that the package exposes. Other packages depend on these types.                                                                                                                       | Every package. This is the first file a reader should open.                                  |
| `<business_functionality>.go` | **Domain services & Business logic.** Defines the business logic of the domain (e.g., `registration.go`, `booking.go`). Stateless — receives dependencies via constructor. Avoid generic `service.go` names.                                                               | Domain packages with business rules to enforce.                                              |
| `repository.go`               | **Storage adapter.** Implements the repository port defined in `<package_name>.go`. Contains SQL queries and row scanning. If a package/domain contains multiple repositories, prefix them with their functionality (e.g., `account_repository.go`, `test_result_repository.go`). | Domain packages with database persistence.                                                   |
| `client.go`                   | **External system adapter.** Implements an integration port (HTTP, gRPC). Contains request building, response parsing, error mapping.                                                                                                                                      | Integration packages (faxPlus, redis/dragonfly, etc).                                           |
| `handler.go`                  | **HTTP ingress handler.** Implements the ogen-generated interface. Thin — parse request → call domain logic → respond. Split into multiple files by name if the service exposes distinct HTTP path groups.                                                                 | The `httpingress/` package.                                                                  |
| `pages.go`                    | **Page data structs (SSR only).** Typed structs passed to `html/template` for rendering. One struct per page or partial. Never `map[string]any`.                                                                                                                           | `httpingress/` in SSR services (`patient`, `admin`).                                         |
| `<workflow_name>.go`          | **Message-driven workflow.** One file per NATS-triggered business process, named by intention (e.g., `fetch_account_data.go`, `process_risk_assessment_result.go`). Contains the workflow struct, constructor, and execution logic.                                       | The `messageingress/` package.                                                               |
| `settings.go` (in `app/`)     | **Service configuration.** Env-var-tagged structs for the service. Loaded once at startup and passed into the composition root.                                                                                                                                            | Every service — lives alongside `service.go` in `internal/<service>/app/`.                  |
| `service.go` (in `app/`)      | **Bootstrap & DI wiring.** Creates dependencies, wires them together, registers with the lifecycle manager.                                                                                                                                                                | `internal/<service>/app/`. Composition root.                                                 |

### Extended Conventions (use when needed)

| File                        | Purpose                                                                                                            | When to Use                                                                        |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `mapper.go`                 | **Data transformation.** Converts between domain models and external representations (protobuf, DTOs, API models). | When transformation logic is non-trivial and clutters the handler or domain logic. |
| `<functionality_mapper>.go` | **Multiple mappers.** Same as mapper.go but when there are several entity mappings, split them into specific files | When the number of entity mappers increases and becomes unmanageable.              |
| `metrics.go`                | **OTel metrics.** Defines counters, histograms, gauges for the package.                                            | Packages with custom observability needs beyond auto-instrumentation.              |
| `errors.go`                 | **Custom error types.** Sentinel errors, error code enums, error constructors.                                     | Packages with domain-specific error semantics.                                     |
| `state.go`                  | **State machine.** Defines states, transitions, and guards.                                                        | Workflow/pipeline packages with explicit state progression.                        |
| `validation.go`             | **Business validation.** Validates input beyond what the API schema covers.                                        | Handler or domain packages with complex validation rules.                          |
| `security.go`               | **Auth & authorization.** JWT validation, permission checks, security middleware.                                  | Handler packages that implement auth concerns.                                     |
| `orchestrator.go`           | **Workflow orchestrator.** Coordinates multi-step processes, manages task execution order.                         | Workflow engine packages.                                                          |

### Files You Won't Name (generated or conventional)

| File        | Notes                                                                                                  |
| ----------- | ------------------------------------------------------------------------------------------------------ |
| `oas_*.go`  | ogen-generated — lives in `httpingress/`, never edit.                                                  |
| `*_test.go` | Go convention. Co-located with the file under test.                                                    |

---

## Package Design Rules

The examples below are illustrative, and do not define or reflect actual project structure.

### 1. The `<package_name>.go` file is the contract

This file answers: "What does this package do and what does it need?"

```go
// patient/patient.go

package patient

import "context"

// Patient is the aggregate root.
type Patient struct {
    ResourceType  string          `json:"resourceType"` // always "Patient"
    ID            string          `json:"id"`           // UUIDv5 as string
    Name          []HumanName     `json:"name"`
    BirthDate     string          `json:"birthDate"`    // YYYY-MM-DD
    Gender        Gender          `json:"gender"`       // male|female|other|unknown
    Extension     []Extension     `json:"extension,omitempty"`
    Address       []Address       `json:"address,omitempty"`
    Telecom       []ContactPoint  `json:"telecom,omitempty"`
    Communication []Communication `json:"communication,omitempty"` // TGV: preferred language
}

type Gender string

const (
    GenderMale     Gender = "male"
    GenderFemale   Gender = "female"
    ...
)

// PatientRepository is the port this package requires for persistence.
// Defined here so callers depend on the interface, not the implementation.
type PatientRepository interface {
    GetByID(ctx context.Context, id string) (*Patient, error)
    Save(ctx context.Context, p *Patient) error
}
```

### 2. Adapters implement ports in the same package

The pragmatic Go trade-off: infrastructure implementations live alongside their
port definitions. This keeps packages self-contained and avoids an
`infrastructure/` layer.

```go
// patient/repository.go

package patient

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the Postgres implementation of PatientRepository.
type Repository struct {
    pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
    return &Repository{pool: pool}
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Patient, error) {
    // SQL query...
}

func (r *Repository) Save(ctx context.Context, p *Patient) error {
    // SQL query...
}
```

### 3. Services are business logic managers

Instead of generic `service.go` files, name the file after the business
functionality it implements.

```go
// patient/registration.go

package patient

import "context"

type RegistrationService struct {
    repo PatientRepository // Depends on the interface, not the implementation
}

func NewRegistrationService(repo PatientRepository) *RegistrationService {
    return &RegistrationService{repo: repo}
}

func (s *RegistrationService) RegisterPatient(ctx context.Context, id string) (*Patient, error) {
    return s.repo.GetByID(ctx, id)
}
```

### 4. HTTPIngresses are thin HTTP handlers

`httpingress/` owns synchronous HTTP ingress. Methods parse requests, call domain
logic, and format responses. No business logic.

```go
// httpingress/handler.go

package httpingress

import "github.com/WillerTravassos/showcase/internal/api/pkg/patient"

func (h *Handler) GetPatient(ctx context.Context, params api.GetPatientParams) (*api.Patient, error) {
    p, err := h.registrationService.RegisterPatient(ctx, params.ID)
    if err != nil {
        return nil, mapError(err)
    }
    return toAPIPatient(p), nil
}
```

If a service exposes distinct HTTP path groups, split into multiple handler
files named after the path group (e.g., `patient.go`, `staff.go`, etc).
For small APIs, a single `handler.go` is fine.

#### Server-side rendering (SSR) services

For `patient` and `admin`, `httpingress/` also owns the typed page data structs
passed to `html/template`. These are **not** domain types — they are view models
that belong next to the handlers that use them.

```go
// httpingress/pages.go

package httpingress

// AppointmentBookingPage is the view model for the appointment booking flow.
// Passed directly to the html/template renderer — never map[string]any.
type AppointmentBookingPage struct {
    Patient      PatientSummary
    Clinics      []ClinicOption
    AvailSlots   []TimeSlot
    FlashError   string
}
```

**MVC layer mapping for SSR services:**

| MVC Layer  | Location                                                                 |
| ---------- | ------------------------------------------------------------------------ |
| Model      | `internal/<service>/pkg/<package>/`                                      |
| View       | `web/<service>/templates/` (template files) + `httpingress/pages.go` (typed data structs) |
| Controller | `internal/<service>/app/httpingress/handler.go`                          |

Template files and typed page structs are intentionally split: templates are
assets (compiled, embedded, or served statically) while page structs are Go
types that must stay close to the handlers that populate them. Keeping them in
`httpingress/` satisfies the Common Closure Principle — the struct and the
handler that fills it change together.

### 5. MessageIngresses handlers are message-driven business processes

`messageingress/` owns asynchronous message ingress (NATS). Each file is one workflow
— a complete business process triggered by a specific message type, named by its
business intention.

```go
// messageingress/fax_test_result_ingestor.go

package messageingress

type FaxTestResultIngestor struct {
    registrationService     patient.RegistrationService
    faxPlusClient           faxplus.Client
}

func NewFaxTestResultIngestor(svc patient.RegistrationService, client faxplus.Client) *FaxTestResultIngestor {
    return &FaxTestResultIngestor{registrationService: svc, faxPlusClient: client}
}

func (w *FaxTestResultIngestor) Consume(ctx context.Context, message *envelope.Message) error {
    // Orchestrate the business process...
}
```

**`httpingress/` vs `messageingress/`:**

- `httpingress/` — HTTP request/response. Implements ogen interface. Stateless,
  synchronous.
- `messageingress/` — NATS message processing. One struct per business process. May
  coordinate multiple domain services and produce side effects (DB writes,
  publishing new messages).

### 6. Integration packages are named after the external system

Not after the protocol. For example, use `grafana/`, not `httpclient/`. Use `/`,
not `grpc/`. The name tells you **what** you're integrating with, and the files
inside tell you **how**.

### 7. `internal/<service-name>/app/service.go` is the composition root

This is where all dependencies are created and wired together. It's the only
place that should know about concrete implementations.

```go
// internal/api/app/service.go

package app

func Start(ctx context.Context, cfg *Settings) error {
    pool := database.Connect(cfg.DatabaseURL)

    patientRepo := patient.NewRepository(pool)
    registrationSvc := patient.NewRegistrationService(patientRepo)
    faxPlusClient := faxplus.NewClient(cfg.FaxPlusURL)

    // HTTP ingress
    handler := httpingress.NewHandler(registrationSvc, faxPlusClient)

    // NATS ingress
    faxTestResultIngestor := messageingress.NewFaxTestResultIngestor(registrationSvc, faxPlusClient)

    // ... register HTTP routes, subscribe NATS topics, start server
}
```

### 8. Combine packages in handlers/workflows to avoid import cycles

If multiple domain packages need to orchestrate logic together, **do not import
them directly into each other**. This creates tight coupling and often leads to
weird import or reference cycles. Instead, combine the logic from multiple
domain packages at the ingress layer within your `httpingress/` or `messageingress/`
packages. The ingress layer should be the orchestrator.

---

## Dependency Flow

**Rules:**

- Domain packages **do not** import each other (unless there's a genuine
  aggregate relationship).
- `httpingress/` and `messageingress/` **import** domain and integration packages.
- `app/` **imports** everything (it's the composition root).
- No package imports `httpingress/`, `messageingress/`, or `app/`.

---

## Testing Conventions

| What              | Where                                                               | Pattern                                        |
| ----------------- | ------------------------------------------------------------------- | ---------------------------------------------- |
| Unit tests        | `*_test.go` co-located with source                                  | Table-driven with testify                      |
| Mocks             | Use wiremock (via tiltfile) to generate external response. Use JSON files to mock behavior locally, and use stubs when running integration test | Mock specific responses, use a value to determine routing to different responses |
| Integration tests | `*_integration_test.go` co-located or in `internal/<>/test/`        | Use `test.RequireTiltCI()`                     |

---

## Suggestions when to Split a Package

A package should be split when:

- It has **more than ~8 files** (excluding tests) — it's doing too much.
- Two groups of files **change independently** — they don't belong together.
- You find yourself wanting **sub-folders** — that's a signal to promote to a
  sibling package.
