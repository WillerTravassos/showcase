# Session 06 — Prescription Workflow

## Claude Code Delegation Prompt

```
You are implementing Session 06 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, dependency flow
  4. specs/conventions/errors.md — sentinel errors per domain package
  5. specs/conventions/tgv.md — TGV Domain 3: CMQ-approved formularies

Sessions 01, 02, and 04 are complete.

Your job is to implement the full prescription lifecycle: FHIR MedicationRequest,
doctor creates, pharmacist fulfils, audit on all access, DAG-linked state.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Session 01 (auth, pkg/http)
- Session 02 (patient model, fhir crypto)
- Session 04 (appointments table exists)

---

## Package Location

```
internal/api/pkg/prescription/
├── prescription.go  # Prescription, PrescriptionSummary types; PrescriptionRepository
│                    # and PrescriptionService interfaces
├── errors.go        # Sentinel errors
├── lifecycle.go     # Create, fulfil, cancel business logic
└── repository.go    # Repository struct implementing PrescriptionRepository
```

HTTP handlers for prescription endpoints live in `internal/api/app/httpingress/`.

---

## Deliverables

### 1. Domain Errors (`internal/api/pkg/prescription/errors.go`)

```go
var (
    ErrNotFound         = errors.New("prescription not found")
    ErrAlreadyFulfilled = errors.New("prescription has already been fulfilled")
    ErrAlreadyCancelled = errors.New("prescription has been cancelled")
    ErrNotActive        = errors.New("prescription is not in active status")
)
```

### 2. FHIR MedicationRequest (`pkg/fhir/medication.go`)

Named types for bounded-value fields.

```go
// pkg/fhir/medication.go
package fhir

type MedicationRequestStatus string

const (
    MedicationRequestStatusActive    MedicationRequestStatus = "active"
    MedicationRequestStatusCompleted MedicationRequestStatus = "completed"
    MedicationRequestStatusCancelled MedicationRequestStatus = "cancelled"
)

type MedicationRequest struct {
    ResourceType       string                  `json:"resourceType"` // "MedicationRequest"
    ID                 string                  `json:"id"`
    Status             MedicationRequestStatus `json:"status"`
    Intent             string                  `json:"intent"` // always "order"
    MedicationCodeable CodeableConcept         `json:"medicationCodeableConcept"`
    Subject            Reference               `json:"subject"`   // Patient/uuid
    Encounter          Reference               `json:"encounter"` // Appointment/uuid
    AuthoredOn         time.Time               `json:"authoredOn"`
    Requester          Reference               `json:"requester"` // Practitioner/uuid
    DosageInstruction  []Dosage                `json:"dosageInstruction"`
    Note               []Annotation            `json:"note,omitempty"`
}
```

### 3. Migration (`migrations/000007_prescriptions.up.sql`)

```sql
CREATE TABLE prescriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id      UUID NOT NULL REFERENCES appointments(id),
    patient_uuid        UUID NOT NULL REFERENCES principals(uuid),
    prescriber_uuid     UUID NOT NULL REFERENCES principals(uuid),
    -- COMPLIANCE: MedicationRequest encrypted AES-256-GCM. Legal basis: treatment.
    fhir_data           BYTEA NOT NULL,
    status              TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'completed', 'cancelled')),
    dispensed_by        UUID REFERENCES principals(uuid),
    dispensed_at        TIMESTAMPTZ,
    dispensing_notes    TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON prescriptions(appointment_id);
CREATE INDEX ON prescriptions(patient_uuid, status);
CREATE INDEX ON prescriptions(status); -- pharmacist queue
```

### 4. Domain Types and Interfaces (`internal/api/pkg/prescription/prescription.go`)

```go
package prescription

type Status string

const (
    StatusActive    Status = "active"
    StatusCompleted Status = "completed"
    StatusCancelled Status = "cancelled"
)

// Prescription is the aggregate root.
type Prescription struct {
    ID             uuid.UUID
    AppointmentID  uuid.UUID
    PatientUUID    uuid.UUID
    PrescriberUUID uuid.UUID
    FHIR           fhir.MedicationRequest
    Status         Status
    DispensedBy    *uuid.UUID
    DispensedAt    *time.Time
    DispensingNotes string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// PrescriptionSummary contains no PII — safe for list responses.
type PrescriptionSummary struct {
    ID             uuid.UUID `json:"id"`
    AppointmentID  uuid.UUID `json:"appointment_id"`
    MedicationName string    `json:"medication_name"` // from FHIR code.text
    Status         Status    `json:"status"`
    AuthoredOn     time.Time `json:"authored_on"`
}

// PrescriptionRepository is the port this package requires for persistence.
type PrescriptionRepository interface {
    Create(ctx context.Context, p *Prescription) error
    GetByID(ctx context.Context, id uuid.UUID) (*Prescription, error)
    ListForAppointment(ctx context.Context, apptID uuid.UUID) ([]Prescription, error)
    ListPending(ctx context.Context) ([]PrescriptionSummary, error)
    UpdateStatus(ctx context.Context, id uuid.UUID, status Status, dispensedBy *uuid.UUID, notes string) error
}

// PrescriptionService is the port for prescription lifecycle operations.
type PrescriptionService interface {
    // COMPLIANCE: role doctor only. Prescriptions linked to an appointment.
    Create(ctx context.Context, req CreateRequest, requester auth.Claims) (*Prescription, error)
    // COMPLIANCE: access by non-patient emits AuditEvent; audit failure blocks response.
    GetByID(ctx context.Context, id uuid.UUID, requester auth.Claims) (*Prescription, error)
    ListForAppointment(ctx context.Context, apptID uuid.UUID, requester auth.Claims) ([]Prescription, error)
    // Returns PrescriptionSummary — no PII. For pharmacist queue.
    ListPendingFulfilment(ctx context.Context, requester auth.Claims) ([]PrescriptionSummary, error)
    // COMPLIANCE: role pharmacist only.
    Fulfil(ctx context.Context, id uuid.UUID, notes string, requester auth.Claims) error
    // COMPLIANCE: role doctor only; only if status=active.
    Cancel(ctx context.Context, id uuid.UUID, requester auth.Claims) error
}
```

### 5. Business Logic (`internal/api/pkg/prescription/lifecycle.go`)

All non-patient access emits `AuditEvent`. Audit failure must propagate — not swallowed.

`GetByID` must:
1. Check `requester.HasClinicAccess(prescription.ClinicID)` or requester is the patient
2. If non-patient: call `auditWriter.Write(...)` — if audit write fails, return error (do not return prescription)
3. Return decrypted prescription

### 6. HTTP Handlers (`internal/api/app/httpingress/`)

Prescription endpoints are added to the existing `Handler` struct:

**POST /api/v1/appointments/:id/prescriptions** — role: doctor

**GET /api/v1/appointments/:id/prescriptions** — roles: doctor, pharmacist, patient (own)

**GET /api/v1/prescriptions** — role: pharmacist (returns `[]PrescriptionSummary`, no PII)

**GET /api/v1/prescriptions/:id** — roles: doctor, pharmacist, patient (own); emits audit

**POST /api/v1/prescriptions/:id/fulfil** — role: pharmacist

**DELETE /api/v1/prescriptions/:id** — role: doctor (cancel; only if active)

All handlers: parse input → call service → `pkghttp.WriteError(w, err)` on failure → render output.

---

## Tests Required

- `TestCreate_onlyDoctor_clinicWorkerReturnsErrForbidden`
- `TestFulfil_onlyPharmacist`
- `TestFulfil_alreadyFulfilled_returnsErrAlreadyFulfilled`
- `TestListPendingFulfilment_returnsSummaryOnly` — no email, DOB, address in response
- `TestGetByID_nonPatientAccess_emitsAuditEvent`
- `TestGetByID_auditFailure_returnsError` — audit write fails → prescription not returned
- `TestFulfil_emitsNATSEvent`

---

## Done Criteria

- Doctor-only create and pharmacist-only fulfil enforced at service layer
- All non-patient access emits audit event; audit failure is fatal to the request
- `PrescriptionSummary` contains no PII
- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` pass
- `go test -mod=vendor ./...` passes
