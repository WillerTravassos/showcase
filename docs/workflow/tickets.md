# Tickets

A ticket is one vertical slice: a thin piece of behaviour that works end to end and that a
user or caller can observe. Its scenarios are the specification the agent builds against
(see `atdd.md`).

## Where tickets live

- Draft in `<project>/tickets/NNN-<slug>.md` (gitignored).
- Publish to the showcase GitHub repository with the project's label:
  `gh issue create --label <project> --title "<title>" --body-file <file>`. Ask the user
  before running it, then rename the draft to the issue number.

## Writing scenarios

- Use Given/When/Then in the project's domain language (`GLOSSARY.md`). One to three
  scenarios per ticket; more means the slice is too thick, so split it.
- Describe what the system does, never how: no endpoints, tables, class names, or UI
  controls. Library crates are the exception; their public API signatures are domain
  language and may appear.
- Use concrete examples with real values. If you cannot write a concrete example, the
  problem is not understood yet; ask.
- Cover each failure path the behaviour handles as its own scenario (invalid input,
  missing resource, unauthorized caller, timeout from a dependency).
- The agent may draft scenarios. They become the specification only once the user marks
  them approved.

Example:

```
Given a patient with no appointments
When they book the 10:00 slot with Dr. Tremblay on 2026-12-01
Then the slot shows as taken for other patients
And the patient sees the appointment in their schedule
```

## Template

```markdown
# <Title in domain language>

## Story
As a <role>, I want <capability>, so that <outcome>.

## Scenarios
Status: draft | approved

### <Scenario name>
Given ...
When ...
Then ...

## Out of scope
- <Behaviour this ticket does not deliver>

## Observability
- Spans: <span names and key attributes the workflow must emit>
- Logs: <info events that mark completed steps; error events with their attributes>

## Security notes
<Trust boundaries touched. If any, the STRIDE table from docs/workflow/threat-model.md.>

## Notes
<Optional design hints. Non-binding: a better design that passes the scenarios wins.>

## Done when
- Every approved scenario passes as an acceptance test.
- `make <lang>-verify PROJECT=<project>` is green.
- Mutation run on the diff reviewed (business logic only).
```

## Existing plans

Stories in a project's `docs/PLAN.md` convert to this format when work on them starts,
not before.
