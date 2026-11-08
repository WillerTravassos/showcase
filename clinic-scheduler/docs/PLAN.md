# clinic-scheduler plan

A booking platform for sexual health testing clinics: patients book clinic appointments
and follow-up calls through a patient portal, and the workforce runs clinics, rooms, and
schedules through an admin portal. Showcase goals: a distributed Go system on Kubernetes
(KinD and Tilt locally), PIPEDA and Quebec Law 25 compliance, FHIR interoperability, and
the criteria of Quebec's TGV certification (`docs/compliance/tgv-matrix.md`). Synthetic
data only.

Stories convert to tickets (`docs/workflow/tickets.md`) when work on them starts. Each
story lists its rules; Given/When/Then scenarios are drafted at ticket time and approved
by the user. Terms follow `GLOSSARY.md`; decisions are in `docs/adr/`.

## Ticket status

Tickets exist only for epics 0 and 1: drafts 001 to 013 in `clinic-scheduler/tickets/`
(gitignored, not yet published). Story 0.2 is split into tickets 002 and 003, story 1.3
into 007 and 008, and ticket 005 (outbox relay and NATS package) is a prefactor for
story 1.2. Create tickets for epic 2 onwards once the work in epics 0 and 1 completes,
answering the open questions below for the stories they block first.

## Decisions

| Area | Decision | ADR |
|---|---|---|
| Templates | templ | 0001 |
| Portal architecture | MVC with view models, two BFFs | 0002 |
| APIs | ConnectRPC internal, OpenAPI and ogen for public FHIR | 0003 |
| Identity | Keycloak, `patient` and `workforce` realms | 0004 |
| Event sourcing | `booking` and `scheduling` only | 0005 |
| Persistence | CNPG Postgres, sqlc, pgx v5, otelpgx, golang-migrate | 0006 |
| Event delivery | Transactional outbox, Go relay, NATS KV lease | 0007 |
| Messaging | NATS JetStream, CloudEvents headers, NACK, DLQ per stream | 0008 |
| Slot hold | NATS KV with TTL, Postgres exclusion constraints | 0009 |
| Booking flow | `internal/dag`, versioned per clinic | 0010 |
| Networking | Cilium CNI and Gateway API | 0011 |
| Mesh and certificates | Linkerd edge, cert-manager, trust-manager | 0012 |
| Theme | Indigo and rose, light and dark, WCAG 2.1 AA | 0013 |
| Synthetic data | Seeder service, k6 for load | 0014 |
| Personal information | Minimization, per-patient keys, OpenBao, crypto-shredding | 0015 |
| Security baseline | ASVS L2, fixed roles, audit of every read | 0016 |
| Acceptance drivers | HTTP (goquery) and browser (chromedp, axe-core) | 0017 |
| Jurisdictions | ISO 3166-2, rickar/cal, UTC storage, English and French | 0018 |

## Cross-cutting rules

These apply to every story from epic 1 on; a ticket's scenarios cover the ones it
touches.

- Every read or change of personal information emits an audit event (ADR 0016).
- Every user-facing string exists in English and French; English is the default.
- Every page passes axe-core in light and dark themes and works by keyboard.
- No personal information in events, logs, spans, metrics, URLs, or error messages.
- Every route declares the roles that may call it; ownership is checked on the server.
- The footer shows the application version (TGV S09.08).
- The seeder grows with each story that adds an internal command it needs.

## Open questions

Answer before the stories that depend on them reach ticket stage.

- **Cancel and reschedule**: can a patient cancel or reschedule a clinic appointment,
  and until when? Affects 1.7, 5.3, 6.5.
- **Minors**: Quebec law covers consent of minors under 14 (TGV P06.11). Is booking
  refused under 14, or does it need a guardian? Affects 1.3, 4.6.
- **Leave over booked appointments**: when an admin records leave over a staff member's
  booked appointments, are they reassigned automatically, listed for an admin, or is the
  leave refused? Affects 2.8.
- **Retention periods**: actual values per data category for Quebec. Affects 9.4.
- **FHIR callers**: who calls `fhir-gateway` and with what credentials (client
  credentials per partner?). Affects epic 8.

## Epic 0: Project scaffold

Goal: an empty, verified project that answers over HTTPS on the new cluster.

### 0.1 Project builds and verifies
As a developer, I want the clinic-scheduler module scaffolded, so that every later ticket
starts from a green `make go-verify`.
- Rules: one Go module; `internal/telemetry` and `internal/logctx` exist; `.go-arch-lint.yml` and `.mockery.yml` exist; templ, buf,
  sqlc, and Tailwind standalone CLI pinned in devbox.
- Verify the setup items never run (devbox shell, Gooze `--diff` flag, mockery v3 schema,
  go-arch-lint schema, acceptance path `ask` permission).
- TGV: S12.02, S12.04, S12.08, S13.01.
- Services: none.

### 0.2 Patient portal answers over HTTPS
As a patient, I want the portal to load securely at `https://patient.localhost`, so that I
can trust the site.
- Rules: Cilium replaces kindnet (cluster reset: **every project's local cluster state is
  lost**); Gateway API routes `patient.localhost`; cert-manager issues the certificate;
  the page shows the theme, language switch, and version; security headers set (CSP,
  `nosniff`, `frame-ancestors 'none'`).
- Check in this ticket: rootful Docker for ports 80 and 443, Cilium host-network gateway
  option name, NATS version with per-key KV TTL.
- TGV: T01, T08, T13, T14, G01, G02, S09.08.
- Services: `patient-web`. ADRs: 0001, 0002, 0011, 0012, 0013, 0018.

## Epic 1: Walking skeleton

Goal: a patient books one clinic appointment end to end, through every layer:
gateway, BFF, domain services, Postgres, outbox, NATS, and traces in Tempo.

### 1.1 Patient sees the bookable days of a clinic
As a patient, I want to see which days a clinic can be booked in the next 45 days, so
that I can plan my visit.
- Rules: days run from today to 45 days ahead in the clinic's time zone; holidays of the
  clinic's jurisdiction and closures are not bookable; a day is bookable when the clinic
  schedule opens it. Public page, no sign-in.
- The seeder creates one Quebec clinic with rooms and a clinic schedule.
- TGV: P11.15.
- Depends on: 0.2. Services: `scheduling`, `patient-web`, `seeder`. ADRs: 0005, 0006, 0018.

### 1.2 Patient sees the free slots of a day
As a patient, I want to see the free times of a clinic on a day, so that I can pick one.
- Rules: a slot exists when the clinic is open and at least one room and one staff
  member on shift are free; breaks are never slots; the patient sees times, never rooms
  or staff.
- `booking` builds its availability projection from `scheduling` events through the
  outbox relay and NATS.
- The seeder creates staff shifts.
- Adds `internal/tempotest`, the Tempo trace helper for acceptance tests.
- Depends on: 1.1. Services: `scheduling`, `booking`, `patient-web`, `seeder`.
  ADRs: 0007, 0008, 0009.

### 1.3 Patient registers and signs in
As a patient, I want to create an account and sign in, so that I can book.
- Rules: self-registration with email verification; lockout after five failed attempts;
  generic failure message; sign-out on every page; configurable idle sign-out; session
  cookie `HttpOnly`, `Secure`, `SameSite=Lax`; CSRF token on state-changing requests; a
  minimal profile exists in `patient` after the first sign-in.
- TGV: P09.02, S06.05, S06.08, S06.09, S06.15, S06.16, S06.17, S06.19, S06.25.
- Depends on: 0.2. Services: Keycloak, `patient-web`, `patient`. ADR: 0004.

### 1.4 Patient holds a slot
As a patient, I want the slot I pick kept for me for ten minutes, so that nobody takes it
while I finish booking.
- Rules: a hold lasts ten minutes; a slot at capacity cannot be held; two patients racing
  for the last place get one hold and one "no longer available"; the page counts down.
- Depends on: 1.2, 1.3. Services: `booking`, `patient-web`. ADR: 0009.

### 1.5 Hold expires
As a patient, I want to be told when my time ran out, so that I can start again.
- Rules: after ten minutes the slot is free for others; the patient's next action returns
  them to the start of booking with a message.
- Depends on: 1.4. Services: `booking`, `patient-web`.

### 1.6 Patient confirms a clinic appointment
As a patient, I want to confirm my held slot, so that my appointment is booked.
- Rules: confirmation ties patient, clinic, room, staff, and slot; it fails after the hold
  expired; a room or staff member is never booked twice in a block; the slot capacity
  drops by one for others.
- `domain.booking.appointment.confirmed` is published; the trace spans gateway to NATS.
- Depends on: 1.4. Services: `booking`, `patient-web`. ADRs: 0005, 0007.

### 1.7 Patient sees their upcoming appointments
As a patient, I want to see my booked appointments, so that I know where and when to go.
- Rules: a patient sees only their own appointments; another patient's appointment
  answers "not found"; each view emits an audit event.
- TGV: P09.03, P09.04, P11.11.
- Depends on: 1.6. Services: `booking`, `patient-web`.

### 1.8 Booking smoke test
As a developer, I want a k6 smoke test of the booking path, so that a broken booking
shows up without clicking through it.
- Rules: per `docs/standards/k6.md`; thresholds on error rate, latency percentiles, and
  checks.
- Depends on: 1.6. Services: k6. ADR: 0014.

## Epic 2: Scheduling administration

Goal: admins run clinics, rooms, and schedules from the admin portal.

### 2.1 Workforce signs in to the admin portal
As a workforce member, I want to sign in with MFA at `https://admin.localhost`, so that
only I can act under my name.
- Rules: MFA required; no self-registration; roles admin, staff, doctor, auditor come
  from the realm; each page lists the roles that may open it; no generic accounts.
- TGV: S06.04, S06.06, S06.13, S06.18, S10.08.
- Depends on: 1.3. Services: Keycloak, `admin-web`. ADRs: 0004, 0016.

### 2.2 Admin creates clinics and rooms
As an admin, I want to add a clinic and its rooms, so that they can be scheduled.
- Rules: only admins; a clinic has a jurisdiction, a time zone, and a block duration; one
  to five rooms per clinic; names unique within their scope.
- Depends on: 2.1. Services: `scheduling`, `admin-web`.

### 2.3 Admin sets the clinic schedule
As an admin, I want to set a clinic's opening hours months ahead, so that patients can
book.
- Rules: holidays of the jurisdiction are never open; hours align to blocks; changes to
  days with bookings are refused.
- Depends on: 2.2. Services: `scheduling`, `admin-web`.

### 2.4 Admin closes a clinic or a room
As an admin, I want to close a room or a clinic for a date range, so that no one books it
during renovation.
- Rules: a closure removes slots for its range; closures over booked appointments are
  refused and list the conflicts.
- Depends on: 2.3. Services: `scheduling`, `booking`, `admin-web`.

### 2.5 Admin changes the block duration
As an admin, I want to change a clinic's block duration from a date, so that
appointments can get longer or shorter.
- Rules: the duration divides 60; it applies from its effective date; dates with bookings
  keep the old duration; breaks stay the last block of each hour.
- Depends on: 2.3. Services: `scheduling`, `admin-web`.

### 2.6 Admin adds staff and doctors
As an admin, I want to add staff and doctors, so that they can be scheduled and sign in.
- Rules: creates a `workforce` account with the right role; staff and doctors are
  assigned to clinics.
- TGV: S06.14.
- Depends on: 2.1. Services: Keycloak, `scheduling`, `admin-web`.

### 2.7 Admin sets staff shifts
As an admin, I want to schedule staff shifts, so that clinics have people to run
appointments.
- Rules: a shift is at most 8 hours; the last block of every hour is a break; a staff
  member can work at several clinics in one day, never in two places at once.
- Depends on: 2.6. Services: `scheduling`, `booking`, `admin-web`.

### 2.8 Admin records leave
As an admin, I want to record vacation, sick leave, and unpaid leave, so that absent
staff and doctors are never booked.
- Rules: leave removes the person's availability for its range; see the open question on
  leave over booked appointments.
- Depends on: 2.7. Services: `scheduling`, `admin-web`.

### 2.9 Admin sets doctor availability
As an admin, I want to set when doctors take follow-up calls and how long a call lasts,
so that patients can book calls.
- Rules: call length is set on the availability, separate from the block duration;
  availability never overlaps leave.
- Depends on: 2.6. Services: `scheduling`, `admin-web`.

### 2.10 Workforce permissions hold on every page
As a workforce member, I want to see and change only what my role allows, so that
information stays on a need-to-know basis.
- Rules: staff read most pages and update their profile, test results, and clinic
  appointments; doctors update test results, their follow-up calls, and their profile;
  only admins add clinics and rooms; forbidden resources answer "not found".
- TGV: P09.01, P09.03, P09.05, S06.03, S06.13.
- Depends on: 2.6. Services: all domain services, `admin-web`. ADR: 0016.

## Epic 3: Schedule views

Goal: the workforce sees every clinic's day at a glance.

### 3.1 Day view of a clinic
As a workforce member, I want a day view of a clinic with its rooms, so that I see what is
booked and what is free.
- Rules: one column per room, one row per block; empty blocks read "Available"; breaks,
  closures, and holidays are shaded; times in the clinic's time zone; patient details
  appear only for roles that need them, and each view emits an audit event.
- TGV: P11.04, P11.15, P09.04.
- Depends on: 2.7, 1.6. Services: `booking`, `scheduling`, `admin-web`.

### 3.2 Toggle staff and doctor lanes
As a workforce member, I want to show or hide staff and doctor schedules in the day view,
so that I see only what I need.
- Depends on: 3.1, 2.9. Services: `admin-web`.

### 3.3 Day view updates live
As a workforce member, I want the day view to update when bookings change, so that I do
not act on stale information.
- Rules: updates arrive through server-sent events fed by domain events.
- Depends on: 3.1. Services: `admin-web`.

### 3.4 Week view of a clinic
As a workforce member, I want a week view, so that I can plan ahead.
- Depends on: 3.1. Services: `admin-web`.

## Epic 4: Booking flow

Goal: the full booking path with questionnaires, test selection, and consent, in an
order each clinic can change.

### 4.1 Booking flow graph library
As a developer, I want `internal/dag` to validate graphs and find the next step, so that
the booking flow can be rearranged safely.
- Rules: rejects cycles, unreachable steps, more than one start, and unmet dependencies;
  `Next` follows branch predicates. Public API names are domain language.
- Depends on: 0.1. Services: none (library). ADR: 0010.

### 4.2 Booking follows the clinic's flow version
As a patient, I want the booking screens to come in the clinic's order, so that I finish
booking the way the clinic set it up.
- Rules: every clinic starts with the default version; a session keeps the version it
  started with; each step emits `ops.booking.flow.<step>.<event>`.
- Depends on: 4.1, 1.6. Services: `booking`, `patient-web`. ADR: 0010.

### 4.3 Personal questionnaire pre-fills from the profile
As a patient, I want my details filled in for me, so that booking is quick.
- Rules: answers pre-fill from the profile; a change updates the profile only when the
  patient ticks "update my profile"; the identity fields follow the clinic's jurisdiction
  profile.
- TGV: P08.05, P11.06, P11.13.
- Depends on: 4.2. Services: `patient`, `patient-web`. ADR: 0015.

### 4.4 Health questionnaire is kept private
As a patient, I want my health answers protected, so that only the people treating me
can read them.
- Rules: never pre-filled; stored encrypted with the patient's key as a draft; attached
  to the appointment on confirmation; destroyed if the hold expires.
- TGV: S07.01, S08.01, P11.03.
- Depends on: 4.2. Services: `patient`, OpenBao, `patient-web`. ADR: 0015.

### 4.5 Patient selects eligible tests
As a patient, I want to choose from the tests that fit my answers, so that I get the
right tests.
- Rules: the clinic's eligibility rules turn answers into allowed tests; the default rule
  set is seeded; a test not allowed by the rules is refused.
- Depends on: 4.4. Services: `booking`, `patient`, `patient-web`.

### 4.6 Patient gives consent
As a patient, I want to know why my information is collected and agree to it, so that I
stay in control.
- Rules: consent at registration and at each booking records purpose, date, and privacy
  notice version; terms of use accepted on first use; consent can be withdrawn; the
  privacy notice is one click away on every page.
- TGV: P01.03, P06.01, P06.02, P06.03, P06.07, P06.09, P08.09, S05.02, S05.03.
- Depends on: 4.2. Services: `patient`, `patient-web`.

### 4.7 Admin rearranges the booking flow
As an admin, I want to reorder a clinic's booking steps and publish them, so that the
clinic can adapt its process.
- Rules: steps move up or down and optional steps toggle; invalid arrangements show the
  validation errors; publishing creates a new version.
- Depends on: 4.2, 2.1. Services: `booking`, `admin-web`.

### 4.8 Admin edits eligibility rules
As an admin, I want to change which answers allow which tests, so that clinics follow
current guidelines.
- Rules: versioned per clinic like the flow.
- Depends on: 4.5. Services: `booking`, `admin-web`.

## Epic 5: Notifications

Goal: patients get the reminders they chose, and account alerts.

### 5.1 Patient chooses reminders
As a patient, I want to pick email or SMS and up to three reminder times, so that I do
not miss my appointment.
- Rules: at most three reminders, all on the day before, in the clinic's time zone.
- Depends on: 1.3. Services: `patient`, `patient-web`.

### 5.2 Reminders are sent
As a patient, I want my reminders to arrive on time, so that I remember my appointment.
- Rules: date, time, clinic address, and a portal link only, never tests or health
  information; neutral sender and subject. Locally Mailpit receives email and a fake
  adapter receives SMS.
- TGV: S11.04.
- Depends on: 5.1, 1.6. Services: `notification`.

### 5.3 Reminders follow appointment changes
As a patient, I want reminders to stop or move when my appointment changes, so that I am
not misled.
- Depends on: 5.2 and the cancel and reschedule question. Services: `notification`.

### 5.4 Account alerts
As a patient or workforce member, I want to be told when someone signs in to my account
or changes my profile, so that I notice misuse.
- TGV: S06.22.
- Depends on: 5.2. Services: Keycloak, `notification`.

## Epic 6: Care

Goal: results reach patients, and patients book their follow-up call.

### 6.1 Staff or doctor enters a test result
As a staff member or doctor, I want to record test results, so that follow-up can begin.
- Rules: stored encrypted with the patient's key; tied to the clinic appointment.
- Depends on: 1.6, 2.10. Services: `care`, `admin-web`. ADR: 0015.

### 6.2 Patient learns results are ready
As a patient, I want to be told my results are in, so that I book my follow-up call.
- Rules: the message says only that results are ready; results show in the portal.
- Depends on: 6.1, 5.2. Services: `care`, `notification`, `patient-web`.

### 6.3 Patient books a follow-up call
As a patient, I want to pick a call time with a doctor, so that I get my results
explained.
- Rules: only after results arrive; uses the hold and confirmation rules with the
  doctor's availability and call length; ties the doctor to the clinic appointment.
- Depends on: 6.2, 2.9. Services: `booking`, `care`, `patient-web`.

### 6.4 Doctor manages follow-up calls
As a doctor, I want to see and update my follow-up calls, so that I can run my day.
- Rules: a doctor sees only their own calls.
- Depends on: 6.3. Services: `care`, `admin-web`.

### 6.5 Admin rebooks a follow-up call
As an admin, I want to move a follow-up call, so that patients are covered when a
doctor's plans change.
- Depends on: 6.3. Services: `booking`, `care`, `admin-web`.

## Epic 7: Audit

Goal: every access to personal information can be reviewed.

### 7.1 Audit records are stored and tamper-evident
As an auditor, I want every access recorded in a store that cannot be edited, so that the
record can be trusted.
- Rules: replays the `AUDIT` stream from the start; append-only with a hash chain; no
  update or delete rights.
- TGV: P09.04, P09.06, P10.04, P11.10, S10.04, S10.09, S11.01.
- Depends on: 1.7. Services: `audit`. ADR: 0016.

### 7.2 Auditor searches and exports audit records
As an auditor, I want to filter and export audit records, so that I can answer an
investigation.
- Rules: filter by person, patient, resource, and period; export CSV and JSON; auditors
  see audit records and nothing else.
- TGV: S10.05, S10.08, P14.05.
- Depends on: 7.1, 2.1. Services: `audit`, `admin-web`.

### 7.3 Auditor verifies the chain
As an auditor, I want a report that checks the hash chain, so that I know nothing was
altered.
- Depends on: 7.1. Services: `audit`, `admin-web`.

### 7.4 Abnormal access raises an alert
As an auditor, I want an alert when someone reads unusually many records, so that misuse
is caught early.
- TGV: S10.06.
- Depends on: 7.1. Services: `audit`, Grafana alerting.

## Epic 8: FHIR gateway

Goal: partners read schedules and appointments through standard FHIR R4.

### 8.1 Partner reads clinics and doctors
As a partner system, I want FHIR `Location` and `Practitioner` resources, so that I can
show clinics and doctors.
- Depends on: 2.6 and the FHIR callers question. Services: `fhir-gateway`. ADR: 0003.

### 8.2 Partner searches schedules and free slots
As a partner system, I want FHIR `Schedule` and `Slot` searches, so that I can show
availability.
- Depends on: 8.1, 1.2. Services: `fhir-gateway`.

### 8.3 Partner reads an appointment
As a partner system, I want FHIR `Appointment` reads, so that I can follow a booking.
- Rules: authorized per partner; each read emits an audit event.
- TGV: I06, P11.08.
- Depends on: 8.2. Services: `fhir-gateway`.

### 8.4 Partner discovers the API
As a partner system, I want the `CapabilityStatement`, so that I know what is supported.
- Depends on: 8.1. Services: `fhir-gateway`.

## Epic 9: Privacy lifecycle

Goal: patients exercise their rights, and data leaves when its purpose is done.

### 9.1 Patient downloads their information
As a patient, I want a copy of everything held about me in a structured format, so that I
can check it or take it elsewhere.
- TGV: P11.11, P18.01, P18.02.
- Depends on: 4.6. Services: `patient`, `booking`, `care`, `patient-web`.

### 9.2 Patient corrects their information
As a patient, I want to correct my information, so that my record is accurate.
- Rules: the correction reaches every service that holds a copy.
- TGV: P09.09, P11.12, P11.13, P11.17.
- Depends on: 4.3. Services: `patient`, `patient-web`.

### 9.3 Patient deletes their account
As a patient, I want my information destroyed, so that it no longer exists anywhere.
- Rules: destroys the patient's key (crypto-shredding) and rows; refused with reasons
  where a legal obligation requires keeping data.
- TGV: P10.06, P10.08, P17.01 to P17.05.
- Depends on: 4.4. Services: `patient`, `care`, OpenBao, `patient-web`.

### 9.4 Information is destroyed at the end of its retention period
As the clinic, I want expired information destroyed automatically, so that nothing is
kept longer than allowed.
- Depends on: 9.3 and the retention periods question. Services: `patient`, `care`.

## Epic 10: Access controls

Goal: access to personal information narrows to time, purpose, and patient choice.

### 10.1 Workforce access follows shifts
As the clinic, I want staff to reach patient information only during their shifts, so
that access matches need.
- TGV: P09.07.
- Depends on: 2.7, 2.10. Services: all domain services.

### 10.2 Patient restricts access
As a patient, I want to restrict who can see parts of my information and for how long,
so that I control sensitive details.
- TGV: P09.10, P09.11.
- Depends on: 2.10. Services: `patient`, `care`, `patient-web`.

### 10.3 Clinician withholds a result
As a doctor, I want to hold back a result from the patient for a time, so that the
patient hears it in a call first when that is safer.
- TGV: P09.12.
- Depends on: 6.2. Services: `care`, `admin-web`.

### 10.4 Admin reviews access
As an admin, I want a list of who has which role and access, so that I can remove what is
no longer needed.
- TGV: S06.12, P09.08.
- Depends on: 2.10. Services: `admin-web`, Keycloak.

## Epic 11: Platform hardening

Goal: the platform matches the security architecture in the ADRs.

### 11.1 Services talk over mTLS
- Linkerd edge injected; authorization policies allow only the expected callers; BFFs
  accept unmeshed traffic only from the gateway.
- TGV: S06.10, S07.02, S11.02. ADR: 0012.

### 11.2 Network is denied by default
- Cilium NetworkPolicy deny-all per namespace, then explicit allows.
- TGV: S11.03, T07. ADR: 0011.

### 11.3 Workforce accounts come from the directory
- A fake Santé Québec directory federates into the `workforce` realm.
- TGV: S06.07, S06.24, T02, T03, T12, T20, I01. ADR: 0004.

### 11.4 Databases are backed up and restorable
- CNPG backups to SeaweedFS through the Barman Cloud plugin; a restore check runs.
- TGV: S09.05, S09.06, T21.

### 11.5 Web pages pass a dynamic scan
- OWASP ZAP baseline scan against the gateway hosts.
- TGV: S12.04, S16.02.

### 11.6 Booking holds under load
- k6 load and stress tests, including many patients racing for one slot.
- TGV: PF01, PF05, T06.

### 11.7 Credential stuffing is detected
- Keycloak brute-force detection plus a Grafana alert on failed sign-in rates.
- TGV: S06.21, S06.23.

## Epic 12: Compliance documents

Written after the software epics. Each document closes the `doc` rows of the TGV matrix.

- Privacy impact assessment (ÉFVP): P03, P04.
- Privacy policy and terms of use: P01, P07.02, S05.01.
- Retention schedule and personal information inventory: P10.02, P14.
- Architecture document: T04, T18.
- Performance document from k6 results: PF01 to PF07.
- User, operations, and installation guides: S09.02, S09.03, S09.04.
- Key management document: S08.01.
