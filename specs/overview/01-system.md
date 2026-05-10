# Overview: System-Wide Conventions

> **Read this file at the start of every session.** It defines rules that apply across all components.

---

## Repo Layout

See `specs/go_package_structure.md` for the full authoritative tree. Key top-level directories:

```
/
├── cmd/                    # Binary entrypoints — one per service
│   ├── api/
│   ├── parser/
│   ├── patient/
│   ├── admin/
│   └── notifier/
├── internal/               # Private application code
│   ├── api/                # Core backend API
│   │   ├── app/            # service.go, settings.go, httpingress/, messageingress/
│   │   └── pkg/            # Domain packages: patient/, appointment/, scheduling/, ...
│   ├── parser/
│   │   ├── app/
│   │   └── pkg/
│   ├── patient/            # Patient web app (SSR)
│   │   ├── app/
│   │   └── pkg/
│   ├── admin/              # Admin portal (SSR)
│   │   ├── app/
│   │   └── pkg/
│   └── notifier/
│       ├── app/
│       └── pkg/
├── pkg/                    # Shared packages (importable by multiple services)
│   ├── auth/               # JWT, RBAC, UUIDv5 identity
│   ├── audit/              # Audit event writing
│   ├── fhir/               # FHIR R4 resource types and mapping helpers
│   ├── db/                 # DB pool, JurisdictionRouter, migration runner
│   ├── queue/              # NATS JetStream client and helpers
│   ├── middleware/         # HTTP middleware (rate limit, CSRF, request ID)
│   ├── http/               # Shared HTTP helpers (WriteError, render JSON)
│   └── log/                # Logger wrapper (zerolog underneath)
├── web/
│   ├── patient/            # Patient app templates and static assets
│   └── admin/              # Admin portal templates and static assets
├── migrations/             # SQL migration files (golang-migrate, append-only)
├── deploy/                 # Kustomize manifests per service (base + overlays)
├── tilt/                   # Tilt local dev configuration
├── infra/                  # OpenTofu IaC
├── specs/                  # This directory
├── vendor/                 # Vendored dependencies
└── Makefile
```

---

## Language & Tooling

- **Go 1.26+** (latest stable) for all services
- **golangci-lint** with project config (`.golangci.yaml`); CI must pass
- **go vet** must pass on every commit
- All packages use the module path `github.com/WillerTravassos/showcase`
- Unit tests live alongside source in `*_test.go` files
- Integration tests are named `*_integration_test.go`. Tests requiring the live Tilt cluster call `test.RequireTiltCI(t)` at the top — skipped when `TILT_CI` env var is not set. No `//go:build` tags for integration tests.
- All dependencies are vendored. Build and test with `-mod=vendor`.

---

## Universal Identity: UUIDv5

Every principal (patient, staff, admin) is identified by a UUIDv5.

Namespace constants and identity helpers live in `pkg/auth/`.

```go
// pkg/auth/identity.go

// Namespaces — defined once, never changed.
var (
    NamespacePatient = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
    NamespaceStaff   = uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
)

// NewPatientUUID derives a UUIDv5 from a stable internal seed generated at registration.
// The seed is a 32-byte random value stored in the principals table — NOT derived from email.
func NewPatientUUID(seed []byte) uuid.UUID {
    return uuid.NewSHA1(NamespacePatient, seed)
}
```

- **UUID seed is a `crypto/rand` 32-byte value generated at registration**, stored in the principals table. It is never derived from email.
- Email is a lookup key only — it must never be used as a UUID seed.
- Email is **immutable** post-registration. No endpoint may accept an email update.
- UUIDs are safe to log; email is never logged.
- Never pass raw email to downstream services; pass UUID + jurisdiction.

---

## FHIR R4

All patient health data is modelled as **HL7 FHIR R4** resources. The `pkg/fhir/` package owns all FHIR types.

Key resources used:
| FHIR Resource | Internal Use |
|---------------|-------------|
| `Patient` | Patient demographics and PII |
| `Practitioner` | Doctors, pharmacists, clinic workers |
| `PractitionerRole` | Role + clinic assignment for a practitioner |
| `Schedule` | A clinic's availability block for a day |
| `Slot` | A single 10-min bookable block |
| `Appointment` | A patient-slot booking |
| `DiagnosticReport` | Lab test result bundle |
| `Observation` | Individual analyte result within a report |
| `MedicationRequest` | Prescription |
| `AuditEvent` | Access log event |

FHIR resources containing PII are stored as **encrypted JSONB** (AES-256-GCM) in CNPG. Non-PII metadata columns (uuid, clinic_id, status, created_at) are stored as regular indexed columns alongside.

---

## PII & Compliance Rules

These rules are **non-negotiable** and must be respected in every session:

1. **Never log PII.** Structured log fields must not contain name, DOB, email, phone, or address. Log UUIDs only.
2. **Never return PII in list endpoints.** List responses return UUID + non-sensitive metadata only. Full PII is returned only in single-resource GET endpoints.
3. **Jurisdiction tagging.** Every patient record has a `jurisdiction` field (`ca`, `us`, `br`). DB writes route to the jurisdiction-appropriate CNPG cluster via `pkg/db/JurisdictionRouter`.
4. **Immutable patient fields.** `email`, `uuid`, `jurisdiction`, `created_at` must never be updatable via any API endpoint.
5. **Audit all non-patient access.** Any read or write of patient data by a non-patient user must emit an `AuditEvent`. Audit writes that fail must propagate the error — silent audit failures are a certification blocker.

---

## Error Handling Convention

All API handlers return a consistent JSON error envelope:

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "appointment not found",
    "request_id": "uuid"
  }
}
```

- Sentinel errors are defined in `internal/<service>/pkg/<domain>/errors.go`. Never inline them.
- Shared HTTP error rendering lives in `pkg/http/`. Use `pkghttp.WriteError` in handlers — never `http.Error`.
- Never return raw Go error strings to clients.

---

## Configuration

All configuration is loaded from environment variables at startup. No config files are read at runtime. Each service defines its own typed config struct in `internal/<service>/app/settings.go`. There is no shared `pkg/config/` — config structs are service-specific.

```go
// internal/api/app/settings.go
package app

type Settings struct {
    DatabaseURL   string `env:"DATABASE_URL,required"`
    NATSUrl       string `env:"NATS_URL,required"`
    JWTSecret     string `env:"JWT_SECRET,required"`
    EncryptionKey string `env:"ENCRYPTION_KEY,required"` // 32-byte hex for AES-256-GCM
    Environment   string `env:"ENV"              envDefault:"development"`
}
```

Use `github.com/caarlos0/env/v11` for parsing. Load once in `main.go` and pass into `app.Start()`.

---

## Dependency Versions (go.mod anchors)

```
github.com/google/uuid               v1.x
github.com/jackc/pgx/v5             v5.x
github.com/nats-io/nats.go           v1.x
github.com/golang-jwt/jwt/v5         v5.x
github.com/caarlos0/env/v11          v11.x
github.com/rs/zerolog                v1.x   (via pkg/log wrapper only — never import directly)
github.com/ogen-go/ogen              latest
github.com/Masterminds/squirrel      v1.x   (repository layer only)
github.com/stretchr/testify          v1.x
golang.org/x/crypto                  latest
```
