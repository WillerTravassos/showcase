# Local Development — Tilt + kind

---

## Overview

Local development runs on a real Kubernetes cluster (kind) managed by Tilt.
What you run locally is structurally identical to production — same CNPG operator,
same NATS JetStream, same Kubernetes networking.

**Never use `docker-compose` as the primary dev workflow.**

---

## Directory Structure

```
tilt/
├── services/       # Local-only service definitions (WireMock, Jaeger, etc.) — never deployed to production
├── deploy/         # Tilt-managed kustomize overlays — local and CI only
│   ├── api/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── local/
│   │       └── ci/
│   ├── parser/
│   ├── patient/
│   ├── admin/
│   └── notifier/
└── tilt.d/         # Per-service tiltfiles — loaded dynamically by Tiltfile
    ├── api.tiltfile
    ├── parser.tiltfile
    ├── patient.tiltfile
    ├── admin.tiltfile
    └── notifier.tiltfile
```

`tilt/deploy/` coexists with the root `deploy/` directory. The root `deploy/` holds
the full kustomize structure for all environments (`dev`, `stg`, `prod`).
`tilt/deploy/` holds only the overlays Tilt needs: `local` and `ci`.

---

## `Tiltfile` — Entry Point

The root `Tiltfile` is the local dev entry point. It dynamically includes every
tiltfile found under `tilt/tilt.d/` and calls `deploy_service(cfg)` on each one
that defines it.

```python
# Tiltfile

cfg = struct(
    # Shared configuration passed to every service tiltfile
    registry = 'localhost:5005',
    namespace = 'std-system',
)

# Dynamically load all service tiltfiles
for f in listdir('tilt/tilt.d'):
    if f.endswith('.tiltfile'):
        include('tilt/tilt.d/' + f)
        if defined('deploy_service'):
            deploy_service(cfg)
```

**Rules:**
- No service-specific resource definitions in `Tiltfile` itself — those belong in `tilt.d/`.
- `Tiltfile` sets up shared infrastructure only (CNPG cluster, NATS, namespaces).
- All flags and their defaults are declared at the top of `Tiltfile`.

---

## `TiltfileCI` — CI Entry Point

`TiltfileCI` is the CI entry point. It includes the root `Tiltfile` and then
disables services that are not needed in CI (e.g. local observability tools).

```python
# TiltfileCI

include('Tiltfile')

# Disable local-only services that are unnecessary in CI
# e.g. k8s_resource('jaeger', labels=['observability'], auto_init=False)
```

CI runs `tilt ci -f TiltfileCI`.

---

## Per-Service Tiltfiles (`tilt/tilt.d/`)

Each service has its own tiltfile. A tiltfile **may** define a `deploy_service(cfg)`
function that the main `Tiltfile` will call. This is the primary extension point
for adding a service to the local environment.

```python
# tilt/tilt.d/api.tiltfile

def deploy_service(cfg):
    docker_build(
        'stdclinic-api',
        '.',
        dockerfile = 'build/Dockerfile.api',
        live_update = [
            sync('./internal', '/app/internal'),
            sync('./cmd/api', '/app/cmd/api'),
            run('go build -o /api ./cmd/api', trigger=['./internal/', './cmd/api/']),
            restart_container(),
        ],
    )

    k8s_yaml(kustomize('tilt/deploy/api/overlays/local'))

    k8s_resource(
        'backend-api',
        port_forwards = ['8080:8080'],
        resource_deps = ['stdclinic-dev', 'nats'],
        labels = ['backend'],
    )
```

**Rules:**
- One tiltfile per service binary — matches `cmd/<service>/`.
- The tiltfile always uses `kustomize('tilt/deploy/<service>/overlays/local')` for its manifests.
- Resource dependencies (`resource_deps`) on shared infrastructure must be declared explicitly.
- Live update rules sync source files first, then rebuild, then restart — in that order.

---

## Local-Only Services (`tilt/services/`)

Services that run locally but are never deployed to production. Each is defined as
its own Tilt resource, typically backed by a container or helm chart.

Examples:
- **WireMock** — HTTP mock server for external API stubs (fax services, DSP, etc.)
- **Jaeger** — distributed tracing UI
- **Mailpit** — local SMTP catch-all for email testing

These are set up in `Tiltfile` directly (not in `tilt.d/`) since they are
infrastructure, not services.

---

## `tilt/deploy/` — Kustomize Overlays

`tilt/deploy/<service>/base/` mirrors the structure of `deploy/<service>/base/` but
may contain Tilt-specific simplifications (e.g. lower resource limits, dev secrets).

`tilt/deploy/<service>/overlays/local/` — overrides for local dev:
- `imagePullPolicy: Never` (images built locally by Tilt/kind)
- Secrets as plaintext (dev values only — never real credentials)
- Reduced resource requests/limits
- HPA disabled

`tilt/deploy/<service>/overlays/ci/` — overrides for CI:
- Differs from `local` only when CI needs different configuration (e.g. different
  port mappings, or a service that should be disabled in CI but enabled locally).

---

## Prerequisites

Install via Makefile targets (which call the scripts in `scripts/tools/`):

```bash
make install-kind     # scripts/tools/install-kind.sh
make install-tilt     # scripts/tools/install-tilt.sh
make install-ctlptl   # scripts/tools/install-ctlptl.sh (optional: manages kind clusters)
```

Additional tools needed:
- `kubectl` — `brew install kubectl`
- `helm` — `brew install helm`
- `golangci-lint` — `brew install golangci-lint`
- `sqlc` — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

---

## Cluster Bootstrap (one-time)

```bash
make cluster-up    # Creates kind cluster and installs CNPG + NATS operators
```

Internally this runs `scripts/local-env/kind-cluster.sh`, which:

1. Creates the kind cluster (`stdclinic-dev`) with port mappings for all services
2. Installs the CNPG operator via Helm
3. Installs the NATS operator via Helm
4. Applies namespace manifests from `tilt/kustomization/namespaces/`

The kind cluster config mounts the repo root and maps service ports:

```yaml
# scripts/local-env/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraPortMappings:
      - containerPort: 30080   # backend API
        hostPort: 8080
      - containerPort: 30081   # patient app
        hostPort: 8081
      - containerPort: 30082   # admin portal
        hostPort: 8082
      - containerPort: 30222   # NATS
        hostPort: 4222
```

---

## Daily Workflow

```bash
make tilt-up      # tilt up
make tilt-down    # tilt down (keeps cluster alive)
make cluster-down # kind delete cluster --name stdclinic-dev
```

`tilt up` opens the Tilt UI at `http://localhost:10350`.

---

## Live Reload Behaviour

| Change | What Tilt does |
|--------|----------------|
| Go source in `internal/` or `cmd/` | Rebuilds binary, syncs to container, restarts |
| Template in `web/*/templates/` | Syncs file to container — templates reload on next request |
| TypeScript in `web/*/ts/` | Runs esbuild locally, syncs compiled JS |
| Migration in `migrations/` | Triggers `make migrate-up` local resource |
| Manifest in `tilt/deploy/*/overlays/local/` | Applies updated manifest to cluster |

---

## Dev Environment Endpoints

| Service | URL |
|---------|-----|
| Backend API | `http://localhost:8080` |
| Patient App | `http://localhost:8081` |
| Admin Portal | `http://localhost:8082` |
| NATS monitoring | `http://localhost:8222` |
| Tilt UI | `http://localhost:10350` |

---

## Makefile Targets

```makefile
# Tool installation
install-kind:       ## Install kind
	./scripts/tools/install-kind.sh

install-tilt:       ## Install Tilt
	./scripts/tools/install-tilt.sh

install-ctlptl:     ## Install ctlptl (kind cluster manager)
	./scripts/tools/install-ctlptl.sh

install-tools: install-kind install-tilt install-ctlptl  ## Install all local dev tools

# Cluster lifecycle
cluster-up:         ## Bootstrap kind cluster with CNPG and NATS
	./scripts/local-env/kind-cluster.sh up

cluster-down:       ## Destroy kind cluster
	./scripts/local-env/kind-cluster.sh down

# Tilt lifecycle
tilt-up:            ## Start local dev environment
	tilt up

tilt-down:          ## Stop Tilt (keeps cluster)
	tilt down

tilt-ci:            ## Run Tilt in CI mode
	tilt ci -f TiltfileCI

# Database
migrate-up:         ## Apply pending migrations (JURISDICTION=ca|us|br|dev)
	migrate -path migrations -database "$(DATABASE_URL_$(shell echo $(JURISDICTION) | tr a-z A-Z))" up

migrate-down:       ## Roll back one migration (JURISDICTION=ca|us|br|dev)
	migrate -path migrations -database "$(DATABASE_URL_$(shell echo $(JURISDICTION) | tr a-z A-Z))" down 1

# Code generation and quality
sqlc:               ## Regenerate sqlc query code
	sqlc generate

lint:               ## Run golangci-lint
	golangci-lint run ./...

test:               ## Run unit tests
	go test -mod=vendor ./...

test-integration:   ## Run integration tests against live cluster (requires TILT_CI=1)
	TILT_CI=1 go test -mod=vendor ./...
```
