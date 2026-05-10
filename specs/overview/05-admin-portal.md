# Overview: Admin Portal

> Reference this file for sessions 13–16.

---

## Binary Entrypoint

`cmd/admin/main.go` — HTTP server on `:8082`. Same tech stack as patient app but separate binary.

---

## Template Layout

```
web/admin/
├── templates/
│   ├── base.html              # Layout with sidebar nav (role-aware)
│   ├── partials/
│   │   ├── dag-timeline.html  # Visual DAG state strip
│   │   ├── note-feed.html
│   │   ├── task-list.html
│   │   ├── result-panel.html
│   │   └── patient-header.html
│   ├── auth/
│   │   └── login.html
│   ├── dashboard.html
│   ├── patients/
│   │   ├── search.html
│   │   └── detail.html
│   ├── appointments/
│   │   └── detail.html        # Central operational hub
│   ├── queue.html             # Clinic worker appointment queue
│   ├── prescriptions/
│   │   ├── list.html
│   │   └── detail.html
│   ├── admin/
│   │   ├── clinics.html
│   │   ├── users.html
│   │   ├── schedules.html
│   │   ├── audit.html
│   │   └── config.html
│   └── schedule/
│       └── my-schedule.html
└── static/
    ├── css/app.css
    └── js/app.js
```

---

## Role-Gated Navigation

The sidebar nav renders links conditionally based on the session role. The `base.html` template receives a `nav` struct computed per-role:

```go
type NavItem struct {
    Label    string
    Path     string
    Roles    []Role  // empty = all roles
}
```

---

## Route Table

| Method | Path | Roles | Notes |
|--------|------|-------|-------|
| GET/POST | `/login` | — | |
| GET | `/` | All | Role-appropriate dashboard |
| GET | `/patients` | All | Search |
| GET | `/patients/:uuid` | All | Detail |
| GET | `/appointments/:id` | All | Detail / hub |
| POST | `/appointments/:id/advance` | Worker, Doctor | DAG transition |
| POST | `/appointments/:id/upload-result` | Worker | PDF upload |
| POST | `/appointments/:id/notes` | All | Add note |
| POST | `/appointments/:id/tasks` | All | Add task |
| PATCH | `/appointments/:id/tasks/:tid` | All | Update task status |
| POST | `/appointments/:id/signoff` | Doctor | Sign off results |
| POST | `/appointments/:id/prescriptions` | Doctor | Create prescription |
| POST | `/appointments/:id/results/release` | All | Release results to patient |
| GET | `/queue` | Worker | Today's appointment queue |
| GET | `/schedule` | Worker | Own schedule |
| GET | `/prescriptions` | Pharmacist | Fulfilment queue |
| POST | `/prescriptions/:id/fulfil` | Pharmacist | Fulfil |
| GET | `/admin/clinics` | Admin | |
| POST/PATCH | `/admin/clinics` | Admin | |
| GET | `/admin/users` | Admin | |
| POST/PATCH | `/admin/users` | Admin | |
| GET | `/admin/schedules` | Admin | |
| POST/PATCH | `/admin/schedules` | Admin | |
| GET | `/admin/audit` | Admin | |
| GET/PATCH | `/admin/config` | Admin | |

---

## Appointment Detail Page — Component Map

This is the most complex page. It is assembled from HTMX partials:

```
/appointments/:id
├── patient-header partial    — name, DOB, UUID, contact
├── dag-timeline partial      — visual state strip + transition buttons (role-gated)
├── result-panel partial      — DiagnosticReport (shown after RESULTS_RECEIVED)
│   └── sign-off button       — Doctor only, shown if status = pending_signoff
├── note-feed partial         — chronological notes, add-note form
├── task-list partial         — open tasks, add-task form
└── prescription panel        — Doctor: write new Rx; Pharmacist: fulfil queue
```

Each partial has its own HTMX endpoint (`/appointments/:id/partials/:name`) for live reload after state changes.

---

## Access Control Rule

Role enforcement and clinic-scoping belong in the **service layer**, not the handler.
Handlers are thin: parse input → call service → render output or map error.
The service receives the caller's claims from context and enforces access rules before touching data.

```go
// internal/admin/app/httpingress/handler.go
func (h *Handler) AppointmentDetail(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    page, err := h.svc.GetAppointmentDetail(r.Context(), id)
    if err != nil {
        pkghttp.WriteError(w, err) // maps domain errors to HTTP status codes
        return
    }
    h.render(w, "appointments/detail.html", page)
}

// internal/admin/pkg/appointment/detail.go
func (s *DetailService) GetAppointmentDetail(ctx context.Context, id string) (*AppointmentDetailPage, error) {
    claims := auth.ClaimsFromCtx(ctx)
    appt, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if !claims.HasClinicAccess(appt.ClinicID) {
        return nil, ErrForbidden // mapped to 403 by pkghttp.WriteError
    }
    // ...
}
```

Cross-clinic data access is only permitted for the `admin` role — enforced inside the service, not via handler conditionals.
