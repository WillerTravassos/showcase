# Overview: Infrastructure & Platform

> Reference this file for session 18 and when writing any Kubernetes or deployment config.

---

## Kubernetes Namespace Layout

```
std-system      — backend API, test parser, internal workers
std-patient     — patient web app
std-admin       — admin portal
std-data        — PostgreSQL, Redis, NATS
std-monitoring  — Prometheus, Grafana, Loki, Tempo
std-ingress     — NGINX ingress controller, cert-manager
```

---

## Service Ports

| Service | Internal Port |
|---------|--------------|
| Backend API | 8080 |
| Patient App | 8081 |
| Admin Portal | 8082 |
| Parser | No HTTP (queue only) |
| Notification Worker | No HTTP (queue only) |

---

## Required Environment Variables (all services)

```
DATABASE_URL          postgresql://user:pass@host/dbname?sslmode=require
REDIS_URL             redis://:password@redis:6379/0
NATS_URL              nats://nats:4222
JWT_SECRET            <32+ char secret>
ENCRYPTION_KEY        <64 hex chars = 32 bytes for AES-256>
ENV                   production
LOG_LEVEL             info
OBJECT_STORAGE_BUCKET <bucket name>
OBJECT_STORAGE_REGION <region>
```

---

## HPA Targets

| Deployment | Min | Max | CPU Target | Custom Metric |
|------------|-----|-----|------------|---------------|
| backend-api | 3 | 20 | 60% | RPS > 500/pod |
| test-parser | 2 | 10 | 70% | NATS queue depth > 20 |
| patient-app | 2 | 15 | 60% | — |
| admin-portal | 2 | 8 | 60% | — |
| notification-worker | 2 | 6 | 50% | NATS queue depth > 50 |

---

## Data Residency

Three PostgreSQL instances (or schemas with separate credentials), one per jurisdiction:

| Jurisdiction | Env Var | Notes |
|---|---|---|
| Canada | `DATABASE_URL_CA` | Default for CA patients |
| United States | `DATABASE_URL_US` | US patient PII must not leave US region |
| Brazil | `DATABASE_URL_BR` | BR patient PII must not leave BR region |

The backend routes writes to the appropriate DB based on `patient.jurisdiction`.

---

## Object Storage Buckets

| Bucket | Purpose | Retention | Access |
|--------|---------|-----------|--------|
| `parser-inbound` | Raw PDFs pending parsing | Auto-delete after 24h post-processing | Parser only |
| `audit-archive` | Long-term audit log export | 7 years, immutable object lock | Admin read only |

---

## Observability Stack

- **Metrics**: Prometheus scrape annotations on all pods; Grafana dashboards in `k8s/monitoring/dashboards/`
- **Logs**: JSON structured logs → Loki via Promtail DaemonSet
- **Traces**: OpenTelemetry SDK → Tempo
- **Alerts**: Alertmanager → PagerDuty webhook

Key alerts to define:
- API error rate > 1% over 5 min
- P99 latency > 500ms over 5 min
- NATS dead-letter queue depth > 0
- Parser `needs_review` queue depth > 10
- Any audit write failure

---

## Helm Chart Structure

```
k8s/
├── charts/
│   ├── backend-api/
│   ├── test-parser/
│   ├── patient-app/
│   ├── admin-portal/
│   └── notification-worker/
├── monitoring/
│   └── dashboards/
└── base/
    ├── namespaces.yaml
    ├── network-policies.yaml
    └── rbac.yaml
```

---

## Security Posture

- **Default-deny NetworkPolicy** in every namespace; explicit allow rules per service pair
- **mTLS** via Istio (or Linkerd) service mesh for all pod-to-pod traffic
- **Secrets** via external-secrets operator pulling from Vault or cloud KMS — never `kubectl create secret` from plaintext
- **Non-root containers**: all Dockerfiles must use `USER nonroot`
- **Read-only root filesystem** on all deployments where possible
