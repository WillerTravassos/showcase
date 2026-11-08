# 0016. ASVS level 2, fixed roles, and audit of every read

Status: accepted
Date: 2026-11-08

## Context

`security.md` sets OWASP ASVS level 1 for internet-facing apps. This system holds health
information and targets TGV certification, which requires least privilege (P09.01,
S06.13), need-to-know access (P09.03), an auditor role (S10.08), and logging of every
access to personal information in a tamper-resistant store (P09.04, P09.06, S10.04,
S10.09).

## Decision

- **Baseline**: OWASP ASVS level 2 for every service.
- **Roles**: fixed roles (patient, staff, doctor, admin, auditor), mapped to permissions
  in an in-code policy table in `internal/authz`, deny by default. Every domain service
  checks the role and the ownership of each resource on every call (patient A never
  reads patient B; a doctor sees only their follow-up calls). Unknown or forbidden
  resources answer "not found".
- **Permissions by role** (summary): admin manages clinics, rooms, schedules, staff,
  doctors, and flows; staff read most pages and update their profile, test results, and
  clinic appointments; doctors update test results, their follow-up calls, and their
  profile; auditors read audit records only.
- **Audit**: every read or change of personal information emits an audit event (actor,
  action, resource IDs, time, portal) on `audit.*`. The `audit` service stores them in an
  append-only table with a hash chain (each record holds the hash of the previous one)
  under a database role without `UPDATE` or `DELETE`, and serves filter, report, and
  export (CSV, JSON) screens to auditors.
- **Later epic**: access limited to a staff member's shift window, patient-requested
  restrictions, and results withheld by a clinician; these are attribute checks in the
  same policy table.

## Alternatives

- **Ory Keto** (relationship-based permissions): useful if per-clinic delegation
  appears; more moving parts today.
- **Audit in Loki logs**: logs are diagnostics with short retention, not records.

## Consequences

- Audit emission is part of every read path from the first epic; the `audit` service
  arrives later and replays the `AUDIT` stream.
- The hash chain makes tampering detectable, not impossible; an auditor report verifies
  the chain.
