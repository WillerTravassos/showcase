# clinic-scheduler

A Go microservice system for booking clinic appointments, with a patient portal and an
admin portal. One Go module for the whole project, with one folder per service under
`services/`. Domain terms are in `GLOSSARY.md`; the delivery plan is `docs/PLAN.md`;
decisions are in `docs/adr/`.

Standards: `docs/standards/go.md` and `docs/standards/go-package-structure.md` (from the
repository root), with the project overrides below. Where an override and a repository
standard disagree, the override wins inside this project.

## Mode: build

You implement tickets end to end through `docs/workflow/atdd.md`: you write the failing
acceptance test, stop at the red checkpoint, implement to green, and stop at the green
checkpoint for review. The user owns the scenarios and approves every checkpoint.

## Services

| Service        | Tier     | Owns                                                                 |
|----------------|----------|----------------------------------------------------------------------|
| `patient-web`  | gateway  | Patient portal (HTMX), public through the Cilium gateway             |
| `admin-web`    | gateway  | Admin portal (HTMX) for the workforce, separate host                 |
| `scheduling`   | domain   | Clinics, rooms, closures, holidays, block duration, shifts, leave, doctor availability (event-sourced) |
| `booking`      | domain   | Slots, holds, clinic appointments, booking flow and flow versions (event-sourced) |
| `patient`      | domain   | Profiles, questionnaires, consent, reminder preferences              |
| `care`         | domain   | Test results, follow-up calls                                         |
| `notification` | domain   | Reminders and account alerts by email and SMS                         |
| `audit`        | domain   | Append-only audit records, auditor reports                            |
| `fhir-gateway` | gateway  | Read-only public FHIR R4 subset                                       |
| `seeder`       | tool     | Synthetic data through the internal APIs                              |

Ask before adding, merging, or splitting a service.

## Overrides

- **Templates**: HTML is rendered with [templ](https://templ.guide) components, which
  auto-escape by context. This replaces the `html/template` rule in `security.md`. Never
  render HTML with `text/template` or string concatenation. See ADR 0001.
- **Page structure**: each page or fragment is an `ingress` handler (controller), a view
  model struct defined next to its templ component in `web/`, and the component. No
  presenter interfaces, no base view model, and domain types never reach a template. See
  ADR 0002.
- **API layout**: a service's `api/` holds `.proto` files and generated ConnectRPC code
  for internal calls (buf), or an OpenAPI spec and generated ogen code for a public API
  (`fhir-gateway` only). This replaces the OpenAPI-only `api/` in
  `go-package-structure.md`. See ADR 0003.
- **Security baseline**: OWASP ASVS level 2 (health data), not level 1. Every read of
  personal information emits an audit event. See ADR 0016.
- **Queries**: sqlc with pgx v5, never hand-built SQL strings. Migrations use
  golang-migrate. A Postgres-only feature is listed in `docs/adr/0006` under
  "CockroachDB inventory" in the same change.
- **Events**: subjects, envelope, and payload rules are in ADR 0008. No personal
  information in any message payload, span, log, or metric: identifiers only.
- **Language**: every user-facing string goes through the message catalogue in English
  and French. English is the default.
- **Accessibility**: pages meet WCAG 2.1 AA in light and dark themes; the browser
  acceptance suite runs axe-core on every page it visits.
- **Compliance**: stories and tickets list the TGV criteria they cover (see
  `docs/compliance/tgv-matrix.md`). Update the matrix when a ticket closes a criterion.

## Done for a ticket

- Every approved scenario passes as an acceptance test.
- `make go-verify PROJECT=clinic-scheduler` is green.
- The mutation run on the diff has been reviewed for tickets with business logic.
- The TGV matrix lists the ticket against every criterion it covers.
- Commit follows `AGENTS.md` with a `Refs: #<issue>` footer.
