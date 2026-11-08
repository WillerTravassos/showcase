# 0013. Indigo and rose theme, light and dark

Status: accepted
Date: 2026-11-08

## Context

The patient portal serves sexual health testing. The look must be friendly and free of
stigma, not cold or clinical. Both portals need light and dark themes, and pages must
meet WCAG 2.1 AA contrast (TGV T01).

## Decision

Indigo as the primary colour, rose as the accent, slate neutrals. Colours are CSS custom
properties declared in Tailwind v4's `@theme`, built with the Tailwind standalone CLI (no
Node). The theme follows `prefers-color-scheme`, with a manual toggle stored in a cookie
so the server renders the right theme on first paint.

| Token | Light | Dark |
|---|---|---|
| `bg` | `#ffffff` | `#0f172a` |
| `surface` | `#f8fafc` | `#1e293b` |
| `text` | `#0f172a` | `#f1f5f9` |
| `muted` | `#475569` | `#94a3b8` |
| `primary` | `#4f46e5` | `#a5b4fc` |
| `on-primary` | `#ffffff` | `#1e1b4b` |
| `accent` | `#be123c` | `#fda4af` |
| `on-accent` | `#ffffff` | `#4c0519` |
| `accent-soft` | `#ffe4e6` | `#4c0519` |
| `available` (empty slot) | `#eef2ff` | `#1e1b4b` |
| `success` | `#15803d` | `#86efac` |
| `warning` | `#b45309` | `#fcd34d` |
| `danger` | `#b91c1c` | `#fca5a5` |
| `border` | `#64748b` | `#64748b` |

## Contrast check

Computed with the WCAG 2.1 relative luminance formula. Text pairs need 4.5:1; borders
and controls need 3:1.

| Pair | Light | Dark |
|---|---|---|
| `text` on `bg` | 17.85 | 16.30 |
| `text` on `surface` | 17.06 | 13.35 |
| `muted` on `bg` | 7.58 | 6.96 |
| `muted` on `surface` | 7.24 | 5.71 |
| `primary` on `bg` | 6.29 | 8.96 |
| `on-primary` on `primary` | 6.29 | 8.02 |
| `accent` on `bg` | 6.29 | 9.44 |
| `on-accent` on `accent` | 6.29 | 8.27 |
| `text` on `accent-soft` | 14.87 | 14.27 |
| `text` on `available` | 15.97 | 14.59 |
| `primary` on `available` | 5.62 | 8.02 |
| `success` on `bg` | 5.02 | 12.71 |
| `warning` on `surface` | 4.80 | 10.15 |
| `danger` on `surface` | 6.18 | 7.71 |
| `border` on `bg` | 4.76 | 3.75 |

Every pair passes AA.

## Alternatives

- **Teal and slate**: the common healthcare look, read as clinical.
- **Neutral greys with one accent**: safe but impersonal.

## Consequences

- Rose and danger red are close in hue, so status is never shown by colour alone: danger
  always carries an icon and text.
- New tokens are added to this table with their contrast, in a superseding ADR.
- The browser acceptance suite runs axe-core in both themes.
