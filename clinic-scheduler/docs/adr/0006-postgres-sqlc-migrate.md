# 0006. Postgres with sqlc, pgx, and golang-migrate

Status: accepted
Date: 2026-11-08

## Context

Queries are never hand-built strings. sqlc and Masterminds squirrel were proposed
together. Every query and command execution needs logs, metrics, and traces. Postgres
runs on CloudNativePG (CNPG) in KinD; a CockroachDB port is planned later as a separate
effort.

## Decision

- Each domain service owns one CNPG database; no service reads another's tables.
- Queries are written in `.sql` files and compiled by [sqlc](https://sqlc.dev) into typed
  Go on [pgx v5](https://github.com/jackc/pgx). Optional filters use `sqlc.narg()`.
- Tracing and metrics come from [otelpgx](https://github.com/exaring/otelpgx) on the
  pgx pool.
- Schema changes are [golang-migrate](https://github.com/golang-migrate/migrate) files;
  sqlc reads the same files as its schema.
- Postgres-only features are allowed. Each one is recorded in the inventory below in
  the change that adds it.
- squirrel is not added. If a query appears that sqlc cannot express (for example a
  user-chosen sort column), a new ADR adds squirrel for that query, with identifiers from
  an allowlist.

## CockroachDB inventory

Postgres-only features in use, to replace during the CockroachDB port:

| Feature | Where | Why |
|---|---|---|
| Exclusion constraints (`btree_gist`) | `booking` | Room and staff overlap guard (ADR 0009) |
| `LISTEN/NOTIFY` | every outbox relay | Wake the relay without waiting for a poll (ADR 0007) |

## Alternatives

- **sqlc plus squirrel**: squirrel is in maintenance mode and its queries escape sqlc's
  schema checking. Most dynamic filters fit `sqlc.narg()`.
- **An ORM (GORM, ent)**: hides SQL, harder to tune and to review.
- **goose or Atlas** for migrations: both fine; golang-migrate was preferred.

## Consequences

- Schema drift fails `sqlc generate` in `make go-verify`.
- Integration tests create a database per test from the migrated template, as
  `go.md` describes.
