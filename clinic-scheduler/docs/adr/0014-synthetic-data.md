# 0014. Synthetic data from a seeder, load from k6

Status: accepted
Date: 2026-11-08

## Context

The system runs on synthetic data only (no production data in development, TGV S12.07).
Development needs clinics, schedules, patients, and many bookings created on demand.
Load and race behaviour (many patients wanting one slot) must also be exercised. k6 was
proposed for both.

## Decision

Two tools for two jobs:

- **`seeder`** (tool tier service): creates clinics, rooms, schedules, staff, doctors,
  patients, and bookings by calling the internal ConnectRPC APIs, never by inserting
  rows, so events, projections, and audit records stay consistent. Data comes from
  [gofakeit](https://github.com/brianvoe/gofakeit) with a fixed seed, so a run is
  repeatable. Triggered from a Tilt button with a size preset (small, medium, large).
  Workforce and patient accounts are created through the Keycloak admin API.
- **k6** (per `docs/standards/k6.md`): drives `patient-web` through the gateway like a
  browser (sign-in, CSRF token, HTMX requests) for smoke, load, and stress tests,
  including the race for one slot.

## Alternatives

- **k6 for seeding**: logging in through the portal and filling forms per booking is slow
  for bulk data and couples seeding to the UI.
- **SQL fixtures**: bypass events and projections, and break with every migration.

## Consequences

- The seeder needs a service account with a dedicated role, never exposed outside the
  cluster.
- k6 results feed the performance document (TGV PF01 to PF07).
