# Overview: Patient Web App

> Reference this file for sessions 10–12.

---

## Binary Entrypoint

`cmd/patient/main.go` — HTTP server on `:8081`. Renders HTML via `html/template`. Makes all backend API calls server-side; the browser never calls the backend API directly.

---

## Template Layout

```
web/patient/
├── templates/
│   ├── base.html          # Root layout: <html>, <head>, nav, footer
│   ├── partials/          # HTMX partial responses (no base layout)
│   │   ├── slot-grid.html
│   │   ├── question-step.html
│   │   ├── appointment-card.html
│   │   └── result-panel.html
│   ├── auth/
│   │   ├── login.html
│   │   └── signup.html
│   ├── register/
│   │   ├── step1-personal.html
│   │   ├── step2-contact.html
│   │   ├── step3-consent.html
│   │   └── complete.html
│   ├── dashboard.html
│   ├── appointments/
│   │   ├── list.html
│   │   └── detail.html
│   ├── book/
│   │   ├── questions.html
│   │   ├── tests.html
│   │   ├── clinic.html
│   │   └── confirm.html
│   └── profile.html
└── static/
    ├── css/app.css        # Tailwind output (generated, not committed)
    └── js/app.js          # Alpine.js compiled from TypeScript
```

---

## HTMX Conventions

- All partial responses return **only the fragment** — no `<html>` or `<head>` wrapper
- Use `HX-Redirect` response header for post-action redirects (e.g. after booking confirmation)
- Use `HX-Trigger` to dispatch custom events for Alpine.js to react to
- Swap targets use `id` attributes, never CSS classes
- All HTMX forms include CSRF token as a hidden input (middleware validates it)

---

## Alpine.js (TypeScript) Conventions

TypeScript source lives in `web/patient/ts/`. Compiled to `web/patient/static/js/app.js` via `esbuild`.

Alpine components are defined as:
```typescript
// web/patient/ts/components/questionTree.ts
Alpine.data('questionTree', () => ({
  answers: {} as Record<string, string>,
  // ...
}))
```

- Never manipulate DOM directly — use Alpine reactive state
- Use `x-data` at the component root, `x-show` / `x-bind` / `x-on` for reactivity
- Alpine state is local (per component); do not share state via global stores unless necessary

---

## Session & Auth

- Cookie-based: `httpOnly`, `Secure`, `SameSite=Strict`
- Session data stored in Redis with key `session:{session_id}`
- Session contains: `user_uuid`, `role`, `access_token`, `booking_in_progress` (booking wizard state)
- Middleware `RequireAuth` redirects unauthenticated requests to `/login`

---

## Route Table

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/login` | `LoginPage` | No |
| POST | `/login` | `LoginSubmit` | No |
| GET | `/signup` | `SignupPage` | No |
| POST | `/signup` | `SignupSubmit` | No |
| GET | `/logout` | `Logout` | Yes |
| GET | `/register` | `RegisterPage` | Yes |
| POST | `/register/step/:n` | `RegisterStep` | Yes |
| GET | `/` | `Dashboard` | Yes |
| GET | `/appointments` | `AppointmentList` | Yes |
| GET | `/appointments/:id` | `AppointmentDetail` | Yes |
| GET | `/book/questions` | `BookQuestions` | Yes |
| POST | `/book/questions` | `BookQuestionsSubmit` | Yes |
| GET | `/book/tests` | `BookTests` | Yes |
| POST | `/book/tests` | `BookTestsSubmit` | Yes |
| GET | `/book/clinic` | `BookClinic` | Yes |
| GET | `/book/slots` | `BookSlotsPartial` (HTMX) | Yes |
| POST | `/book/confirm` | `BookConfirm` | Yes |
| POST | `/book/complete` | `BookComplete` | Yes |
| GET | `/profile` | `ProfilePage` | Yes |
| POST | `/profile` | `ProfileUpdate` | Yes |

---

## Key UI Rules

- Test results are **only rendered** when appointment state is `RESULTS_RELEASED`
- Email field on profile page renders as read-only `<span>`, not `<input>`
- Abnormal analyte results are highlighted with a red badge
- Mobile-first layouts using Tailwind responsive prefixes (`sm:`, `md:`)
