# Session 02 — Patient & FHIR Model

## Claude Code Delegation Prompt

```
You are implementing Session 02 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, MVC handler shape
  4. specs/conventions/database.md — CNPG, PII as BYTEA, migration rules
  5. specs/conventions/errors.md — sentinel errors per domain package
  6. specs/conventions/tgv.md — patient record export requirement, consent rules

Session 01 is complete (auth, JurisdictionRouter, pkg/http, pkg/log all exist).

Your job is to implement the Patient domain: FHIR R4 Patient resource type, AES-256-GCM
PII encryption, per-jurisdiction DB routing, patient CRUD, and the TGV patient record
export endpoint.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Session 01 (auth, JurisdictionRouter, `pkg/http`, `pkg/log` all exist)

---

## Package Location

All code in this session lives under `internal/api/pkg/patient/` following the DDD
structure from `specs/go_package_structure.md`:

```
internal/api/pkg/patient/
├── patient.go       # Aggregate root struct, named types, port interfaces (PatientRepository, PatientService)
├── errors.go        # Sentinel errors for this domain
├── registration.go  # Business logic for patient registration
├── profile.go       # Business logic for patient profile read/update/export
└── repository.go    # Repository struct implementing PatientRepository
```

HTTP handlers live in `internal/api/app/httpingress/` — not in the domain package.

---

## Deliverables

### 1. FHIR Types (`pkg/fhir/`)

Minimal hand-written Go structs for FHIR R4. Only include fields this system uses.
Named types for all bounded-value fields — never plain `string`.

```go
// pkg/fhir/patient.go
package fhir

type Patient struct {
    ResourceType  string          `json:"resourceType"` // always "Patient"
    ID            string          `json:"id"`           // UUIDv5 as string
    Name          []HumanName     `json:"name"`
    BirthDate     string          `json:"birthDate"`    // YYYY-MM-DD
    Gender        Gender          `json:"gender"`
    Extension     []Extension     `json:"extension,omitempty"`
    Address       []Address       `json:"address,omitempty"`
    Telecom       []ContactPoint  `json:"telecom,omitempty"`
    Communication []Communication `json:"communication,omitempty"` // TGV: preferred language
}

type Gender string

const (
    GenderMale    Gender = "male"
    GenderFemale  Gender = "female"
    GenderOther   Gender = "other"
    GenderUnknown Gender = "unknown"
)

type Communication struct {
    Language  CodeableConcept `json:"language"`
    Preferred bool            `json:"preferred"`
}
```

Extension URLs:
- `http://stdclinic/fhir/ext/biological-sex` — valueCode: `male|female|intersex`
- `http://stdclinic/fhir/ext/pronouns` — valueString: free text

### 2. AES-256-GCM Encryption (`pkg/fhir/crypto.go`)

```go
// Encrypt encrypts plaintext using AES-256-GCM.
// The 12-byte random nonce is prepended to the ciphertext.
// key must be exactly 32 bytes.
func Encrypt(key []byte, plaintext []byte) ([]byte, error)

// Decrypt decrypts ciphertext produced by Encrypt.
func Decrypt(key []byte, ciphertext []byte) ([]byte, error)

// EncryptResource marshals v to JSON then encrypts. Used for FHIR resource storage.
// COMPLIANCE: PII encrypted at rest per PIPEDA s.4.7, HIPAA §164.312(a)(2)(iv), LGPD Art.46.
func EncryptResource(key []byte, v any) ([]byte, error)

// DecryptResource decrypts then unmarshals into v.
func DecryptResource(key []byte, data []byte, v any) error
```

### 3. Migration (`migrations/000002_patients.up.sql`)

```sql
CREATE TABLE patients (
    uuid            UUID PRIMARY KEY REFERENCES principals(uuid) ON DELETE CASCADE,
    -- COMPLIANCE: FHIR Patient resource encrypted AES-256-GCM.
    -- Legal basis: treatment (PIPEDA s.4.3, HIPAA §164.506, LGPD Art.7 VI).
    fhir_data       BYTEA NOT NULL,
    jurisdiction    jurisdiction NOT NULL,
    status          user_status NOT NULL DEFAULT 'incomplete',
    preferred_lang  TEXT NOT NULL DEFAULT 'fr-CA', -- TGV: French default for QC
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE patient_consents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_uuid    UUID NOT NULL REFERENCES patients(uuid) ON DELETE CASCADE,
    version         TEXT NOT NULL,       -- e.g. "v2024-01"
    signed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ip_address      INET,
    -- COMPLIANCE: Consent record required by PIPEDA s.4.3, LGPD Art.8, TGV Domain 2.
    consent_text_hash TEXT NOT NULL      -- SHA-256 of the consent document shown to patient
);
CREATE INDEX ON patient_consents(patient_uuid);
```

### 4. Domain Package (`internal/api/pkg/patient/patient.go`)

Aggregate root, named types, and port interfaces — all in the package entry-point file.

```go
package patient

import (
    "context"
    "github.com/WillerTravassos/showcase/pkg/auth"
    "github.com/WillerTravassos/showcase/pkg/fhir"
)

type Status string

const (
    StatusIncomplete Status = "incomplete"
    StatusActive     Status = "active"
    StatusSuspended  Status = "suspended"
    StatusDeleted    Status = "deleted"
)

type Patient struct {
    UUID          uuid.UUID
    FHIR          fhir.Patient
    Jurisdiction  auth.Jurisdiction
    Status        Status
    PreferredLang string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// PatientSummary is safe for list endpoints — no full PII.
type PatientSummary struct {
    UUID         uuid.UUID        `json:"uuid"`
    Initials     string           `json:"initials"` // derived at read time, never stored
    Status       Status           `json:"status"`
    Jurisdiction auth.Jurisdiction `json:"jurisdiction"`
}

// Initials derives display-safe initials from the FHIR name. Never returns full name.
func (p *Patient) Initials() string

// PatientRepository is the port this domain requires for persistence.
type PatientRepository interface {
    // COMPLIANCE: all writes go to the patient's jurisdiction cluster via JurisdictionRouter.
    Create(ctx context.Context, p *Patient) error
    GetByUUID(ctx context.Context, id uuid.UUID, jurisdiction auth.Jurisdiction) (*Patient, error)
    Update(ctx context.Context, p *Patient) error
    SoftDelete(ctx context.Context, id uuid.UUID, jurisdiction auth.Jurisdiction) error
    ListByClinic(ctx context.Context, clinicID uuid.UUID, jurisdiction auth.Jurisdiction, page, perPage int) ([]*PatientSummary, int, error)
    Search(ctx context.Context, query string, clinicID *uuid.UUID, jurisdiction auth.Jurisdiction) ([]*PatientSummary, error)
    RecordConsent(ctx context.Context, patientUUID uuid.UUID, version, consentTextHash, ip string) error
}

// RegistrationService is the port for patient registration.
type RegistrationService interface {
    Register(ctx context.Context, req RegisterRequest) (*Patient, error)
}

// ProfileService is the port for patient profile access and management.
type ProfileService interface {
    GetForPatient(ctx context.Context, requesterUUID uuid.UUID) (*Patient, error)
    // COMPLIANCE: staff access emits AuditEvent; audit failure blocks the response.
    GetForStaff(ctx context.Context, patientUUID uuid.UUID, requester auth.Claims) (*Patient, error)
    UpdateMutableFields(ctx context.Context, patientUUID uuid.UUID, req UpdateRequest, requester auth.Claims) (*Patient, error)
    Search(ctx context.Context, query string, requester auth.Claims) ([]*PatientSummary, error)
    // TGV Domain 2 & 5: export full FHIR Bundle for patient record portability.
    ExportRecord(ctx context.Context, patientUUID uuid.UUID, requester auth.Claims) (*fhir.Bundle, error)
}

type RegisterRequest struct {
    FHIR            fhir.Patient
    Jurisdiction    auth.Jurisdiction
    ConsentVersion  string
    ConsentTextHash string
    IPAddress       string
    PreferredLang   string // defaults to "fr-CA" if empty
}

type UpdateRequest struct {
    // Mutable fields only. Email, UUID, jurisdiction, created_at are rejected if present.
    Name          *[]fhir.HumanName
    Address       *[]fhir.Address
    Telecom       *[]fhir.ContactPoint // phone only — email ContactPoint is rejected
    Gender        *fhir.Gender
    Extensions    *[]fhir.Extension
    PreferredLang *string
}
```

### 5. Sentinel Errors (`internal/api/pkg/patient/errors.go`)

```go
var (
    ErrNotFound               = errors.New("patient not found")
    ErrAlreadyExists          = errors.New("patient already exists")
    ErrEmailImmutable         = errors.New("email cannot be changed after registration")
    ErrJurisdictionImmutable  = errors.New("jurisdiction cannot be changed after registration")
    ErrConsentRequired        = errors.New("consent must be recorded before activation")
    ErrInvalidJurisdiction    = errors.New("invalid jurisdiction")
)
```

### 6. Repository (`internal/api/pkg/patient/repository.go`)

Concrete `Repository` struct implementing `PatientRepository`:

```go
// Repository is the CNPG adapter for PatientRepository.
type Repository struct {
    router     *db.JurisdictionRouter
    encKey     []byte
}

func NewRepository(router *db.JurisdictionRouter, encKey []byte) *Repository
```

`GetByUUID` decrypts `fhir_data` before returning. List/search methods return
`PatientSummary` — no decryption, no PII exposure.

### 7. Business Logic

`registration.go` — `RegistrationService` implementation:
- Validates `RegisterRequest`, sets `PreferredLang` default to `"fr-CA"`
- Derives UUID from seed stored in `principals` (reads existing seed, does not re-generate)
- Encrypts FHIR resource, writes to jurisdiction cluster
- Records consent via `PatientRepository.RecordConsent`

`profile.go` — `ProfileService` implementation:
- `GetForStaff`: checks clinic access, emits audit event, returns full patient record. Audit failure → return error, do not return patient.
- `UpdateMutableFields`: rejects email-via-Telecom with `ErrEmailImmutable`, never touches UUID/jurisdiction/created_at.
- `ExportRecord`: assembles FHIR Bundle with all related resources.

### 8. Auth HTTP Handlers (`internal/api/app/httpingress/`)

Patient endpoints are added to the existing `Handler` struct in `httpingress/`:

```go
type Handler struct {
    // ... existing auth fields from session 01
    registration patient.RegistrationService
    profile      patient.ProfileService
    log          log.Logger
}
```

**POST /api/v1/patients** — role: patient (self-registration)

**GET /api/v1/patients/:uuid** — patient sees own; staff sees any in their clinic(s)

**PATCH /api/v1/patients/:uuid** — mutable fields only; rejects immutable fields with 422

**GET /api/v1/patients** — staff only; returns `[]PatientSummary`; supports `?q=` search

**GET /api/v1/patients/:uuid/export** — TGV Domain 2 & 5: returns FHIR Bundle
- Roles: patient (own), admin
- Assembles: Patient + all Appointments + DiagnosticReports + Observations + MedicationRequests
- Response: `application/fhir+json`

All handlers: parse input → call service → `pkghttp.WriteError(w, err)` on failure → render output. No business logic in handlers.

---

## Tests Required

- `TestEncryptDecryptRoundTrip` — table-driven with multiple payloads
- `TestEncryptResource_decryptRestoresOriginal`
- `TestPatientInitials_derivedFromFHIRName`
- `TestRegister_setsPreferredLangDefault_frCA` — if preferred_lang omitted, defaults to fr-CA
- `TestRegister_immutableFieldsSetByService` — uuid, jurisdiction, created_at not caller-settable
- `TestUpdateMutableFields_rejectsEmailInTelecom` — returns ErrEmailImmutable
- `TestUpdateMutableFields_rejectsJurisdictionChange` — returns ErrJurisdictionImmutable
- `TestGetForStaff_emitsAuditEvent`
- `TestGetForStaff_auditFailure_returnsError` — audit write fails → patient not returned
- `TestSearch_returnsSummaryOnly` — PatientSummary has no email, DOB, full name fields
- `TestExportRecord_returnsFHIRBundle`

---

## Done Criteria

- `go build ./...` and `go vet ./...` pass
- `go test -mod=vendor ./...` passes
- `golangci-lint run ./...` passes
- Patient FHIR data is encrypted at rest (BYTEA column, AES-256-GCM)
- Staff access always emits audit event; audit failure blocks the response
- Email field is unmodifiable via any PATCH endpoint
- `GET /api/v1/patients/:uuid/export` returns a valid FHIR Bundle
