# Session 03 — Scheduling Engine

## Claude Code Delegation Prompt

```
You are implementing Session 03 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, dependency flow
  4. specs/conventions/database.md — CNPG, migration conventions
  5. specs/conventions/errors.md — sentinel errors per domain package

Sessions 01 and 02 are complete.

Your job is to implement the scheduling subsystem: clinic configuration, FHIR Schedule
and Slot resources, the slot generation algorithm, booking with double-booking prevention,
cancellation, and staff-to-slot assignment.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Session 01 (auth, JurisdictionRouter, pkg/http)
- Session 02 (patient model, jurisdiction routing pattern)

---

## Package Location

```
internal/api/pkg/scheduling/
├── scheduling.go    # Slot, Schedule types; SchedulingRepository and SchedulingService interfaces
├── errors.go        # Sentinel errors
├── generator.go     # Slot generation algorithm
├── booking.go       # Booking and cancellation business logic
└── repository.go    # Repository struct implementing SchedulingRepository

internal/api/pkg/clinic/
├── clinic.go        # Clinic type; ClinicRepository interface
├── errors.go        # Sentinel errors
└── repository.go    # Repository struct implementing ClinicRepository
```

HTTP handlers for scheduling and clinic endpoints live in `internal/api/app/httpingress/`.

---

## Deliverables

### 1. Domain Errors

`internal/api/pkg/scheduling/errors.go`:
```go
var (
    ErrSlotNotFound      = errors.New("slot not found")
    ErrSlotNotFree       = errors.New("slot is not available for booking")
    ErrSlotIsBreak       = errors.New("slot is a staff break period")
    ErrScheduleNotFound  = errors.New("schedule not found")
    ErrDoubleBooking     = errors.New("slot was taken by a concurrent booking")
)
```

`internal/api/pkg/clinic/errors.go`:
```go
var (
    ErrClinicNotFound    = errors.New("clinic not found")
    ErrInvalidBlockSize  = errors.New("block size must be 5, 10, 15, or 20 minutes")
)
```

### 2. FHIR Scheduling Types (`pkg/fhir/scheduling.go`)

Named types for all bounded-value fields — never plain `string`.

```go
// pkg/fhir/scheduling.go
package fhir

type SlotStatus string

const (
    SlotStatusFree          SlotStatus = "free"
    SlotStatusBusy          SlotStatus = "busy"
    SlotStatusBusyUnavailable SlotStatus = "busy-unavailable"
)

type SlotType string

const (
    SlotTypeInPerson SlotType = "in_person"
    SlotTypePhone    SlotType = "phone"
)

type Schedule struct {
    ResourceType    string      `json:"resourceType"` // "Schedule"
    ID              string      `json:"id"`
    Actor           []Reference `json:"actor"`
    PlanningHorizon Period      `json:"planningHorizon"`
}

type Slot struct {
    ResourceType    string           `json:"resourceType"` // "Slot"
    ID              string           `json:"id"`
    Schedule        Reference        `json:"schedule"`
    Status          SlotStatus       `json:"status"`
    Start           time.Time        `json:"start"`
    End             time.Time        `json:"end"`
    AppointmentType *CodeableConcept `json:"appointmentType,omitempty"`
}
```

### 3. Migration

`migrations/000003_clinics.up.sql`:
```sql
CREATE TABLE clinics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    address         JSONB NOT NULL,
    jurisdiction    jurisdiction NOT NULL,
    timezone        TEXT NOT NULL,            -- IANA tz, e.g. "America/Toronto"
    day_start_time  TIME NOT NULL DEFAULT '09:00',
    day_end_time    TIME NOT NULL DEFAULT '21:00',
    block_minutes   INT NOT NULL DEFAULT 10
        CHECK (block_minutes IN (5, 10, 15, 20)),
    break_minutes   INT NOT NULL DEFAULT 10
        CHECK (break_minutes >= 0 AND break_minutes < 60),
    afterhours_referral_url TEXT,            -- TGV Domain 1
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE schedules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id   UUID NOT NULL REFERENCES clinics(id),
    date        DATE NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(clinic_id, date)
);

CREATE TABLE slots (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id         UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    clinic_id           UUID NOT NULL REFERENCES clinics(id),
    start_time          TIMESTAMPTZ NOT NULL,
    end_time            TIMESTAMPTZ NOT NULL,
    status              TEXT NOT NULL DEFAULT 'free'
        CHECK (status IN ('free', 'busy', 'busy-unavailable')),
    slot_type           TEXT NOT NULL DEFAULT 'in_person'
        CHECK (slot_type IN ('in_person', 'phone')),
    practitioner_uuid   UUID REFERENCES principals(uuid),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON slots(clinic_id, start_time, status);
CREATE INDEX ON slots(schedule_id);
CREATE INDEX ON slots(practitioner_uuid);
```

`migrations/000004_appointments.up.sql`:
```sql
CREATE TABLE appointments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_uuid        UUID NOT NULL REFERENCES principals(uuid),
    slot_id             UUID NOT NULL REFERENCES slots(id),
    clinic_id           UUID NOT NULL REFERENCES clinics(id),
    practitioner_uuid   UUID REFERENCES principals(uuid),
    slot_type           TEXT NOT NULL DEFAULT 'in_person',
    test_panel          JSONB NOT NULL DEFAULT '[]',
    current_state       TEXT NOT NULL DEFAULT 'BOOKED',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON appointments(patient_uuid);
CREATE INDEX ON appointments(clinic_id, current_state);
CREATE INDEX ON appointments(slot_id);
```

### 4. Domain Types and Interfaces (`internal/api/pkg/scheduling/scheduling.go`)

```go
package scheduling

// SchedulingRepository is the port this package requires for persistence.
type SchedulingRepository interface {
    GetSlot(ctx context.Context, slotID uuid.UUID) (*Slot, error)
    ListFreeSlots(ctx context.Context, clinicID uuid.UUID, date time.Time, slotType SlotType) ([]Slot, error)
    CreateSlots(ctx context.Context, slots []Slot) error
    UpdateSlotStatus(ctx context.Context, slotID uuid.UUID, status SlotStatus) error
    AssignPractitioner(ctx context.Context, slotID, practitionerUUID uuid.UUID) error
    UnassignPractitioner(ctx context.Context, slotID uuid.UUID) error
    GetSchedule(ctx context.Context, clinicID uuid.UUID, date time.Time) (*Schedule, error)
    CreateSchedule(ctx context.Context, s *Schedule) error
    CreateAppointment(ctx context.Context, a *Appointment) error
    GetAppointment(ctx context.Context, apptID uuid.UUID) (*Appointment, error)
    UpdateAppointmentState(ctx context.Context, apptID uuid.UUID, state string) error
}

// SchedulingService is the port for slot and appointment operations.
type SchedulingService interface {
    ListFreeSlots(ctx context.Context, clinicID uuid.UUID, date time.Time, slotType SlotType) ([]Slot, error)
    GetSlot(ctx context.Context, slotID uuid.UUID) (*Slot, error)
    BookSlot(ctx context.Context, req BookingRequest) (*Appointment, error)
    CancelAppointment(ctx context.Context, apptID uuid.UUID, cancelledBy auth.Claims) error
    GenerateSchedule(ctx context.Context, clinicID uuid.UUID, date time.Time) error
    AssignPractitioner(ctx context.Context, slotID, practitionerUUID uuid.UUID) error
    UnassignPractitioner(ctx context.Context, slotID uuid.UUID) error
}

type BookingRequest struct {
    PatientUUID uuid.UUID
    SlotID      uuid.UUID
    TestPanel   []string
    BookedBy    auth.Claims
}
```

### 5. Slot Generation Algorithm (`internal/api/pkg/scheduling/generator.go`)

```go
// GenerateForDate creates all Slot records for a clinic on a given date.
// Idempotent: returns existing slots if already generated for clinic+date.
// Marks the last break_minutes of each hour as busy-unavailable.
// Does NOT assign practitioners — that is a separate operation (session 05).
func GenerateForDate(ctx context.Context, repo SchedulingRepository, c *clinic.Clinic, date time.Time) ([]Slot, error)
```

Default config (block=10min, break=10min, 09:00–21:00):
- `:00–:10` ✓ free, `:10–:20` ✓, `:20–:30` ✓, `:30–:40` ✓, `:40–:50` ✓
- `:50–:00` ✗ `busy-unavailable` (break)
- 5 bookable slots/hour × 12 hours = 60 bookable slots/day

All times are stored as TIMESTAMPTZ in UTC; the clinic timezone is used only for
display and for determining which wall-clock slots fall within operating hours.

### 6. Booking Logic (`internal/api/pkg/scheduling/booking.go`)

**BookSlot** must:
1. `SELECT ... FOR UPDATE` on the slot row — prevents concurrent double-booking
2. Verify `slot.status == SlotStatusFree` — return `ErrSlotNotFree` if not
3. Set `slot.status = SlotStatusBusy` and create `appointments` record in the same transaction
4. Publish `appointments.booked` to NATS
5. Invalidate Redis slot-availability cache key `slots:{clinic_id}:{date}:{type}`

**ListFreeSlots** caches results in Redis for 30 seconds.
Key: `slots:{clinic_id}:{date}:{type}`. Cache is invalidated by BookSlot and CancelAppointment.

### 7. HTTP Handlers (`internal/api/app/httpingress/`)

Scheduling and clinic endpoints are added to the existing `Handler` struct:

**GET /api/v1/slots** — `?clinic_id=&date=YYYY-MM-DD&type=in_person|phone`
No auth required (public availability). Slot data is non-PII.

**POST /api/v1/appointments** — role: patient or staff

**DELETE /api/v1/appointments/:id** — cancel; patient cancels own; staff cancels any in clinic

**GET /api/v1/clinics** — `?jurisdiction=` filter

**GET /api/v1/clinics/:id**

All handlers: parse input → call service → `pkghttp.WriteError(w, err)` on failure → render output.

---

## Tests Required

- `TestGenerateForDate_breakSlotsAtCorrectTimes` — table-driven: 09:50 is break, 09:40 is free
- `TestGenerateForDate_idempotent` — calling twice for same clinic+date produces no duplicates
- `TestGenerateForDate_customBlockSize` — 15-min blocks produce correct slot boundaries
- `TestBookSlot_success_slotBecomesbusy`
- `TestBookSlot_concurrentBooking_onlyOneSucceeds` — two goroutines, one gets ErrSlotNotFree
- `TestBookSlot_invalidatesCache`
- `TestCancelAppointment_slotReturnsFree`
- `TestListFreeSlots_cacheHit` — second call does not hit DB
- `TestListFreeSlots_cacheInvalidatedOnBook`

---

## Done Criteria

- Slot generation produces correct break pattern for all block sizes
- Double-booking prevented via `SELECT FOR UPDATE` under concurrent load
- Slot cache invalidated on booking and cancellation
- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` all pass
- `go test -mod=vendor ./...` passes
