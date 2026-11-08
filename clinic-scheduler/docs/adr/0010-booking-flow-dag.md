# 0010. Booking flow as a versioned directed acyclic graph

Status: accepted
Date: 2026-11-08

## Context

The booking screens (slot selection, personal questionnaire, health questionnaire, test
selection, confirmation) must change order per clinic without code changes, and some
paths branch on answers. Every clinic starts from the same default order. A booking in
progress must not break when an admin changes the flow.

## Decision

- **`internal/dag`**: a pure project library with no I/O: nodes, edges, validation (no
  cycles, one start, every node reachable, every declared dependency satisfied), and
  `Next(state)` that follows branch predicates. Tested as a black box.
- **`booking/internal/flow`**: the domain package that uses it. Each step declares in
  code the steps it depends on (test selection needs the health questionnaire). An admin
  arranges steps per clinic; publishing creates an immutable flow version. Every clinic
  starts with the default version.
- **Pinning**: a booking session records its flow version and keeps it until it ends.
- **Session state**: NATS KV holds the session (current step, hold reference, flow
  version) with a TTL tied to the hold. Questionnaire answers go straight to `patient`
  as an encrypted draft, attached to the appointment on confirmation and destroyed if the
  hold expires.
- **Events**: `ops.booking.flow.<step>.started`, `.completed`, `.skipped` (ADR 0008).
- **Editor**: an HTMX list where an admin moves steps up and down and toggles optional
  ones; validation errors from `internal/dag` show inline.

## Alternatives

- **A fixed ordered list**: no branching on answers.
- **A workflow engine (Temporal)**: durable execution is more than a ten-minute screen
  flow needs.
- **A drag-and-drop canvas editor**: more front-end code for no new behaviour.

## Consequences

- New step types still need code; only order and optional steps are configurable.
- Old flow versions stay readable while any session references them.
