# 0001. templ for server-rendered HTML

Status: accepted
Date: 2026-11-08

## Context

Both portals are server-rendered HTML driven by HTMX. HTMX swaps fragments, so the same
markup is rendered as a full page and as a partial. The goals are simplicity,
readability, and safe output: every page shows personal information, so escaping must be
automatic and context-aware (HTML body, attribute, URL, JavaScript).

## Decision

Render HTML with [templ](https://templ.guide). A component is a typed Go function
(`templ SlotList(vm SlotListView)`), compiled to Go by `templ generate`. Full pages
compose fragment components, so an HTMX request renders only the fragment it needs.
`security.md` names `html/template`; the project `AGENTS.md` overrides it with templ.

## Alternatives

- **`html/template`** (standard library): no dependency and context-aware escaping, but
  templates are string-typed: a misspelt field or a wrong type fails at runtime, and
  fragments need named `define` blocks that are hard to follow.
- **Mustache or Handlebars** (`cbroglie/mustache`, `aymerick/raymond`): logic-less and
  familiar, but escaping is HTML-body only, not context-aware, so attributes and URLs
  need manual care. Rejected on security.
- **gomponents**: HTML written as Go function calls (`Div(Class("x"), ...)`). Type-safe,
  no codegen, but markup is harder to read and review than HTML.
- **Raw string building** (`fmt.Fprintf`): no escaping. Rejected.

## Consequences

- Type errors in templates fail the build, not the request.
- One more generator in the toolchain: `templ generate` runs in `make go-verify` and in
  Tilt live update, and generated `_templ.go` files are committed.
- templ's language server gives editor completion; reviewers read HTML-shaped code.
