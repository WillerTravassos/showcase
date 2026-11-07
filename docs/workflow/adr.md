# Architecture decision records

An ADR captures one decision that shapes a project, with the context that made it the
right call. Rationale that does not belong in code comments lives here or in commit
messages.

Write one when a choice is costly to reverse, was debated between real alternatives, or
will surprise a future reader (a library, a boundary, a data model, a protocol).

## Location and format

`<project>/docs/adr/NNNN-<slug>.md`, numbered in order. An ADR is never rewritten; a later
ADR supersedes it and both link to each other.

```markdown
# NNNN. <Decision in a few words>

Status: proposed | accepted | superseded by NNNN
Date: YYYY-MM-DD

## Context
<The forces at play: requirements, constraints, what hurts today.>

## Decision
<What we will do, stated plainly.>

## Alternatives
<Each option considered and why it lost.>

## Consequences
<What gets easier, what gets harder, what to watch.>
```
