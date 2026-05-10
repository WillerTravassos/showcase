# Session 18 — Infrastructure & Kubernetes

## Claude Code Delegation Prompt

```
You are implementing Session 18 of the STD Clinic Management System.

Before writing any code, read in this order:
  1. specs/CLAUDE.md — hard rules, never violate
  2. specs/go_package_structure.md — project layout, file naming conventions
  3. specs/conventions/tilt.md — Tilt + kind local dev workflow, deploy_service pattern
  4. specs/conventions/iac.md — OpenTofu IaC structure, namespace layout
  5. specs/conventions/database.md — CNPG cluster manifests, per-jurisdiction layout
  6. specs/conventions/tgv.md — data residency requirements per jurisdiction,
     audit archive retention (10 years), S3 bucket object lock

All application sessions (01–17) are complete. All binaries build successfully.

Your job is to create all Dockerfiles, Kubernetes manifests (Helm charts),
the Tiltfile for local dev, OpenTofu IaC modules, and Prometheus alerting rules.

Do not use docker-compose as the primary dev workflow — use Tilt + kind.
Do not reference generic postgres images — use CNPG operator manifests.
Do not write Terraform — write OpenTofu (.tofu files).

Work autonomously. Add TODO comments for genuine ambiguities.
When done: helm lint deploy/charts/* && tofu validate infra/environments/dev
```

---

## Dependencies
All sessions (01–17) complete.

---

## Directory Structure to Create

```
build/
├── Dockerfile.api
├── Dockerfile.parser      # requires tesseract + poppler
├── Dockerfile.patient
├── Dockerfile.admin
└── Dockerfile.notifier

deploy/
├── charts/
│   ├── backend-api/
│   ├── test-parser/
│   ├── patient-app/
│   ├── admin-portal/
│   └── notification-worker/
├── cnpg/
│   ├── cluster-ca.yaml    # stdclinic-ca CNPG cluster
│   ├── cluster-us.yaml
│   ├── cluster-br.yaml
│   └── cluster-dev.yaml   # single-instance dev cluster for kind
├── nats/
│   ├── values.prod.yaml
│   └── values.dev.yaml
└── dev/
    ├── namespace.yaml
    ├── backend-api.yaml
    ├── patient-app.yaml
    ├── admin-portal.yaml
    ├── test-parser.yaml
    └── notification-worker.yaml

infra/
├── modules/
│   ├── eks-cluster/
│   ├── cnpg-cluster/
│   ├── object-storage/
│   ├── networking/
│   └── dns/
└── environments/
    ├── dev/
    ├── staging/
    │   ├── ca/
    │   ├── us/
    │   └── br/
    └── production/
        ├── ca/
        ├── us/
        └── br/

k8s/
├── base/
│   ├── namespaces.yaml
│   ├── network-policies.yaml
│   └── rbac.yaml
└── monitoring/
    ├── alerts.yaml        # PrometheusRule CRD
    └── dashboards/
        └── backend-api.json

Tiltfile
scripts/
├── kind-config.yaml
└── cluster-bootstrap.sh
```

---

## Deliverables

### 1. Dockerfiles

All images: non-root user, read-only root filesystem where possible.
Use `gcr.io/distroless/static-debian12` for Go binaries without system dependencies.

`build/Dockerfile.api`:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
# All dependencies are vendored — no network access needed at build time.
COPY go.mod go.sum vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=builder /api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
```

`build/Dockerfile.parser` — requires system packages for Tesseract and poppler:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum vendor/ ./vendor/
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o /parser ./cmd/parser

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       tesseract-ocr tesseract-ocr-fra tesseract-ocr-eng poppler-utils \
    && rm -rf /var/lib/apt/lists/*
RUN useradd -u 10001 -m nonroot
COPY --from=builder /parser /parser
USER nonroot
ENTRYPOINT ["/parser"]
```

Equivalent Dockerfiles for `patient`, `admin`, `notifier` following the distroless + `-mod=vendor` pattern.
Note: always copy `vendor/` into the builder stage — never run `go mod download` in CI or Docker (all deps are vendored).

### 2. Helm Charts

Each chart under `deploy/charts/{service}/` with:
- `Chart.yaml`
- `values.yaml`
- `templates/deployment.yaml`
- `templates/service.yaml`
- `templates/hpa.yaml`
- `templates/serviceaccount.yaml`

`values.yaml` example (`backend-api`):
```yaml
replicaCount: 3
image:
  repository: {registry}/stdclinic-api
  tag: latest
  pullPolicy: IfNotPresent

resources:
  requests: { cpu: 250m, memory: 256Mi }
  limits:   { cpu: 1000m, memory: 512Mi }

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 60

service:
  port: 8080

# Secrets injected via external-secrets operator — not in values.yaml
```

HPA for `test-parser` must include NATS queue depth metric:
```yaml
# deploy/charts/test-parser/templates/hpa.yaml
metrics:
  - type: External
    external:
      metric:
        name: nats_consumer_num_pending
        selector:
          matchLabels: { stream: parser-inbound }
      target:
        type: Value
        value: "20"
```

### 3. CNPG Cluster Manifests

`deploy/cnpg/cluster-ca.yaml` — production CA cluster per `specs/conventions/database.md`.
`deploy/cnpg/cluster-us.yaml` — US cluster in `us-east-1`.
`deploy/cnpg/cluster-br.yaml` — BR cluster in `sa-east-1`.

`deploy/cnpg/cluster-dev.yaml` — single-instance dev cluster for kind:
```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: stdclinic-dev
  namespace: std-data
spec:
  instances: 1
  postgresql:
    parameters:
      max_connections: "100"
  bootstrap:
    initdb:
      database: stdclinic_dev
      owner: app
      secret:
        name: stdclinic-dev-app-secret
  storage:
    size: 10Gi
    storageClass: standard  # kind default storage class
```

Dev cluster uses schemas per jurisdiction (ca, us, br) in one database —
acceptable only in local dev. Production uses separate clusters.

### 4. Network Policies (`k8s/base/network-policies.yaml`)

Default-deny all ingress in every namespace, then explicit allows:

```yaml
# Default deny
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: default-deny-ingress, namespace: std-system }
spec:
  podSelector: {}
  policyTypes: [Ingress]
# Repeat for std-patient, std-admin, std-data, std-monitoring

# Allow patient-app → backend API
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: allow-patient-to-api, namespace: std-system }
spec:
  podSelector:
    matchLabels: { app: backend-api }
  ingress:
    - from:
        - namespaceSelector: { matchLabels: { name: std-patient } }
      ports: [{ port: 8080 }]
```

Required allow rules:
- `std-patient` → `std-system` (backend-api :8080)
- `std-admin` → `std-system` (backend-api :8080)
- `std-system` (parser) → `std-system` (backend-api :8080)
- `std-system` (notifier) → `std-system` (backend-api :8080)
- `std-system` → `std-data` (CNPG :5432, NATS :4222, Redis :6379)
- `std-monitoring` → all namespaces (metrics :9090)
- ingress controller → `std-patient` :8081, `std-admin` :8082

### 5. Tiltfile

```python
# Tiltfile
load('ext://helm_resource', 'helm_resource')
load('ext://namespace', 'namespace_create')

# Namespaces
namespace_create('std-system')
namespace_create('std-patient')
namespace_create('std-admin')
namespace_create('std-data')
namespace_create('std-monitoring')

# CNPG operator (installed once via bootstrap script)
# NATS
helm_resource('nats', 'nats/nats',
  namespace='std-data',
  flags=['--values', 'deploy/nats/values.dev.yaml'],
  labels=['data'])

# CNPG dev cluster
k8s_yaml('deploy/cnpg/cluster-dev.yaml')
k8s_resource('stdclinic-dev', labels=['data'])

# Migrations (after DB ready)
local_resource('migrate-dev',
  cmd='make migrate-up JURISDICTION=dev',
  resource_deps=['stdclinic-dev'],
  labels=['data'])

# Backend API
docker_build('stdclinic-api', '.',
  dockerfile='build/Dockerfile.api',
  live_update=[
    sync('./internal', '/app/internal'),
    sync('./cmd/api', '/app/cmd/api'),
    run('go build -o /api ./cmd/api', trigger=['./internal/', './cmd/api/']),
    restart_container(),
  ])
k8s_yaml('deploy/dev/backend-api.yaml')
k8s_resource('backend-api',
  port_forwards=['8080:8080'],
  resource_deps=['stdclinic-dev', 'nats', 'migrate-dev'],
  labels=['backend'])

# Patient app
docker_build('stdclinic-patient', '.',
  dockerfile='build/Dockerfile.patient',
  live_update=[
    sync('./internal', '/app/internal'),
    sync('./web/patient', '/app/web/patient'),
    restart_container(),
  ])
k8s_yaml('deploy/dev/patient-app.yaml')
k8s_resource('patient-app',
  port_forwards=['8081:8081'],
  resource_deps=['backend-api'],
  labels=['frontend'])

# Admin portal
docker_build('stdclinic-admin', '.',
  dockerfile='build/Dockerfile.admin',
  live_update=[
    sync('./internal', '/app/internal'),
    sync('./web/admin', '/app/web/admin'),
    restart_container(),
  ])
k8s_yaml('deploy/dev/admin-portal.yaml')
k8s_resource('admin-portal',
  port_forwards=['8082:8082'],
  resource_deps=['backend-api'],
  labels=['frontend'])

# Test parser
docker_build('stdclinic-parser', '.', dockerfile='build/Dockerfile.parser')
k8s_yaml('deploy/dev/test-parser.yaml')
k8s_resource('test-parser',
  resource_deps=['backend-api', 'nats'],
  labels=['workers'])

# Notification worker
docker_build('stdclinic-notifier', '.', dockerfile='build/Dockerfile.notifier')
k8s_yaml('deploy/dev/notification-worker.yaml')
k8s_resource('notification-worker',
  resource_deps=['backend-api', 'nats'],
  labels=['workers'])

# Dev seed data (manual trigger)
local_resource('seed',
  cmd='go run ./scripts/seed/main.go',
  resource_deps=['migrate-dev'],
  auto_init=False,
  labels=['data'])
```

### 6. OpenTofu — Production Module

`infra/modules/cnpg-cluster/main.tofu` — creates EKS cluster resource definitions for
CNPG operator, IAM roles for IRSA (pod-level S3 access), and backup S3 bucket.

`infra/modules/object-storage/main.tofu`:
```hcl
# TGV: audit archive bucket with 10-year object lock
resource "aws_s3_bucket" "audit_archive" {
  bucket = "stdclinic-audit-archive-${var.jurisdiction}"
  tags   = local.common_tags
}
resource "aws_s3_bucket_object_lock_configuration" "audit" {
  bucket = aws_s3_bucket.audit_archive.id
  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 10
    }
  }
}
resource "aws_s3_bucket_versioning" "audit" {
  bucket = aws_s3_bucket.audit_archive.id
  versioning_configuration { status = "Enabled" }
}

# Parser inbound bucket: auto-delete after 24h
resource "aws_s3_bucket" "parser_inbound" {
  bucket = "stdclinic-parser-inbound-${var.jurisdiction}"
}
resource "aws_s3_bucket_lifecycle_configuration" "parser_inbound" {
  bucket = aws_s3_bucket.parser_inbound.id
  rule {
    id     = "auto-delete"
    status = "Enabled"
    expiration { days = 1 }
  }
}
```

`infra/environments/production/ca/main.tofu` uses relative module references:
```hcl
module "networking" {
  source       = "../../../modules/networking"
  jurisdiction = "ca"
  environment  = "production"
}
module "eks" {
  source       = "../../../modules/eks-cluster"
  # ...
}
module "storage" {
  source       = "../../../modules/object-storage"
  jurisdiction = "ca"
  environment  = "production"
}
```

### 7. Prometheus Alerts (`k8s/monitoring/alerts.yaml`)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: stdclinic-alerts
  namespace: std-monitoring
spec:
  groups:
    - name: stdclinic
      rules:
        - alert: HighErrorRate
          expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.01
          for: 5m
          labels: { severity: critical }
          annotations: { summary: "API error rate above 1%" }

        - alert: HighLatencyP99
          expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 0.5
          for: 5m
          labels: { severity: warning }

        - alert: AuditWriteFailure
          expr: increase(audit_write_failures_total[5m]) > 0
          for: 0m
          labels: { severity: critical }
          annotations: { summary: "Audit log write failure — TGV certification at risk" }

        - alert: ParserReviewQueueHigh
          expr: nats_consumer_num_pending{consumer="parser-review"} > 10
          for: 10m
          labels: { severity: warning }

        - alert: DeadLetterQueueNonEmpty
          expr: nats_consumer_num_pending{consumer="dead-letter"} > 0
          for: 1m
          labels: { severity: critical }

        - alert: MADODeclarationOverdue
          expr: stdclinic_mado_declarations_overdue_total > 0
          for: 0m
          labels: { severity: critical }
          annotations: { summary: "Overdue MADO declaration — regulatory deadline missed" }

        - alert: CNPGReplicationLag
          expr: cnpg_pg_replication_lag > 30
          for: 5m
          labels: { severity: warning }
          annotations: { summary: "CNPG replica lag > 30s" }
```

### 8. Metrics Instrumentation

Add to all HTTP binaries if not already present from earlier sessions:

```go
// internal/metrics/metrics.go
var (
    HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "http_requests_total",
    }, []string{"method", "path", "status"})

    HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Buckets: prometheus.DefBuckets,
    }, []string{"method", "path"})

    AuditWriteFailures = promauto.NewCounter(prometheus.CounterOpts{
        Name: "audit_write_failures_total",
    })

    ParserRunsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "parser_runs_total",
    }, []string{"outcome"}) // success | needs_review | failed

    // TGV: MADO overdue count for alerting
    MADODeclarationsOverdue = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "stdclinic_mado_declarations_overdue_total",
        Help: "Number of MADO declarations past their submission deadline",
    })
)
```

Expose `/metrics` on all HTTP services. NetworkPolicy: allow `std-monitoring` scrape only.

### 9. Makefile

```makefile
.PHONY: dev dev-reset build lint test test-integration migrate-up migrate-down sqlc helm-lint tofu-validate

dev:          ## Start Tilt dev environment
	tilt up

dev-down:     ## Stop Tilt (keeps kind cluster)
	tilt down

dev-reset:    ## Destroy and recreate kind cluster
	kind delete cluster --name stdclinic-dev
	bash scripts/cluster-bootstrap.sh
	tilt up

build:        ## Build all Docker images
	docker build -f build/Dockerfile.api     -t stdclinic-api .
	docker build -f build/Dockerfile.parser  -t stdclinic-parser .
	docker build -f build/Dockerfile.patient -t stdclinic-patient .
	docker build -f build/Dockerfile.admin   -t stdclinic-admin .
	docker build -f build/Dockerfile.notifier -t stdclinic-notifier .

lint:         ## Run golangci-lint
	golangci-lint run -mod=vendor ./...

test:         ## Run unit tests
	go test -mod=vendor ./...

test-integration: ## Run integration tests (requires running Tilt dev env)
	TILT_CI=1 go test -mod=vendor ./...

migrate-up:   ## Apply pending migrations (JURISDICTION=ca|us|br|dev)
	migrate -path migrations \
	  -database "$(DATABASE_URL_$(shell echo $(JURISDICTION) | tr a-z A-Z))" up

migrate-down: ## Roll back one migration
	migrate -path migrations \
	  -database "$(DATABASE_URL_$(shell echo $(JURISDICTION) | tr a-z A-Z))" down 1

sqlc:         ## Regenerate sqlc query code
	sqlc generate

helm-lint:    ## Lint all Helm charts
	helm lint deploy/charts/backend-api
	helm lint deploy/charts/test-parser
	helm lint deploy/charts/patient-app
	helm lint deploy/charts/admin-portal
	helm lint deploy/charts/notification-worker

tofu-validate: ## Validate all OpenTofu environments
	cd infra/environments/dev && tofu init -backend=false && tofu validate
	cd infra/environments/production/ca && tofu init -backend=false && tofu validate
```

---

## Validation Checks

Run before declaring done:

```bash
# All Go builds pass
go build -mod=vendor ./...
go vet -mod=vendor ./...

# All images build
docker build -f build/Dockerfile.api .
docker build -f build/Dockerfile.parser .
docker build -f build/Dockerfile.patient .
docker build -f build/Dockerfile.admin .
docker build -f build/Dockerfile.notifier .

# Helm charts valid
helm lint deploy/charts/*

# OpenTofu valid
cd infra/environments/dev && tofu init -backend=false && tofu validate

# Tilt config valid (dry run)
tilt ci --timeout 2m
```

---

## Done Criteria

- All 5 Docker images build with non-root users
- All 5 Helm charts pass `helm lint`
- CNPG cluster manifests present for all 4 environments (ca, us, br, dev)
- `Tiltfile` starts all services in kind with live reload
- OpenTofu modules and environments are structurally valid
- Network policies cover all required service pairs
- MADO overdue metric present for TGV alerting
- Audit archive S3 bucket has 10-year object lock (TGV retention)
- No docker-compose references in primary dev workflow
- No Terraform references — OpenTofu only (`.tofu` files)
