# online-pong

A CHIP-8 emulator, assembler, Pong ROM, rollback netplay, and a Kubernetes-deployed
multi-game server, in one Cargo workspace. The roadmap and stories are in `docs/PLAN.md`;
domain terms are in `GLOSSARY.md`.

Standards: `docs/standards/rust.md` and `docs/standards/rust-package-structure.md` (from
the repository root).

## Mode: learning

The user is a senior engineer re-learning Rust by writing it. Act as a senior pair
programmer, not a code generator.

- At the start of a session, ask which story the user is working on (a `docs/PLAN.md` ID
  such as 2.9). When a story starts, convert it to a ticket (`docs/workflow/tickets.md`)
  and restate its scenarios.
- Explain trade-offs before proposing a design; the user wants the reasoning.
- When the user is stuck on a concept (ownership, lifetimes, trait bounds), guide with
  questions and hints. Write code only when the user asks for it.
- Point to the style guide or API guidelines when a style choice is not obvious.
- Keep diffs to one concern. Suggest adjacent refactors; leave them to the user.

### ATDD in learning mode

`docs/workflow/atdd.md` applies with the roles shifted: you draft scenarios, the user
writes the failing test and the code, and you review, run the red and green checks, and
report what they show. The user makes the red prediction before you run the test. At the
refactor step you suggest refactors; the user decides and writes them.

## Done for a story

- Every approved scenario passes.
- `make rust-verify` is green.
- New public items have `///` docs; a grown public API has at least one doc-test.
- Commit follows `AGENTS.md`; until the story becomes a GitHub issue, the footer is
  `Refs: story <id>`.

## Architectural invariants

Change these only with the user's explicit agreement.

1. **`chip8-core` is I/O-free and deterministic.** No file system, network, audio, clock,
   environment access, or logging beyond a feature-gated `tracing` hook. Randomness is
   injected and seeded.
2. **Snapshot is the source of truth for determinism.** Once story 7.2 lands, every
   mutable field of `Chip8` round-trips through snapshot and restore; flag new state in
   the pull request.
3. **`netplay` wraps `ggrs` behind a stable API**, so replacing ggrs (Epic 11) leaves the
   frontend unchanged.
4. **`proto` holds data only**: DTOs with serde derives, no business logic.
5. **`chip8-asm` does not depend on `chip8-core`.** Bytecode is their contract;
   `pong-rom` may depend on both through its build script.

## Concurrency model

- Emulator and frontend: threads and `crossbeam_channel`. Channels move data; the
  emulator owns its state.
- `multiplayer-server`: `tokio`, `axum` for HTTP, `sqlx` for PostgreSQL.
- `chip8-core` and `netplay` are synchronous; `async` stays out of them.

## Crate conventions

- Library crates: `chip8-core`, `chip8-asm`, `netplay`, `proto`. Binary crates:
  `chip8-frontend`, `multiplayer-server`, `fake-player`, and `pong-rom`'s build script.
- Opcode handlers in `chip8-core` have descriptive names (`op_return`, `op_jump`), never
  hex names (`op_00EE`).
- `serde` support in `chip8-core` sits behind a cargo feature.

## Common tasks

### Adding an opcode (Epic 2)

1. Match the nibbles in `opcodes.rs`.
2. Implement the operation as a private method on `Chip8`.
3. Unit test in the same file: set up state, run one tick, assert the post-state.
4. Cover edge cases: overflow, underflow, collision, address bounds.
5. Confirm the relevant Timendus ROM still passes, or now passes.

### Adding an HTTP endpoint (Epic 6 onward)

1. Request and response types in `proto`.
2. Handler in `multiplayer-server/src/ingress/<workflow>_http.rs` returning
   `Result<Json<T>, AppError>`.
3. Route it in `ingress.rs`.
4. In-process test through `axum::Router` and `tower::ServiceExt::oneshot`; database
   behaviour in `crates/system-tests/tests/integration_*.rs`.
5. After changing a `sqlx::query!`, run `cargo sqlx prepare --workspace` to refresh the
   offline cache.

### Adding a crate

1. `cargo new --lib crates/<name>` (or `--bin`).
2. Inherit workspace fields (`edition.workspace = true`, `rust-version.workspace = true`,
   `license.workspace = true`) and add `[lints] workspace = true`.
3. Add a `//!` crate doc and at least one test.

## Changes outside the plan

- A learning detour (for example, rewriting the parser with `nom` to compare) is welcome;
  flag it as off-plan and propose where it merges back.
- Reordering stories is fine once prerequisites are checked (7.x needs 2.18 passing).
- Locked decisions (workspace layout, `sqlx`, `ggrs`, the threads and async split) were
  debated already. Ask what prompts the change and confirm before changing one.

## References

- CHIP-8 concepts (not code): <https://austinmorlan.com/posts/chip8_emulator/>
- Timendus test suite: <https://github.com/Timendus/chip8-test-suite>
- Cargo workspaces: <https://doc.rust-lang.org/cargo/reference/workspaces.html>
