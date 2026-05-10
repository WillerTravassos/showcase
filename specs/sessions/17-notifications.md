# Session 17 — Notifications Service

## Claude Code Delegation Prompt

```
You are implementing Session 17 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md
  4. specs/conventions/tooling.md — github.com/WillerTravassos/showcase/pkg/log
  5. specs/conventions/tgv.md — French language requirement: all notifications must have
     fr-CA template; en-CA is optional. Patient preferred_lang determines which to send.

Session 01 is complete (config, NATS). Session 04 is complete (DAG side-effect events).

Your job is to build the notification worker binary: NATS consumer, template engine
(FR + EN variants), email dispatcher, recipient resolution, and notification log.

Work autonomously. Add TODO comments for genuine ambiguities.
When done: go build ./cmd/notifier && go vet ./...
```

---

## Dependencies
- Session 01 (config, NATS, `github.com/WillerTravassos/showcase/pkg/log`)
- Session 04 (NATS events published by DAG side effects)

---

## Package Location

```
internal/notifier/
├── app/
│   ├── service.go      # package app — composition root, wires all dependencies
│   └── settings.go     # Settings struct with env-var config
├── pkg/
│   └── notification/
│       ├── notification.go  # EventType, Channel, Recipient named types + interfaces
│       ├── model.go         # TemplateData, Template structs
│       ├── worker.go        # Worker — NATS consumer loop
│       ├── templates.go     # Engine — template rendering
│       ├── resolver.go      # Resolver — recipient lookup
│       └── email.go         # EmailDispatcher interface + SMTPDispatcher
cmd/notifier/
└── main.go             # calls app.Start() only
```

---

## Deliverables

### 1. Notifier Binary (`cmd/notifier/main.go`)

Standalone worker. No HTTP server. Subscribes to NATS streams on startup.

Config in `internal/notifier/app/settings.go`:
```go
type Settings struct {
    SMTPHost        string `env:"SMTP_HOST,required"`
    SMTPPort        int    `env:"SMTP_PORT" envDefault:"587"`
    SMTPUsername    string `env:"SMTP_USERNAME,required"`
    SMTPPassword    string `env:"SMTP_PASSWORD,required"`
    SMTPFromAddress string `env:"SMTP_FROM,required"`
    SMTPFromName    string `env:"SMTP_FROM_NAME" envDefault:"Clinique STI"`
    // SMS optional — disabled if empty
    SMSProviderURL  string `env:"SMS_PROVIDER_URL"`
    SMSAPIKey       string `env:"SMS_API_KEY"`
    NATSURL         string `env:"NATS_URL,required"`
    DatabaseURL     string `env:"DATABASE_URL,required"`
    BackendAPIURL   string `env:"BACKEND_API_URL,required"`
}
```

Composition root: `internal/notifier/app/service.go` (`package app`) wires all dependencies.
`cmd/notifier/main.go` calls `app.Start(ctx)` only.

Wire order in `service.go`/`Start()`:
1. Load config → init `github.com/WillerTravassos/showcase/pkg/log` logger
2. Connect NATS
3. Connect to DB (for notification_log writes and template reads)
4. Build `BackendClient` (to resolve recipient contact info)
5. Build `TemplateEngine`
6. Build `EmailDispatcher`
7. Build `Worker`; call `worker.Start(ctx)`
8. Graceful shutdown on SIGTERM

### 2. Notification Model (`internal/notifier/pkg/notification/notification.go` and `model.go`)

```go
type EventType string
const (
    EventAppointmentBooked    EventType = "appointment_booked"
    EventAppointmentCancelled EventType = "appointment_cancelled"
    EventAppointmentNoShow    EventType = "appointment_no_show"
    EventResultsReleased      EventType = "results_released"
    EventPhoneApptBooked      EventType = "phone_appt_booked"
    EventResultsReceived      EventType = "results_received"       // → doctor
    EventPrescriptionFulfilled EventType = "prescription_fulfilled"
    EventStaffAbsenceConflict EventType = "staff_absence_conflict" // → admin
    EventParserReviewNeeded   EventType = "parser_review_needed"   // → admin
    EventCertificationExpiring EventType = "certification_expiring" // → admin
    EventUserCreated          EventType = "user_created"           // → new staff
)

type Channel string
const (
    ChannelEmail Channel = "email"
    ChannelSMS   Channel = "sms"
)

// Lang is a named type for language codes.
type Lang string
const (
    LangFR Lang = "fr-CA"
    LangEN Lang = "en-CA"
)

type Recipient struct {
    UUID          uuid.UUID
    Email         string
    Name          string
    PreferredLang Lang // "fr-CA" or "en-CA"
}

// NotificationRepository defines the port for writing notification logs.
type NotificationRepository interface {
    WriteLog(ctx context.Context, entry LogEntry) error
    LoadTemplates(ctx context.Context) ([]Template, error)
}
```

### 3. Template Engine (`internal/notifier/pkg/notification/templates.go`)

Templates stored in DB (`notification_templates` table — see migration below).
Loaded at startup into memory. Support simple `{{.FieldName}}` substitution.

```go
type Template struct {
    EventType  EventType
    Channel    Channel
    Lang       Lang   // LangFR | LangEN
    Subject    string // email only
    Body       string // plain text with {{.Var}} placeholders
    // SECURITY NOTE: Body is loaded from DB (admin-controlled), not from user input.
    // Never allow patient-supplied content in Body — it is executed as a Go text/template.
}

type TemplateData struct {
    PatientName     string
    ClinicName      string
    AppointmentDate string
    AppointmentTime string
    PortalURL       string
    MedicationName  string
    DoctorName      string
    // add fields as needed
}

type Engine struct {
    templates map[string]Template // key: "{event_type}:{channel}:{lang}"
    log       log.Logger
}

// Render selects the template for the event+channel+lang combination.
// Falls back to LangFR if the requested lang has no template.
// Returns ErrTemplateNotFound if no template exists for this event+channel.
func (e *Engine) Render(eventType EventType, channel Channel, lang Lang, data TemplateData) (subject, body string, err error)
```

TGV requirement: **French templates are mandatory for all event types**.
English templates are optional. If a patient's `PreferredLang` is `en-CA` but no English
template exists for the event, fall back to `fr-CA`.

### 4. Migration (`migrations/000009_notifications.up.sql`)

```sql
CREATE TABLE notification_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type  TEXT NOT NULL,
    channel     TEXT NOT NULL DEFAULT 'email',
    lang        TEXT NOT NULL DEFAULT 'fr-CA',
    subject     TEXT,
    body        TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT 'v1',
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(event_type, channel, lang, version)
);

-- Seed: French templates for all event types (mandatory for TGV)
INSERT INTO notification_templates (event_type, channel, lang, subject, body) VALUES
('appointment_booked', 'email', 'fr-CA',
 'Confirmation de rendez-vous — {{.ClinicName}}',
 'Bonjour {{.PatientName}},\n\nVotre rendez-vous est confirmé pour le {{.AppointmentDate}} à {{.AppointmentTime}}.\n\nCliquez ici pour voir vos rendez-vous: {{.PortalURL}}'),
('results_released', 'email', 'fr-CA',
 'Vos résultats sont disponibles',
 'Bonjour {{.PatientName}},\n\nVos résultats de dépistage sont maintenant disponibles dans votre espace sécurisé: {{.PortalURL}}'),
-- ... add all event types (see subscription table below)
;

CREATE TABLE notification_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_uuid  UUID NOT NULL,
    channel         TEXT NOT NULL,
    event_type      TEXT NOT NULL,
    lang_used       TEXT NOT NULL,  -- which lang template was actually sent
    status          TEXT NOT NULL CHECK (status IN ('sent', 'failed', 'skipped')),
    error           TEXT,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON notification_log(recipient_uuid, event_type);
CREATE INDEX ON notification_log(status, sent_at);
```

### 5. Recipient Resolver (`internal/notifier/pkg/notification/resolver.go`)

```go
type Resolver struct {
    backend BackendClient
    log     log.Logger
}

// EventPayload carries the structured NATS event data parsed from JSON.
// Use typed fields rather than map[string]any.
type EventPayload struct {
    AppointmentID  *uuid.UUID `json:"appointment_id,omitempty"`
    PatientUUID    *uuid.UUID `json:"patient_uuid,omitempty"`
    PractitionerUUID *uuid.UUID `json:"practitioner_uuid,omitempty"`
    ClinicID       *uuid.UUID `json:"clinic_id,omitempty"`
    UserUUID       *uuid.UUID `json:"user_uuid,omitempty"`
    // Add fields as needed for each event type
}

// Resolve returns recipient(s) for the given event.
// Patient events → patient email + preferred_lang from backend.
// Staff events → staff email(s) based on role + clinic_id.
// PII (email) fetched from backend on each call — never cached in notifier.
func (r *Resolver) Resolve(ctx context.Context, eventType EventType, payload EventPayload) ([]Recipient, error)
```

Routing table:

| Event | Recipient |
|---|---|
| appointment_booked | Patient |
| appointment_cancelled | Patient |
| appointment_no_show | Patient |
| results_released | Patient |
| phone_appt_booked | Patient |
| results_received | Assigned doctor for appointment |
| prescription_fulfilled | Patient |
| staff_absence_conflict | Admin(s) for the clinic |
| parser_review_needed | Admin(s) for the clinic |
| certification_expiring | Admin(s) for the clinic |
| user_created | The new staff member |

### 6. Email Dispatcher (`internal/notifier/pkg/notification/email.go`)

```go
type EmailDispatcher interface {
    Send(ctx context.Context, to, name, subject, body string) error
}

// SMTPDispatcher uses net/smtp or go-mail.
// From name is set to SMTPFromName config value.
// Plain text only — no HTML email.
type SMTPDispatcher struct { /* cfg fields */ }
```

### 7. Worker (`internal/notifier/pkg/notification/worker.go`)

```go
type Worker struct {
    nats     *nats.Conn
    db       *pgxpool.Pool
    email    EmailDispatcher
    engine   *Engine
    resolver *Resolver
    log      log.Logger
}

func (w *Worker) Start(ctx context.Context) error
func (w *Worker) processEvent(ctx context.Context, subject string, data []byte) error
```

**NATS subscriptions** (durable consumers):

| NATS Subject | EventType |
|---|---|
| `appointments.booked` | appointment_booked |
| `appointments.cancelled` | appointment_cancelled |
| `appointments.no_show` | appointment_no_show |
| `appointments.results_released` | results_released |
| `appointments.phone_booked` | phone_appt_booked |
| `appointments.results_received` | results_received |
| `prescriptions.fulfilled` | prescription_fulfilled |
| `staff.absence.conflict` | staff_absence_conflict |
| `parser.review` | parser_review_needed |
| `staff.certification.expiring` | certification_expiring |
| `user.created` | user_created |

**processEvent** must:
1. Parse NATS payload into `EventPayload` — on parse failure **NACK** the message (it is
   unprocessable and must go to the dead-letter queue, not be retried indefinitely)
2. Call `resolver.Resolve` to get recipients
3. For each recipient: call `engine.Render` with recipient's `PreferredLang`
4. Call `email.Send`
5. Write `notification_log` record regardless of send success or failure
6. On send failure: log error at `error` level via `github.com/WillerTravassos/showcase/pkg/log`; write `status=failed`
7. **ACK the NATS message** after processing (even when email dispatch fails) — email send
   failures are logged and must not cause infinite retry loops. Only unprocessable messages
   (step 1 failure) go to dead-letter via NACK.

---

## Tests Required

- `TestTemplateEngine_rendersFrench`
- `TestTemplateEngine_fallsBackToFrench_whenEnglishMissing`
- `TestTemplateEngine_unknownEvent_returnsErrTemplateNotFound`
- `TestWorker_appointmentBooked_sendsEmail` — mock NATS msg → mock dispatcher called
- `TestWorker_sendFailure_stillACKsMessage`
- `TestWorker_sendFailure_writesFailedLog`
- `TestResolver_patientEvent_returnsPatientEmailAndLang`
- `TestResolver_staffAbsenceConflict_returnsAdminEmails`
- `TestWorker_usesRecipientPreferredLang`

---

## Done Criteria

- Notifier subscribes to all NATS subjects with durable consumers
- All sends (success or failure) recorded in `notification_log`
- French template always used if no English template exists (TGV requirement)
- Send failures ACK the NATS message (no infinite retry)
- No direct zerolog imports — only `github.com/WillerTravassos/showcase/pkg/log`
- `go build ./cmd/notifier` and `go vet ./...` pass
