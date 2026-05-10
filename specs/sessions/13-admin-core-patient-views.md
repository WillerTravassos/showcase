# Session 13 — Admin Portal: Core & Patient Views

## Claude Code Delegation Prompt

```
You are implementing Session 13 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC handler shape, typed template data structs
  4. specs/overview/05-admin-portal.md — role-gated nav, appointment detail component map
  5. specs/conventions/tgv.md — audit requirements for staff access to patient data

Sessions 01 and 02 are complete (backend auth and patient APIs). Session 03 is complete.

Your job is to build the Admin Portal binary: server, session, role-aware nav, login,
patient search, patient detail, and the appointment detail page shell (DAG timeline +
patient header; other panels stubbed for sessions 14 and 15).

MVC: handlers are thin controllers. No business logic in handlers.
All template data is typed structs — never map[string]any.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/admin && go vet ./...
```

---

## Dependencies
- Session 01 (backend auth API)
- Session 02 (patient API)
- Session 03 (appointments API)

---

## Package Location

```
internal/admin/
├── app/
│   ├── service.go          # package app — composition root
│   ├── settings.go         # env-var config for the admin portal
│   └── httpingress/
│       ├── handler.go      # Handler struct with all injected dependencies
│       ├── pages.go        # All typed page data structs (view models)
│       ├── session.go      # Redis-backed session middleware
│       ├── renderer.go     # html/template renderer
│       └── apiclient.go    # Backend API HTTP client (server-side only)
└── pkg/                    # TODO: domain packages TBD before implementation starts
cmd/admin/
└── main.go                 # Calls app.Start() only
```

---

## Deliverables

### 1. MVC Handler Shape

```go
type PatientHandler struct {
    client BackendClient
    sess   *session.SessionManager
    render *Renderer
    log    log.Logger
}
```

### 2. Role-Aware Navigation (`internal/admin/app/httpingress/handler.go` or separate `nav.go` in httpingress)

```go
type NavItem struct {
    Label  string
    Path   string
    Active bool
}

// BuildNav returns nav items visible to the given role.
// Never returns an item for a role that should not see it.
func BuildNav(role auth.Role, currentPath string) []NavItem
```

Nav item visibility:

| Item | Worker | Doctor | Pharmacist | Admin |
|---|---|---|---|---|
| Dashboard | ✓ | ✓ | ✓ | ✓ |
| Patients | ✓ | ✓ | ✓ | ✓ |
| Queue | ✓ | — | — | — |
| My Schedule | ✓ | — | — | — |
| Prescriptions | — | ✓ | ✓ | — |
| Admin | — | — | — | ✓ |

### 3. Typed Template Data Structs (`internal/admin/app/httpingress/pages.go`)

All typed page data structs live in `pages.go` alongside the handlers that populate them.
Never use `map[string]any` for template data.

```go
type AdminLoginPageData struct {
    Error string
    CSRF  string
}

type AdminDashboardData struct {
    Role            auth.Role
    // Worker-specific
    QueueSummary    *QueueSummary
    // Doctor-specific
    PendingSignoffs int
    PhoneAppts      []AppointmentSummary
    // Pharmacist-specific
    PendingRx       int
    // Admin-specific
    ActiveClinics   int
    StaffCount      int
    TodayAppts      int
    Nav             []NavItem
}

type PatientSearchData struct {
    Query    string
    Results  []patient.PatientSummary
    Nav      []NavItem
}

type PatientDetailData struct {
    Patient  *patient.Patient
    Appts    []AppointmentSummary
    Nav      []NavItem
}

type AppointmentDetailData struct {
    Appointment AppointmentDetail
    Patient     *patient.Patient
    Nav         []NavItem
    // Panels below are stubbed in this session (implemented in 14 & 15)
    // DAGTimeline and PatientHeader are implemented here
}

type DAGTimelineData struct {
    States  []DAGStepData
    Current appointment.State
}

type DAGStepData struct {
    State   appointment.State
    Label   string
    Done    bool
    Active  bool
    Future  bool
}
```

### 4. Login Handler (in `internal/admin/app/httpingress/handler.go`)

**GET /login** — render `auth/login.html`

**POST /login**:
- Call backend login
- If role == `patient`: destroy session, re-render login with error "Ce portail est réservé au personnel de la clinique."
- On success: redirect to `/`

### 5. Dashboard Handler

Role-appropriate data loaded server-side. No AJAX.

### 6. Patient Search (`GET /patients`)

HTMX live search: `hx-get="/patients" hx-target="#results" hx-trigger="input delay:300ms"`

Results table: UUID (first 8 chars), initials, status badge, jurisdiction badge, "Voir" link.
No full PII in list — only `PatientSummary`.

### 7. Patient Detail (`GET /patients/:uuid`)

Fetch full patient. Access emits AuditEvent (handled by backend — admin portal just calls the API).

Template shows:
- `partials/patient-header.html` — name, DOB, UUID, phone, jurisdiction badge
- Appointment history table (summary only)

### 8. Appointment Detail Shell (`GET /appointments/:id`)

**Clinic access guard:** enforced in the backend service layer (see `specs/overview/05-admin-portal.md`).
The admin portal handler calls the backend API and forwards the JWT; the backend returns 403
if the caller lacks clinic access. The handler maps the 403 response using `pkghttp.WriteError`.

**Implement fully in this session:**

`partials/patient-header.html` — patient name, DOB, UUID, phone, jurisdiction.

`partials/dag-timeline.html` — horizontal stepper. All states in order. Current state highlighted.
States before current: `Done`. Current: `Active`. States after: `Future`.

Each partial has its own endpoint for HTMX reload:
- `GET /appointments/:id/partials/patient-header`
- `GET /appointments/:id/partials/dag-timeline`

**Stubbed in this session** (empty div with id, implemented in 14 & 15):
- `#result-panel`, `#note-feed`, `#task-list`, `#prescription-panel`

---

## Tests Required

- `TestLogin_patientRole_rejected` — patient JWT → error, no session
- `TestBuildNav_clinicWorker` — has Queue, no Admin, no Prescriptions
- `TestBuildNav_pharmacist` — has Prescriptions, no Queue
- `TestBuildNav_admin` — has Admin, no Queue
- `TestPatientSearch_htmxRequest_returnsPartialOnly`
- `TestAppointmentDetail_crossClinicAccess_returns403`
- `TestDAGTimeline_currentStateIsActive`
- `TestDAGTimeline_statesBeforeCurrentAreDone`

---

## Done Criteria

- Admin portal starts and all pages render
- Patient role login rejected with French error message
- Appointment detail page loads with patient header and DAG timeline
- Cross-clinic access denied for non-admin roles
- `go build ./cmd/admin` and `go vet ./...` pass
