# 0007. Transactional outbox with a Go relay

Status: accepted
Date: 2026-11-08

## Context

Every state change must reach NATS JetStream exactly as committed: never an event for a
rolled-back change, never a lost event for a committed one. Debezium (CDC, change data
capture: reading the Postgres write-ahead log) was proposed. The requirement is the most
failure-proof option that does not add latency.

## Decision

Each service writes its events to an `outbox` table in the same transaction as the
change (for event-sourced services, the event table is the outbox). A relay in the
project package `internal/outbox` publishes them:

- Wakes on `LISTEN/NOTIFY` and also polls every second as a fallback.
- Reads unpublished rows in commit order, publishes each to JetStream with
  `Nats-Msg-Id` set to the event ID (JetStream drops duplicates inside its dedup window),
  waits for the publish acknowledgement, then marks the row published.
- One relay is active per service: replicas compete for a lease key in NATS KV (create
  with a TTL, renew by compare-and-set). This keeps per-aggregate order.
- Consumers still check the aggregate version and ignore events they have already
  applied, as a second safety net.
- Metrics: `clinic.outbox.lag` (age of the oldest unpublished row, `ms`) and published
  and failed counts. Spans link the publish to the trace that wrote the row.

## Alternatives

| Risk | Debezium | Go relay |
|---|---|---|
| Relay down for hours | Replication slot retains WAL until the disk fills and the database stops | Rows wait in the table; the database is unaffected |
| CNPG failover | Slot can be lost unless failover slots are configured | Nothing to lose |
| Moving parts | JVM pod, offsets store, replication slot | One goroutine per service |
| Latency | Milliseconds | Milliseconds with `LISTEN/NOTIFY` |
| CockroachDB | Not supported | Same code (poll only) |

- **Publish directly after commit**: loses events when the process dies between commit
  and publish.

## Consequences

- At-least-once delivery, made effectively once by JetStream dedup and consumer version
  checks.
- Published rows are pruned after a retention window.
- `LISTEN/NOTIFY` is in the CockroachDB inventory (ADR 0006); without it the relay polls.
