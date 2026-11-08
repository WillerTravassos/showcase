# Acceptance test-driven development

The active workflow for every ticket. The human owns intent: the wish, the story, and the
concrete examples. The agent turns approved examples into code. A ticket's scenarios are
the specification; any implementation that makes them pass is valid.

Source: Dave Farley, "The Industry Is Catching Up: The Only Way to Generate AI Code That
Works" (Modern Software Engineering).

## The loop

Work one scenario at a time. Each step ends on its checkpoint before the next begins.

1. **Read the ticket.** Confirm its scenarios are marked approved. If a scenario is
   missing, vague, or describes a design instead of a behaviour, stop and ask.
2. **Write the acceptance test** for one scenario, in the project's acceptance path (see
   below). Name it with the scenario's domain words from `GLOSSARY.md`.
3. **Red checkpoint.** Before running the test, state the prediction: red, the reason,
   and the expected failure message. Run the test and show the failing output next to
   the prediction. If they differ, stop and explain the difference before going on. The
   failure must be the missing behaviour (an assertion), not a compile error or broken
   setup. Wait for approval. Red is the proof that the test can catch a wrong answer.
4. **Implement** the smallest change that turns the test green. Add integration tests
   where the behaviour touches infrastructure that Tilt runs; like acceptance tests, they
   carry most of the weight. Add unit tests only where a self-contained piece needs its
   own specification.
5. **Refactor** with every test green: remove duplication, clarify names, simplify.
   Tests stay unedited, except renames that follow a production rename. When nothing
   needs refactoring, say so and move on.
6. **Green checkpoint.** Run the scenario's acceptance test (`make go-acceptance` or
   `make rust-acceptance`) and the project's verify target (`make go-verify` or
   `make rust-verify`), and show both results. The verify target does not run
   acceptance tests. Wait for review.
7. **Commit** after the user approves, then move to the next scenario.

A scenario is done when its acceptance test passes, the verify target is green, and no
approved test was edited.

Prediction before red comes from predictive TDD (Ted M. Young); the refactor step is the
third phase of red, green, refactor.

## Acceptance tests are locked

Once the user approves a red acceptance test, it is a contract. To change it, stop and
explain why; the user approves the change as a contract change. Claude Code enforces this
with an `ask` permission on the paths below; other harnesses rely on this rule.

| Language | Acceptance path                                          |
|----------|----------------------------------------------------------|
| Go       | `<project>/test/acceptance/`                             |
| Rust     | `<project>/crates/system-tests/tests/acceptance_*.rs`    |

## Test layers for services

Acceptance tests read like the scenario. Keep protocol detail out of them:

1. **Test case**: the scenario in domain language.
2. **DSL**: domain-named helpers (`bookAppointment`, `cancelAppointment`) in
   `test/acceptance/dsl/`.
3. **Protocol driver**: the HTTP or UI client the DSL calls, in
   `test/acceptance/driver/`.
4. **System under test**: the deployed services in Tilt.

A library crate (for example `chip8-core`) skips the DSL and driver: its public API is the
driver.

## Finding holes in the specification

A mutant that survives means a scenario is missing. Run mutation testing on the ticket's
diff before the green checkpoint when the change holds business logic:

- Go: `make go-mutate PROJECT=<project>`
- Rust: `make rust-mutate PROJECT=<project>`

Report surviving mutants to the user as candidate scenarios; the user decides which become
new scenarios.
