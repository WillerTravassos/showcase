# -*- mode: Python -*-
# Shell for every project in the repository. Shared infrastructure always runs; only the
# active project's resources are enabled.
#
#   tilt up                         # active project from tilt_config.json, else online-pong
#   tilt up -- --project=<name>     # start with a specific project
#
# The navigation buttons switch the active project without restarting Tilt.

version_settings(check_updates=True, constraint=">=0.37.0")

# Our data is sensitive, as such, we don't want to load any info into 3rd party clouds unless necessary
disable_snapshots()

if k8s_context() != "kind-local":
    fail("can only run tilt cluster in local context")

load("./tilt/lib/Tiltfile", "SHARED_LABEL", "kustomize_overlay", "project_switch_buttons", "wait_for_webhook")
load("./online-pong/tilt/Tiltfile", ONLINE_PONG_RESOURCES="RESOURCES")
load("./clinic-scheduler/tilt/Tiltfile", CLINIC_SCHEDULER_RESOURCES="RESOURCES")

PROJECT_RESOURCES = {
    "online-pong": ONLINE_PONG_RESOURCES,
    "clinic-scheduler": CLINIC_SCHEDULER_RESOURCES,
}

config.define_string("project", usage="Project to enable: " + ", ".join(PROJECT_RESOURCES.keys()))
cfg = config.parse()
active = cfg.get("project", "online-pong")
if active not in PROJECT_RESOURCES:
    fail("unknown project {}, expected one of {}".format(active, PROJECT_RESOURCES.keys()))

# Shared infrastructure.
kustomize_overlay("./tilt/kustomize/namespaces/overlays/local")
kustomize_overlay("./tilt/kustomize/otel-lgtm/overlays/local")
kustomize_overlay("./deployments/gateway-api/overlays/local")
kustomize_overlay("./deployments/cilium/overlays/local")
kustomize_overlay("./deployments/cert-manager/overlays/local")
kustomize_overlay("./deployments/trust-manager/overlays/local")
kustomize_overlay("./tilt/kustomize/local-ca/overlays/local")

k8s_resource(
    new_name="namespaces",
    objects=[
        "observability:namespace",
        "online-pong:namespace",
        "clinic-scheduler:namespace",
    ],
    labels=[SHARED_LABEL],
)
k8s_resource(
    "lgtm",
    objects=["lgtm-data:persistentvolumeclaim"],
    port_forwards=[
        port_forward(3000, 3000, name="grafana"),
        port_forward(3200, 3200, name="tempo"),
        port_forward(4318, 4318, name="otlp-http"),
    ],
    labels=[SHARED_LABEL],
)

# Networking: Cilium starts its Gateway API controller only when the CRDs exist first.
k8s_resource(
    new_name="gateway-api-crds",
    objects=[
        name + ".gateway.networking.k8s.io:customresourcedefinition"
        for name in [
            "backendtlspolicies",
            "gatewayclasses",
            "gateways",
            "grpcroutes",
            "httproutes",
            "listenersets",
            "referencegrants",
            "tcproutes",
            "tlsroutes",
            "udproutes",
        ]
    ] + [
        "safe-upgrades.gateway.networking.k8s.io:validatingadmissionpolicy",
        "safe-upgrades.gateway.networking.k8s.io:validatingadmissionpolicybinding",
    ],
    labels=[SHARED_LABEL],
)
k8s_resource(
    "cilium-operator",
    objects=["cilium:gatewayclass"],
    resource_deps=["gateway-api-crds"],
    labels=[SHARED_LABEL],
)
k8s_resource("cilium", resource_deps=["gateway-api-crds"], labels=[SHARED_LABEL])
k8s_resource("cilium-envoy", resource_deps=["cilium"], labels=[SHARED_LABEL])

# Certificates: cert-manager, then trust-manager (its webhook certificate comes from
# cert-manager), then the local CA and its trust bundle.
for name in ["cert-manager", "cert-manager-cainjector", "cert-manager-webhook"]:
    k8s_resource(name, resource_deps=["cilium"], labels=[SHARED_LABEL])
wait_for_webhook(
    "cert-manager-webhook-ready",
    "./tilt/kustomize/local-ca/base/bootstrap-cluster-issuer.yaml",
    ["cert-manager-webhook"],
)
k8s_resource(
    "trust-manager",
    objects=["trust-manager:certificate", "trust-manager:issuer"],
    resource_deps=["cert-manager-webhook-ready"],
    labels=[SHARED_LABEL],
)
wait_for_webhook("trust-manager-webhook-ready", "./tilt/kustomize/local-ca/base/bundle.yaml", ["trust-manager"])
k8s_resource(
    new_name="local-ca",
    objects=[
        "local-ca-bootstrap:clusterissuer",
        "local-ca:certificate",
        "local-ca:clusterissuer",
        "local-ca-bundle:bundle",
    ],
    resource_deps=["cert-manager-webhook-ready", "trust-manager-webhook-ready"],
    labels=[SHARED_LABEL],
)

SHARED_RESOURCES = [
    "namespaces",
    "lgtm",
    "gateway-api-crds",
    "cilium-operator",
    "cilium",
    "cilium-envoy",
    "cert-manager",
    "cert-manager-cainjector",
    "cert-manager-webhook",
    "cert-manager-webhook-ready",
    "trust-manager",
    "trust-manager-webhook-ready",
    "local-ca",
]
config.set_enabled_resources(SHARED_RESOURCES + PROJECT_RESOURCES[active])
project_switch_buttons(PROJECT_RESOURCES.keys(), active, config.main_dir)
