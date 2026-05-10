# Session 14 — Admin Portal: Workflow Operations

## Claude Code Delegation Prompt

```
You are implementing Session 14 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC, typed template data
  4. specs/overview/05-admin-portal.md — appointment detail component map
  5. specs/conventions/tgv.md — clinical note requirements on DAG transitions

Session 13 is complete (admin portal shell, appointment detail shell with stubbed panels).
Session 04 is complete (DAG backend APIs exist).
Sessions 08/09 are complete (parser; needs_review NATS subject active).

Your job is to implement workflow panels on the appointment detail page: DAG transition
controls, PDF upload, notes feed, task list, needs_review manual entry, and queue page.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/admin && go vet ./...
```

---

## Dependencies
- Session 13 (admin portal shell)
- Session 04 (DAG advance, notes, tasks APIs)
- Sessions 08/09 (parser.review NATS subject)

---

## Deliverables

### 1. Typed Template Data Structs (add to `internal/admin/app/httpingress/pages.go`)

```go
type TransitionButtonStyle string

const (
    TransitionButtonStylePrimary TransitionButtonStyle = "primary"
    TransitionButtonStyleWarning TransitionButtonStyle = "warning"
    TransitionButtonStyleDanger  TransitionButtonStyle = "danger"
)

// Transition buttons shown below DAG timeline
type TransitionButton struct {
    Label            string
    ToState          appointment.State
    Style            TransitionButtonStyle
    Confirm          bool   // show hx-confirm modal before submitting
    // TGV: if the transition requires a clinical note, show note field inline
    RequiresNote     bool
    RequiredNoteType appointment.NoteType
}

type DAGTimelineData struct {
    // from session 13 — extend with transition buttons
    States      []DAGStepData
    Current     string
    Transitions []TransitionButton
}

type NotesFeedData struct {
    Notes         []Note
    AppointmentID uuid.UUID
    CSRF          string
}

type TaskListData struct {
    Tasks         []Task
    AppointmentID uuid.UUID
    CSRF          string
}

type UploadPanelData struct {
    AppointmentID uuid.UUID
    CSRF          string
    Uploaded      bool
    Error         string
}

type ReviewResultData struct {
    AppointmentID uuid.UUID
    PartialReport *pipeline.LabReport // from needs_review payload (partial, low-confidence fields)
    FailedFields  []string
    CSRF          string
}
```

### 2. DAG Transition Controls

Extend `dag-timeline.html` partial with transition buttons below the stepper.

```go
// internal/admin/app/httpingress/handler.go
// AvailableTransitions returns the valid next-state buttons for the current state and role.
// TGV: if the transition requires a clinical note (from specs/conventions/tgv.md),
// set RequiresNote=true so the template renders an inline note textarea.
func availableTransitions(currentState appointment.State, role auth.Role) []TransitionButton
```

Each button: `hx-post="/appointments/:id/advance"` with `hx-confirm` for Confirm=true transitions.

**POST /appointments/:id/advance**:
- Forward to backend DAG advance API (include note in payload if RequiredNoteType set)
- Return refreshed `dag-timeline` partial with `HX-Trigger: dag-updated`

If the backend returns `ErrCounsellingRequired` or `ErrClinicalNoteRequired`:
- Re-render the timeline partial with an inline error message
- Do not clear the note textarea

### 3. PDF Upload Panel

Shown when state = `RESULTS_RECEIVED` and role in `[clinic_worker, admin]`.

Template renders `partials/upload-panel.html` with `UploadPanelData`.

**POST /appointments/:id/upload-result**:
1. Validate: file is PDF, max 20MB
2. Upload to S3 bucket `stdclinic-parser-inbound-{jurisdiction}`
   Key: `{appointment_id}/{uuid}.pdf`
3. Publish NATS message to `parser.inbound` with `appointment_id`, `uploaded_by`, `file_reference`
4. Return success partial: "Résultat téléversé et en file d'attente pour traitement."
5. On error: return `UploadPanelData{Error: "..."}` partial

### 4. Needs-Review Manual Entry

Backend endpoints (add to `internal/api/app/httpingress/handler.go`):
```
GET  /api/v1/admin/parser/review-queue       — list pending needs_review items
POST /api/v1/appointments/:id/results        — submit DiagnosticReportBundle
```

Admin portal:
- **GET /appointments/:id/review-result** — render `partials/review-result.html` with `ReviewResultData`
- Low-confidence fields highlighted in red
- Correct field values in form inputs
- **POST /appointments/:id/review-result** — submit corrected report to backend; on success redirect to appointment detail

### 5. Notes Feed

**GET /appointments/:id/partials/notes** — returns `NotesFeedData`; renders `partials/note-feed.html`

Chronological list, newest first. Author name (not UUID), timestamp, note type badge, body.

Add-note form at bottom:
```html
<form hx-post="/appointments/:id/notes"
      hx-target="#note-feed" hx-swap="outerHTML">
  <select name="note_type">
    <option value="general">Général</option>
    <option value="clinical">Clinique</option>
    <!-- pre_test_counselling shown only to clinic_worker and doctor -->
    {{ if .CanAddCounselling }}
    <option value="pre_test_counselling">Counseling pré-test</option>
    {{ end }}
  </select>
  <textarea name="body" required></textarea>
  <button type="submit">Ajouter une note</button>
</form>
```

**POST /appointments/:id/notes** — forward to backend; return refreshed `#note-feed` partial.

### 6. Task List

**GET /appointments/:id/partials/tasks** — returns `TaskListData`; renders `partials/task-list.html`

Tasks grouped: open → in_progress → done.

Status update via `<select hx-patch="/appointments/:id/tasks/:tid" hx-trigger="change">` returning refreshed partial.

Add-task form submits via `hx-post` returning refreshed `#task-list`.

### 7. Appointment Queue (`GET /queue`)

Role: clinic_worker only.

Fetch today's appointments for the worker's clinic(s). Display in a table ordered by slot_start.
Grouped: in-progress states at top, BOOKED below.

Auto-refresh every 60s:
```html
<tbody id="queue-body"
       hx-get="/queue/partial"
       hx-trigger="every 60s"
       hx-swap="innerHTML">
```

**GET /queue/partial** (HTMX): returns table body rows only (no base layout).

---

## Tests Required

- `TestAvailableTransitions_workerBOOKED` — returns ARRIVED, NO_SHOW, CANCELLED buttons
- `TestAvailableTransitions_doctorBOOKED` — no ARRIVED button
- `TestAvailableTransitions_requiresNote_arrivedToSamples` — RequiresNote=true
- `TestAdvanceHandler_counsellingError_rendersInlineError`
- `TestPDFUpload_notPDF_returns422`
- `TestPDFUpload_success_publishesNATSMessage`
- `TestNotesFeed_htmxPartial_noBaseLayout`
- `TestAddNote_refreshesFeed`
- `TestTaskStatusUpdate_htmxPatch_refreshesList`
- `TestQueue_autoRefreshAttribute` — verify `hx-trigger="every 60s"` in rendered HTML

---

## Done Criteria

- All appointment detail panels implemented (no stubs from session 13 remain)
- DAG transitions fire and update timeline inline
- TGV clinical note requirement surfaced in UI when applicable
- Notes and tasks update via HTMX without full page reload
- PDF upload queues file for parsing
- Queue page auto-refreshes
- `go build ./cmd/admin` and `go vet ./...` pass
