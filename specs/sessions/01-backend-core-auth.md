# Session 01 — Backend Core & Auth

## Claude Code Delegation Prompt

```
You are implementing Session 01 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/architecture.md — DDD structure, MVC, dependency flow
  4. specs/conventions/tooling.md — logging, error handling, testing rules
  5. specs/conventions/database.md — CNPG, migration conventions, PII as BYTEA
  6. specs/conventions/errors.md — sentinel errors, wrapping, HTTP mapping

Your job is to build the foundational backend skeleton: Go Standard Project Layout,
service configuration, CNPG-aware DB connection router, HTTP server with middleware
chain, and the complete authentication system (signup, login, JWT, refresh rotation).

Work autonomously. Add TODO comments for genuine ambiguities. Do not ask questions.
When done: go build ./... && go vet ./... && go test -mod=vendor ./...
```

---

## Scope

Runnable backend skeleton and auth system. No domain logic yet — no patients, no
appointments. This is the foundation every other session builds on.

## Dependencies
None. This is session 1.

---

## Deliverables

### 1. Project Skeleton

Initialize Go module. Use the module path defined in `specs/CLAUDE.md`.

Create the full directory structure from `specs/go_package_structure.md`. All
packages must be importable with no circular dependencies. Empty `doc.go` placeholder
files are acceptable for packages not implemented in this session.

### 2. Configuration (`internal/api/app/settings.go`)

Config lives in the service's `app/` package — there is no shared `pkg/config/`.

```go
// internal/api/app/settings.go
package app

type Settings struct {
    // Per-jurisdiction CNPG connection strings (injected from CNPG-generated k8s Secrets)
    DatabaseURLCA    string        `env:"DATABASE_URL_CA,required"`
    DatabaseURLUS    string        `env:"DATABASE_URL_US,required"`
    DatabaseURLBR    string        `env:"DATABASE_URL_BR,required"`
    DatabaseURLCARO  string        `env:"DATABASE_URL_CA_RO"` // read-only replica
    DatabaseURLUSRO  string        `env:"DATABASE_URL_US_RO"`
    DatabaseURLBRRO  string        `env:"DATABASE_URL_BR_RO"`

    NATSUrl          string        `env:"NATS_URL,required"`
    JWTSecret        string        `env:"JWT_SECRET,required"`
    EncryptionKey    string        `env:"ENCRYPTION_KEY,required"` // 64 hex chars = 32 bytes
    Environment      string        `env:"ENV"             envDefault:"development"`
    LogLevel         string        `env:"LOG_LEVEL"       envDefault:"info"`
    Port             int           `env:"PORT"            envDefault:"8080"`
    JWTAccessTTL     time.Duration `env:"JWT_ACCESS_TTL"  envDefault:"15m"`
    JWTRefreshTTL    time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`
}

func LoadSettings() (*Settings, error)
```

Startup validation (fail fast via `github.com/WillerTravassos/showcase/pkg/log`):
- `EncryptionKey` must decode from hex to exactly 32 bytes
- All `DATABASE_URL_*` must be non-empty
- `JWTSecret` must be >= 32 characters

### 3. Database (`pkg/db/`)

#### Jurisdiction Router (`pkg/db/router.go`)

```go
// JurisdictionRouter holds one pool per jurisdiction and routes writes/reads accordingly.
type JurisdictionRouter struct {
    ca   *pgxpool.Pool
    us   *pgxpool.Pool
    br   *pgxpool.Pool
    caRO *pgxpool.Pool
    usRO *pgxpool.Pool
    brRO *pgxpool.Pool
}

func NewJurisdictionRouter(ctx context.Context, cfg RouterConfig) (*JurisdictionRouter, error)

// Write returns the primary pool for the given jurisdiction.
func (r *JurisdictionRouter) Write(jurisdiction string) (*pgxpool.Pool, error)

// Read returns the read-replica pool (falls back to primary if RO not configured).
func (r *JurisdictionRouter) Read(jurisdiction string) (*pgxpool.Pool, error)
```

`RouterConfig` is a plain struct (not the service `Settings`) so `pkg/db/` stays decoupled:

```go
// pkg/db/router.go
type RouterConfig struct {
    CADSN, USDSN, BRDSN       string
    CARoDSN, USRoDSN, BRRoDSN string
}
```

Pool config per jurisdiction: max 25 conns, min 5, max lifetime 1h, max idle 30m.

#### Audit Pool (`pkg/db/audit.go`)

A separate pool for INSERT-only audit writes, using the `audit_writer` DB user:

```go
func NewAuditPool(ctx context.Context, dsn string) (*pgxpool.Pool, error)
```

The audit DSN is derived from the primary DSN by swapping the user to `audit_writer`.
It is passed the CA cluster DSN since audit events are per-cluster (one audit schema
per CNPG cluster).

#### Migrations (`pkg/db/migrate.go`)

```go
// RunMigrations applies all pending migrations using golang-migrate.
// Must be called for each jurisdiction cluster at startup.
func RunMigrations(dsn string) error
```

### 4. Initial Migration

`migrations/000001_init.up.sql`:

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Enums
CREATE TYPE user_role AS ENUM (
    'patient', 'clinic_worker', 'doctor', 'pharmacist', 'admin'
);
CREATE TYPE jurisdiction AS ENUM ('ca', 'us', 'br');
CREATE TYPE user_status AS ENUM ('incomplete', 'active', 'suspended', 'deleted');

-- Global identity table: UUID → jurisdiction mapping only. No PII.
-- This table is the single cross-jurisdiction lookup. It lives in the CA cluster
-- and contains no patient health data — only routing metadata.
CREATE TABLE identity_map (
    uuid            UUID PRIMARY KEY,
    email_hash      TEXT NOT NULL UNIQUE, -- SHA-256(lowercase email) for lookup
    jurisdiction    jurisdiction NOT NULL,
    role            user_role NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-jurisdiction principals table: holds the stable UUID seed — never the raw seed.
-- The seed is generated once with crypto/rand and is the basis for UUIDv5 derivation.
CREATE TABLE principals (
    uuid            UUID PRIMARY KEY,
    seed            BYTEA NOT NULL,       -- 32-byte crypto/rand seed; never changes
    role            user_role NOT NULL,
    jurisdiction    jurisdiction NOT NULL,
    status          user_status NOT NULL DEFAULT 'incomplete',
    clinic_ids      UUID[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-jurisdiction user credentials table
CREATE TABLE user_credentials (
    uuid            UUID PRIMARY KEY REFERENCES principals(uuid) ON DELETE CASCADE,
    email_encrypted BYTEA NOT NULL,   -- COMPLIANCE: AES-256-GCM, key from ENCRYPTION_KEY env
    password_hash   TEXT NOT NULL,    -- bcrypt cost 12
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_uuid   UUID NOT NULL REFERENCES principals(uuid) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE, -- SHA-256 of raw token; raw never stored
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,          -- non-null = already rotated, reject reuse
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON refresh_tokens(user_uuid);
CREATE INDEX ON refresh_tokens(expires_at);
```

`migrations/000001_init.down.sql` — full reversal.

### 5. Identity (`pkg/auth/identity.go`)

```go
// Namespace UUIDs — fixed forever. Changing these breaks all existing identities.
var (
    NamespacePatient = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
    NamespaceStaff   = uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
)

// NewPatientUUID derives a deterministic UUIDv5 from the patient namespace and a
// stable seed generated at registration via crypto/rand. The seed is stored in the
// principals table — it is never derived from email.
func NewPatientUUID(seed []byte) uuid.UUID {
    return uuid.NewSHA1(NamespacePatient, seed)
}

// NewStaffUUID derives a deterministic UUIDv5 from the staff namespace and seed.
func NewStaffUUID(seed []byte) uuid.UUID {
    return uuid.NewSHA1(NamespaceStaff, seed)
}

// EmailHash returns the SHA-256 hex digest of the lowercased email.
// Used only for DB lookup in identity_map — never stored as a human-readable field.
func EmailHash(email string) string
```

### 6. JWT (`pkg/auth/jwt.go`)

```go
type Claims struct {
    Subject   uuid.UUID   `json:"sub"`
    Role      Role        `json:"role"`
    ClinicIDs []uuid.UUID `json:"clinic_ids"`
    jwt.RegisteredClaims
}

type JWTConfig struct {
    Secret     string
    AccessTTL  time.Duration
    RefreshTTL time.Duration
}

func IssueAccessToken(cfg JWTConfig, claims Claims) (string, error)
// IssueRefreshToken returns a cryptographically random token (raw) and its SHA-256 hash.
// Only the hash is stored. Raw is sent to client in httpOnly cookie.
func IssueRefreshToken() (raw string, hash string, err error)
func ValidateAccessToken(cfg JWTConfig, tokenStr string) (Claims, error)
```

Algorithm: HS256. Access token TTL: 15 min. Refresh token: opaque 32-byte random, base64url encoded.

### 7. RBAC (`pkg/auth/rbac.go`)

```go
type Role string

const (
    RolePatient      Role = "patient"
    RoleClinicWorker Role = "clinic_worker"
    RoleDoctor       Role = "doctor"
    RolePharmacist   Role = "pharmacist"
    RoleAdmin        Role = "admin"
)

func (c Claims) HasClinicAccess(clinicID uuid.UUID) bool
func (c Claims) IsAdmin() bool
```

### 8. Shared Auth Errors (`pkg/auth/errors.go`)

```go
var (
    ErrUnauthorized       = errors.New("unauthorized")
    ErrForbidden          = errors.New("forbidden")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrTokenExpired       = errors.New("token expired")
    ErrTokenReused        = errors.New("refresh token already used")
)
```

### 9. HTTP Error Utility (`pkg/http/errors.go`)

Central error-to-HTTP mapper. All handlers call `pkghttp.WriteError` — never write status codes directly.

```go
// WriteError maps a domain error to an HTTP JSON response using errors.Is inspection.
// 500 responses log the full error chain; 4xx responses log at info level.
func WriteError(w http.ResponseWriter, r *http.Request, err error)
```

Mapping (sentinel → status → code):

| Sentinel | Status | JSON code |
|---|---|---|
| `auth.ErrUnauthorized`, `auth.ErrTokenExpired`, `auth.ErrTokenReused` | 401 | `UNAUTHORIZED` |
| `auth.ErrForbidden` | 403 | `FORBIDDEN` |
| `*ErrNotFound` (any domain) | 404 | `RESOURCE_NOT_FOUND` |
| `*ErrAlreadyExists` | 409 | `CONFLICT` |
| `*ErrImmutable` | 422 | `FIELD_IMMUTABLE` |
| `*ErrValidation` | 422 | `VALIDATION_ERROR` |
| anything else | 500 | `INTERNAL_ERROR` |

### 10. Auth Middleware (`pkg/middleware/auth.go`)

```go
// Authenticate validates the Bearer JWT and injects Claims into context.
// Returns 401 for missing/invalid/expired tokens.
// Must use github.com/WillerTravassos/showcase/pkg/log — never fmt or stdlib log.
func Authenticate(cfg auth.JWTConfig) func(http.Handler) http.Handler

func ClaimsFromCtx(ctx context.Context) (auth.Claims, bool)

// RequireRole returns a middleware that returns 403 if the request's role is not in roles.
func RequireRole(roles ...auth.Role) func(http.Handler) http.Handler
```

### 11. Auth Domain Package (`internal/api/pkg/auth/`)

Business logic for auth lives here — distinct from the shared `pkg/auth/` (which holds
types and helpers importable by all services).

- `auth.go` — `AuthService` interface + `Session` type
- `registration.go` — signup logic: generate seed, derive UUID, bcrypt password, write principals + user_credentials + identity_map
- `session.go` — login, refresh, logout logic
- `repository.go` — `Repository` implementing `AuthRepository`
- `errors.go` — `ErrEmailAlreadyRegistered`, `ErrUserNotFound`, etc.

### 12. Auth HTTP Handlers (`internal/api/app/httpingress/`)

Handler struct follows the controller shape from `specs/conventions/architecture.md`:

```go
// internal/api/app/httpingress/handler.go
package httpingress

type Handler struct {
    auth apiauth.AuthService
    log  log.Logger
}
```

**POST /api/v1/auth/signup**
- Body: `{ "email": string, "password": string, "role": string, "jurisdiction": string }`
- Reject `role: patient` from this endpoint if caller is not authenticated (patients sign up via patient app flow)
- Validate: password >= 12 chars, jurisdiction in `[ca, us, br]`
- Return `{ "data": { "uuid": "...", "role": "..." } }`

**POST /api/v1/auth/login**
- Lookup `identity_map` by email_hash → get jurisdiction → route to correct cluster
- Verify bcrypt; issue access + refresh token
- Set `refresh_token` cookie: `httpOnly`, `Secure`, `SameSite=Strict`
- Return `{ "data": { "access_token": "...", "expires_in": 900 } }`

**POST /api/v1/auth/refresh**
- Read refresh token from cookie; look up hash; verify not used, not expired
- Mark old `used_at = now()`; issue new pair (rotation)

**POST /api/v1/auth/logout**
- Mark refresh token used; clear cookie

### 13. Composition Root (`internal/api/app/service.go`)

Wire order:
1. Load `Settings` (fail fast on error)
2. Init `pkg/log` logger from settings
3. `pkg/db.NewJurisdictionRouter` (one pool per jurisdiction)
4. `pkg/db.NewAuditPool`
5. `pkg/db.RunMigrations` for each jurisdiction DSN
6. NATS connection (`pkg/queue`)
7. Construct repositories → services → handlers
8. Build router with middleware chain (`pkg/middleware`)
9. Start HTTP server; graceful shutdown on SIGTERM (30s drain)

---

## Tests Required

Table-driven where applicable. All assertions via `testify/require` and `testify/assert`.

- `TestNewPatientUUID_deterministic` — same seed always produces same UUID
- `TestNewPatientUUID_differentFromStaff` — same seed, different namespace
- `TestEmailHash_lowercaseNormalization` — `User@Example.COM` == `user@example.com`
- `TestIssueAndValidateAccessToken_roundTrip`
- `TestValidateAccessToken_expiredToken_returnsErrTokenExpired`
- `TestIssueRefreshToken_hashDiffersFromRaw`
- `TestRefreshTokenRotation_oldHashInvalidated`
- `TestLogin_wrongPassword_returnsErrInvalidCredentials`
- `TestSignup_duplicateEmail_returns409` (integration: `*_integration_test.go`, calls `test.RequireTiltCI(t)`)
- `TestRequireRole_wrongRole_returns403`
- `TestWriteError_mapsErrForbiddenTo403`
- `TestWriteError_unknownError_returns500_doesNotLeakMessage`

---

## Done Criteria

- `go build ./cmd/api` succeeds
- `go vet ./...` passes
- `golangci-lint run ./...` passes (including forbidigo — no direct zerolog imports)
- All unit tests pass with `go test -mod=vendor ./...`
- `/health` returns `200 {"status":"ok"}`
- Auth endpoints work end-to-end in local kind cluster via `tilt up`
- No `docker-compose` references anywhere in this session's code
