# Showcase: CHIP-8 Emulator + Pong Project Plan

A Rust learning project: build a CHIP-8 emulator, an assembler, a Pong ROM, networked multiplayer, and a multi-game server. Final goal is a portfolio piece that demonstrates idiomatic Rust across the stack — emulator core, parsing, GUI, networking, async server, and Kubernetes-native deployment. Code must be enterprise level.

## Decisions Locked In

| Area | Decision |
|---|---|
| Emulator scope | Start CHIP-8, expand to SUPER-CHIP, then XO-CHIP (last) |
| GUI | `winit` + `pixels` |
| Assembler | Hand-rolled in Rust, expanded alongside emulator |
| Netcode | Emulator-level (authoritative emulator state, snapshot-based) |
| Rollback | `ggrs` first, custom implementation as final epic |
| Persistence | `sqlx` with PostgreSQL (CNPG in Kubernetes) |
| Concurrency | Threads + channels for emulator/client; `tokio` for server |
| Testing | Unit per opcode, integration via Timendus ROMs, e2e via fake-player workload |
| CI | GitHub Actions: fmt, clippy `-D warnings`, test, doc, deny |
| Deployment | Kustomize + Tilt locally; Argo CD + Kargo for promotion |
| Story sizing | ~1–3 hour sessions; `[L]` flag on longer ones |

## Workspace Layout

A Cargo workspace with multiple crates. `chip8-core` is deliberately I/O-free — this is what makes deterministic snapshotting (and therefore rollback) tractable later.

```
online-pong/
├── Cargo.toml                    # workspace manifest
├── Cargo.lock
├── rust-toolchain.toml           # pin stable, components: rustfmt, clippy
├── rustfmt.toml
├── clippy.toml
├── deny.toml                     # cargo-deny config
├── .github/workflows/ci.yml
├── README.md
├── CLAUDE.md                     # handoff doc for Claude Code sessions
├── PLAN.md                       # this file
├── crates/
│   ├── chip8-core/               # the emulator: pure logic, no I/O
│   │   ├── Cargo.toml
│   │   ├── src/
│   │   │   ├── lib.rs
│   │   │   ├── cpu.rs
│   │   │   ├── memory.rs
│   │   │   ├── display.rs
│   │   │   ├── keypad.rs
│   │   │   ├── timers.rs
│   │   │   ├── opcodes.rs
│   │   │   └── error.rs
│   │   └── tests/                # integration tests + Timendus ROMs
│   │       ├── opcodes.rs
│   │       └── roms/
│   ├── chip8-asm/                # assembler: text → bytecode
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── lexer.rs
│   │       ├── parser.rs
│   │       ├── codegen.rs
│   │       └── error.rs
│   ├── chip8-frontend/           # winit + pixels desktop app
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── main.rs
│   │       ├── app.rs
│   │       ├── render.rs
│   │       ├── input.rs
│   │       └── audio.rs
│   ├── pong-rom/                 # Pong source in your assembler's syntax
│   │   ├── Cargo.toml            # build.rs invokes chip8-asm
│   │   ├── build.rs
│   │   └── src/pong.s8
│   ├── netplay/                  # ggrs integration, session types
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs
│   │       ├── session.rs
│   │       ├── input.rs
│   │       └── snapshot.rs
│   ├── multiplayer-server/          # tokio + axum, multi-game orchestrator
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── main.rs
│   │       ├── api.rs
│   │       ├── matchmaking.rs
│   │       ├── session.rs
│   │       └── db.rs
│   └── proto/                      # shared wire types (serde)
│       ├── Cargo.toml
│       └── src/lib.rs
├── deploy/
│   ├── kustomize/
│   │   ├── base/
│   │   └── overlays/{local,prod}
│   ├── tilt/Tiltfile
│   └── argo/                     # added in Epic 9
└── tools/
    └── fake-player/              # k8s workload for e2e tests
```

**Note on `mod.rs` vs the modern style:** Current idiomatic Rust (2018+) prefers `foo.rs` + `foo/` directory over `foo/mod.rs`. Some older guides (including parts of Jeremy Chone's content) still show `mod.rs`; the official style guide and `rustfmt` defaults align with the modern form shown above.

## Epic Breakdown

| # | Epic | Outcome |
|---|---|---|
| 1 | Project Foundation | Workspace, CI, lints, docs scaffolding |
| 2 | CHIP-8 Core Emulator | Headless emulator passing Timendus tests |
| 3 | Desktop Frontend | winit + pixels app, you can play public-domain ROMs |
| 4 | CHIP-8 Assembler | Text → `.ch8` bytecode, round-trips Timendus |
| 5 | Pong ROM | Local 2-player Pong built with your assembler |
| 6 | Server + Scoreboard | axum + sqlx + CNPG, registered players, leaderboard |
| 7 | Online Multiplayer (ggrs) | Two clients play Pong over network with rollback |
| 8 | Emulator Expansion: SUPER-CHIP | Core + assembler support 128×64 |
| 9 | Multi-Game Server | N concurrent sessions, matchmaking, k8s deployment, Argo/Kargo |
| 10 | XO-CHIP | Final emulator/assembler expansion |
| 11 | Custom Rollback Netcode | Replace ggrs with your own implementation |

Epics 7 and 9 are split deliberately — get one networked game working end-to-end before tackling the multi-tenant server. Otherwise Kubernetes pod networking and rollback determinism end up being debugged in the same session, which kills the fun.

## Stories

Format: each story has acceptance criteria (testable, not vibes) and a "Rust notes" block when there's something specific to learn or notice. Stories are ~1–3 hours unless tagged `[L]`.

---

### Epic 1 — Project Foundation

#### 1.1 Initialize workspace
**AC:**
- `cargo new --vcs none online-pong` then converted to workspace.
- Root `Cargo.toml` declares `[workspace]` with `members = ["crates/*"]` and a `[workspace.package]` with shared `edition = "2021"`, `rust-version`, `license`, `repository`.
- `[workspace.dependencies]` block centralizes versions for `serde`, `thiserror`, `anyhow`, `tracing`.
- `cargo build` succeeds with no member crates yet.

**Rust notes:** Workspace inheritance via `package.edition.workspace = true` in member crates is the modern way to avoid version drift. Read the Cargo book section on workspace inheritance before doing this — it's small but worth seeing once.

#### 1.2 Pin toolchain and configure formatters
**AC:**
- `rust-toolchain.toml` pins `channel = "stable"`, components `["rustfmt", "clippy"]`.
- `rustfmt.toml` empty or minimal (defaults align with the official style guide).
- `clippy.toml` set with `msrv` matching workspace.
- `cargo fmt --check` and `cargo clippy --all-targets --all-features -- -D warnings` both pass on empty workspace.

#### 1.3 CI pipeline
**AC:**
- GitHub Actions workflow runs on push and PR.
- Jobs: `fmt`, `clippy` (with `-D warnings`), `test`, `doc` (with `RUSTDOCFLAGS="-D warnings"`), `deny` (using `cargo-deny`).
- Matrix across `ubuntu-latest`, `macos-latest`, `windows-latest`.
- Caching via `Swatinem/rust-cache`.

**Rust notes:** `RUSTDOCFLAGS="-D warnings"` catches broken intra-doc links — this is what makes rustdoc actually useful long-term.

#### 1.4 README and contributing docs
**AC:**
- README has: project pitch, workspace map, build/test commands, architecture-decision links.
- Each crate has a `lib.rs`/`main.rs` doc-comment header (`//!`) explaining purpose. Empty crates can have placeholder comments now.

---

### Epic 2 — CHIP-8 Core Emulator

This is where most of the re-learning happens. Stories are intentionally fine-grained because each one has a green test at the end.

#### 2.1 Define the `Chip8` struct and constants
**AC:**
- `chip8-core/src/lib.rs` exports a `Chip8` struct with: 4KB `memory: [u8; 4096]`, 16 `v: [u8; 16]` registers, `i: u16`, `pc: u16`, `stack: [u16; 16]`, `sp: u8`, `delay_timer: u8`, `sound_timer: u8`, `display: [bool; 64*32]`, `keypad: [bool; 16]`.
- Constants module with `MEMORY_SIZE`, `DISPLAY_WIDTH`, `DISPLAY_HEIGHT`, `START_ADDRESS = 0x200`, `FONTSET_START_ADDRESS = 0x50`.
- `Chip8::new()` initializes `pc = 0x200` and loads the standard 80-byte fontset at `0x50`.
- Unit test: `new()` produces a zero-state machine with PC at 0x200 and fontset bytes correct.

**Rust notes:** Use fixed-size arrays, not `Vec`. CHIP-8 memory is bounded; encoding that in the type makes out-of-bounds compile-time-knowable in many cases, and fixed arrays are stack-allocated with no heap variance — important for determinism. Since Rust 1.47 const generics, `Default` works for all `[T; N]` where `T: Default`; initializing with `[0u8; 4096]` directly is equally idiomatic. Stack pointer `sp: u8` requires `as usize` casts when indexing — explicit and intentional.

#### 2.2 Error type
**AC:**
- `error.rs` defines `Chip8Error` enum with variants: `StackOverflow`, `StackUnderflow`, `InvalidOpcode(u16)`, `InvalidAddress(u16)`, `RomTooLarge { size: usize }`.
- Derives via `thiserror::Error`.
- Crate's public `Result<T>` alias.

**Rust notes:** `thiserror` for libraries, `anyhow` for binaries. `chip8-core` uses `thiserror`; `chip8-frontend` and `multiplayer-server` use `anyhow`. This split is the standard Rust convention.

#### 2.3 ROM loading
**AC:**
- `Chip8::load_rom(&mut self, rom: &[u8]) -> Result<()>` copies bytes to `memory[0x200..]`.
- Returns `RomTooLarge` if `rom.len() > 4096 - 0x200`.
- Unit tests: empty ROM, max-size ROM, oversize ROM rejected.

#### 2.4 Fetch-decode cycle skeleton
**AC:**
- `Chip8::tick(&mut self)` fetches a 16-bit big-endian opcode from `memory[pc..pc+2]`, advances PC by 2, dispatches to a `decode_and_execute` function that for now only `unimplemented!()`s.
- Decode function uses pattern matching on nibbles: `let nibbles = (op>>12, (op>>8)&0xF, (op>>4)&0xF, op&0xF);`.
- Unit test: a ROM containing opcode `0x00E0` causes `tick` to be called and PC advances correctly (catch the panic via `should_panic` until 2.5).

**Rust notes:** Pattern matching on tuples of nibbles is far cleaner than nested `if`. This is the moment Rust's `match` exhaustiveness starts paying off.

#### 2.5–2.16 — One opcode group per story
Group the 35 opcodes by family. Each story implements a group, with one unit test per opcode.

| Story | Opcodes | Notes |
|---|---|---|
| 2.5 | `00E0`, `00EE` | Display clear, return — touches stack |
| 2.6 | `1NNN`, `2NNN`, `00EE` (revisit) | Jumps and calls |
| 2.7 | `3XNN`, `4XNN`, `5XY0`, `9XY0` | Skip-on-equal/not-equal |
| 2.8 | `6XNN`, `7XNN`, `8XY0` | Register loads/adds |
| 2.9 | `8XY1`–`8XY7`, `8XYE` | ALU ops; flag register `VF` semantics |
| 2.10 | `ANNN`, `BNNN` | I register, jump-with-V0 |
| 2.11 | `CXNN` | Random — pull `rand` crate |
| 2.12 | `DXYN` | Draw sprite + collision — biggest single opcode |
| 2.13 | `EX9E`, `EXA1` | Keypad skip |
| 2.14 | `FX07`, `FX15`, `FX18` | Timer reads/writes |
| 2.15 | `FX0A` | Wait-for-key — interesting state machine |
| 2.16 | `FX1E`, `FX29`, `FX33`, `FX55`, `FX65` | Memory ops, BCD, font lookup |

**AC pattern (per story):**
- Each opcode has a unit test that sets up minimal state, executes one tick, and asserts the post-state.
- Edge cases tested: `8XY4` overflow sets `VF = 1`, `8XY5` underflow sets `VF = 0`, `DXYN` collision sets `VF = 1`, stack overflow on 17th call returns `StackOverflow`.

**Rust notes for the group:**
- 2.9: `u8::overflowing_add` and `overflowing_sub` give you the carry flag for free. Resist the urge to cast to `u16` and back.
- 2.11: `rand::rngs::SmallRng` seeded explicitly — keep RNG injectable so tests are deterministic and rollback works later.
- 2.12: This is the XOR-blit story. You'll want a helper function; that's fine, keep it private to the module.
- 2.15: Model "waiting for key" as `enum CpuState { Running, WaitingForKey { register: u8 } }`. Don't use a bool flag.

#### 2.17 Timer tick
**AC:**
- `Chip8::tick_timers()` decrements `delay_timer` and `sound_timer` if non-zero.
- Documented: caller must invoke at 60Hz; `tick()` runs at instruction rate (~500–700Hz).
- Unit test: timers don't underflow.

#### 2.18 Timendus test ROM integration
**AC:**
- `tests/roms/` contains the Timendus suite (vendored, with attribution in README).
- `tests/opcodes.rs` runs each ROM headlessly for N cycles, snapshots the display buffer, compares to a known-good fixture.
- `corax+`, `flags`, `quirks` (CHIP-8 mode) all pass.

**Rust notes:** `include_bytes!` macro embeds ROMs into the test binary. Faster than file I/O and makes tests hermetic.

#### 2.19 Public API polish + rustdoc pass
**AC:**
- Every public item has a `///` doc comment with at least one runnable example where applicable.
- `cargo doc --no-deps` produces clean output, no warnings.
- At least 5 doc-tests exist and pass under `cargo test --doc`.
- `#![warn(missing_docs)]` at crate root.

**Rust notes:** Doc-tests are real tests. They catch API drift. This is the killer feature of rustdoc and it's what separates "wrote some Rust" from "wrote idiomatic Rust."

---

### Epic 3 — Desktop Frontend

#### 3.1 winit + pixels skeleton
**AC:** Window opens at 64×32 logical / 640×320 physical, closes cleanly, no rendering yet.

#### 3.2 Architecture: emulator thread + render thread
**AC:**
- Main thread runs `winit` event loop (winit requires this on macOS).
- Emulator runs on a dedicated thread, ticking at configurable IPS.
- Communication via `crossbeam_channel`: input events → emulator, frame snapshots → renderer.
- Documented in module docs why this split exists.

**Rust notes:** This is the first real ownership/threading story. `Arc` is *not* the answer here — channels are. Coming from Go, the temptation is to share state with `Arc<Mutex<_>>`; resist. The emulator owns its state, frontend owns its state, channels move data. This is the lesson.

#### 3.3 Render frame to pixels buffer
**AC:** Emulator's `[bool; 2048]` display buffer rendered as monochrome at correct scale. Resize works.

#### 3.4 Keypad mapping
**AC:** Standard 1234/QWER/ASDF/ZXCV → CHIP-8 0–F mapping; configurable via const table; key-up and key-down both delivered.

#### 3.5 Audio (sound timer beep)
**AC:** When `sound_timer > 0`, a square wave plays via `cpal`. Stops when timer hits 0.

**Rust notes:** `cpal` is the idiomatic cross-platform audio crate. Its callback model is the second exposure to "you don't `Arc<Mutex>` your way out of this, you use channels or atomics."

#### 3.6 ROM loading via CLI + drag-drop
**AC:** `cargo run -p chip8-frontend -- path/to/rom.ch8` works; window also accepts dropped files. Use `clap` with derive.

#### 3.7 Run the IBM logo + a public-domain test ROM end-to-end
**AC:** IBM logo renders correctly. Pong (public ROM, used here only for validation; a custom Pong comes later) playable with keypad.

---

### Epic 4 — CHIP-8 Assembler

#### 4.1 Define the assembly syntax
**AC:** Document grammar in `chip8-asm/README.md`: labels, mnemonics, register syntax (`V0`–`VF`), immediates (decimal/hex/binary), `db` directive, comments. Keep it close to common CHIP-8 conventions.

#### 4.2 Lexer
**AC:** Tokenize source into `Token` enum: `Mnemonic`, `Register(u8)`, `Immediate(u16)`, `Label(String)`, `Comma`, `Newline`, `Comment`. Unit-tested against fixtures.

**Rust notes:** Hand-roll the lexer; don't reach for `nom` or `pest` yet. CHIP-8 assembly is small enough that hand-rolling teaches you more about iterators, `Peekable`, and `&str` lifetimes.

#### 4.3 Parser → AST
**AC:** Token stream → `Vec<Statement>` where `Statement` is `Instruction { op, operands }` or `LabelDef(String)` or `Directive(...)`. Errors with line/column.

#### 4.4 Two-pass codegen
**AC:** Pass 1 resolves label addresses; pass 2 emits bytecode. Forward references work. Output is `Vec<u8>` ready to load.

#### 4.5 CLI binary
**AC:** `chip8-asm input.s8 -o output.ch8`. Errors print with source context (use `codespan-reporting` or `ariadne`).

**Rust notes:** `ariadne` produces gorgeous Rust-compiler-style error output. Worth it for the polish; this is portfolio material.

#### 4.6 Round-trip test
**AC:** Hand-written test program assembles, runs in `chip8-core`, produces expected display output. Add to integration tests.

#### 4.7 Disassembler (small bonus)
**AC:** `chip8-asm --disasm input.ch8` emits readable assembly. Useful for debugging Pong later.

---

### Epic 5 — Pong ROM

#### 5.1 Pong design doc
**AC:** Short markdown doc: memory map, sprite layouts, game loop pseudocode, input mapping, scoring rules (first to N).

#### 5.2–5.6 Implement Pong incrementally
Stories: paddles render → ball renders → ball moves → collision → scoring/win. Each commits a working `.ch8` you can run in your frontend.

#### 5.7 `pong-rom` build integration
**AC:** `pong-rom/build.rs` runs `chip8-asm` to produce `pong.ch8` at build time. Frontend can depend on `pong-rom` and `include_bytes!` the artifact.

**Rust notes:** `build.rs` scripts are how you do compile-time codegen in Cargo. This is where you learn `cargo:rerun-if-changed=`.

---

### Epic 6 — Server + Scoreboard

#### 6.1 `proto` crate
**AC:** Shared types: `PlayerId`, `MatchResult`, `ScoreboardEntry`, request/response DTOs. `serde` derives. No business logic.

#### 6.2 axum skeleton
**AC:** `multiplayer-server` boots, `/health` returns 200, structured logging via `tracing` + `tracing-subscriber` JSON formatter.

#### 6.3 sqlx + CNPG locally via Tilt
**AC:** `Tiltfile` brings up CNPG cluster. App connects via `DATABASE_URL`. Migrations via `sqlx migrate`.

**Rust notes:** `sqlx::query!` macro checks SQL at compile time against your live DB. Set up `SQLX_OFFLINE=true` with `cargo sqlx prepare` so CI doesn't need a DB. This is the canonical sqlx workflow.

#### 6.4 Schema and migrations
**AC:** Tables: `players(id, name, created_at)`, `matches(id, started_at, ended_at, game_type)`, `match_players(match_id, player_id, score, won)`. Indexes on common queries.

#### 6.5 Player registration endpoints
**AC:** `POST /players`, `GET /players/:id`. Tested via integration tests using `axum::Router` + `tower::ServiceExt::oneshot` (no real network).

#### 6.6 Match-result ingestion
**AC:** `POST /matches` accepts a completed match payload; transactionally inserts match + per-player rows.

#### 6.7 Scoreboard endpoint
**AC:** `GET /scoreboard?game=pong&order=wins` returns ranked list with W-L-PS-PA. Pagination via cursor.

#### 6.8 Frontend integration
**AC:** Frontend posts match results on game-end; displays scoreboard via a simple in-app menu.

---

### Epic 7 — Online Multiplayer (ggrs)

#### 7.1 Determinism audit
**AC:** Replay test: same ROM + same input sequence + same RNG seed = byte-identical display buffer after N ticks. **Must pass before ggrs integration**, or rollback will desync.

**Rust notes:** This is the architectural payoff of keeping `chip8-core` I/O-free. If this test fails, you have hidden non-determinism — almost always RNG or timer coupling — and you fix it now, not after rollback breaks mysteriously.

#### 7.2 Snapshot/restore on `Chip8`
**AC:** `Chip8::snapshot() -> Snapshot` and `Chip8::restore(&Snapshot)`. Snapshot is `Clone + Serialize`. Round-trip test asserts byte-equality of state.

#### 7.3 ggrs `SessionBuilder` integration
**AC:** `netplay::Session` wraps `ggrs`, implements `ggrs::Config` with `Input = u16` (keypad bitmask), `State = Snapshot`.

#### 7.4 P2P transport
**AC:** UDP socket, NAT-naive (LAN first). Two binaries on same machine can connect.

#### 7.5 Pong over network end-to-end
**AC:** Two frontends on LAN play one Pong game. Match result posted to server.

#### 7.6 Rollback stress test
**AC:** Inject artificial latency (50/100/200ms); game remains playable. No desyncs over 5-minute session.

---

### Epic 8 — SUPER-CHIP

#### 8.1 Mode flag in `Chip8`
**AC:** `enum Mode { Chip8, SuperChip }`; `Chip8::with_mode()` constructor.

#### 8.2 Hi-res display (128×64)
**AC:** Display buffer dynamically sized; `00FF` enables hi-res, `00FE` disables.

#### 8.3 SUPER-CHIP opcodes
**AC:** Implement `00CN`, `00FB`, `00FC`, `00FD`, `DXY0` (hi-res sprite), `FX30` (hi-res font), `FX75`, `FX85`. Unit tests per opcode.

#### 8.4 Quirks configuration
**AC:** Configurable quirks (shift, load/store, jump) — see Timendus quirks ROM. Default to SUPER-CHIP-correct behavior in that mode.

#### 8.5 Assembler support
**AC:** New mnemonics added; `--target schip` flag.

#### 8.6 Frontend resize
**AC:** Frontend handles dynamic display size without restart.

---

### Epic 9 — Multi-Game Server

#### 9.1 Session manager (tokio)
**AC:** Server maintains map of active sessions; `tokio::spawn` per session; channels for I/O.

#### 9.2 Matchmaking
**AC:** `POST /matchmake` queues a player; pairs and returns connection info. Configurable game-types.

#### 9.3 Relay vs P2P decision
**AC:** Architecture doc compares; pick relay for hostile-NAT robustness in v1. Rollback still client-driven.

#### 9.4 Containerization
**AC:** Multi-stage `Dockerfile` for server. Minimal final image (`distroless` or `gcr.io/distroless/cc`).

#### 9.5 Kustomize base + overlays
**AC:** `deploy/kustomize/base` with deployments, services, CNPG cluster. `overlays/local` for Tilt; `overlays/prod` placeholder.

#### 9.6 Tilt dev loop
**AC:** `tilt up` brings up server + DB + CNPG operator; live-reload on Rust file change via `cargo watch`.

#### 9.7 Argo CD + Kargo
**AC:** ArgoCD application manifest; Kargo pipeline for promotion local→staging→prod (prod can be a placeholder cluster).

#### 9.8 Fake-player e2e workload `[L]`
**AC:** `tools/fake-player` is a binary that runs a headless emulator + scripted inputs. Kustomize deploys two replicas; they find each other via matchmaking and play a full Pong game; CI job asserts a match record lands in DB.

**Rust notes:** This is the first real `tokio` story for the client side too — fake-players don't need a window, so async makes sense here. Or keep them threads-based and learn the contrast.

#### 9.9 Observability
**AC:** Server emits `tracing` spans → OTLP → Grafana/Tempo (or just stdout JSON for v1). `/metrics` Prometheus endpoint via `axum-prometheus`.

---

### Epic 10 — XO-CHIP

#### 10.1 Extended memory
**AC:** 64KB addressable; `i` register widened where needed; load/store handle the wider range.

#### 10.2 XO-CHIP opcodes
**AC:** `5XY2`, `5XY3`, `F000 NNNN`, `FN01`, `FX02`, plane selection, scrolling extensions. Unit tests per opcode.

#### 10.3 Color planes
**AC:** Up to 4-color display via 2 planes; frontend renders correctly.

#### 10.4 XO-CHIP audio
**AC:** Sample-based audio buffer (`FX02` pattern, `FX3A` pitch); `cpal` plays it.

#### 10.5 Assembler support and round-trip tests
**AC:** Assembler supports XO-CHIP mnemonics; Octo-compatible test programs round-trip.

---

### Epic 11 — Custom Rollback Netcode

By the time you reach this, you'll have lived inside ggrs long enough to know what you want to change. Don't over-specify it now — write a design doc as Story 11.0 reflecting what you've learned, then break it down. Likely shape: input prediction, confirmation frames, snapshot ring buffer, desync detection. Replace `netplay` crate's ggrs guts while keeping its public API stable so the frontend doesn't notice.

---

## Cross-Cutting Conventions

A few things that come up across many stories — calling them out once so they don't surprise you:

- **`#![deny(unsafe_code)]`** in every crate except where you genuinely need it (you probably won't). Portfolio signal.
- **`Default` for arrays > 32**: not implemented in stdlib for historical reasons. You'll write `impl Default for Chip8` manually.
- **Newtypes for IDs**: `PlayerId(Uuid)`, `MatchId(Uuid)`. Don't pass raw `Uuid` around. The #1 thing Go developers underuse when they come to Rust.
- **`#[must_use]`** on functions returning `Result` or builder types you care about. Cheap, idiomatic, catches bugs.
- **Feature flags** on `chip8-core`: `serde` behind a feature so the core doesn't force serde on consumers that don't snapshot. Standard pattern.
- **No `async` in `chip8-core` or `netplay`.** They're sync. The async boundary is the server crate.
