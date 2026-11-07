# Go package structure

A pragmatic Domain-Driven Design layout: packages group what changes together, and the Go
package itself is the boundary. Based on the
[Golang Standard Project Layout](https://github.com/golang-standards/project-layout)
(a community convention, not an official Go standard), with the deviations listed at the
end.

## Principles

- **Common Closure.** A domain package holds its aggregate (the entity that guards the
  domain's rules), its ports (interfaces to the outside), its business logic, and its
  adapters. A business-rule change usually touches all of them in one pull request.
- **Dependencies flow inward**: `ingress` → domain and integration packages → shared
  libraries. Domain packages never import each other; workflows that span domains are
  composed in `ingress`.
- **The package is the boundary.** No `domain/`, `application/`, or `infrastructure/`
  wrapper folders.
- **Flat by default.** Nothing deeper than `internal/<package>/`. When a package grows past
  a handful of files, split it by sub-domain, never by layer.

## Project layout

One Go module per project. Each service is a folder under `services/`; its `internal/`
is private to that service.

```
clinic-scheduler/
├── go.mod                              # one module for the whole project
├── .mockery.yml                        # fake generation config
├── .go-arch-lint.yml                   # dependency rules
├── GLOSSARY.md                         # ubiquitous language (shared domain vocabulary)
├── docs/adr/                           # architecture decision records
├── internal/                           # project-wide shared code, private to the project
│   ├── logctx/                         # context log attributes (see observability.md)
│   ├── telemetry/                      # OTel and slog bootstrap
│   ├── tempotest/                      # Tempo trace assertions for acceptance tests
│   └── testdb/                         # per-test database from a template
├── services/
│   └── booking/
│       ├── Dockerfile
│       ├── cmd/main.go                 # entry point, calls service.Run
│       ├── api/                        # OpenAPI spec and generated code
│       ├── web/                        # templates, static assets, web.go with //go:embed
│       ├── internal/
│       │   ├── config/config.go        # typed environment configuration
│       │   ├── appointment/            # domain package
│       │   │   ├── appointment.go      # aggregate root + ports
│       │   │   ├── booking.go          # business logic
│       │   │   └── repository.go       # PostgreSQL adapter for the port
│       │   ├── patient/                # domain package
│       │   ├── notifier/               # integration package
│       │   │   ├── notifier.go         # port + request/response models
│       │   │   └── client.go           # adapter for the external system
│       │   ├── ingress/                # entry points, one file per workflow
│       │   │   ├── handler.go          # HTTP handler struct and constructor
│       │   │   ├── book_appointment_http.go
│       │   │   └── appointment_reminder_consumer.go
│       │   └── service/service.go      # composition root: wiring and lifecycle
│       └── test/integration/           # needs Tilt infrastructure
├── test/
│   ├── acceptance/                     # cross-service scenarios (see atdd.md)
│   │   ├── dsl/
│   │   └── driver/
│   └── k6/                             # see k6.md
└── deployments/
    └── booking/{base,overlays/local}/  # kustomize manifests
```

## Service tiers

1. **Domain service**: business rules, persistence, one or more aggregates. Uses domain
   packages, integration packages, `ingress`, and `service`.
2. **Gateway**: protocol bridging and message translation without rich domain logic.
   Integration packages, `ingress`, and `service`; no domain packages.
3. **Tool**: developer utilities and proxies. One core package plus `ingress` and
   `service`.

## File roles

| File                         | Role                                                         |
|------------------------------|--------------------------------------------------------------|
| `<package>.go`               | Aggregate root and ports (domain) or port and models (integration) |
| `<business_capability>.go`   | Business logic, named for what it does (`booking.go`, never `service.go`) |
| `repository.go`              | Storage adapter implementing a port                          |
| `client.go`                  | Adapter for an external system                               |
| `mapper.go`                  | Conversion between domain models and external representations |
| `<workflow>_http.go`         | HTTP entry point, in `ingress/`                              |
| `<workflow>_consumer.go`     | Message entry point (Kafka, NATS), in `ingress/`             |

Prefix a role file by capability when a package has more than one
(`appointment_repository.go`, `reminder_client.go`).

## Dependency rules

- Domain packages import only the standard library, shared project packages
  (`<project>/internal/...`), and third-party libraries.
- Integration packages never import domain packages.
- `ingress` imports domain and integration packages and composes them.
- `service` is the only package that imports everything; it wires and starts the service.
- `make go-arch PROJECT=<project>` enforces these rules from `.go-arch-lint.yml`
  ([go-arch-lint](https://github.com/fe3dback/go-arch-lint)). Every new package is added
  to that file in the same change.

## Embedded web assets

`//go:embed` cannot reach parent directories, so the embedding file lives in `web/` itself
(`web/web.go`, package `web`, exporting an `fs.FS`). `ingress` imports `web` to serve it.
TypeScript sources compile into `web/static/` (see `typescript.md`).

## Deviations from the community layout

- `internal/<package>/` is flat. The layout's `internal/app/` and `internal/pkg/` levels
  are not used.
- Kubernetes manifests live in `<project>/deployments/<service>/`; Dockerfiles live in the
  service folder rather than `build/package/`.
- Project-wide shared code lives in `<project>/internal/` rather than `pkg/`, keeping it
  private to the project.
