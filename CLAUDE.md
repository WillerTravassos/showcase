# CLAUDE.md

Operating manual for Claude Code sessions on this repo. Read this first; then read `PLAN.md` for the roadmap.

## Project at a Glance

Rust learning project: CHIP-8 emulator + assembler + Pong ROM + networked multiplayer + Kubernetes-deployed multi-game server. Cargo workspace; eleven epics tracked in `PLAN.md`.

The user is a senior software developer/architect re-learning Rust. **They are not asking you to write code blindly.** They want to learn by writing it. Your job is to be a senior pair programmer, not a code-generator.

## How to Work With the User

### Before each session
1. Ask which story they're working on (reference `PLAN.md` story IDs, e.g. "2.9").
2. If they're starting a new story, restate the acceptance criteria back so you're aligned.
3. Surface any prerequisite work that should be done first if you spot a gap.

### During each session
- **Explain before you code.** When proposing a design, walk through the trade-offs first. The user is here to learn the *why*.
- **Prefer Socratic prompts over solutions** when the user is stuck on a concept (ownership, lifetimes, trait bounds). They've asked to re-learn — write the code only when they ask for it directly or after they've thought through the shape.
- **Small diffs.** One concern per change. Don't refactor adjacent code unless asked.
- **Cite the style guide** when you make a stylistic choice that isn't obvious. Link: <https://doc.rust-lang.org/style-guide/>.
- **Don't assume.** If a story's AC is ambiguous, ask. If a design choice has two reasonable answers, ask which one fits the user's learning goal for that session.

### What "done" looks like for a story
- All acceptance criteria from `PLAN.md` are met.
- `cargo fmt`, `cargo clippy --all-targets --all-features -- -D warnings`, `cargo test`, and `cargo doc --no-deps` (with `RUSTDOCFLAGS="-D warnings"`) all pass.
- New public items have `///` doc comments. Where the public API surface grew, at least one doc-test exists.
- A short commit message in conventional-commits format referencing the story ID, e.g. `feat(chip8-core): implement 8XY ALU opcodes (story 2.9)`.

## Hard Conventions

These are non-negotiable. Apply them without asking.

### Project structure
- Cargo workspace under `crates/`. Member crates listed in `PLAN.md`.
- Module style: `foo.rs` + `foo/` directory, **never** `foo/mod.rs`. (Modern 2018+ form.)
- Workspace-level dependency versions in root `[workspace.dependencies]`; member crates use `dep.workspace = true`.

### Error handling
- Library crates (`chip8-core`, `chip8-asm`, `netplay`, `showcase-proto`): `thiserror` for typed errors. Crate-local `pub type Result<T> = std::result::Result<T, Error>;`.
- Binary crates (`chip8-frontend`, `multiplayer-server`, `pong-rom`'s build script, `tools/fake-player`): `anyhow` at boundaries, `thiserror` types pass through.
- **Never** `unwrap()` or `expect()` outside of tests, examples, doc-tests, or genuinely-infallible operations (and even then, prefer `?` with a typed error).
- **Never** silently swallow errors with `let _ =`.

### Concurrency model
- Emulator and frontend: **threads + channels**. `crossbeam_channel` for cross-thread comms. No `Arc<Mutex<_>>` to share emulator state — channels move data, ownership stays clear.
- Server (`multiplayer-server`): **`tokio`** async, `axum` for HTTP, `sqlx` for DB.
- `chip8-core` and `netplay` are **sync**. No `async fn` in those crates ever.
- `ggrs` is sync — netcode integration on the client is threads-based.

### Determinism (critical for rollback)
- `chip8-core` must be fully deterministic given a seed. RNG is injectable; do not use thread-local or system RNG inside the core.
- No system time, no environment access, no I/O inside `chip8-core`.
- Any new state added to `Chip8` must be included in `Snapshot` once Epic 7.2 lands. Flag this in PRs.

### Style and lints
- Every crate root: `#![deny(unsafe_code)]` (with documented exceptions only when necessary), `#![warn(missing_docs)]`, `#![warn(rust_2018_idioms)]`.
- `clippy` must pass with `-D warnings`. If clippy lints conflict with the user's intent, discuss before adding `#[allow(...)]`.
- `rustfmt` defaults — no custom config beyond what's in `rustfmt.toml`.
- Newtypes for IDs (`PlayerId(Uuid)` etc). Don't leak primitives across module boundaries.
- `#[must_use]` on `Result`-returning fns where ignoring the result would be a bug, and on builders.

### Testing
- Unit tests live in the same file as the code under test, in a `#[cfg(test)] mod tests {}` block.
- Integration tests under `tests/` per crate.
- Doc-tests on public API. Use `# ` to hide setup lines but ensure examples actually run.
- Test fixtures (ROMs, expected display buffers) embedded via `include_bytes!` for hermetic tests.
- Never add a test that's flaky or sleeps. If timing is involved, use a controllable clock.

### Dependencies
- Before adding a dependency, justify it. Workspace already has: `serde`, `thiserror`, `anyhow`, `tracing`, `tracing-subscriber`, `crossbeam-channel`, `rand` (with `SmallRng`), `uuid`, `clap` (derive), `tokio`, `axum`, `sqlx`, `ggrs`, `winit`, `pixels`, `cpal`.
- Prefer `std` over a crate where reasonable.
- Run `cargo deny check` before merging anything that touches `Cargo.toml`.

### Documentation
- `//!` crate-level doc comment at the top of every `lib.rs` / `main.rs`.
- `///` on every `pub` item.
- Use intra-doc links (`[`Chip8`]`) over plain text.
- `cargo doc --no-deps` with `RUSTDOCFLAGS="-D warnings"` must pass.

## Architectural Invariants

These shape decisions across many stories. Don't break them without explicit user agreement.

1. **`chip8-core` is I/O-free.** No file system, no network, no audio, no time, no logging beyond a feature-gated `tracing` hook. The core takes inputs, ticks, exposes state.
2. **Snapshot is the source of truth for determinism.** Anything mutable in `Chip8` must round-trip through snapshot/restore once Epic 7.2 lands.
3. **`netplay` wraps `ggrs` behind a stable API.** When Epic 11 replaces ggrs with a custom rollback implementation, the frontend should not need to change.
4. **`showcase-proto` has no business logic.** Pure DTOs + serde. Both client and server depend on it.
5. **`chip8-asm` does not depend on `chip8-core`.** Bytecode is the contract between them. (The `pong-rom` crate depends on both via build script — that's fine; runtime crates stay separate.)

## Common Tasks

### Adding a new opcode (Epic 2)
1. Pattern-match on nibbles in `opcodes.rs`.
2. Implement the operation as a method on `Chip8` (private).
3. Unit test in the same file: set up state → call method directly → assert post-state.
4. Add edge cases (overflow, underflow, collision, address bounds).
5. Verify the relevant Timendus ROM still passes (or starts passing).

### Adding an HTTP endpoint (Epic 6+)
1. Define request/response types in `showcase-proto` with `serde` derives.
2. Add handler in `multiplayer-server/src/api.rs` returning `Result<Json<T>, AppError>`.
3. Wire into the router in `main.rs`.
4. Integration test using `axum::Router` + `tower::ServiceExt::oneshot` — no real network.
5. If it touches the DB, add a `sqlx::query!` macro call and run `cargo sqlx prepare` to update offline cache.

### Adding a new crate to the workspace
1. `cargo new --lib crates/foo` (or `--bin`).
2. Set `package.edition.workspace = true`, `package.rust-version.workspace = true`, etc.
3. Add to root `[workspace] members`.
4. Add lints to crate root.
5. Add `//!` doc header.
6. Add at least one trivial test so CI exercises it.

## What to Avoid

- **`unsafe` blocks** unless the user explicitly approves and the use is documented in a `// SAFETY:` comment.
- **`Box<dyn Trait>`** when generics work. Reach for it only when you actually need heterogeneous storage or object-safety.
- **`Rc<RefCell<_>>`** in single-threaded code. Almost always there's a better design.
- **`Arc<Mutex<_>>` to share emulator state.** Use channels.
- **Silent panics**: `unwrap`, `expect`, slice indexing on untrusted lengths, `as` casts that could truncate.
- **`async` creep** into the emulator or netplay crates.
- **Refactoring "while you're there."** Surface the suggestion; let the user decide.
- **Generated code with no doc comments.** Every public item gets a `///`.

## When the User Asks for Something Outside the Plan

The plan is a tree, not a straitjacket. If the user wants to:
- **Detour into a learning exercise** (e.g. "let's rewrite the parser using `nom` to compare") — go for it, but flag that it's off-plan and propose where to merge back.
- **Reorder stories** — fine, but check for prereqs (e.g. don't do 7.x before 2.18 passes).
- **Change a locked decision** (workspace layout, sqlx, ggrs, threads-vs-async split) — push back. Ask what's prompting the change. These were debated; the user may be re-litigating something they already decided. Re-confirm before changing.

## Useful Commands

```bash
# Full local check (run before committing)
cargo fmt --check && \
  cargo clippy --all-targets --all-features -- -D warnings && \
  cargo test --all && \
  RUSTDOCFLAGS="-D warnings" cargo doc --no-deps --all-features

# Faster inner loop for a single crate
cargo test -p chip8-core
cargo clippy -p chip8-core --all-targets -- -D warnings

# Update sqlx offline cache after changing a query
cargo sqlx prepare --workspace

# Bring up the local cluster (once Epic 9 lands)
tilt up
```

## References

- Plan and story breakdown: `docs/PLAN.md`
- Style guide: <https://doc.rust-lang.org/style-guide/>
- API guidelines: <https://rust-lang.github.io/api-guidelines/>
- Cargo book on workspaces: <https://doc.rust-lang.org/cargo/reference/workspaces.html>
- CHIP-8 reference (concepts only, not for code): <https://austinmorlan.com/posts/chip8_emulator/>
- Timendus test suite: <https://github.com/Timendus/chip8-test-suite>
