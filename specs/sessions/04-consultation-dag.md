# Session 04 — Consultation Workflow DAG

## Claude Code Delegation Prompt

```
You are implementing Session 04 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD package structure, dependency flow
  4. specs/conventions/errors.md — sentinel errors per domain package
  5. specs/conventions/tgv.md — pre-test counselling enforcement, MADO trigger, clinical note requirements

Sessions 01, 02, and 03 are complete.

Your job is to implement the consultation workflow as an event-sourced DAG state machine,
including TGV-required clinical note enforcement, the MADO evaluation hook, notes, and tasks.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Dependencies
- Sessions 01, 02, 03

---

## Package Location

```
internal/api/pkg/appointment/
├── appointment.go    # State type, AppointmentEvent, Note, Task types;
│                     # DAGRepository and DAGService interfaces
├── errors.go         # Sentinel errors
├── statemachine.go   # Transition graph, role permissions, clinical note rules
├── dag.go            # DAGService implementation (Advance, CurrentState, EventHistory)
├── notes.go          # Note and task business logic
├── mado.go           # MADOEvaluator interface and implementation
└── repository.go     # Repository struct implementing DAGRepository
```

HTTP handlers for appointment DAG endpoints live in `internal/api/app/httpingress/`.

---

## DAG Definition

```
BOOKED              → ARRIVED, CANCELLED, NO_SHOW
ARRIVED             → SAMPLES_COLLECTED, CANCELLED
SAMPLES_COLLECTED   → SENT_TO_LAB
SENT_TO_LAB         → RESULTS_RECEIVED
RESULTS_RECEIVED    → PHONE_APPT_BOOKED
PHONE_APPT_BOOKED   → PHONE_APPT_COMPLETED, PHONE_APPT_BOOKED  (re-book = self-loop)
PHONE_APPT_COMPLETED → RESULTS_RELEASED
RESULTS_RELEASED    → CLOSED
CANCELLED           → (terminal)
NO_SHOW             → (terminal)
CLOSED              → (terminal)
```

Role permissions per transition:

| Transition | Permitted Roles |
|---|---|
| BOOKED → ARRIVED | clinic_worker |
| ARRIVED → SAMPLES_COLLECTED | clinic_worker |
| SAMPLES_COLLECTED → SENT_TO_LAB | clinic_worker |
| SENT_TO_LAB → RESULTS_RECEIVED | system (parser), clinic_worker |
| RESULTS_RECEIVED → PHONE_APPT_BOOKED | clinic_worker |
| PHONE_APPT_BOOKED → PHONE_APPT_COMPLETED | doctor |
| PHONE_APPT_COMPLETED → RESULTS_RELEASED | clinic_worker, doctor |
| RESULTS_RELEASED → CLOSED | system (auto) |
| * → CANCELLED | patient (own, before ARRIVED), clinic_worker, admin |
| BOOKED → NO_SHOW | clinic_worker |

---

## Deliverables

### 1. Domain Errors (`internal/api/pkg/appointment/errors.go`)

```go
var (
    ErrInvalidTransition      = errors.New("transition is not valid from current state")
    ErrTransitionNotPermitted = errors.New("role not permitted for this transition")
    ErrCounsellingRequired    = errors.New("pre-test counselling note required before samples can be collected")
    ErrClinicalNoteRequired   = errors.New("clinical note required for this transition")
    ErrAppointmentNotFound    = errors.New("appointment not found")
    ErrTerminalState          = errors.New("appointment is in a terminal state")
)
```

### 2. Migration (`migrations/000005_dag.up.sql`)

```sql
CREATE TABLE appointment_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id  UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    from_state      TEXT NOT NULL,
    to_state        TEXT NOT NULL,
    actor_uuid      UUID NOT NULL REFERENCES principals(uuid),
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    payload         JSONB NOT NULL DEFAULT '{}'
    -- COMPLIANCE: append-only. Enforced by DB role REVOKE UPDATE, DELETE.
    -- TGV Domain 5: full clinical encounter history for record integrity.
);
CREATE INDEX ON appointment_events(appointment_id, occurred_at);
-- REVOKE UPDATE, DELETE ON appointment_events FROM app_user;

CREATE TABLE appointment_notes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id  UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    author_uuid     UUID NOT NULL REFERENCES principals(uuid),
    note_type       TEXT NOT NULL DEFAULT 'general',
    -- note_type values: general | pre_test_counselling | clinical | phone_summary
    -- TGV Domain 2: pre_test_counselling required before SAMPLES_COLLECTED transition.
    -- TGV Domain 3: clinical note required for PHONE_APPT_COMPLETED and RESULTS_RELEASED.
    body            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON appointment_notes(appointment_id, note_type);

CREATE TABLE appointment_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id  UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    created_by      UUID NOT NULL REFERENCES principals(uuid),
    assigned_role   user_role,
    title           TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'in_progress', 'done')),
    due_hint        TEXT,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON appointment_tasks(appointment_id, status);

-- TGV Domain 3 & 8: MADO declaration tracking
CREATE TABLE mado_declarations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id  UUID NOT NULL REFERENCES appointments(id),
    patient_uuid    UUID NOT NULL REFERENCES principals(uuid),
    clinic_id       UUID NOT NULL REFERENCES clinics(id),
    physician_uuid  UUID REFERENCES principals(uuid),
    condition_code  TEXT NOT NULL,   -- e.g. "HIV", "SYPHILIS_INFECTIOUS"
    icd10_code      TEXT NOT NULL,
    observation_id  UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending_physician_review'
        CHECK (status IN (
            'pending_physician_review',
            'ready_for_submission',
            'submitted',
            'acknowledged',
            'overdue',
            'evaluation_failed'  -- written when MADOEvaluator encounters an error; never silent
        )),
    deadline_at     TIMESTAMPTZ NOT NULL,  -- 48h or 7d from DiagnosticReport.issued
    submitted_at    TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    dsp_region      TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON mado_declarations(appointment_id);
CREATE INDEX ON mado_declarations(status, deadline_at);

-- TGV Domain 3: physician sign-off on DiagnosticReport before results can be released
CREATE TABLE appointment_signoffs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id  UUID NOT NULL REFERENCES appointments(id),
    physician_uuid  UUID NOT NULL REFERENCES principals(uuid),
    signed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes           TEXT,
    UNIQUE(appointment_id)  -- one sign-off per appointment
);
```

### 3. State and Types (`internal/api/pkg/appointment/appointment.go`)

```go
package appointment

type State string

const (
    StateBooked             State = "BOOKED"
    StateArrived            State = "ARRIVED"
    StateSamplesCollected   State = "SAMPLES_COLLECTED"
    StateSentToLab          State = "SENT_TO_LAB"
    StateResultsReceived    State = "RESULTS_RECEIVED"
    StatePhoneApptBooked    State = "PHONE_APPT_BOOKED"
    StatePhoneApptCompleted State = "PHONE_APPT_COMPLETED"
    StateResultsReleased    State = "RESULTS_RELEASED"
    StateClosed             State = "CLOSED"
    StateCancelled          State = "CANCELLED"
    StateNoShow             State = "NO_SHOW"
)

type NoteType string

const (
    NoteTypeGeneral            NoteType = "general"
    NoteTypePreTestCounselling NoteType = "pre_test_counselling"
    NoteTypeClinical           NoteType = "clinical"
    NoteTypePhoneSummary       NoteType = "phone_summary"
)

type TaskStatus string

const (
    TaskStatusOpen       TaskStatus = "open"
    TaskStatusInProgress TaskStatus = "in_progress"
    TaskStatusDone       TaskStatus = "done"
)

// AdvancePayload carries structured data for a state transition.
// Use typed fields — never map[string]any in domain code.
type AdvancePayload struct {
    CancellationReason string `json:"cancellation_reason,omitempty"`
    SlotID             string `json:"slot_id,omitempty"` // for PHONE_APPT_BOOKED
}

// DAGRepository is the port this package requires for persistence.
type DAGRepository interface {
    AppendEvent(ctx context.Context, e *AppointmentEvent) error
    ListEvents(ctx context.Context, apptID uuid.UUID) ([]AppointmentEvent, error)
    UpdateCurrentState(ctx context.Context, apptID uuid.UUID, state State) error
    GetLatestNoteOfType(ctx context.Context, apptID uuid.UUID, noteType NoteType, after time.Time) (*Note, error)
    AppendNote(ctx context.Context, n *Note) error
    ListNotes(ctx context.Context, apptID uuid.UUID) ([]Note, error)
    CreateTask(ctx context.Context, t *Task) error
    ListTasks(ctx context.Context, apptID uuid.UUID) ([]Task, error)
    UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status TaskStatus) error
}

// DAGService is the port for appointment workflow operations.
type DAGService interface {
    Advance(ctx context.Context, req AdvanceRequest) (*AppointmentEvent, error)
    CurrentState(ctx context.Context, apptID uuid.UUID) (State, error)
    EventHistory(ctx context.Context, apptID uuid.UUID) ([]AppointmentEvent, error)
    AddNote(ctx context.Context, apptID uuid.UUID, authorUUID uuid.UUID, noteType NoteType, body string) (*Note, error)
    ListNotes(ctx context.Context, apptID uuid.UUID) ([]Note, error)
    AddTask(ctx context.Context, apptID uuid.UUID, req AddTaskRequest) (*Task, error)
    ListTasks(ctx context.Context, apptID uuid.UUID) ([]Task, error)
    UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status TaskStatus, actor uuid.UUID) (*Task, error)
}

type AdvanceRequest struct {
    AppointmentID uuid.UUID
    ToState       State
    Actor         auth.Claims
    Payload       AdvancePayload
}
```

### 4. State Machine (`internal/api/pkg/appointment/statemachine.go`)

```go
var ValidTransitions map[State][]State
var PermittedRoles map[string][]auth.Role // key: "FROM→TO"

func IsValidTransition(from, to State) bool
func IsPermittedRole(from, to State, role auth.Role) bool
func IsTerminal(s State) bool

// RequiresClinicalNote returns true for transitions that TGV mandates a note for.
// ARRIVED→SAMPLES_COLLECTED requires NoteTypePreTestCounselling.
// PHONE_APPT_COMPLETED and RESULTS_RELEASED require NoteTypeClinical.
func RequiresClinicalNote(from, to State) (required bool, noteType NoteType)
```

### 5. DAG Service Implementation (`internal/api/pkg/appointment/dag.go`)

**Advance** must execute these steps in order:

1. Load current state via `CurrentState` (Redis cache → event replay on miss)
2. `IsValidTransition` → `ErrInvalidTransition` on failure
3. `IsPermittedRole` → `ErrTransitionNotPermitted` on failure
4. **TGV clinical note enforcement** — call `RequiresClinicalNote(current, toState)`:
   - If required, query for a note of the required type created after the appointment reached its current state
   - If not found: return `ErrCounsellingRequired` or `ErrClinicalNoteRequired` as appropriate
   - This check is in the service, not the handler
5. Persist `AppointmentEvent` (INSERT only, never UPDATE)
6. Update `appointments.current_state` cache column
7. Dispatch side effects asynchronously via NATS (see below)
8. **TGV MADO hook** — if `toState == StateResultsReceived`: call `MADOEvaluator.Evaluate(ctx, apptID)` asynchronously. On evaluation error, write a `mado_declarations` record with `status = 'evaluation_failed'` — never silent.
9. Return the event

### 6. MADO Evaluator (`internal/api/pkg/appointment/mado.go`)

Called when results are received. Inspects DiagnosticReport Observations for positive
results matching MADO-listed conditions.

```go
type MADOEvaluator interface {
    // Evaluate checks DiagnosticReport Observations for the appointment.
    // For each positive MADO condition found, creates a mado_declarations record.
    // Asynchronous: called after RESULTS_RECEIVED transition without blocking it.
    // On evaluation error: writes a record with status=evaluation_failed — never silent.
    Evaluate(ctx context.Context, apptID uuid.UUID) error
}

// MADOConditions maps ICD-10 codes to MADO conditions and their deadlines.
// Source: specs/conventions/tgv.md MADO table.
var MADOConditions = map[string]MADOCondition{
    "B20":  {Name: "HIV",               DeadlineHours: 48,  DSPOnly: false},
    "Z21":  {Name: "HIV",               DeadlineHours: 48,  DSPOnly: false},
    "A51":  {Name: "SYPHILIS_INFECTIOUS", DeadlineHours: 48, DSPOnly: false},
    "A54":  {Name: "GONORRHOEA",        DeadlineHours: 168, DSPOnly: true},
    "A56":  {Name: "CHLAMYDIA",         DeadlineHours: 168, DSPOnly: true},
    "B16":  {Name: "HEPATITIS_B_ACUTE", DeadlineHours: 48,  DSPOnly: true},
    "B17.1":{Name: "HEPATITIS_C_ACUTE", DeadlineHours: 48,  DSPOnly: true},
    "A55":  {Name: "LGV",               DeadlineHours: 48,  DSPOnly: false},
}
```

### 7. Side Effects (`internal/api/pkg/appointment/dag.go`)

Side effects are published to NATS asynchronously after the event is persisted.
Publish errors are logged but do not roll back the transition — the state change is already committed.

| Transition | NATS Subject | Payload |
|---|---|---|
| → ARRIVED | `appointments.arrived` | appt_id, clinic_id, patient_uuid |
| → SENT_TO_LAB | `appointments.sent_to_lab` | appt_id, clinic_id |
| → RESULTS_RECEIVED | `appointments.results_received` | appt_id |
| → PHONE_APPT_BOOKED | `appointments.phone_booked` | appt_id, slot_id |
| → RESULTS_RELEASED | `appointments.results_released` | appt_id, patient_uuid |
| → RESULTS_RELEASED | auto-advance to CLOSED (immediate) | internal |
| → CANCELLED | `appointments.cancelled` | appt_id, cancelled_by |
| → NO_SHOW | `appointments.no_show` | appt_id |

### 8. HTTP Handlers (`internal/api/app/httpingress/`)

DAG endpoints are added to the existing `Handler` struct:

**POST /api/v1/appointments/:id/advance**
Body: `{ "to_state": "ARRIVED", "payload": { "cancellation_reason": "..." } }`

**GET /api/v1/appointments/:id/events**

**POST /api/v1/appointments/:id/notes**
Body: `{ "note_type": "general", "body": "..." }`

**GET /api/v1/appointments/:id/notes**

**POST /api/v1/appointments/:id/tasks**

**GET /api/v1/appointments/:id/tasks**

**PATCH /api/v1/appointments/:id/tasks/:tid**

All handlers: parse input → call service → `pkghttp.WriteError(w, err)` on failure → render output.

---

## Tests Required

- `TestIsValidTransition_allEdges` — exhaustive table-driven test
- `TestIsPermittedRole_doctorCannotArrivePatient`
- `TestRequiresClinicalNote_arrivedToSamplesCollected` — returns true, NoteTypePreTestCounselling
- `TestRequiresClinicalNote_bookedToArrived` — returns false
- `TestAdvance_blockedByCounsellingRequirement` — no counselling note → ErrCounsellingRequired
- `TestAdvance_passesWhenCounsellingNotePresent`
- `TestAdvance_persistsEvent`
- `TestAdvance_invalidTransition_returnsErrInvalidTransition`
- `TestAdvance_wrongRole_returnsErrTransitionNotPermitted`
- `TestCurrentState_replayFromEvents` — state derived from event log, not stored field
- `TestAdvance_dispatchesSideEffect` — mock NATS publisher receives message
- `TestAdvance_sideEffectFailure_doesNotRollbackTransition`
- `TestAdvance_resultsReceived_triggersMADOEvaluation` — mock MADOEvaluator called
- `TestMADOEvaluator_evaluationError_writesFailedRecord` — error → evaluation_failed status record written
- `TestMADOEvaluator_positiveBMO_createsDeclaration` (integration: `*_integration_test.go`, calls `test.RequireTiltCI(t)`)

---

## Done Criteria

- All DAG edges enforced correctly including role permissions
- TGV pre-test counselling enforced at service layer before SAMPLES_COLLECTED
- Clinical note required for PHONE_APPT_COMPLETED and RESULTS_RELEASED transitions
- MADO evaluator triggered asynchronously on RESULTS_RECEIVED; errors write evaluation_failed record
- Event history is immutable, append-only, replayable
- `go build ./...`, `go vet ./...`, `golangci-lint run ./...` pass
- `go test -mod=vendor ./...` passes
