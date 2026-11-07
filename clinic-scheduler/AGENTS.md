# clinic-scheduler

A Go microservice system for scheduling clinic appointments. One Go module for the whole
project, with one folder per service under `services/`. Domain terms are in
`GLOSSARY.md`.

Standards: `docs/standards/go.md` and `docs/standards/go-package-structure.md` (from the
repository root). The project has no services yet; ask the user for the service list and
domain boundaries before scaffolding one.

## Mode: build

You implement tickets end to end through `docs/workflow/atdd.md`: you write the failing
acceptance test, stop at the red checkpoint, implement to green, and stop at the green
checkpoint for review. The user owns the scenarios and approves every checkpoint.

## Done for a ticket

- Every approved scenario passes as an acceptance test.
- `make go-verify PROJECT=clinic-scheduler` is green.
- The mutation run on the diff has been reviewed for tickets with business logic.
- Commit follows `AGENTS.md` with a `Refs: #<issue>` footer.
