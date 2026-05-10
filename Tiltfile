# -*- mode: Python -*-
version_settings(check_updates=True, constraint=">=0.36.3")

# Our data is sensitive, as such, we don't want to load any info into 3rd party clouds unless necessary
disable_snapshots()

if k8s_context() != "kind-local":
    fail("can only run tilt cluster in local context")

k8s_yaml(kustomize("./tilt/kustomization/namespaces/base"))


