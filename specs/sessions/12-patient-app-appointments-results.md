# Session 12 — Patient Web App: Appointments & Results

## Claude Code Delegation Prompt

```
You are implementing Session 12 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC, typed template data structs
  4. specs/overview/04-patient-app.md

Session 10 is complete (app shell). Sessions 03 and 04 are complete on the backend.

Your job is to implement the dashboard, appointment list, appointment detail, and
result display. Results must only render when DAG state is RESULTS_RELEASED.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/patient && go vet ./...
```

---

## Dependencies
- Session 10 (app shell, BackendClient, Renderer, SessionManager)
- Sessions 03, 04 (appointment and DAG APIs)

---

## Package Location

Dashboard and appointment handlers are added to `internal/patient/app/httpingress/handler.go`.
Typed page data structs go in `internal/patient/app/httpingress/pages.go`.

```
web/patient/templates/
├── dashboard.html
├── appointments/list.html
├── appointments/detail.html
└── partials/
    ├── appointment-card.html
    └── result-panel.html
```

---

## Deliverables

### 1. Backend Client Extensions

```go
type AppointmentSummary struct {
    ID           uuid.UUID          `json:"id"`
    ClinicName   string             `json:"clinic_name"`
    ClinicTZ     string             `json:"clinic_timezone"` // IANA tz for display
    SlotStart    time.Time          `json:"slot_start"`
    SlotType     fhir.SlotType      `json:"slot_type"`
    CurrentState appointment.State  `json:"current_state"`
}

type AppointmentDetail struct {
    AppointmentSummary
    TestPanel []TestItem         `json:"test_panel"`
    Results   *DiagnosticResults `json:"results,omitempty"` // nil until RESULTS_RELEASED
    Events    []EventSummary     `json:"events"`
}

type DiagnosticResults struct {
    ReportDate time.Time       `json:"report_date"`
    PanelName  string          `json:"panel_name"`
    Analytes   []AnalyteSummary `json:"analytes"`
}

type AnalyteSummary struct {
    Name           string `json:"name"`
    Value          string `json:"value"`
    Unit           string `json:"unit"`
    ReferenceRange string `json:"reference_range"`
    Abnormal       bool   `json:"abnormal"`
}
```

### 2. Typed Template Data Structs (add to `internal/patient/app/httpingress/pages.go`)

```go
type DashboardData struct {
    NextAppointment  *AppointmentSummary  // nil if none
    RecentPast       []AppointmentSummary // last 3 closed/cancelled
    HasBookingCTA    bool                 // true when NextAppointment is nil
}

type AppointmentListData struct {
    Upcoming []AppointmentSummary
    Past     []AppointmentSummary
}

type AppointmentDetailData struct {
    Appointment AppointmentDetail
    NextStepMsg string    // contextual message per DAG state (French first)
    ShowResults bool      // true only when CurrentState == "RESULTS_RELEASED"
}
```

### 3. Dashboard Handler (`GET /`)

Fetch all patient appointments from backend. Split server-side:
- `NextAppointment`: first upcoming by slot_start (non-terminal states)
- `RecentPast`: last 3 by slot_start (terminal states: CLOSED, CANCELLED, NO_SHOW)
- `HasBookingCTA`: true if `NextAppointment == nil`

All data loaded on page render — no AJAX.

### 4. Appointment List (`GET /appointments`)

Fetch all patient appointments. Split into `Upcoming` and `Past` for `AppointmentListData`.

Each entry renders `partials/appointment-card.html`. This partial is shared with dashboard.

`appointment-card.html` must include:
- Date/time formatted using clinic's IANA timezone (`ClinicTZ`)
- Clinic name
- Slot type badge (In Person / En personne, or Phone / Téléphone)
- State badge — colour mapping (applied via Tailwind class in template):

| State | Tailwind colour |
|---|---|
| BOOKED | `bg-blue-100 text-blue-800` |
| ARRIVED | `bg-indigo-100 text-indigo-800` |
| SAMPLES_COLLECTED / SENT_TO_LAB | `bg-yellow-100 text-yellow-800` |
| RESULTS_RECEIVED / PHONE_APPT_BOOKED / PHONE_APPT_COMPLETED | `bg-orange-100 text-orange-800` |
| RESULTS_RELEASED | `bg-green-100 text-green-800` |
| CLOSED | `bg-gray-100 text-gray-700` |
| CANCELLED / NO_SHOW | `bg-red-100 text-red-800` |

### 5. Appointment Detail (`GET /appointments/:id`)

Fetch `AppointmentDetail` from backend. Build `AppointmentDetailData`:

`ShowResults`: `detail.Results != nil` (backend only returns results after RESULTS_RELEASED)

`NextStepMsg` — French first, determined server-side by `CurrentState`:

| State | Message |
|---|---|
| BOOKED | "Votre rendez-vous est confirmé. Veuillez vous présenter à la clinique à l'heure prévue." |
| ARRIVED | "Bienvenue à la clinique." |
| SAMPLES_COLLECTED / SENT_TO_LAB | "Vos échantillons sont en cours d'analyse au laboratoire." |
| RESULTS_RECEIVED / PHONE_APPT_BOOKED | "Un rendez-vous téléphonique de suivi a été planifié. Vos résultats seront partagés après l'appel." |
| PHONE_APPT_COMPLETED | "Votre médecin a examiné vos résultats. Ils vous seront communiqués sous peu." |
| RESULTS_RELEASED | "Vos résultats sont disponibles ci-dessous." |
| CLOSED | "Ce rendez-vous est terminé." |
| CANCELLED | "Ce rendez-vous a été annulé." |

### 6. Results Partial (`partials/result-panel.html`)

Only rendered when `ShowResults == true`. Data type: `DiagnosticResults`.

```html
{{ range .Analytes }}
<div class="flex justify-between items-center border rounded px-4 py-2
            {{ if .Abnormal }}border-red-400 bg-red-50{{ else }}border-gray-200{{ end }}">
  <span class="font-medium">{{ .Name }}</span>
  <span>{{ .Value }} {{ .Unit }}</span>
  <span class="text-sm text-gray-400">Réf: {{ .ReferenceRange }}</span>
  {{ if .Abnormal }}
  <span class="text-xs px-2 py-1 rounded bg-red-100 text-red-700">Anormal</span>
  {{ end }}
</div>
{{ end }}
```

When `ShowResults == false`, render:
```html
<p class="text-gray-500 italic">
  Vos résultats apparaîtront ici après votre rendez-vous téléphonique de suivi.
</p>
```

---

## Tests Required

- `TestDashboard_noAppointments_hasCTA`
- `TestDashboard_withUpcoming_showsNextAppointment`
- `TestAppointmentList_groupsCorrectly` — table-driven across all states
- `TestAppointmentDetail_resultsNil_showResultsFalse`
- `TestAppointmentDetail_resultsPresent_showResultsTrue`
- `TestAppointmentDetail_nextStepMsg_allStates` — table-driven
- `TestResultPanel_abnormalRow_hasRedClass`

---

## Done Criteria

- Results never shown when state is not RESULTS_RELEASED
- State badge colours correct for all states
- French messages used throughout
- `go build ./cmd/patient` and `go vet ./...` pass
