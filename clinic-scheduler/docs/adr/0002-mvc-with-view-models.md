# 0002. MVC with view models for the portals

Status: accepted
Date: 2026-11-08

## Context

The portals are HTMX front ends over internal services. The question was whether MVC or
MVVM fits. MVVM relies on two-way data binding between a client-side view and its view
model; HTMX has no client-side model, the server renders HTML and the browser swaps it.
The layout must also fit `go-package-structure.md` (`ingress` for entry points, `web/`
for templates) and stay readable.

## Decision

Use MVC with an explicit view model per page or fragment:

- **Controller**: an `ingress` handler (`book_slot_http.go`). It parses and validates the
  request, calls the domain service over ConnectRPC, and builds the view model.
- **View model**: a plain struct defined next to its templ component, holding exactly
  the values the screen shows (already formatted for the locale).
- **View**: the templ component. It renders the full page, or only the fragment when the
  request carries the `HX-Request` header.

A portal's `web/` folder holds three Go packages:

- `web` (`web.go`): the `//go:embed` of `static/` and the message catalogue, plus the
  sources that build into `static/` (`css/` for Tailwind, `ts/` for TypeScript).
- `web/pages`: one templ component per full page, with its view model next to it
  (`home.templ` holds `HomeView` and `Home`). Each page wraps its content in
  `components.Layout`.
- `web/components`: the layout and the fragments that pages compose, with their view
  models (`layout.templ`, `language_switch.templ`).

`pages` may import `components`; `components` never imports `pages`. `.go-arch-lint.yml`
enforces it.

Rules that keep it readable:

- No presenter interfaces, no base view model, no generic mapping layer. A page's view
  model holds the layout's view model as a field (composition), not as a base.
- Domain and protobuf types never reach a template; the handler maps them. This also
  keeps personal information off screens that do not need it (TGV P11.15).
- A fragment with one or two values takes them as plain component arguments instead of
  a view model.

## Alternatives

- **MVVM**: needs client-side binding (Alpine.js or a framework). It splits state between
  browser and server, which HTMX is designed to avoid.
- **Plain MVC with domain types in templates**: fewer types, but templates couple to
  the internal API and can leak fields.
- **Flat `web/` package**: no imports between packages, but pages, fragments, and assets
  share one folder, growing by two files (`.templ` and `_templ.go`) per screen, and
  nothing separates a page from a fragment.
- **One `components` package** (the templ guide's example app): one package fewer, but
  the same lack of separation between pages and fragments.
- **One package per screen** (`web/booking/`, `web/home/`): scales best for many
  screens, premature with few.

## Consequences

- Each screen is three small files to read in order: handler, view model, component.
- View models are trivially testable: build one, render it, assert the HTML.
- Mapping code grows with the number of screens; keep each mapper next to its handler.
- A fragment that an HTMX request renders alone lives in `components` and is reused by
  its page, so the two never drift.
- The Tailwind source scans `../**/*.templ`, so classes in every `web` package are kept.
