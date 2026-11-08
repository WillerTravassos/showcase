# 0017. HTTP and browser acceptance drivers

Status: accepted
Date: 2026-11-08

## Context

Acceptance tests drive the deployed system through its public protocol (`atdd.md`). For
the portals that protocol is HTML over HTTP with HTMX. Most behaviour is visible in
server responses; the countdown, some HTMX swaps, and accessibility need a real browser.
TGV T01 requires accessible web pages.

## Decision

- **HTTP driver** (`test/acceptance/driver/http`): a Go client with a cookie jar that
  signs in through Keycloak, sends the CSRF token and `HX-Request` header, and parses
  responses with [goquery](https://github.com/PuerkitoBio/goquery). Most scenarios use it.
- **Browser driver** (`test/acceptance/driver/browser`): [chromedp](https://github.com/chromedp/chromedp)
  driving headless Chrome, for the hold countdown, HTMX swaps that depend on JavaScript,
  and an [axe-core](https://github.com/dequelabs/axe-core) scan of each visited page in
  both themes.
- The DSL hides which driver a step uses.
- Both dial 127.0.0.1 and set the `Host` header for `*.localhost` (ADR 0011).

## Alternatives

- **HTTP only**: fast, but cannot see the countdown or check accessibility.
- **Browser only**: slow and flaky for the bulk of scenarios.
- **Playwright for Go**: needs a Node driver download; chromedp is pure Go.

## Consequences

- devbox provides a pinned headless Chrome.
- Browser scenarios stay few; each one names why it needs a browser.
