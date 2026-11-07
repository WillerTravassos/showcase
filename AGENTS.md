# AGENTS.md

Standing rules for any agent working in this repository. Each project folder has its own
`AGENTS.md` that adds project rules and names the agent's role there (learning or build
mode). Read it before touching that project.

## Repository map

| Project            | Language | Mode     | Standards                                                  |
|--------------------|----------|----------|------------------------------------------------------------|
| `online-pong/`     | Rust     | learning | `docs/standards/rust.md`, `rust-package-structure.md`      |
| `clinic-scheduler/`| Go       | build    | `docs/standards/go.md`, `go-package-structure.md`          |

Shared local infrastructure (Tilt shell, otel-lgtm, namespaces) lives in `tilt/`.
`make help` lists every command. CI runs the same make targets, so a green local run is a
green CI run.

## Ask, never assume

When a requirement, file placement, name, or behaviour is unclear, ask the user before
writing code. Offer a recommended answer with the question.

## Standards to load

Load the matching standard before writing or reviewing code in that language:

- **Go**: `docs/standards/go.md` and `docs/standards/go-package-structure.md`.
- **Rust**: `docs/standards/rust.md` and `docs/standards/rust-package-structure.md`.
- **TypeScript**: `docs/standards/typescript.md`.
- **k6 tests**: `docs/standards/k6.md`.
- **Logs, traces, metrics**: `docs/standards/observability.md`.
- **Security**: `docs/standards/security.md` applies to all code. Run the threat model in
  `docs/workflow/threat-model.md` whenever a change touches a trust boundary.

## Workflow

- **Active workflow**: `docs/workflow/atdd.md` (acceptance test-driven development). Read
  it before starting any ticket.
- **Writing tickets**: `docs/workflow/tickets.md`.
- **Recording a decision** (costly to reverse, or chosen between real alternatives):
  `docs/workflow/adr.md`.
- One ticket per branch, named `<project>/<issue-number>-<slug>`, and one pull request per
  ticket. Ask before pushing, opening a pull request, or creating a GitHub issue.

## Testing

Tests are executable specifications of behaviour. Each behaviour has a test for the happy
path and one for each failure path it handles. Coverage is a by-product, never a target.

- **Tiers by directory**, never by build tags:
  - **unit**: next to the code, no infrastructure. For self-contained packages or crates
    that can be black-boxed.
  - **integration**: talks to infrastructure that Tilt runs (database, broker).
  - **acceptance**: drives the deployed system end to end through its public protocol.
- Integration and acceptance tests carry most of the weight.
- Fakes come from a library (mockery for Go, mockall for Rust), only at system boundaries
  (ports to external systems, time, randomness). Code you own is exercised for real.
- Test code follows the same standards and linters as production code.
- An approved acceptance test is a contract. If one looks wrong, stop and ask; editing,
  skipping, or deleting it needs the user's approval.

## Comments

Comments describe what the code does, how to use it, or the constraint it obeys (a
protocol, a specification, a business rule). Write them for a reader who has only the code.

- The development story stays out of comments: no "I chose", "previously", "now we",
  "fixed", or "as discussed". Rationale goes in the commit message or an ADR.
- Allowed: a short note naming what a construct replaces (for example, "replaces the
  manual index loop").
- Technical terms get a plain-language gloss in parentheses the first time they appear in
  a comment, for example "idempotent (safe to repeat)".
- Comment only what the code cannot say itself. Language standards add doc comment rules.

## Prose

Applies to comments, docs, commit messages, tickets, and pull requests:

- Canadian spelling (behaviour, colour, licence as a noun, initialize). Identifiers use
  the ecosystem's US spelling (`Color`, `Initialize`).
- Use commas, colons, parentheses, or separate sentences where an em dash might go. The em
  dash character never appears.

## Commit messages

Three parts, in order:

1. **Subject**: `type(scope): summary`, for example `feat(clinic-scheduler): book an
   appointment`. Scope is the project or crate.
2. **Goal paragraph**: what the change does and why it exists. Motivation, not a file list.
3. **Change list**: bullets of the actual changes, each at most 180 characters. Ask before
   going longer.

End with a `Refs: #<issue>` footer when a ticket exists.

```
fix(multiplayer-server): close idle sessions after the timeout

Idle sessions held their UDP sockets forever, so a busy node ran out of ports after a
few hours of matchmaking churn.

- Close sessions with no input for SESSION_IDLE_TIMEOUT (default 60s) and release their sockets
- Emit a session.closed span event with the close reason

Refs: #42
```
