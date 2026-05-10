# Session 11 — Patient Web App: Booking Flow

## Claude Code Delegation Prompt

```
You are implementing Session 11 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC handler shape, typed template data structs
  4. specs/overview/04-patient-app.md — HTMX conventions, Alpine.js patterns

Session 10 is complete (patient app shell, session, BackendClient, Renderer exist).
Sessions 03 and 04 are complete on the backend (slots and appointments APIs exist).

Your job is to implement the 4-step appointment booking wizard.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/patient && go vet ./...
```

---

## Dependencies
- Session 10 (app shell, SessionManager, BackendClient, Renderer)
- Sessions 03, 04 (backend slots and appointments APIs)

---

## Package Location

Booking handlers are added to `internal/patient/app/httpingress/handler.go`.
Typed page data structs go in `internal/patient/app/httpingress/pages.go`.

```
web/patient/templates/book/
├── questions.html
├── tests.html
├── clinic.html
└── confirm.html
web/patient/templates/partials/slot-grid.html
web/patient/ts/components/questionTree.ts
```

---

## Booking Wizard State

Stored in `SessionData.BookingWIP` (defined in session 10):

```go
type BookingState struct {
    Step          int
    Answers       map[string]string // question_id → answer value
    SelectedTests []string          // test codes confirmed by patient
    ClinicID      *uuid.UUID
    SlotID        *uuid.UUID
}
```

State persists across page reloads. Cleared on successful booking or explicit cancel.

---

## Deliverables

### 1. Backend Client Extensions

Add to existing `BackendClient` interface:

```go
ListQuestions(ctx context.Context, token string) ([]Question, error)
GetSuggestedTests(ctx context.Context, token string, answers map[string]string) ([]SuggestedTest, error)
ListFreeSlots(ctx context.Context, clinicID uuid.UUID, date, slotType string) ([]Slot, error)
BookAppointment(ctx context.Context, token string, req BookingRequest) (*Appointment, error)
ListClinics(ctx context.Context, token, jurisdiction string) ([]Clinic, error)
```

### 2. Typed Template Data Structs (add to `internal/patient/app/httpingress/pages.go`)

```go
type BookQuestionsData struct {
    Questions []Question
    CSRF      string
    Step      int
}

type BookTestsData struct {
    SuggestedTests []SuggestedTest
    CSRF           string
    Step           int
}

type BookClinicData struct {
    Clinics  []Clinic
    CSRF     string
    Step     int
}

type SlotGridData struct {
    Slots    []Slot
    ClinicID uuid.UUID
    Date     string
}

type BookConfirmData struct {
    ClinicName  string
    ClinicAddr  string
    SlotStart   time.Time
    SlotType    string
    Tests       []SuggestedTest
    CSRF        string
}
```

### 3. Step 1 — Health Questionnaire (`GET|POST /book/questions`)

**GET**: fetch questions from backend; render `book/questions.html` with `BookQuestionsData`.

Questions rendered via Alpine.js `questionTree` component that drives conditional visibility:

```typescript
// web/patient/ts/components/questionTree.ts
Alpine.data('questionTree', () => ({
    answers: {} as Record<string, string>,
    isVisible(questionID: string, condition: {parentID: string, value: string} | null): boolean {
        if (!condition) return true
        return this.answers[condition.parentID] === condition.value
    },
    canProceed(): boolean {
        // all required visible questions are answered
    }
}))
```

**POST**: save answers to `BookingWIP.Answers`; call `GetSuggestedTests`; redirect to `/book/tests`.

### 4. Step 2 — Suggested Tests (`GET|POST /book/tests`)

**GET**: load suggested tests from backend (answers already in session); render `book/tests.html`.

Each test: name, description, `mandatory bool`.
- Mandatory tests: rendered as checked, disabled checkbox
- Optional tests: rendered as checked checkbox (unchecking removes from selection)

**POST**: save selected codes to `BookingWIP.SelectedTests`; redirect to `/book/clinic`.

### 5. Step 3 — Clinic & Slot Selection (`GET /book/clinic`, `GET /book/slots`)

**GET /book/clinic**: fetch clinics for patient's jurisdiction; render `book/clinic.html`.

On clinic selection, HTMX loads the slot grid:
```html
<div hx-get="/book/slots?clinic_id={{ .ID }}&date={{ today }}"
     hx-trigger="click"
     hx-target="#slot-panel"
     hx-swap="innerHTML">
```

**GET /book/slots** (HTMX partial): `?clinic_id=&date=YYYY-MM-DD`
- Calls backend `ListFreeSlots`
- Returns `partials/slot-grid.html` — grid of time buttons
- Free slots: clickable; on click sets hidden input `slot_id` and enables "Continue" button
- Busy slots: greyed out, not clickable
- Alpine.js handles slot selection state locally:

```typescript
Alpine.data('slotPicker', () => ({
    selectedSlot: null as string | null,
    select(id: string) { this.selectedSlot = id }
}))
```

**POST /book/clinic**: save `ClinicID` and `SlotID` to `BookingWIP`; redirect to `/book/confirm`.

### 6. Step 4 — Confirmation (`GET /book/confirm`, `POST /book/complete`)

**GET /book/confirm**: assemble `BookConfirmData` from `BookingWIP` + backend lookups; render `book/confirm.html`.

**POST /book/complete**:
1. Call `client.BookAppointment` with `BookingWIP` data
2. On success: clear `BookingWIP`; set `HX-Redirect` header to `/appointments/{new_id}`
3. On 409 (slot taken): re-render confirm page with typed error data:
   ```go
   type BookConfirmData struct {
       // ...existing fields...
       SlotTakenError bool   // true if slot was taken between selection and confirm
   }
   ```
   Error message: "Ce créneau vient d'être pris. Veuillez en choisir un autre." (French first)

---

## Tests Required

- `TestBookStep1_savesAnswersToSession`
- `TestBookStep1_fetchesSuggestedTestsAfterAnswers`
- `TestSlotGrid_htmxPartial_noBaseLayout` — HX-Request header → no `<html>` wrapper
- `TestBookStep3_savesClinidAndSlotToSession`
- `TestBookComplete_clearsBookingStateOnSuccess`
- `TestBookComplete_slotConflict_renders409WithTypedError`
- `TestQuestionTree_conditionalVisibility` — vitest unit test for Alpine component

---

## Done Criteria

- Full 4-step flow creates an appointment and clears booking state
- Slot grid loads via HTMX without full page reload
- Slot conflict (409) handled gracefully with French error message
- `go build ./cmd/patient` and `go vet ./...` pass
