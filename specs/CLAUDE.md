# CLAUDE.md — STD Clinic Management System

This file is read automatically by Claude Code at the start of every session.
**All rules here are non-negotiable constraints. Do not deviate. Do not ask for permission to deviate.**
If a session spec conflicts with a rule here, this file wins.

---

## Behaviour Rules

**Ask questions. Do not assume.**
When a requirement, path, or design decision is ambiguous, ask before writing code.
Never fill in gaps with assumptions — surface them explicitly.

---

## What This System Is

A multi-jurisdiction, multi-clinic STD testing management platform targeting **Quebec's TGV
(Trousse globale de vérification) certification** and compliance with PIPEDA (CA), HIPAA (US),
and LGPD (BR). It handles patient registration, appointment booking, lab result ingestion,
consultation workflows, and prescription management across isolated regional clusters.

Any code you write must be defensible to a Quebec MSSS auditor.

---

## Before You Write Any Code

1. Read this file fully
2. Read `specs/go_package_structure.md` — package layout and file naming conventions
3. Read `specs/conventions/architecture.md`
4. Read `specs/conventions/tooling.md`
5. Read the session spec file you have been given
6. If the session touches the database, read `specs/conventions/database.md`
7. If the session touches local dev or Tilt, read `specs/conventions/tilt.md`
8. If the session touches infrastructure or IaC, read `specs/conventions/iac.md`
9. If the session touches compliance or reporting, read `specs/conventions/tgv.md`

---

## Hard Rules — Never Violate These

### Identity
- Every principal uses UUIDv5. Namespace constants live in `pkg/auth/`. Never generate a UUID any other way for a user principal.
- UUIDv5 is derived from a stable internal seed generated at registration — **not from email**. Email is a lookup key only and must never be used as a UUID seed.
- Email is immutable after registration. No endpoint may accept an email update.
- Patients are identified by UUID in all logs, queues, and inter-service calls. Email never appears outside the encrypted FHIR resource and the auth layer.

### Logging
- **Never import `log`, `fmt` for logging, or `github.com/rs/zerolog` directly.**
- Always import `github.com/WillerTravassos/showcase/pkg/log` and use only that wrapper.
- No PII in any log line. Log UUIDs, resource types, durations, and error codes — nothing else.
- Every log line at `error` level or above must include `err`, `request_id`, and the relevant resource UUID.

### Error Handling
- Read `specs/conventions/errors.md` before writing any error handling.
- Never return raw Go error strings to HTTP clients.
- Always wrap errors with context using `fmt.Errorf("doing X: %w", err)`.
- Sentinel errors are defined in `internal/<service>/pkg/<domain>/errors.go`. Never define them inline.

### Compliance
- Any function that reads, writes, or mutates patient PII must have a comment `// COMPLIANCE: <reason>` explaining the legal basis (treatment, consent, legal obligation).
- Non-patient staff accessing patient data must trigger an audit event. No exceptions.
- Audit writes that fail must propagate the error up and result in a 500. Silent audit failures are a certification blocker.

### Testing
- Every exported function must have at least one success-path test and at least one failure-path test. Functions with more than two logical branches use table-driven tests.
- Write tests to verify logic correctness. Do not write tests purely for coverage.
- Never use `t.Fatal` outside `TestMain`. Use `require.*` from testify.
- Integration tests are named `*_integration_test.go`. Tests that require the live Tilt cluster call `test.RequireTiltCI(t)` at the top — this skips the test when `TILT_CI` env var is not set. No `//go:build` tags for integration tests.

### Code Structure
- Follow `go_package_structure.md` exactly. Read it before writing any file.
- No business logic in HTTP handlers. Handlers live in `internal/<service>/app/httpingress/`. They are thin: parse input → call domain service → render output.
- No direct database calls outside of repository files (`repository.go` or `*_repository.go`).
- Interfaces are defined in `<package_name>.go` alongside the types they operate on. The interface name is `<Domain>Repository` (e.g. `PatientRepository`). The concrete implementation is named `Repository`.
- All dependencies are wired in `internal/<service>/app/service.go` (`package app`). No DI frameworks.

### Types
- Never use plain `string` for a field that has a bounded or meaningful set of values. Define a named type (`type Gender string`) with a `const` block. Named types belong in `<package_name>.go`.

### Database
- **Never reference generic PostgreSQL.** This project uses CloudNativePG (CNPG). Read `specs/conventions/database.md`.
- Never write raw SQL outside of `*_queries.sql` files (used by sqlc) or repository implementations.
- For complex dynamic queries, use `github.com/Masterminds/squirrel` inside repository files only — it must never be imported outside the repository layer.
- Every migration file is append-only. Never modify an existing migration. New migrations only.
- All PII columns are `BYTEA` with application-level AES-256-GCM encryption. Never store PII as plain `TEXT`.

### Dependencies
- All dependencies are vendored. Run `go mod vendor` after any change to `go.mod`. Build and test with `-mod=vendor`.

### Go Version
- Always target the **latest stable Go release**. Use `//go:build` constraints, not `// +build`.
- Use `any` not `interface{}`. Use `min()`/`max()` builtins where applicable.

---

## Project Module Path

```
github.com/WillerTravassos/showcase
```

All internal imports use this module path. Never use relative imports.

---

## Key Architectural Decisions (summaries — full detail in convention files)

| Decision | Choice | File |
|----------|--------|------|
| Package structure | `specs/go_package_structure.md` — authoritative source | `specs/go_package_structure.md` |
| Backend architecture | Pragmatic DDD — `internal/<service>/pkg/<domain>/` | `conventions/architecture.md` |
| Frontend architecture | Server-side MVC — `httpingress/` handlers, `pages.go` view models, `web/` templates | `conventions/architecture.md` |
| Type safety | Named types for all bounded-value fields — never plain `string` | `conventions/architecture.md` |
| Database | CloudNativePG (CNPG), one cluster per jurisdiction | `conventions/database.md` |
| Query generation | sqlc for static queries, squirrel for dynamic — repository layer only | `conventions/tooling.md` |
| Queue | NATS JetStream (`pkg/queue/`) | `conventions/tooling.md` |
| Local dev | Tilt + kind, dynamic `tilt/tilt.d/` loading | `conventions/tilt.md` |
| IaC | OpenTofu, AWS primary | `conventions/iac.md` |
| Logging | `pkg/log` wrapper (zerolog underneath) | `conventions/tooling.md` |
| Migrations | golang-migrate, append-only | `conventions/tooling.md` |
| Testing | stdlib + testify, success+failure per export, TILT_CI for cluster tests | `conventions/tooling.md` |
| Dependencies | Vendored via `go mod vendor`, build with `-mod=vendor` | `conventions/tooling.md` |
| Health certification | Quebec TGV | `conventions/tgv.md` |
| Cross-jurisdiction identity | Single global UUIDv5 (stable seed, not email), routed to regional CNPG cluster | `conventions/architecture.md` |
