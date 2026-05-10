# Error Handling Conventions

---

## Philosophy

Errors are part of the domain. They are not strings — they are typed values that carry meaning
to callers. The error handling strategy has three layers: domain errors, transport mapping,
and client presentation.

---

## Layer 1 — Domain Sentinel Errors

Each domain package defines its sentinel errors in `internal/<service>/pkg/<domain>/errors.go`.

```go
// internal/api/pkg/patient/errors.go
package patient

import "errors"

var (
    ErrNotFound        = errors.New("patient not found")
    ErrEmailImmutable  = errors.New("email cannot be changed after registration")
    ErrAlreadyExists   = errors.New("patient already exists")
    ErrInvalidJurisdiction = errors.New("invalid jurisdiction")
    ErrConsentRequired = errors.New("consent must be recorded before activation")
)
```

Rules:
- Sentinel errors are `var`, not `const`, and use `errors.New`.
- Names are `Err{PascalCase}`. Always prefixed with `Err`.
- They describe the domain condition, not the HTTP outcome. `ErrNotFound` — not `Err404`.
- Never define sentinel errors inline inside a function or method body.
- The set of sentinel errors for a package is the public contract. Document additions in the PR.

---

## Layer 2 — Error Wrapping

All errors flowing upward must be wrapped with context at every layer boundary.

```go
// Repository implementation — wraps DB errors
func (r *repository) GetByUUID(ctx context.Context, id uuid.UUID) (*Patient, error) {
    row, err := r.db.QueryRow(ctx, getPatientQuery, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, ErrNotFound  // sentinel — no wrapping needed
        }
        return nil, fmt.Errorf("patient repository get %s: %w", id, err)
    }
    // ...
}

// Service implementation — wraps service-layer errors
func (s *service) GetForStaff(ctx context.Context, id uuid.UUID, claims auth.Claims) (*Patient, error) {
    if !claims.HasClinicAccess(clinicID) {
        return nil, fmt.Errorf("get patient %s: %w", id, ErrForbidden)
    }
    p, err := s.repo.GetByUUID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get patient for staff %s: %w", id, err)
    }
    return p, nil
}
```

Rules:
- **Always use `%w`** (not `%v` or `%s`) when wrapping errors that callers may need to inspect with `errors.Is` or `errors.As`.
- Wrap at layer boundaries: DB → repository, repository → service, service → handler.
- Do not double-wrap. If you are re-returning an error from your own package, just return it as-is.
- Error messages use lowercase, no trailing punctuation, describe the operation: `"scheduling slot abc123"` not `"Error scheduling slot abc123!"`.

---

## Layer 3 — HTTP Transport Mapping

Handlers never expose internal errors to clients. The mapping lives in `pkg/http/errors.go`.

```go
// pkg/http/errors.go

type ErrorCode string

const (
    CodeUnauthorized  ErrorCode = "UNAUTHORIZED"
    CodeForbidden     ErrorCode = "FORBIDDEN"
    CodeNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
    CodeConflict      ErrorCode = "CONFLICT"
    CodeValidation    ErrorCode = "VALIDATION_ERROR"
    CodeImmutable     ErrorCode = "FIELD_IMMUTABLE"
    CodeInternal      ErrorCode = "INTERNAL_ERROR"
)

type ErrorResponse struct {
    Error ErrorBody `json:"error"`
}

type ErrorBody struct {
    Code      ErrorCode `json:"code"`
    Message   string    `json:"message"`
    RequestID string    `json:"request_id"`
}

// WriteError maps a domain error to an HTTP response.
// It uses errors.Is to detect sentinel types and choose the correct status code.
func WriteError(w http.ResponseWriter, r *http.Request, err error)
```

The mapping table (in `WriteError`):

| Sentinel / condition | HTTP Status | Code |
|---|---|---|
| `auth.ErrUnauthorized` | 401 | `UNAUTHORIZED` |
| Any `ErrForbidden` | 403 | `FORBIDDEN` |
| Any `ErrNotFound` | 404 | `RESOURCE_NOT_FOUND` |
| Any `ErrAlreadyExists` | 409 | `CONFLICT` |
| Any `ErrEmailImmutable` | 422 | `FIELD_IMMUTABLE` |
| Any `ErrInvalid*` or `ErrConsentRequired` | 422 | `VALIDATION_ERROR` |
| Anything else | 500 | `INTERNAL_ERROR` |

Rules:
- Handlers call `pkghttp.WriteError(w, r, err)` and return immediately after. Never write a status code and then also call WriteError.
- The 500 path logs the full internal error (including wrapped chain) at `error` level. It never exposes the internal message to the client — the client gets only `"an internal error occurred"`.
- 4xx responses do not log at `error` level — they log at `info` (they are expected conditions).

---

## Audit-Critical Error Rule

If an error occurs **after an audit event has been emitted** but **before the response is sent**, the handler must return 500 even if the business operation succeeded. Partial success (operation OK, audit failed) is treated as full failure. This is a TGV certification requirement.

```go
// Example: audit must succeed or the whole request fails
result, err := svc.GetPatient(ctx, id)
if err != nil {
    httputil.WriteError(w, r, err)
    return
}
if err := auditWriter.Write(ctx, auditEvent); err != nil {
    // Do NOT return the result — audit failure means the request failed
    log.FromCtx(ctx).Error().Err(err).Msg("audit write failed")
    httputil.WriteError(w, r, fmt.Errorf("audit: %w", err))
    return
}
render.JSON(w, result)
```

---

## Panic Recovery

- A `middleware.Recoverer` middleware catches panics in handlers and converts them to 500 responses.
- Panics must be logged at `error` level with the stack trace.
- Never intentionally panic in application code. Use error returns.
- The only acceptable `panic` calls are in `main()` startup for truly unrecoverable conditions (e.g. `uuid.MustParse` for compile-time constants).
