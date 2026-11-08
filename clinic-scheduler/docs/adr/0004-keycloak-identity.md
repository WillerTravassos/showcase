# 0004. Keycloak with patient and workforce realms

Status: accepted
Date: 2026-11-08

## Context

Patients register themselves; the workforce (staff, doctors, admins, auditors) must come
from the Santé Québec directory through federation or LDAPS (TGV S06.07, S06.24, T20,
I01). TGV also requires mandatory MFA (S06.06), lockout after five failed attempts
(S06.16), generic login errors (S06.17), configurable idle logout (S06.19), and alerts on
sign-in and profile change (S06.22). The approach should be cloud native.

## Decision

Run [Keycloak](https://www.keycloak.org) (a CNCF incubating project) with two realms:

- `patient`: self-registration, email verification, optional MFA.
- `workforce`: accounts created by admins or federated from the directory, MFA required,
  no self-registration.

Locally, a fake "Santé Québec directory" (OpenLDAP, or a third realm acting as Entra ID
over OIDC) federates into `workforce`. Each BFF runs the OIDC authorization code flow
with PKCE, keeps tokens server-side, and gives the browser only an opaque `HttpOnly`,
`Secure`, `SameSite=Lax` session cookie. Domain services check the access token
(signature through JWKS, issuer, audience, expiry) on every call. Roles come from realm
roles in the token. Login themes use Keycloak's FreeMarker themes, styled with the
project palette, in English and French.

## Alternatives

- **Ory Kratos**: Go, lightweight, headless (login UI built in HTMX). No LDAP and, as far
  as known, no built-in lockout in the self-hosted edition, both TGV items. Not in the
  CNCF.
- **Zitadel**: Go, built-in login UI and LDAP, but limited theming and a smaller
  ecosystem.
- **Building authentication**: password storage, MFA, recovery, and federation are a
  large surface with high risk.

## Consequences

- TGV login controls are realm settings, verified by acceptance scenarios.
- Keycloak is a JVM workload: plan its memory in KinD. Realm config is imported from
  versioned JSON so the cluster rebuilds identically.
- Login screens do not use templ; they share colour tokens with the portals.
