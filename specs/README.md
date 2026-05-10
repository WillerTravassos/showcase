# STD Clinic Management System — Spec-Driven Development Index

Read CLAUDE.md at repo root before any session

## How to Use These Specs with Claude Code

### Every session starts the same way

```bash
claude "Read CLAUDE.md, then read specs/sessions/01-backend-core-auth.md and implement it."
```

Claude Code reads `CLAUDE.md` automatically at session start. The session file's delegation
prompt also explicitly instructs it to read `CLAUDE.md` first. This double-enforcement means
the architecture conventions are never skipped.

### Rules
- `CLAUDE.md` at repo root wins over everything. If a session spec conflicts with it, `CLAUDE.md` wins.
- Read the relevant `specs/conventions/` files before starting — the session delegation prompt tells you which ones.
- If something is ambiguous, add a `TODO:` comment and continue. Do not invent behaviour.
- All code must compile and pass `go vet ./...` and `golangci-lint run ./...` before a session is done.
- Write tests alongside implementation, not after.

---

## Convention Files

Read `CLAUDE.md` first. Then the relevant conventions:

| File | When to read |
|------|-------------|
| [conventions/architecture.md](conventions/architecture.md) | Every session |
| [conventions/tooling.md](conventions/tooling.md) | Every session |
| [conventions/errors.md](conventions/errors.md) | Any session writing domain logic or handlers |
| [conventions/database.md](conventions/database.md) | Any session touching DB, migrations, CNPG |
| [conventions/tgv.md](conventions/tgv.md) | Sessions 04, 07, 09, 15, 16, 17 — and any time patient data is handled |
| [conventions/tilt.md](conventions/tilt.md) | Session 18, or when setting up local dev |
| [conventions/iac.md](conventions/iac.md) | Session 18 |

---

## Component Overviews

| File | Covers |
|------|--------|
| [overview/01-system.md](overview/01-system.md) | UUIDv5, FHIR, compliance summary |
| [overview/02-backend.md](overview/02-backend.md) | Router, middleware, NATS subjects |
| [overview/03-parser.md](overview/03-parser.md) | Pipeline stages, LabReport contract |
| [overview/04-patient-app.md](overview/04-patient-app.md) | Templates, HTMX, Alpine patterns |
| [overview/05-admin-portal.md](overview/05-admin-portal.md) | Role nav, appointment detail map |
| [overview/06-infrastructure.md](overview/06-infrastructure.md) | K8s namespaces, HPA, observability |

---

## Development Sessions

### Backend API
| # | Session | Depends On | Key Conventions |
|---|---------|------------|-----------------|
| [01](sessions/01-backend-core-auth.md) | Backend Core & Auth | — | architecture, tooling, database |
| [02](sessions/02-patient-fhir-model.md) | Patient & FHIR Model | 01 | architecture, database, tgv |
| [03](sessions/03-scheduling-engine.md) | Scheduling Engine | 01, 02 | architecture, database |
| [04](sessions/04-consultation-dag.md) | Consultation DAG | 01, 02, 03 | architecture, errors, tgv |
| [05](sessions/05-staff-absence-scheduling.md) | Staff & Absence Scheduling | 01, 03 | architecture, tgv |
| [06](sessions/06-prescription-workflow.md) | Prescription Workflow | 01, 02, 04 | architecture, errors, tgv |
| [07](sessions/07-audit-log.md) | Audit Log | 01 | architecture, database, tgv |

### Test Parser
| # | Session | Depends On | Key Conventions |
|---|---------|------------|-----------------|
| [08](sessions/08-parser-ocr.md) | Parser — OCR Stage | 01 | tooling |
| [09](sessions/09-parser-ai-extraction.md) | Parser — AI Extraction | 08 | tooling, tgv |

### Patient Web App
| # | Session | Depends On | Key Conventions |
|---|---------|------------|-----------------|
| [10](sessions/10-patient-app-auth-registration.md) | Auth & Registration | 01, 02 | architecture, tgv |
| [11](sessions/11-patient-app-booking.md) | Booking Flow | 10, 03, 04 | architecture |
| [12](sessions/12-patient-app-appointments-results.md) | Appointments & Results | 10, 04 | architecture, tgv |

### Admin Portal
| # | Session | Depends On | Key Conventions |
|---|---------|------------|-----------------|
| [13](sessions/13-admin-core-patient-views.md) | Core & Patient Views | 01, 02 | architecture, tgv |
| [14](sessions/14-admin-workflow-operations.md) | Workflow Operations | 13, 04 | architecture, tgv |
| [15](sessions/15-admin-clinical-views.md) | Doctor & Pharmacist Views | 13, 06 | architecture, tgv |
| [16](sessions/16-admin-config.md) | Admin Config & Management | 13, 05, 07 | architecture, tgv |

### Platform
| # | Session | Depends On | Key Conventions |
|---|---------|------------|-----------------|
| [17](sessions/17-notifications.md) | Notifications Service | 01, 04 | tooling, tgv |
| [18](sessions/18-infrastructure.md) | Infrastructure & Kubernetes | All | tilt, iac, database |
