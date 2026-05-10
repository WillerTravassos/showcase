# Session 05 — Staff & Absence Scheduling

## Claude Code Delegation Prompt

```
You are implementing Session 05 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, dependency flow
  4. specs/conventions/database.md — CNPG, migration conventions
  5. specs/conventions/tgv.md — staff certification tracking (TGV Domain 4 & 6)

Sessions 01 and 03 are complete.

Your job is to implement staff roster management, shift-to-slot assignment, absence
recording, statutory holiday calendars, and TGV-required staff certification tracking.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Session 01 (auth, principals table)
- Session 03 (slots, schedules, clinic model)

---

## Package Location

```
internal/api/pkg/staff/
├── staff.go         # StaffProfile, Shift, Absence, Certification, Holiday types;
│                    # StaffRepository and StaffService interfaces
├── errors.go        # Sentinel errors
├── scheduling.go    # Shift assignment and absence registration logic
├── certification.go # Certification and training management logic
└── repository.go    # Repository struct implementing StaffRepository
```

HTTP handlers for staff endpoints live in `internal/api/app/httpingress/`.

**Important:** `StaffService.AssignShift` must call `scheduling.SchedulingService.AssignPractitioner`
for each matching slot. This cross-domain orchestration is performed by injecting
`scheduling.SchedulingService` into `StaffService` at the composition root (`internal/api/app/service.go`).
Domain packages do not import each other directly — the scheduling dependency is injected via interface.

---

## Deliverables

### 1. Domain Errors (`internal/api/pkg/staff/errors.go`)

```go
var (
    ErrStaffNotFound        = errors.New("staff member not found")
    ErrShiftConflict        = errors.New("shift overlaps with existing shift")
    ErrAbsenceConflict      = errors.New("absence period overlaps booked appointments")
    ErrCertificationExpired = errors.New("staff certification has expired")
)
```

### 2. Migration (`migrations/000006_staff_scheduling.up.sql`)

```sql
CREATE TABLE staff_shifts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_uuid   UUID NOT NULL REFERENCES principals(uuid),
    clinic_id           UUID NOT NULL REFERENCES clinics(id),
    shift_date          DATE NOT NULL,
    start_time          TIME NOT NULL,
    end_time            TIME NOT NULL,
    created_by          UUID NOT NULL REFERENCES principals(uuid),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(practitioner_uuid, clinic_id, shift_date)
);
CREATE INDEX ON staff_shifts(clinic_id, shift_date);

CREATE TABLE staff_absences (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_uuid   UUID NOT NULL REFERENCES principals(uuid),
    clinic_id           UUID NOT NULL REFERENCES clinics(id),
    start_date          DATE NOT NULL,
    end_date            DATE NOT NULL,
    reason              TEXT CHECK (reason IN ('sick', 'holiday', 'other')),
    approved_by         UUID REFERENCES principals(uuid),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON staff_absences(practitioner_uuid, start_date, end_date);

CREATE TABLE statutory_holidays (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jurisdiction    jurisdiction NOT NULL,
    date            DATE NOT NULL,
    name_fr         TEXT NOT NULL,   -- TGV: French name required for QC
    name_en         TEXT NOT NULL,
    UNIQUE(jurisdiction, date)
);

-- TGV Domain 4 & 6: staff certification tracking
CREATE TABLE staff_certifications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_uuid   UUID NOT NULL REFERENCES principals(uuid),
    cert_type           TEXT NOT NULL,   -- e.g. "ITSS_COUNSELLING", "PHLEBOTOMY"
    issuer              TEXT NOT NULL,
    issued_at           DATE NOT NULL,
    expires_at          DATE,            -- null = no expiry
    certificate_url     TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON staff_certifications(practitioner_uuid, expires_at);

-- TGV Domain 6: training records
CREATE TABLE staff_training (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practitioner_uuid   UUID NOT NULL REFERENCES principals(uuid),
    course_name         TEXT NOT NULL,
    completed_at        DATE NOT NULL,
    provider            TEXT NOT NULL,
    certificate_url     TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON staff_training(practitioner_uuid);
```

### 3. Domain Types and Interfaces (`internal/api/pkg/staff/staff.go`)

```go
package staff

type AbsenceReason string

const (
    AbsenceReasonSick    AbsenceReason = "sick"
    AbsenceReasonHoliday AbsenceReason = "holiday"
    AbsenceReasonOther   AbsenceReason = "other"
)

// StaffRepository is the port this package requires for persistence.
type StaffRepository interface {
    CreateShift(ctx context.Context, s *Shift) error
    GetShiftsForPractitioner(ctx context.Context, practUUID uuid.UUID, from, to time.Time) ([]Shift, error)
    GetShiftsForClinicDate(ctx context.Context, clinicID uuid.UUID, date time.Time) ([]Shift, error)
    CreateAbsence(ctx context.Context, a *Absence) error
    ListAbsences(ctx context.Context, practUUID uuid.UUID) ([]Absence, error)
    IsHoliday(ctx context.Context, jurisdiction string, date time.Time) (bool, error)
    ListHolidays(ctx context.Context, jurisdiction string, year int) ([]Holiday, error)
    UpsertHoliday(ctx context.Context, h *Holiday) error
    AddCertification(ctx context.Context, c *Certification) error
    ListCertifications(ctx context.Context, practUUID uuid.UUID) ([]Certification, error)
    ExpiringCertifications(ctx context.Context, clinicID uuid.UUID, withinDays int) ([]Certification, error)
}

// StaffService is the port for staff roster and schedule operations.
type StaffService interface {
    AssignShift(ctx context.Context, req AssignShiftRequest) error
    RegisterAbsence(ctx context.Context, req AbsenceRequest) error
    GetPractitionerSchedule(ctx context.Context, practUUID uuid.UUID, from, to time.Time) ([]ShiftDay, error)
    IsHoliday(ctx context.Context, jurisdiction string, date time.Time) (bool, error)
    ListHolidays(ctx context.Context, jurisdiction string, year int) ([]Holiday, error)
    UpsertHoliday(ctx context.Context, h Holiday) error
    // TGV Domain 4 & 6
    AddCertification(ctx context.Context, req AddCertificationRequest) error
    ListCertifications(ctx context.Context, practUUID uuid.UUID) ([]Certification, error)
    ExpiringCertifications(ctx context.Context, clinicID uuid.UUID, withinDays int) ([]Certification, error)
}
```

### 4. Business Logic

#### `internal/api/pkg/staff/scheduling.go`

**AssignShift**:
1. Acquire `SELECT ... FOR UPDATE` on the `staff_shifts` row for this practitioner+clinic+date to prevent concurrent schedule conflicts
2. Create `staff_shifts` record
3. Find all `free` slots in clinic for the date within the shift time range
4. Call injected `scheduling.SchedulingService.AssignPractitioner` for each matching slot

**RegisterAbsence**:
1. Create `staff_absences` record
2. For each day in range: unassign all `free` slots assigned to this practitioner
3. For `busy` slots (booked appointments): emit `staff.absence.conflict` NATS event — do NOT automatically cancel; admin must review

The `scheduling.SchedulingService` dependency is injected into `StaffService` at the
composition root — not imported directly by the staff package. Declare it as a local
interface inside `scheduling.go`:

```go
// schedulingService is a local interface matching the methods StaffService needs from scheduling.
// Injected at startup from internal/api/app/service.go.
type schedulingService interface {
    AssignPractitioner(ctx context.Context, slotID, practitionerUUID uuid.UUID) error
    UnassignPractitioner(ctx context.Context, slotID uuid.UUID) error
    ListFreeSlots(ctx context.Context, clinicID uuid.UUID, date time.Time, slotType scheduling.SlotType) ([]scheduling.Slot, error)
}
```

#### `internal/api/pkg/staff/certification.go`

**ExpiringCertifications**: used by notification worker (session 17) to send 30-day expiry alerts.

### 5. HTTP Handlers (`internal/api/app/httpingress/`)

Staff endpoints are added to the existing `Handler` struct:

**GET /api/v1/staff/:uuid/schedule** — admin or own practitioner

**POST /api/v1/staff/:uuid/absences** — admin only

**GET /api/v1/staff/:uuid/absences** — admin only

**GET /api/v1/admin/schedules/unassigned?clinic_id=&date=** — admin only

**POST /api/v1/staff/:uuid/certifications** — admin only

**GET /api/v1/staff/:uuid/certifications** — admin or own

**GET /api/v1/admin/certifications/expiring?days=30** — admin only

All handlers: parse input → call service → `pkghttp.WriteError(w, err)` on failure → render output.

---

## Tests Required

- `TestAssignShift_assignsOnlySlotsWithinTimeRange`
- `TestAssignShift_skipsBreakSlots`
- `TestAssignShift_concurrentAssignment_onlyOneSucceeds` — SELECT FOR UPDATE prevents double-assign
- `TestRegisterAbsence_unassignsOnlyFreeSlots`
- `TestRegisterAbsence_busySlot_emitsConflictEvent`
- `TestIsHoliday_knownHoliday_returnsTrue`
- `TestExpiringCertifications_returnsWithin30Days`

---

## Done Criteria

- Shift assignment correctly scopes to time range
- Shift assignment uses SELECT FOR UPDATE to prevent concurrent schedule conflicts
- Absence registration unassigns free slots, flags booked slots via NATS
- Certification expiry query works and is callable by notification worker
- Cross-domain call to scheduling goes through injected interface, not direct import
- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` pass
- `go test -mod=vendor ./...` passes
