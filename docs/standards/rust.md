# Rust standard

Rust code follows, in order of precedence:

1. [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/) for public APIs
2. [Rust Style Guide](https://doc.rust-lang.org/style-guide/), applied by `rustfmt`
3. The workspace lints in `Cargo.toml` (`[workspace.lints]`) and `clippy.toml`, which are
   the source of truth for lint levels

Package layout follows `rust-package-structure.md`. Logging and tracing follow
`observability.md`.

## Toolchain

- `rust-toolchain.toml` pins stable for building, testing, and clippy.
- `rustfmt.toml` uses nightly-only options (import grouping and granularity, field
  alignment), so formatting runs on nightly: `make rust-fmt` (`cargo +nightly fmt`).
- Every member crate opts into the workspace lints with `[lints] workspace = true`. The
  workspace sets `unsafe_code = "forbid"`, `missing_docs = "warn"`, and clippy `pedantic`
  plus `nursery` as warnings; CI turns warnings into errors.
- An `#[allow(...)]` or `#[expect(...)]` needs the user's agreement and a `reason = "..."`.

## Style

- Modules use `foo.rs` plus a `foo/` directory, never `foo/mod.rs`.
- Shared dependency versions live in `[workspace.dependencies]`; member crates use
  `dep.workspace = true`.
- Newtypes for identifiers and units (`PlayerId(Uuid)`, `Hertz(u32)`). Primitives stay
  inside the module that owns them.
- Model states as enums, not boolean flags (`CpuState::WaitingForKey { register }`).
- Prefer generics over `Box<dyn Trait>`; reach for trait objects only for heterogeneous
  collections or plugin boundaries.
- Ownership flows through the program: move data through channels or function arguments.
  Shared mutable state (`Arc<Mutex<_>>`, `Rc<RefCell<_>>`) needs a design reason the user
  agreed to.
- `#[must_use]` on functions whose result must be handled, and on builders.
- Prefer `std` over a crate when the result is equally clear. A new dependency needs a
  one-line justification in the pull request and a passing `make rust-deny`.

## Errors

- Library crates define typed errors with `thiserror` and a crate-local
  `pub type Result<T> = std::result::Result<T, Error>;`.
- Binary crates use `anyhow` at the top level and add context with `.context(...)`;
  library errors pass through unchanged.
- `unwrap()`, `expect()`, unchecked indexing on untrusted lengths, and truncating `as`
  casts belong only in tests, examples, and doc-tests. Production code uses `?`, `get()`,
  and `try_from`.
- Every `Result` is handled; `let _ =` on a `Result` needs a comment saying why the
  failure is harmless.
- An error is logged once, at the boundary that handles it. See `observability.md`.

## Documentation

The repository comment rules in `AGENTS.md` apply. Rust adds:

- Every crate root starts with a `//!` comment explaining the crate's purpose.
- Every `pub` item has a `///` comment: what it does, its errors (`# Errors`), its panics
  (`# Panics`) if any, and an example when the use is not obvious.
- Use intra-doc links (`` [`Chip8`] ``) rather than plain names.
- Doc-tests are real tests: they must run, using `# ` to hide setup lines.
- `make rust-doc` (rustdoc with warnings as errors) passes.

## Testing

The repository testing rules in `AGENTS.md` apply. Rust specifics:

- **Unit tests** live in a `#[cfg(test)] mod tests` block at the bottom of the file under
  test.
- **Hermetic black-box tests** (no infrastructure, such as ROM test suites) live in each
  crate's `tests/` directory.
- **Integration and acceptance tests** that need Tilt live only in the
  `crates/system-tests` crate (`publish = false`), as `tests/integration_*.rs` and
  `tests/acceptance_*.rs`. The default test run excludes that crate.
- **Fixtures and cases** use [`rstest`](https://docs.rs/rstest): `#[fixture]` for setup,
  `#[case]` for tables of inputs. Compare structures with `pretty_assertions::assert_eq`.
- **Fakes** use [`mockall`](https://docs.rs/mockall) on port traits with
  `#[cfg_attr(test, mockall::automock)]`, so release builds never contain them. Tests in
  another crate enable mocks through a `test-util` cargo feature:
  `#[cfg_attr(any(test, feature = "test-util"), mockall::automock)]`.
- **No sleeping or wall-clock time**: inject a clock or tick source, and inject seeded
  random number generators.
- Embed fixture files with `include_bytes!` so tests stay hermetic.
- **Traces in tests**: assert spans in-process with
  `opentelemetry_sdk::trace::InMemorySpanExporter` (cargo feature `testing`).

## Commands

| Task                         | Make target                                  | Bacon job          |
|------------------------------|----------------------------------------------|--------------------|
| Format (nightly)             | `make rust-fmt PROJECT=<project>`            |                    |
| Lint                         | `make rust-lint PROJECT=<project>`           | `clippy-all`       |
| Unit and hermetic tests      | `make rust-test PROJECT=<project>`           | `test`             |
| Doc-tests                    | `make rust-test-doc PROJECT=<project>`       | `test-doc`         |
| Integration tests (Tilt up)  | `make rust-integration PROJECT=<project>`    | `test-integration` |
| Acceptance tests (Tilt up)   | `make rust-acceptance PROJECT=<project>`     | `test-acceptance`  |
| Docs with warnings as errors | `make rust-doc PROJECT=<project>`            | `doc`              |
| Dependency audit             | `make rust-deny PROJECT=<project>`           | `deny`             |
| Mutation testing on the diff | `make rust-mutate PROJECT=<project>`         |                    |
| Everything before review     | `make rust-verify PROJECT=<project>`         |                    |
