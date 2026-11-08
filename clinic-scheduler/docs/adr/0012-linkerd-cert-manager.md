# 0012. Linkerd mTLS with cert-manager as the CA

Status: accepted
Date: 2026-11-08

## Context

TGV requires authenticated and encrypted communication between components (S06.10,
S07.02, S11.02) and HTTPS for web access (T08). Learning a Kubernetes service mesh is a
project goal. Linkerd generates one-year certificates by hand at install unless an
external issuer manages them.

## Decision

- [cert-manager](https://cert-manager.io) with a self-signed root CA, and
  [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) to distribute the
  trust bundle.
- cert-manager issues and rotates Linkerd's trust anchor and identity issuer, and the
  gateway TLS certificates for `*.localhost`.
- [Linkerd](https://linkerd.io) edge release, pinned version, injected into every
  application namespace and NATS. Pod-to-pod traffic gets mTLS (mutual TLS: both sides
  prove identity) automatically.
- Postgres keeps CNPG's own TLS; services connect with `sslmode=verify-full`.
- Linkerd `AuthorizationPolicy`: domain services accept only meshed identities of their
  allowed callers. BFFs and `fhir-gateway` accept unmeshed traffic only from the Cilium
  gateway.
- Service-to-service calls also carry the user's JWT (ADR 0004): the mesh proves which
  workload is calling, the token proves which person.

## Alternatives

- **Cilium mutual authentication and WireGuard**: one tool fewer, but mutual auth is
  still beta and gives less to learn about meshes.
- **Istio ambient mode**: no sidecars, more concepts to learn first.
- **cert-manager certificates per service, TLS in each binary**: no mesh, and every
  service manages its own certificates and rotation.

## Consequences

- **Gateway hop gap**: Cilium's gateway Envoy is not a meshed pod, so the gateway to BFF
  hop is not Linkerd mTLS. On one KinD node that traffic never leaves the host; in a
  multi-node cluster, enable Cilium WireGuard for node-to-node encryption.
- Linkerd stable releases are vendor-only; edge releases are pinned and upgraded on
  purpose.
- Cilium needs `socketLB.hostNamespaceOnly=true` so its socket load balancing does not
  bypass the Linkerd proxy.
