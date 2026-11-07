# Rust package structure

The same pragmatic Domain-Driven Design principles as `go-package-structure.md`, mapped to
Cargo: a **crate is a bounded context** (a self-contained area of the domain with its own
vocabulary), and a **module is an aggregate** (the entity that guards a set of rules,
together with its ports and adapters).

## Principles

- **Common Closure.** A module holds its aggregate, its ports, its business logic, and its
  adapters. A business-rule change usually stays inside one module.
- **Dependencies flow inward**: binary crates → domain library crates → shared crates
  (wire types, telemetry). Domain crates never depend on each other unless one genuinely
  contains the other's aggregate. Cargo enforces crate boundaries at compile time.
- **The module is the boundary.** No `domain/`, `application/`, or `infrastructure/`
  wrapper modules.
- **Flat by default.** One level of modules under the crate root; split a module that
  grows by sub-domain, never by layer.
- **Visibility is the lock.** Items are private or `pub(crate)` by default. Only the
  crate root re-exports the public API.

## Project layout

One Cargo workspace per project.

```
online-pong/
├── Cargo.toml                         # [workspace], shared lints and dependency versions
├── Cargo.lock                         # committed
├── GLOSSARY.md                        # ubiquitous language
├── docs/{PLAN.md,adr/}
├── crates/
│   ├── chip8-core/                    # library crate: one bounded context
│   │   ├── src/
│   │   │   ├── lib.rs                 # //! crate docs, public re-exports
│   │   │   ├── error.rs
│   │   │   ├── opcodes.rs
│   │   │   └── display.rs
│   │   └── tests/                     # hermetic black-box tests only
│   ├── multiplayer-server/            # binary crate: a deployable service
│   │   ├── Dockerfile
│   │   └── src/
│   │       ├── main.rs                # composition root: config, telemetry, wiring
│   │       ├── config.rs              # typed environment configuration
│   │       ├── matchmaking.rs         # aggregate root + ports (traits)
│   │       ├── matchmaking/
│   │       │   ├── queue.rs           # business logic
│   │       │   └── repository.rs      # PostgreSQL adapter for the port
│   │       ├── scoreboard.rs
│   │       ├── scoreboard/repository.rs
│   │       ├── ingress.rs             # router construction
│   │       └── ingress/
│   │           ├── register_player_http.rs   # one file per workflow
│   │           └── report_match_http.rs
│   ├── proto/                         # shared wire types, no business logic
│   └── system-tests/                  # Tilt-backed tests, publish = false
│       └── tests/
│           ├── integration_*.rs
│           └── acceptance_*.rs
├── test/k6/                           # see k6.md
└── deployments/
    └── multiplayer-server/{base,overlays/local}/
```

## File roles

| File                                  | Role                                                    |
|---------------------------------------|---------------------------------------------------------|
| `<aggregate>.rs`                      | Aggregate root type and its port traits                 |
| `<aggregate>/<capability>.rs`         | Business logic, named for what it does (`queue.rs`)     |
| `<aggregate>/repository.rs`           | Storage adapter implementing a port trait               |
| `<integration>/client.rs`             | Adapter for an external system                          |
| `<aggregate>/mapper.rs`               | Conversion between domain types and wire or row types   |
| `ingress/<workflow>_http.rs`          | HTTP entry point                                        |
| `ingress/<workflow>_consumer.rs`      | Message entry point                                     |
| `main.rs`                             | Composition root: the only place every module is wired  |

## Dependency rules

- Domain modules use port traits for anything outside the process; adapters implement
  them.
- `ingress` modules compose domain modules; domain modules never import `ingress`.
- A library crate that must stay deterministic or I/O-free (for example `chip8-core`)
  states that invariant in its project `AGENTS.md`.
- Shared wire types (`proto`) hold data and serde derives only.
