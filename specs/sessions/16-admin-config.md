# Session 16 — Admin Portal: Admin Config & Management

## Claude Code Delegation Prompt

```
You are implementing Session 16 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC, typed template data structs
  4. specs/overview/05-admin-portal.md
  5. specs/conventions/tgv.md — TGV Domain 8: KPI reports; Domain 1: afterhours referral;
     Domain 4: staff certification expiry; Domain 6: training records; MADO submission tracking

Session 13 is complete (admin portal shell).
Session 05 is complete (staff scheduling backend).
Session 07 is complete (audit log backend).

All routes in this session are role: admin only.
Apply RequireRole(auth.RoleAdmin) to the entire /admin/* route group.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/admin && go vet ./...
```

---

## Dependencies
- Session 13 (admin portal shell)
- Session 05 (staff scheduling APIs)
- Session 07 (audit log API)

---

## Package Location

Config, reporting, and admin management handlers are added to `internal/admin/app/httpingress/handler.go`
(split into multiple files by path group if needed: `clinics.go`, `users.go`, etc.).
Typed page data structs go in `internal/admin/app/httpingress/pages.go`.

```
web/admin/templates/admin/
├── clinics.html
├── users.html
├── schedules.html
├── audit.html
├── config.html
└── reporting.html
```

---

## Deliverables

### 1. Typed Template Data Structs

```go
// handlers/clinics.go
type ClinicsPageData struct {
    Clinics []ClinicRow
    Nav     []NavItem
    CSRF    string
}
type ClinicRow struct {
    ID           uuid.UUID
    Name         string
    Jurisdiction string
    Timezone     string
    BlockMinutes int
    BreakMinutes int
    Active       bool
    AfterhoursURL string  // TGV Domain 1
}
type ClinicFormData struct {
    Clinic       *ClinicRow  // nil for new
    Jurisdictions []string
    Timezones    []string    // IANA tz list filtered by jurisdiction
    BlockSizes   []int       // [5, 10, 15, 20]
    Error        string
    CSRF         string
}

// handlers/users.go
type UsersPageData struct {
    Users   []UserRow
    Clinics []ClinicRow  // for assignment select
    Roles   []string     // excludes "patient"
    Nav     []NavItem
    CSRF    string
}
type UserRow struct {
    UUID         uuid.UUID
    EmailDisplay string    // shown in admin view only
    Role         string
    ClinicNames  []string
    Status       string
}

// handlers/schedules.go
type SchedulesPageData struct {
    Clinics  []ClinicRow
    Nav      []NavItem
    CSRF     string
}
type ShiftGridData struct {
    Date      string
    ClinicID  uuid.UUID
    Staff     []StaffRow
    Slots     []SlotCell
}
type StaffRow struct {
    UUID     uuid.UUID
    Name     string
    Role     string
}
type SlotCell struct {
    SlotID           uuid.UUID
    StartTime        string
    PractitionerUUID *uuid.UUID
    Status           string  // free | busy | break | assigned
}
type AbsenceFormData struct {
    PractitionerUUID uuid.UUID
    HasConflicts     bool
    ConflictCount    int
    CSRF             string
}

// handlers/audit.go
type AuditPageData struct {
    Events    []AuditEventRow
    Filters   AuditFilters
    Page      int
    HasMore   bool
    Nav       []NavItem
    CSRF      string
}
type AuditEventRow struct {
    ID           uuid.UUID
    ActorUUID    uuid.UUID
    ActorRole    string
    Action       string
    ResourceType string
    ResourceID   *uuid.UUID
    OccurredAt   time.Time
    IPAddress    string
    CrossClinic  bool
    IsOverdue    bool
}
type AuditFilters struct {
    ActorUUID    string
    ResourceType string
    Action       string
    From         string
    To           string
    CrossClinic  bool
}

// handlers/reporting.go — TGV Domain 8
type ReportingPageData struct {
    Clinics  []ClinicRow
    Nav      []NavItem
}
type KPIReportData struct {
    ClinicName          string
    Period              string
    AvgWaitHours        float64
    AvgTurnaroundHours  float64
    MADOCompliancePct   float64
    NoShowPct           float64
    TestCompletionPct   float64
    OverdueMADOCount    int
}

// handlers/config.go
type ConfigPageData struct {
    IncompleteExpiryDays    int
    SlotCacheTTLSeconds     int
    ParserThreshold         float64
    ParserWorkerCount       int
    NotificationTemplates   []NotificationTemplate
    TestPanels              []TestPanel
    Nav                     []NavItem
    CSRF                    string
}
type NotificationTemplate struct {
    EventType  string
    SubjectFR  string
    BodyFR     string
    SubjectEN  string
    BodyEN     string
}
type TestPanel struct {
    Code              string
    NameFR            string
    NameEN            string
    Tests             []TestItem
    GuidelineRef      string  // TGV Domain 3: e.g. "Guide ITSS MSSS 2023"
}
```

### 2. Clinic Management (`/admin/clinics`)

**GET /admin/clinics** — render `admin/clinics.html` with `ClinicsPageData`

"Ajouter une clinique" button opens modal (Alpine.js `x-show` overlay).

**POST /admin/clinics** — create; redirect to `/admin/clinics` on success

**PATCH /admin/clinics/:id** — HTMX form submit; returns refreshed row partial

**PATCH /admin/clinics/:id/toggle** — toggle active/inactive; returns refreshed row

Timezone select must be filtered by jurisdiction:
- `ca` → Canadian IANA timezones (`America/Toronto`, `America/Vancouver`, etc.)
- `us` → US IANA timezones
- `br` → Brazilian IANA timezones (`America/Sao_Paulo`, etc.)

TGV Domain 1: `afterhours_referral_url` field is required for TGV-certified clinics —
show a warning badge in the clinics table if it is empty.

### 3. User Management (`/admin/users`)

**GET /admin/users** — filters: role, clinic, status (via HTMX `hx-get` on change)

Table shows `email` (admin-only view — this is the one place full email is displayed).

Role select for new users must **not include "patient"** — patients register via patient app only.

**POST /admin/users** — create staff user; auto-generate temporary password (shown once); send welcome notification via NATS event `user.created`.

**PATCH /admin/users/:uuid/suspend** — requires confirmation modal (Alpine.js); returns refreshed row

**PATCH /admin/users/:uuid/reinstate** — returns refreshed row

**PATCH /admin/users/:uuid/clinics** — update clinic assignments

### 4. Staff Scheduling (`/admin/schedules`)

Two Alpine.js tabs: **Shift Assignment** and **Absence Management**.

**Shift Assignment tab:**
- Clinic + date selector
- Shift grid loaded via HTMX: `GET /admin/schedules/grid?clinic_id=&date=`
- Returns `ShiftGridData`; renders `partials/shift-grid.html`
- Click unassigned slot → modal with staff dropdown to assign
- `POST /admin/schedules/assign` → assign practitioner to shift

**Absence Management tab:**
- Staff selector → loads absence calendar via HTMX
- "Ajouter une absence" form: start date, end date, reason select
- If `HasConflicts == true`: show warning with conflict count before confirming
- `POST /admin/staff/:uuid/absences`

**GET /admin/schedules/grid** (HTMX partial) — returns shift grid fragment only

### 5. Audit Log Viewer (`/admin/audit`)

Filter bar applies via `hx-get="/admin/audit/results" hx-trigger="change"` returning table partial.

"Load more" pagination: `hx-get="/admin/audit/results?page={n}" hx-swap="beforeend"` appends rows.

**GET /admin/audit/results** (HTMX partial) — table body rows only

**GET /admin/audit/export** — download link producing file from backend export endpoint
- Buttons: "Exporter CSV" and "Exporter JSON" with current filters applied
- TGV: export is for regulatory submission to MSSS auditor

### 6. TGV KPI Reporting (`/admin/reporting`)

**GET /admin/reporting** — render `admin/reporting.html` with `ReportingPageData`

Clinic + date range selector. "Générer le rapport" button loads report via HTMX:

**GET /admin/reporting/kpi?clinic_id=&from=&to=** (HTMX partial):
- Calls backend `ReportingService` endpoints (defined below)
- Returns rendered `KPIReportData` partial

Backend endpoints to add (`internal/domain/reporting/`):

```go
// internal/domain/reporting/service.go
type Service interface {
    WaitTimeReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*WaitTimeReport, error)
    TurnaroundReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*TurnaroundReport, error)
    MADOComplianceReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*MADOComplianceReport, error)
    NoShowReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*NoShowReport, error)
    TestCompletionReport(ctx context.Context, clinicID uuid.UUID, period DateRange) (*TestCompletionReport, error)
}
```

API endpoints (add to backend router):
```
GET /api/v1/admin/reporting/wait-time?clinic_id=&from=&to=
GET /api/v1/admin/reporting/turnaround?clinic_id=&from=&to=
GET /api/v1/admin/reporting/mado-compliance?clinic_id=&from=&to=
GET /api/v1/admin/reporting/no-show?clinic_id=&from=&to=
GET /api/v1/admin/reporting/test-completion?clinic_id=&from=&to=
```

KPI report displayed with:
- Metric name (French label)
- Current value
- Target threshold (configurable in system config)
- Green/amber/red status indicator vs target
- Export to PDF button (`TODO: implement in future session`)

### 7. System Config (`/admin/config`)

Three Alpine.js tabs: **Général**, **Modèles de notifications**, **Panneaux de tests**

**Général tab:** sliders/inputs for threshold values; `PATCH /admin/config`

**Modèles de notifications tab:**
- List of event types
- Each has FR and EN subject + body template fields
- TGV: French templates are mandatory; English is optional but encouraged
- Edit inline; `PATCH /admin/config/templates/:event_type`
- Returns success toast via `HX-Trigger: {"showToast": "Modèle sauvegardé"}`

**Panneaux de tests tab:**
- List panels with their test items
- `guideline_reference` field per panel (TGV Domain 3)
- Add/remove tests; mark tests mandatory (cannot be deselected by patient)
- `PATCH /admin/config/panels/:code`

---

## Tests Required

- `TestClinicCreate_requiresAfterhoursURL_showsWarningIfMissing`
- `TestClinicToggle_htmxReturnsRowPartial`
- `TestUserCreate_patientRoleNotAvailable`
- `TestUserSuspend_requiresConfirmModal`
- `TestShiftGrid_htmxPartial_noBaseLayout`
- `TestAuditResults_htmxPartial_appendsRows`
- `TestAuditExport_csvFormat_correctHeaders`
- `TestKPIReport_htmxPartial`
- `TestConfig_save_returnsToastTrigger`
- `TestNotificationTemplate_frenchRequired_englishOptional`

---

## Done Criteria

- Admin can fully manage clinics (including TGV afterhours referral URL), users, staff schedules
- Audit log searchable, filterable, paginated, and exportable for MSSS submission
- TGV KPI reports generated from real data via ReportingService
- System config editable including French-first notification templates
- `go build ./cmd/admin` and `go vet ./...` pass
