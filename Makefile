#####################################
# Target variables
#####################################
# Tools come from devbox (`devbox install`); CI runs these same targets.
# Every recipe resolves tools from the devbox profile first, inside `devbox shell` or not.
export PATH := $(CURDIR)/.devbox/nix/profile/default/bin:$(PATH)
.DEFAULT_GOAL := help

PROJECT ?= online-pong
PROJECT_DIR := $(CURDIR)/$(PROJECT)

KIND_CLUSTER_CONFIG := $(CURDIR)/tilt/cluster.yaml

# Mutation testing only mutates lines changed since this ref.
MUTATION_BASE ?= origin/master
GOOZE_VERSION := v1.3.0

RUST_MANIFEST_PATH := $(PROJECT_DIR)/Cargo.toml
RUST_SYSTEM_TESTS := system-tests
RUST_TEST_PACKAGE ?=
RUST_TEST_NAME_MATCH ?=

# Shared logic to decide between -p package or the whole workspace without Tilt-backed tests
_RUST_TEST_PACKAGE_TARGET = $(if $(RUST_TEST_PACKAGE),-p $(RUST_TEST_PACKAGE),--workspace --exclude $(RUST_SYSTEM_TESTS))

# Pass --project to Tilt only when PROJECT is given on the command line, so tilt_config.json
# (the last project chosen in the Tilt UI) stays the default otherwise.
_TILT_ARGS = $(if $(filter command line,$(origin PROJECT)),-- --project=$(PROJECT),)

.PHONY: help
help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

#####################################
# Development environment
#####################################

.PHONY: dev-setup
dev-setup: ## Enter the dev environment; installs every pinned tool on first run (devbox)
	@command -v devbox >/dev/null || { echo "devbox not found, install it from https://www.jetify.com/docs/devbox/installing-devbox" >&2; exit 1; }
	@devbox shell

#####################################
# Cluster and Tilt
#####################################

.PHONY: cluster-create
cluster-create: ## Create the local kind cluster and registry
	@./scripts/local-env/kind-cluster.sh create ${KIND_CLUSTER_CONFIG}

.PHONY: cluster-delete
cluster-delete: ## Delete the local kind cluster
	@./scripts/local-env/kind-cluster.sh delete ${KIND_CLUSTER_CONFIG}

.PHONY: cluster-reset
cluster-reset: ## Delete and recreate the local kind cluster
	@./scripts/local-env/kind-cluster.sh reset ${KIND_CLUSTER_CONFIG}

.PHONY: tilt-up
tilt-up: cluster-create ## Start Tilt (PROJECT=<name> picks the active project)
	@tilt up $(_TILT_ARGS)

.PHONY: tilt-down
tilt-down: ## Stop Tilt and delete its resources
	@tilt down

.PHONY: tilt-ci
tilt-ci: cluster-create ## Run Tilt until every resource is healthy, then exit (CI)
	@tilt ci $(_TILT_ARGS)

.PHONY: smoke
smoke: ## Run the project's k6 smoke test in the cluster
	@tilt trigger $(PROJECT)-k6-smoke

.PHONY: load
load: ## Run the project's k6 load test in the cluster
	@tilt trigger $(PROJECT)-k6-load

.PHONY: stress
stress: ## Run the project's k6 stress test in the cluster
	@tilt trigger $(PROJECT)-k6-stress

#####################################
# Repository checks
#####################################

.PHONY: secrets-scan
secrets-scan: ## Scan git history for committed secrets (gitleaks)
	@gitleaks git --redact --verbose .

.PHONY: trivy
trivy: ## Scan the project and shared infra for misconfiguration (trivy)
	@trivy config --exit-code 1 $(PROJECT_DIR) $(CURDIR)/tilt/kustomize

.PHONY: trivy-image
trivy-image: ## Scan a container image for vulnerabilities (IMAGE=<ref>)
	@trivy image --exit-code 1 --severity HIGH,CRITICAL $(IMAGE)

#####################################
# Golang
#####################################

.PHONY: go-fmt
go-fmt: ## Format Go code (goimports, golines)
	@cd $(PROJECT_DIR) && golangci-lint fmt

.PHONY: go-lint
go-lint: ## Lint Go code, formatting included
	@cd $(PROJECT_DIR) && golangci-lint run ./...

.PHONY: go-lint-fix
go-lint-fix: ## Lint Go code and apply automatic fixes
	@cd $(PROJECT_DIR) && golangci-lint run --fix ./...

.PHONY: go-arch
go-arch: ## Check package dependency rules (go-arch-lint)
	@cd $(PROJECT_DIR) && go-arch-lint check

.PHONY: go-mocks
go-mocks: ## Regenerate fakes from .mockery.yml
	@cd $(PROJECT_DIR) && mockery

.PHONY: go-test
go-test: ## Run unit tests with the race detector
	@cd $(PROJECT_DIR) && go test -race $$(go list ./... | grep -v '/test/')

.PHONY: go-integration
go-integration: ## Run integration tests against Tilt infrastructure
	@cd $(PROJECT_DIR) && go test -race -count=1 ./services/*/test/integration/...

.PHONY: go-acceptance
go-acceptance: ## Run acceptance tests against the system in Tilt
	@cd $(PROJECT_DIR) && go test -count=1 ./test/acceptance/...

.PHONY: go-mutate
go-mutate: ## Mutation test the lines changed since MUTATION_BASE (gooze)
	@cd $(PROJECT_DIR) && go run gooze.dev/pkg/gooze@$(GOOZE_VERSION) run --diff $(MUTATION_BASE) ./...

.PHONY: go-vuln
go-vuln: ## Check dependencies and code for known vulnerabilities (govulncheck)
	@cd $(PROJECT_DIR) && govulncheck ./...

.PHONY: go-verify
go-verify: go-lint go-arch go-test go-vuln ## Everything a change must pass before review

.PHONY: go-tidy
go-tidy: ## Tidy and verify the Go module
	@cd $(PROJECT_DIR) && go mod tidy && go mod verify

.PHONY: go-cache-clean
go-cache-clean: ## Clear the Go build and module caches
	go clean -cache && go clean -modcache

#####################################
# Rust
#####################################

.PHONY: rust-build
rust-build:
	cargo build --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-check
rust-check: ## Type-check the workspace (cargo check)
	cargo check --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-fmt
rust-fmt: ## Format Rust code (nightly rustfmt)
	cargo +nightly fmt --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-fmt-check
rust-fmt-check: ## Fail if Rust code is not formatted
	cargo +nightly fmt --manifest-path $(RUST_MANIFEST_PATH) --check

.PHONY: rust-lint
rust-lint: ## Lint Rust code (clippy, warnings as errors)
	cargo clippy --manifest-path $(RUST_MANIFEST_PATH) --all --all-targets --all-features -- -D warnings

.PHONY: rust-lint-fix
rust-lint-fix:
	cargo clippy --manifest-path $(RUST_MANIFEST_PATH) --all --all-targets --all-features --fix --allow-dirty -- -D warnings

.PHONY: rust-release
rust-release:
	cargo build --release --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-run
rust-run:
	cargo run --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-open-doc
rust-open-doc:
	cargo doc --open --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-fix
rust-fix:
	cargo fix --allow-dirty --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-test
rust-test: ## Run unit and hermetic tests (RUST_TEST_PACKAGE, RUST_TEST_NAME_MATCH)
	# --all-targets only includes binaries, libraries, integration tests, and benchmarks, but specifically excludes doc tests. Reason is to split doc tests, since they can be slower
	RUST_BACKTRACE=1 cargo test --manifest-path $(RUST_MANIFEST_PATH) $(_RUST_TEST_PACKAGE_TARGET) $(RUST_TEST_NAME_MATCH) --all-targets

.PHONY: rust-test-doc
rust-test-doc: ## Run doc-tests
	RUST_BACKTRACE=1 cargo test --doc --manifest-path $(RUST_MANIFEST_PATH) $(_RUST_TEST_PACKAGE_TARGET) $(RUST_TEST_NAME_MATCH)

.PHONY: rust-integration
rust-integration: ## Run integration tests against Tilt infrastructure
	RUST_BACKTRACE=1 cargo test --manifest-path $(RUST_MANIFEST_PATH) -p $(RUST_SYSTEM_TESTS) --test 'integration_*'

.PHONY: rust-acceptance
rust-acceptance: ## Run acceptance tests against the system in Tilt
	RUST_BACKTRACE=1 cargo test --manifest-path $(RUST_MANIFEST_PATH) -p $(RUST_SYSTEM_TESTS) --test 'acceptance_*'

.PHONY: rust-doc
rust-doc: ## Build docs with warnings as errors
	RUSTDOCFLAGS="-D warnings" cargo doc --manifest-path $(RUST_MANIFEST_PATH) --workspace --no-deps --all-features

.PHONY: rust-deny
rust-deny: ## Audit dependencies: advisories, licences, bans (needs deny.toml)
	cargo deny --manifest-path $(RUST_MANIFEST_PATH) check

.PHONY: rust-mutate
rust-mutate: ## Mutation test the lines changed since MUTATION_BASE (cargo-mutants)
	@mkdir -p $(CURDIR)/tmp
	cd $(PROJECT_DIR) && git diff --relative $(MUTATION_BASE) > $(CURDIR)/tmp/mutants.diff && \
		cargo mutants --in-diff $(CURDIR)/tmp/mutants.diff

.PHONY: rust-verify
rust-verify: rust-fmt-check rust-lint rust-test rust-test-doc rust-doc ## Everything a change must pass before review

#####################################
# TypeScript
#####################################

.PHONY: ts-lint
ts-lint: ## Lint and check formatting of TypeScript (biome)
	@biome check $(PROJECT_DIR)

.PHONY: ts-fmt
ts-fmt: ## Format TypeScript (biome)
	@biome format --write $(PROJECT_DIR)
