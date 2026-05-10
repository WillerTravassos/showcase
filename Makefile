#####################################
# Target variables
#####################################
CTLPTL_VERSION := 0.9.0

KIND_VERSION := 0.31.0
KIND_CLUSTER_CONFIG := $(CURDIR)/tilt/cluster.yaml

RUST_MANIFEST_PATH := ./online-pong/Cargo.toml
RUST_TEST_PACKAGE ?=
RUST_TEST_NAME_MATCH ?=

TILT_VERSION := 0.37.3

# Shared logic to decide between -p package or --workspace
_RUST_TEST_PACKAGE_TARGET = $(if $(RUST_TEST_PACKAGE),-p $(RUST_TEST_PACKAGE),--workspace)

#####################################
# Cluster
#####################################

.PHONY: cluster-create
cluster-create:
	@./scripts/local-env/kind-cluster.sh create ${KIND_CLUSTER_CONFIG}

.PHONY: cluster-delete
cluster-delete:
	@./scripts/local-env/kind-cluster.sh delete ${KIND_CLUSTER_CONFIG}

.PHONY: cluster-reset
cluster-reset:
	@./scripts/local-env/kind-cluster.sh reset ${KIND_CLUSTER_CONFIG}

.PHONY: tilt-down
tilt-down:
	@./tools/tilt down

.PHONY: tilt-up
tilt-up: cluster-create
	@./tools/tilt up

#####################################
# Tooling
#####################################

.PHONY: install-tools
install-tools: install-ctlptl install-kind install-tilt

.PHONY: install-ctlptl
install-ctlptl:
	@echo "=> Installing ctlptl ${CTLPTL_VERSION}"
	@bash -c "source $(CURDIR)/scripts/tools/install-ctlptl.sh ${CTLPTL_VERSION}"

.PHONY: install-kind
install-kind:
	@echo "=> Install Kind local cluster ${KIND_VERSION}"
	@bash -c "source $(CURDIR)/scripts/tools/install-kind.sh ${KIND_VERSION}"

.PHONY: install-tilt
install-tilt:
	@echo "=> Install Kind local cluster ${TILT_VERSION}"
	@bash -c "source $(CURDIR)/scripts/tools/install-tilt.sh ${TILT_VERSION}"

#####################################
# Golang
#####################################
.PHONY: go-cache-clean
go-cache-clean:
	go clean -cache && go clean -modcache

.PHONY: go-lint 
go-lint:
	@golangci-lint run

.PHONY: go-lint-fix
go-lint-fix:
	@golangci-lint run --fix

.PHONY: go-vendor
go-vendor:
	@go mod tidy && go mod vendor && go mod verify

#####################################
# Rust
#####################################
.PHONY: rust-build
rust-build:
	cargo build --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-check 
rust-check:
	cargo check --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-format 
rust-format:
	cargo +nightly fmt --manifest-path $(RUST_MANIFEST_PATH)

.PHONY: rust-lint
rust-lint:
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
rust-test:
	# --all-targets only includes binaries, libraries, integration tests, and benchmarks, but specifically excludes doc tests. Reason is to split doc tests, since they can be slower
	cargo test --manifest-path $(RUST_MANIFEST_PATH) $(_RUST_TEST_PACKAGE_TARGET) $(RUST_TEST_NAME_MATCH) --all-targets

.PHONY: rust-test-doc
rust-test-doc:
	cargo test --doc --manifest-path $(RUST_MANIFEST_PATH) $(_RUST_TEST_PACKAGE_TARGET) $(RUST_TEST_NAME_MATCH)

