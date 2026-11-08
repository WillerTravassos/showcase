# 0008. NATS JetStream subjects, envelope, and streams

Status: accepted
Date: 2026-11-08

## Context

Services exchange domain events (facts about the system: an appointment was confirmed)
and operational events (execution of commands and booking flow steps). Admin portal
reads must also produce audit events. Streams, consumers, and dead letter queues (DLQs,
where messages go after repeated failures) are declared through NACK, the NATS
Kubernetes controller.

## Decision

**Subjects**

| Kind | Pattern | Example |
|---|---|---|
| Domain | `domain.<service>.<aggregate>.<event>` | `domain.booking.appointment.confirmed` |
| Operational | `ops.<service>.<thing>.<event>` | `ops.booking.hold.expired` |
| Booking flow step | `ops.booking.flow.<step>.<event>` | `ops.booking.flow.health_questionnaire.completed` |
| Audit | `audit.<portal>.<action>` | `audit.admin.viewed` |

Steps are a fixed set declared in code, so step names in subjects stay low in number,
and consumers can filter per step (`ops.booking.flow.test_selection.>`).

**Envelope**: JSON payloads defined by Go types, with CloudEvents attributes in NATS
headers: `ce-id` (also `Nats-Msg-Id`), `ce-type` (`appointment.confirmed.v1`),
`ce-source` (service), `ce-time`, plus `traceparent` for trace propagation. Payloads
carry the aggregate ID and version.

**No personal information** in any payload: identifiers only.

**Streams**: `DOMAIN`, `OPS`, and `AUDIT`, each with a `<NAME>_DLQ` stream. A consumer
moves a message to the DLQ after its maximum delivery count, with the failure reason in
a header. All streams and consumers are NACK manifests in `deployments/`.

**Package**: `internal/nats` owns connections (reconnect, drain on shutdown), publish
with dedup IDs, consumers with DLQ routing, and logs, metrics, and traces for each.

## Alternatives

- **Protobuf payloads**: one schema language with Connect, but messages are easier to
  inspect as JSON, and Go types are enough inside one module.
- **A single stream**: simpler, but audit retention and operational volume differ from
  domain events.
- **Step name in the payload only** (`ops.booking.flow.step_completed`): no per-step
  filtering.

## Consequences

- Event type names are versioned; consumers ignore types they do not know.
- Stream retention is set per stream (audit longest, operational shortest).
