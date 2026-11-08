# 0011. Cilium as CNI and Gateway API implementation

Status: accepted
Date: 2026-11-08

## Context

The internal services must never be reachable from outside the cluster; only the two
portals and `fhir-gateway` are public. Kubernetes NetworkPolicy must actually be
enforced. The long-term direction is a multi-cloud setup (with NATS JetStream and,
later, CockroachDB). The current KinD cluster (`tilt/cluster.yaml`) uses kindnet, the
default kube-proxy, and has no host port mappings.

## Decision

- Replace kindnet with [Cilium](https://cilium.io): `networking.disableDefaultCNI: true`
  and `kubeProxyMode: none` in `tilt/cluster.yaml`, Cilium installed with
  `kubeProxyReplacement=true` as a shared Tilt resource. Hubble gives flow visibility.
- Cilium implements the Gateway API (`gatewayAPI.enabled=true`, Gateway API CRDs
  installed first). One Gateway in host-network mode listens on 80 and 443, with
  `extraPortMappings` for those ports on the control-plane node.
- Routes: `patient.localhost` to `patient-web`, `admin.localhost` to `admin-web`,
  `fhir.localhost` to `fhir-gateway`, each with its own TLS certificate (ADR 0012).
- NetworkPolicy: deny all by default per namespace, then allow gateway to BFFs and
  `fhir-gateway`, BFFs to domain services, services to their database, NATS, OpenBao,
  Keycloak, and the OTLP collector.
- Cilium ClusterMesh is the path to multi-cloud later.

## Alternatives

- **Envoy Gateway or Traefik on kindnet**: kindnet's NetworkPolicy support is limited,
  and neither gives ClusterMesh.
- **Tilt port-forwards only**: no real ingress path to test.

## Consequences

- Changing the CNI recreates the cluster (`make cluster-reset`): every project's local
  state is lost. Ticket 0 carries the warning.
- Linkerd on Cilium needs `socketLB.hostNamespaceOnly=true` (ADR 0012).
- Host ports 80 and 443 need a rootful Docker; otherwise use 8080 and 8443 (checked in
  ticket 0, along with the exact host-network option name for the installed version).
- Browsers resolve `*.localhost` to 127.0.0.1; the Go acceptance driver dials 127.0.0.1
  and sets the `Host` header.
