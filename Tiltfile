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

load("./tilt/lib/Tiltfile", "SHARED_LABEL", "kustomize_overlay", "project_switch_buttons")
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

config.set_enabled_resources(["namespaces", "lgtm"] + PROJECT_RESOURCES[active])
project_switch_buttons(PROJECT_RESOURCES.keys(), active, config.main_dir)
