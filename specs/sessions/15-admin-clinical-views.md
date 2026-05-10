# Session 15 — Admin Portal: Doctor & Pharmacist Views

## Claude Code Delegation Prompt

```
You are implementing Session 15 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC handler shape, typed template data structs
  4. specs/overview/05-admin-portal.md — appointment detail component map
  5. specs/conventions/tgv.md — TGV Domain 3: sign-off gates result release,
     CMQ formulary reference on prescriptions, MADO declaration status visible to doctor

Session 13 is complete (admin portal shell, appointment detail shell).
Session 14 is complete (workflow ops panels, notes, tasks).
Session 06 is complete (prescription backend APIs exist).
Session 04 is complete (MADO declarations table exists).

Your job is to implement the clinical panels on the appointment detail page:
results + sign-off, prescription creation, pharmacist fulfilment, MADO declaration
status, and the pharmacist prescription queue.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/admin && go vet ./...
```

---

## Dependencies
- Session 13 (admin portal shell)
- Session 14 (appointment detail with all workflow panels)
- Session 06 (prescription backend APIs)
- Session 04 (MADO declarations)

---

## Package Location

Clinical and prescription handlers are added to `internal/admin/app/httpingress/handler.go`.
Typed page data structs go in `internal/admin/app/httpingress/pages.go`.

```
web/admin/templates/partials/
├── result-panel.html
├── prescription-panel.html
└── mado-panel.html
web/admin/templates/prescriptions/
├── list.html
└── detail.html
```

---

## Deliverables

### 1. Typed Template Data Structs (add to `internal/admin/app/httpingress/pages.go`)

```go
type ResultPanelData struct {
    AppointmentID  uuid.UUID
    Results        *DiagnosticResults  // nil if not yet received
    SignedOff       bool               // true if doctor has already signed off
    CanSignOff      bool               // true if role=doctor AND results present AND not yet signed off
    SignOffNotes    string             // populated after sign-off
    CSRF            string
}

type MADOPanelData struct {
    AppointmentID uuid.UUID
    Declarations  []MADODeclaration
    // MADODeclaration mirrors internal/domain/dag/mado.go model
}

type MADODeclaration struct {
    ID            uuid.UUID
    ConditionName string
    ICD10Code     string
    Status        appointment.MADOStatus // named type from internal/api/pkg/appointment
    DeadlineAt    time.Time
    IsOverdue     bool   // DeadlineAt.Before(time.Now()) && status != submitted|acknowledged
}

type PrescriptionPanelData struct {
    AppointmentID  uuid.UUID
    Prescriptions  []PrescriptionItem
    CanCreate      bool  // role == doctor
    CanFulfil      bool  // role == pharmacist
    CSRF           string
}

type PrescriptionItem struct {
    ID              uuid.UUID
    MedicationName  string
    DosageText      string
    Status          prescription.Status // named type from internal/api/pkg/prescription
    AuthoredOn      time.Time
    DispensingNotes string
}

type PrescriptionQueueData struct {
    Pending []PrescriptionSummary
    Nav     []NavItem
}

type PrescriptionDetailData struct {
    Prescription PrescriptionItem
    PatientName  string  // initials only — PatientSummary.Initials()
    Nav          []NavItem
    CanFulfil    bool
    CSRF         string
}
```

### 2. Results & Sign-Off Panel

**GET /appointments/:id/partials/results** — returns `ResultPanelData`; renders `partials/result-panel.html`

Panel is role-aware in template logic:

For **all staff** (after `RESULTS_RECEIVED`):
- DiagnosticReport analytes table — same visual as patient app but always visible to staff
- Abnormal rows: `border-red-400 bg-red-50`

For **doctor only** (`CanSignOff == true`):
- "Signer les résultats" button below analytes table
- Inline textarea for sign-off notes (optional)
- Form: `hx-post="/appointments/:id/signoff" hx-target="#result-panel" hx-swap="outerHTML"`

After sign-off (`SignedOff == true`):
- Replace button with green badge "Résultats signés ✓"
- Show sign-off notes if present

**POST /appointments/:id/signoff** — role: doctor only:
1. Call backend `POST /api/v1/appointments/:id/signoff` — records to `appointment_signoffs` table (one row per appointment, UNIQUE constraint enforced). The sign-off is distinct from DAG events and notes.
2. Return refreshed `result-panel` partial

### 3. MADO Declarations Panel

**GET /appointments/:id/partials/mado** — returns `MADOPanelData`; renders `partials/mado-panel.html`

Shown only when at least one MADO declaration exists for the appointment.

Each declaration row:
- Condition name and ICD-10 code
- Status badge (colour-coded: pending=yellow, ready=blue, submitted=green, overdue=red)
- Deadline with countdown if not yet submitted

Overdue declarations (`IsOverdue == true`) rendered with red background and alert icon.

Doctor can advance status from `pending_physician_review → ready_for_submission` via a button:
`hx-post="/appointments/:id/mado/:declaration_id/ready" hx-target="#mado-panel" hx-swap="outerHTML"`

**POST /appointments/:id/mado/:declaration_id/ready** — role: doctor only:
- Call backend to update MADO declaration status
- Return refreshed `mado-panel` partial

### 4. Prescription Panel

**GET /appointments/:id/partials/prescriptions** — returns `PrescriptionPanelData`; renders `partials/prescription-panel.html`

**Doctor view** (`CanCreate == true`):
- List of existing prescriptions with status badges
- "Ajouter une ordonnance" button — Alpine.js `x-show` expands inline form:

```html
<div x-data="{ open: false }">
  <button @click="open = !open">Ajouter une ordonnance</button>
  <form x-show="open"
        hx-post="/appointments/:id/prescriptions"
        hx-target="#prescription-panel"
        hx-swap="outerHTML">
    <input name="medication_name" placeholder="Nom du médicament" required>
    <input name="medication_code" placeholder="Code (ex: DIN)" required>
    <textarea name="dosage_text" placeholder="Posologie" required></textarea>
    <!-- TGV Domain 3: CMQ formulary reference -->
    <input name="formulary_reference" placeholder="Référence formulaire CMQ (optionnel)">
    <textarea name="notes" placeholder="Notes additionnelles (optionnel)"></textarea>
    <button type="submit">Créer l'ordonnance</button>
  </form>
</div>
```

**POST /appointments/:id/prescriptions** — role: doctor only; return refreshed panel.

**Pharmacist view** (`CanFulfil == true`):
- Same prescription list
- Each active prescription has "Exécuter" button that expands inline fulfilment form:

```html
<form x-show="fulfilling"
      hx-post="/prescriptions/:rx_id/fulfil"
      hx-target="#prescription-panel"
      hx-swap="outerHTML">
  <textarea name="dispensing_notes" placeholder="Notes de dispensation"></textarea>
  <button type="submit">Marquer comme dispensé</button>
</form>
```

**POST /prescriptions/:id/fulfil** — role: pharmacist only; return refreshed panel.

### 5. Doctor Dashboard Extensions

Extend the doctor dashboard (`AdminDashboardData`) populated in session 13:

**Pending Sign-Offs section:**
- Query: `GET /api/v1/appointments?state=RESULTS_RECEIVED&practitioner_uuid={me}`
- Show count badge + list of up to 5, each with patient initials, clinic, date, "Ouvrir" link

**Upcoming Phone Appointments:**
- Query: `GET /api/v1/appointments?state=PHONE_APPT_BOOKED&practitioner_uuid={me}`
- Show date, time, patient initials, "Ouvrir" link

**MADO Overdue Alerts:**
- Query backend for MADO declarations where `status NOT IN (submitted, acknowledged)` AND `deadline_at < now()`
- Show count badge in dashboard header; list of overdue with appointment link
- Red alert styling

### 6. Pharmacist Prescription Queue (`GET /prescriptions`)

Role: pharmacist only. Renders `prescriptions/list.html` with `PrescriptionQueueData`.

Fetches `GET /api/v1/prescriptions` (returns `[]PrescriptionSummary` — no PII).

Table columns: patient initials, medication name, prescribing doctor initials, prescribed at, appointment link, "Voir et exécuter" button.

**GET /prescriptions/:id** — renders `prescriptions/detail.html` with `PrescriptionDetailData`.
Full prescription fetched from backend (backend emits AuditEvent).
Fulfilment form at bottom.

---

## Tests Required

- `TestSignOff_onlyDoctor_workerReturns403`
- `TestSignOff_success_returnsRefreshedPanel`
- `TestResultPanel_doctorCanSignOff_showsButton`
- `TestResultPanel_afterSignOff_showsBadgeNotButton`
- `TestResultPanel_pharmacistCannotSignOff`
- `TestMADOPanel_overdueDeclaration_hasRedStyling`
- `TestMADOReady_onlyDoctor`
- `TestPrescriptionCreate_onlyDoctor`
- `TestPrescriptionFulfil_onlyPharmacist`
- `TestPrescriptionPanel_doctorSeesCreateButton`
- `TestPrescriptionPanel_pharmacistSeesFulfilButton`
- `TestPrescriptionPanel_workerSeesNeitherButton`
- `TestPrescriptionQueue_noPIIInList` — no email, DOB, full name in rendered output

---

## Done Criteria

- Doctor can sign off results inline on appointment detail page
- MADO declarations visible to doctor with overdue highlighting
- Doctor can advance MADO status to ready_for_submission
- Doctor can create prescriptions; pharmacist can fulfil them
- Both actions update inline via HTMX without full page reload
- Role enforcement correct — wrong role returns 403
- `go build ./cmd/admin` and `go vet ./...` pass
