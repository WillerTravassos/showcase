# Kubernetes workload standard

Every service in this repository runs in Kubernetes and follows the pod lifecycle below,
whatever its language. A service that does not drain on stop drops requests on every
rollout.

## Shutdown sequence

When Kubernetes deletes a pod, two things start at the same time: the kubelet begins
stopping the container, and the pod's endpoint is marked terminating so the gateway and
Cilium stop routing to it. The second takes a few seconds to reach every proxy, so a
pod that stops listening at once still receives traffic and fails it.

| Step | Owner | What happens |
|------|-------|--------------|
| 1 | Kubernetes | `preStop` sleep (5 seconds) while routing to the pod stops |
| 2 | Kubernetes | SIGTERM to the container |
| 3 | App | Report not ready; stop accepting connections |
| 4 | App | Drain in-flight requests (at most 15 seconds) |
| 5 | App | Close dependencies (database pools, broker connections) |
| 6 | App | Flush telemetry (at most 5 seconds); exit 0 |
| 7 | Kubernetes | SIGKILL if the pod is still running at the end of the grace period (30 seconds) |

The steps 1 to 6 budget (5 + 15 + 5 seconds) leaves a 5 second margin inside the grace
period. Change a timeout only together with the others.

- Requests that are draining keep running: their context must not be cancelled by the
  stop signal (Go: `http.Server.BaseContext` returns `context.WithoutCancel(ctx)`).
- The `preStop` wait uses the native `sleep` action, which needs no shell in the image
  (distroless images have none).
- Exit 0 after a clean drain. A non-zero exit or a SIGKILL (exit 137) on a rollout is a
  bug.

## Probes

Probes are served over plain HTTP on their own port (`probe`, 8081, `PROBE_ADDR`). The
gateway never routes to it, so the internet cannot read them.

| Path | Answers | Checks |
|------|---------|--------|
| `/livez` | 200 while the process can serve; failing restarts the container | Nothing external. A dependency outage must never restart every pod |
| `/readyz` | 200 when the pod should receive traffic, 503 otherwise | Dependencies needed to serve; 503 during startup and once the stop signal arrives |

- Bodies are plain text (`OK`, `Service Unavailable`) with `Cache-Control: no-store`. No
  version, hostnames, or check details.
- Probe requests are not traced or logged; they would flood Tempo and Loki.
- gRPC-only services may use the `grpc.health.v1` protocol with native `grpc` probes
  instead.

## Manifests

Every service's `base/kustomization.yaml` lists the shared component, which adds the
probes, the `probe` port, `PROBE_ADDR`, the `preStop` sleep, and the grace period:

```yaml
components:
  - ../../../../deployments/components/workload-lifecycle
```

## Implementations

- **Go**: `clinic-scheduler/internal/lifecycle` (`lifecycle.Run`). Dependency closes
  and the telemetry flush are `defer`s in the service's `Run`, so they run after the
  drain in reverse order.
- **Rust** and **TypeScript**: implement this standard when the project ships its first
  service.

## Verifying

An acceptance test restarts the deployment while patients load the service and asserts
that no request fails and the replaced pod exits 0
(`clinic-scheduler/test/acceptance/portal_restart_test.go`).
