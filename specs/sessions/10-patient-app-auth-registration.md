# Session 10 — Patient Web App: Auth & Registration

## Claude Code Delegation Prompt

```
You are implementing Session 10 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — MVC handler shape, typed template data structs
  4. specs/conventions/tooling.md — github.com/WillerTravassos/showcase/pkg/log, no direct zerolog
  5. specs/overview/04-patient-app.md — template layout, HTMX conventions, session model
  6. specs/conventions/tgv.md — French language default (fr-CA) for Quebec patients

Sessions 01 and 02 are complete (backend auth and patient APIs exist).

Your job is to build the Patient Web App binary: server, template engine, session
middleware, login/signup, and the multi-step patient registration wizard.
MVC: handlers are controllers, templates are views, domain types are models.
No business logic in handlers — they parse input, call the backend API client, render.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/patient && go vet ./...
```

---

## Dependencies
- Session 01 (backend auth API at :8080)
- Session 02 (patient API)

---

## Package Location

```
internal/patient/
├── app/
│   ├── service.go          # package app — composition root
│   ├── settings.go         # env-var config for the patient web app
│   └── httpingress/
│       ├── handler.go      # Handler struct with all injected dependencies
│       ├── pages.go        # All typed page data structs (view models)
│       ├── session.go      # Redis-backed session middleware
│       ├── renderer.go     # html/template renderer
│       └── apiclient.go    # Backend API HTTP client (server-side only)
└── pkg/                    # TODO: domain packages TBD before implementation starts
cmd/patient/
└── main.go                 # Calls app.Start() only
web/patient/
├── templates/
│   ├── base.html
│   ├── auth/login.html
│   ├── auth/signup.html
│   ├── register/step1-personal.html
│   ├── register/step2-contact.html
│   ├── register/step3-consent.html
│   ├── register/complete.html
│   └── profile.html
└── ts/
    └── components/
```

---

## Deliverables

### 1. MVC Handler Shape

All handlers are methods on a struct with injected dependencies. No globals.

```go
type AuthHandler struct {
    client BackendClient
    sess   *SessionManager
    render *Renderer
    log    log.Logger
}

// Handler methods are thin controllers:
// 1. Parse + validate HTTP input shape (field present, correct type)
// 2. Call backend client
// 3. On error → httputil.WriteError or render error template
// 4. On success → render template with typed data struct
```

### 2. Typed Template Data Structs (`internal/patient/app/httpingress/pages.go`)

All template data is a named typed struct — never `map[string]any`.
All page data structs live in `pages.go` alongside the handlers that populate them.

```go
// internal/patient/app/httpingress/pages.go
package httpingress

type LoginPageData struct {
    Error string
    CSRF  string
}

type SignupPageData struct {
    Error         string
    CSRF          string
    Jurisdictions []string
}

type RegisterStep1Data struct {
    CSRF       string
    Step       int
    TotalSteps int
    Error      string
    Prefill    *fhir.Patient // if resuming
}
// ... one struct per registration step

type ProfilePageData struct {
    Patient      *patient.Patient
    EmailDisplay string  // read-only display value — never in a form input
    CSRF         string
    Success      bool
    Error        string
}
```

### 3. Session Middleware (`internal/patient/app/httpingress/session.go`)

```go
type SessionData struct {
    UserUUID        uuid.UUID
    Role            auth.Role
    AccessToken     string
    PreferredLang   string  // TGV: fr-CA default
    RegistrationWIP *RegistrationState
    BookingWIP      *BookingState // used in session 11
    CSRFToken       string
}

type RegistrationState struct {
    Step        int
    PersonalData  *fhir.Patient // partial, assembled across steps
    ContactDone   bool
    ConsentDone   bool
}

// SessionManager provides Get, Save, Clear on Redis.
// Cookie: "session_id", httpOnly, Secure, SameSite=Strict, 7-day expiry.
type SessionManager struct { redis *redis.Client }

func (m *SessionManager) RequireAuth(next http.Handler) http.Handler
// RequireAuth redirects to /login if no valid session.
// If session exists but patient status=incomplete: redirects to /register at correct step.
```

### 4. Backend API Client (`internal/patient/app/httpingress/apiclient.go`)

Server-side only — browser never calls the backend directly.

```go
type BackendClient interface {
    Login(ctx context.Context, email, password string) (*LoginResponse, error)
    Signup(ctx context.Context, email, password, jurisdiction string) (*SignupResponse, error)
    GetPatient(ctx context.Context, token string, uuid uuid.UUID) (*patient.Patient, error)
    CreatePatient(ctx context.Context, token string, req CreatePatientRequest) error
    UpdatePatient(ctx context.Context, token string, uuid uuid.UUID, req UpdatePatientRequest) error
    RecordConsent(ctx context.Context, token string, patientUUID uuid.UUID, version, hash, ip string) error
}
```

### 5. Template Renderer (`internal/patient/app/httpingress/renderer.go`)

```go
type Renderer struct {
    templates *template.Template
    log       log.Logger
}

// Render executes named template. If HX-Request header present, renders partial only (no base layout).
func (r *Renderer) Render(w http.ResponseWriter, req *http.Request, name string, data any) error
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data any) error
```

In `ENV=development`: reload templates on every request (no restart needed for template changes).
In production: load once at startup.

### 6. Login & Signup Handlers (`internal/patient/app/httpingress/handler.go`)

**GET /login** — render `auth/login.html` with `LoginPageData`

**POST /login**:
- Call `client.Login`; on error render `auth/login.html` with `LoginPageData{Error: "..."}`
- On success: create session, set `PreferredLang` to patient's `preferred_lang` from backend
- Redirect to `/` or `/register` if `status=incomplete`

**GET /signup** — render `auth/signup.html`

**POST /signup**:
- Call `client.Signup`; then auto-login; redirect to `/register`

**GET /logout** — clear session; redirect to `/login`

### 7. Registration Wizard (in `internal/patient/app/httpingress/handler.go`)

Resumable multi-step flow. Progress persisted in `SessionData.RegistrationWIP`.

**GET /register** — detect current step from `RegistrationWIP.Step`; render correct step template

**POST /register/step/1** — personal: family name, given names, birthdate, gender, biological sex extension, pronouns extension
- Save to `RegistrationWIP.PersonalData`
- Redirect to `/register` (renders step 2)

**POST /register/step/2** — contact: address (determines jurisdiction validation), phone
- Add to `RegistrationWIP.PersonalData`
- Redirect to `/register` (renders step 3)

**POST /register/step/3** — consent:
- Render consent text v{version} (fetched from backend config)
- Compute `consent_text_hash = SHA-256(consent_text)`
- On submit: call `client.RecordConsent`; set `RegistrationWIP.ConsentDone = true`
- Redirect to `/register/complete`

**GET /register/complete** — call `client.CreatePatient` with assembled FHIR Patient; clear `RegistrationWIP`; render `register/complete.html`

TGV: set `PreferredLang` to `fr-CA` in FHIR Patient `communication` field unless patient explicitly selected English during signup.

### 8. Profile Page (in `internal/patient/app/httpingress/handler.go`)

**GET /profile** — render `profile.html` with `ProfilePageData`

**POST /profile** — submit mutable fields only

Email field: **always rendered as `<p>` not `<input>`**. If email appears in POST body, it is silently dropped before calling `client.UpdatePatient` — never forwarded to backend.

---

## Templates

All Tailwind CSS. Mobile-first. French text for all labels (with English fallback via `preferred_lang` in template data).

`base.html`: nav links (Dashboard, Appointments, Profile, Logout), `{{ template "content" . }}` slot.

Partial requests (`HX-Request: true`) skip `base.html` — renderer detects and renders fragment only.

---

## Tests Required

- `TestLoginHandler_success_createsSession`
- `TestLoginHandler_wrongPassword_rendersErrorNotRedirect`
- `TestRequireAuth_noSession_redirectsToLogin`
- `TestRequireAuth_incompleteStatus_redirectsToRegister`
- `TestRegistrationStep1_savesProgressToSession`
- `TestRegistrationComplete_assemblesFHIRPatientFromWIP`
- `TestRegistrationComplete_setsFrenchAsDefaultLang`
- `TestProfileHandler_emailDroppedFromUpdateRequest` — POST with email field → UpdateRequest has no email
- `TestRenderer_htmxRequest_noBaseLayout` — HX-Request header → partial only

---

## Done Criteria

- `go build ./cmd/patient` and `go vet ./...` pass
- `golangci-lint run ./...` passes (no direct zerolog)
- Login, signup, registration wizard work end-to-end in Tilt local env
- Email is unmodifiable from profile page
- French is the default language for new patients
- No `docker-compose` references
