# Online Pong

A Rust learning project: a CHIP-8 emulator, an assembler, a Pong ROM, rollback
netplay, and a multi-game Kubernetes-deployed server. Demonstrates idiomatic
Rust across the stack — emulator core, parsing, GUI, async networking, and
cloud-native deployment.

## Workspace Layout

```
crates/
├── chip8-core/          # Pure CHIP-8 emulator logic — I/O-free, deterministic
├── chip8-asm/           # Assembler: .s8 source → .ch8 bytecode
├── chip8-frontend/      # winit + pixels desktop app
├── pong-rom/            # Pong ROM source, assembled at build time via build.rs
├── netplay/             # ggrs rollback netplay integration
├── multiplayer-server/  # tokio + axum multi-game server
└── proto/               # Shared serde wire types (no business logic)
```

## Build & Test

```bash
# Full check — run before committing
cargo fmt --check && \
  cargo clippy --all-targets --all-features -- -D warnings && \
  cargo test --all && \
  RUSTDOCFLAGS="-D warnings" cargo doc --no-deps --all-features

# Single-crate inner loop
cargo test -p chip8-core
cargo clippy -p chip8-core --all-targets -- -D warnings

# Update sqlx offline cache after changing a query
cargo sqlx prepare --workspace
```

## Architecture Decisions

Key decisions are locked in [`docs/PLAN.md`](../docs/PLAN.md). Highlights:

| Area | Decision |
|---|---|
| GUI | `winit` + `pixels` |
| Netcode | Emulator-level snapshot-based rollback |
| Rollback library | `ggrs` (replaced by custom implementation in Epic 11) |
| Persistence | `sqlx` + PostgreSQL (CNPG in Kubernetes) |
| Concurrency | Threads + channels for emulator/client; `tokio` for server |
| Deployment | Kustomize + Tilt locally; Argo CD + Kargo for promotion |

Session conventions and hard constraints: [`CLAUDE.md`](../CLAUDE.md).
