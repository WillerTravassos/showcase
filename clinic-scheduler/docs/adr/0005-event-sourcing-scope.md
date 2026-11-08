# 0005. Event sourcing for booking and scheduling only

Status: accepted
Date: 2026-11-08

## Context

Appointments and schedules must be rebuildable from their history and act as an audit
trail of every change. Full event sourcing everywhere would add projections, rebuilds,
and event upcasting (converting old event versions) to services that do not need them.
Personal information must stay erasable (Law 25, PIPEDA), and events are never deleted.

## Decision

- **`booking` and `scheduling`**: aggregates are event-sourced. The event table is the
  source of truth; the current state of an aggregate is the fold (replay) of its events.
  Reads go through projection tables, rebuilt from events when their code changes.
  Appends use optimistic concurrency: an event carries the aggregate version it expects,
  and a unique `(aggregate_id, version)` constraint rejects a concurrent write.
- **Other services**: state tables are the source of truth, and each change writes an
  outbox event row in the same transaction.
- **No personal information in events**: events carry patient IDs only. Names,
  contact details, and answers live in `patient` (see ADR 0015).

## Alternatives

- **Event sourcing everywhere**: more uniform, much more machinery for profile and
  notification data, and it conflicts with erasure.
- **State tables plus audit log everywhere**: simpler, but no rebuild from history for
  the two domains where it matters most.

## Consequences

- Event types are versioned (`appointment.confirmed.v1`); a breaking change adds a new
  version and an upcaster.
- Projection lag is visible to the user only through the booking projection of
  scheduling events; the slot exclusion constraints still guard correctness (ADR 0009).
- Snapshots are deferred until a fold measurably slows a request.
