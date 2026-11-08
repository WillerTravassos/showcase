# 0009. Slot holds in NATS KV, guaranteed by Postgres

Status: accepted
Date: 2026-11-08

## Context

When a patient selects a slot, they have ten minutes to confirm. Two patients must never
get the same room or staff member in the same block, and an abandoned hold must release
itself. A slot has a capacity (free rooms and free staff), and the patient never picks
the room or staff member.

## Decision

- **Supply**: `booking` keeps a projection of `scheduling` events (opening hours,
  holidays, closures, shifts, breaks, leave). A slot exists at (clinic, block) when the
  clinic is open, no holiday or closure applies, and at least one room and one staff
  member on shift at that clinic are free. Capacity is the smaller of the two counts.
- **Hold**: on selection, `booking` picks a free room and staff member and creates two
  NATS KV keys, `hold.room.<room>.<block>` and `hold.staff.<staff>.<block>`, with a
  ten-minute TTL. KV create is atomic: if a key exists, the next candidate is tried, and
  when none is left the slot is full. The TTL releases an abandoned hold.
- **Confirmation**: `booking` appends `appointment.confirmed` and inserts the appointment
  row in one transaction. Postgres exclusion constraints on (room, time range) and
  (staff, time range) are the final guarantee if KV ever loses state. The hold keys are
  then deleted.
- **Countdown**: the server sends the hold's expiry time; the page counts down. Any
  request after expiry returns the patient to the start of the booking flow.

## Alternatives

- **Hold rows in Postgres with an expiry column**: exclusion constraints cannot use
  `now()`, so expired rows need a sweeper before they free the slot.
- **Advisory locks**: tied to a connection, not to a ten-minute browser session.

## Consequences

- Needs NATS with per-key TTL on KV (check the minimum server version in ticket 0).
- KV state is lost on a NATS wipe; the only effect is that in-flight holds release
  early, and confirmation still cannot double-book.
- Metrics: `clinic.hold.created`, `clinic.hold.expired`, `clinic.appointment.booked`.
