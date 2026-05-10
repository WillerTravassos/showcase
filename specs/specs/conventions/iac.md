# IaC Conventions — OpenTofu (AWS Primary)

---

## Tool

**OpenTofu** (open-source Terraform fork) manages all cloud infrastructure.
Never use Terraform or any cloud console to create resources that should be managed as code.
The only exception is the OpenTofu state backend itself (S3 bucket + DynamoDB table),
which is bootstrapped manually once per AWS account.

---

## Directory Layout

```
infra/
├── modules/                    # Reusable internal modules
│   ├── cnpg-cluster/           # CNPG cluster + IAM + S3 backup bucket
│   ├── eks-cluster/            # EKS cluster + node groups + IRSA
│   ├── nats-cluster/           # NATS JetStream via Helm in EKS
│   ├── object-storage/         # S3 bucket with encryption + versioning
│   ├── dns/                    # Route53 records
│   └── networking/             # VPC, subnets, security groups
│
├── environments/
│   ├── dev/                    # Shared dev environment (single region)
│   │   ├── main.tofu
│   │   ├── variables.tofu
│   │   ├── outputs.tofu
│   │   └── backend.tofu
│   ├── staging/
│   │   ├── ca/                 # Canada staging
│   │   ├── us/                 # US staging
│   │   └── br/                 # Brazil staging
│   └── production/
│       ├── ca/                 # Canada production (ca-central-1)
│       ├── us/                 # US production (us-east-1)
│       └── br/                 # Brazil production (sa-east-1)
│
├── global/                     # Resources not tied to a region
│   ├── iam/                    # IAM roles and policies
│   └── route53/                # Hosted zones
│
└── scripts/
    ├── init.sh                 # One-time state backend bootstrap
    └── plan-all.sh             # Plan all environments
```

---

## State Management

Remote state in S3 with DynamoDB locking. One state file per environment+jurisdiction.

```hcl
# infra/environments/production/ca/backend.tofu
terraform {
  backend "s3" {
    bucket         = "stdclinic-tofu-state"
    key            = "production/ca/terraform.tfstate"
    region         = "ca-central-1"
    encrypt        = true
    dynamodb_table = "stdclinic-tofu-locks"
  }
}
```

State bucket is in `ca-central-1` (primary). State files for US and BR environments are
also stored in this bucket (state files contain infra metadata, not patient data).

---

## Module Conventions

### Module Interface Pattern

Every module has a consistent interface:

```
modules/{name}/
├── main.tofu       # Resources
├── variables.tofu  # Input variables with descriptions and validation
├── outputs.tofu    # Output values (connection strings, ARNs, etc.)
└── README.md       # What this module creates and how to use it
```

Variable definitions always include `description` and `type`. Validation blocks for
constrained values:

```hcl
# modules/cnpg-cluster/variables.tofu
variable "jurisdiction" {
  description = "Jurisdiction this cluster serves. Determines AWS region and data residency rules."
  type        = string
  validation {
    condition     = contains(["ca", "us", "br"], var.jurisdiction)
    error_message = "Jurisdiction must be ca, us, or br."
  }
}

variable "instance_count" {
  description = "Number of PostgreSQL instances (1 for dev, 3 for production)."
  type        = number
  default     = 3
  validation {
    condition     = var.instance_count >= 1 && var.instance_count <= 5
    error_message = "Instance count must be between 1 and 5."
  }
}
```

### Module Versioning

Internal modules are referenced by relative path. External providers are pinned:

```hcl
# infra/environments/production/ca/main.tofu
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.0"
    }
  }
}

module "eks" {
  source = "../../../modules/eks-cluster"
  # ...
}
```

---

## Per-Jurisdiction Environment Pattern

Each jurisdiction environment (`production/ca`, `production/us`, `production/br`) creates:
- 1 EKS cluster
- 1 CNPG cluster (3 instances)
- NATS JetStream (via Helm resource in EKS)
- VPC with private subnets for DB and workloads
- S3 buckets: `parser-inbound-{jurisdiction}`, `audit-archive-{jurisdiction}`
- IAM roles for IRSA (pod-level AWS permissions)
- Route53 records pointing to the regional EKS load balancer

```hcl
# infra/environments/production/ca/main.tofu
provider "aws" {
  region = "ca-central-1"
}

module "networking" {
  source       = "../../../modules/networking"
  jurisdiction = "ca"
  environment  = "production"
}

module "eks" {
  source         = "../../../modules/eks-cluster"
  jurisdiction   = "ca"
  environment    = "production"
  vpc_id         = module.networking.vpc_id
  subnet_ids     = module.networking.private_subnet_ids
  node_min       = 3
  node_max       = 10
  node_type      = "m6i.xlarge"
}

module "cnpg" {
  source          = "../../../modules/cnpg-cluster"
  jurisdiction    = "ca"
  environment     = "production"
  instance_count  = 3
  storage_size_gb = 200
  backup_bucket   = module.backup_bucket.name
}

module "parser_bucket" {
  source       = "../../../modules/object-storage"
  name         = "stdclinic-parser-inbound-ca"
  jurisdiction = "ca"
  lifecycle_rules = [{
    id      = "auto-delete-processed"
    enabled = true
    expiration_days = 1  # Delete after 24h
  }]
}

module "audit_bucket" {
  source       = "../../../modules/object-storage"
  name         = "stdclinic-audit-archive-ca"
  jurisdiction = "ca"
  versioning   = true
  object_lock  = true
  object_lock_retention_years = 7
}
```

---

## Multi-Cloud Readiness

The module layer abstracts cloud-specific resources. If the project expands beyond AWS:

1. Add a `provider` variable to each module (`aws`, `gcp`, `azure`)
2. Create provider-specific resource blocks under `if var.provider == "aws"` pattern
3. Outputs remain identical regardless of provider

For now, all modules are AWS-only. The abstraction layer is achieved through module interfaces —
not through a Terraform abstraction library.

---

## Secrets Management

Infrastructure-level secrets (DB passwords, encryption keys) are generated by OpenTofu and
stored in AWS Secrets Manager. The CNPG operator and `external-secrets` operator pull from
Secrets Manager into Kubernetes Secrets.

```hcl
# Generate DB password
resource "random_password" "db_app_password" {
  length  = 32
  special = false
}

# Store in Secrets Manager
resource "aws_secretsmanager_secret" "db_app" {
  name = "stdclinic/${var.jurisdiction}/db/app-password"
  # KMS key for encryption
  kms_key_id = aws_kms_key.secrets.id
}

resource "aws_secretsmanager_secret_version" "db_app" {
  secret_id     = aws_secretsmanager_secret.db_app.id
  secret_string = random_password.db_app_password.result
}
```

**Never output secrets from OpenTofu modules.** Secrets flow: Secrets Manager → external-secrets operator → Kubernetes Secret → pod env var.

---

## CI/CD for IaC

```yaml
# .github/workflows/tofu.yml
on:
  pull_request:
    paths: ['infra/**']

jobs:
  plan:
    strategy:
      matrix:
        environment: [staging/ca, staging/us, staging/br]
    steps:
      - uses: opentofu/setup-opentofu@v1
      - run: tofu init
        working-directory: infra/environments/${{ matrix.environment }}
      - run: tofu plan -out=plan.tfplan
        working-directory: infra/environments/${{ matrix.environment }}
      - name: Post plan as PR comment
        # ... post plan output as PR comment
```

Apply to production requires manual approval in GitHub Actions environment protection rules.

---

## Naming Conventions

All AWS resources follow: `stdclinic-{environment}-{jurisdiction}-{resource-type}`

Examples:
- `stdclinic-production-ca-eks` (EKS cluster)
- `stdclinic-production-ca-cnpg` (RDS/CNPG identifier)
- `stdclinic-production-ca-vpc`
- `stdclinic-audit-archive-ca` (S3 bucket — no environment prefix, bucket names are global)

Tags on all resources:
```hcl
default_tags {
  tags = {
    Project      = "stdclinic"
    Environment  = var.environment
    Jurisdiction = var.jurisdiction
    ManagedBy    = "opentofu"
  }
}
```
