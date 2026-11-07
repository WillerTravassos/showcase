# Security standard

Secure-by-default coding rules for every language here, based on the
[OWASP Application Security Verification Standard](https://owasp.org/www-project-application-security-verification-standard/)
level 1 (the baseline for any internet-facing application). Run the threat model in
`docs/workflow/threat-model.md` for any change that touches a trust boundary.

## Input

- Validate every input where it crosses a trust boundary (HTTP request, message, file,
  environment, CLI flag): type, length, range, and format. Reject what fails; never try
  to repair it.
- Parse into domain types at the boundary (`AppointmentID`, `Email`); inner code accepts
  only the parsed types.
- Bound everything: request body size, header size, collection lengths, page sizes, and
  recursion depth.

## Output

- HTML is rendered through auto-escaping templates (Go `html/template`; never
  `text/template` for HTML). Untrusted data never reaches `innerHTML`.
- SQL uses parameterized queries only (`$1` placeholders, `sqlx::query!`). Identifiers
  that cannot be parameterized come from an allowlist.
- Error responses carry a stable code and a safe message; stack traces, SQL, and internal
  hostnames stay in the logs.

## Authentication and authorization

- Deny by default: every route declares who may call it.
- Check authorization on the server for every request and every resource, including
  ownership (patient A can never read patient B).
- Return "not found" for resources the caller should not know exist.
- Session cookies are `HttpOnly`, `Secure`, and `SameSite=Lax` or stricter.
  State-changing HTMX requests carry a CSRF token (sent with `hx-headers`).
- Passwords, if ever stored, are hashed with Argon2id. Tokens and IDs that must be
  unguessable come from a cryptographic random source (Go `crypto/rand`, Rust the operating
  system generator through `getrandom`). Seeded generators are for determinism, never for
  secrets.

## Secrets

- Secrets reach the process through environment variables sourced from Kubernetes
  Secrets, never from files in the repository or images.
- Secrets, tokens, passwords, and personal data never appear in logs, spans, metrics,
  error messages, or URLs.
- `make secrets-scan` (gitleaks) passes before review.

## Network and resources

- Every server sets timeouts (Go `http.Server` `ReadHeaderTimeout`, `ReadTimeout`,
  `WriteTimeout`, `IdleTimeout`; Rust tower `TimeoutLayer`) and a body limit (Go
  `http.MaxBytesReader`; Rust `RequestBodyLimitLayer`).
- Every outbound call has a timeout and a context or cancellation path.
- Outbound URLs built from user input are checked against an allowlist of hosts (server
  side request forgery protection).
- Responses set `Content-Security-Policy`, `X-Content-Type-Options: nosniff`, and
  `frame-ancestors 'none'` unless a page must be embedded.

## Dependencies

- Add a dependency only with a one-line justification in the pull request.
- `make go-vuln` (govulncheck) and `make rust-deny` (cargo-deny: advisories, licences,
  banned crates) pass before review.

## Containers and Kubernetes

Every workload in a kustomize `base` sets:

- a minimal image (distroless or `scratch`), pinned by version, running as a non-root
  user;
- `securityContext`: `runAsNonRoot: true`, `readOnlyRootFilesystem: true`,
  `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`,
  `seccompProfile.type: RuntimeDefault`;
- CPU and memory requests and limits;
- `automountServiceAccountToken: false` unless it talks to the Kubernetes API.

`make trivy PROJECT=<project>` scans manifests for misconfiguration and images for known
vulnerabilities.

## Security testing

- **Abuse-case scenarios**: each threat model mitigation that rejects behaviour becomes an
  acceptance scenario (unauthorized access, oversized input, malformed payloads, replayed
  requests).
- **Fuzzing** for every parser and decoder of untrusted bytes: Go native fuzzing
  (`func FuzzX(f *testing.F)`), Rust `cargo-fuzz`. Commit the seed corpus.
- **Dynamic scanning**: an OWASP ZAP baseline scan against Tilt is added once a project
  serves HTTP.

## Tooling

| Check                         | Command                            | Scope              |
|-------------------------------|------------------------------------|--------------------|
| Secrets in git history        | `make secrets-scan`                | repository         |
| Go static analysis (gosec)    | `make go-lint PROJECT=<project>`   | Go                 |
| Go known vulnerabilities      | `make go-vuln PROJECT=<project>`   | Go                 |
| Rust advisories and licences  | `make rust-deny PROJECT=<project>` | Rust               |
| Manifests and images          | `make trivy PROJECT=<project>`     | Kubernetes, images |
