#!/usr/bin/env bash
# The direct above allows to run bash on different systems, specially on those which bash shell is not found in the common /bin/bash path

# Set strict mode for bash script
# -e            - exit on error
# -u            - exit on unset variable
# -o pipefail   - exit on pipefail
#
# Addtional flags:
# -x            - "debug mode" of bash, it prints each command it executed. Same as "-o xtrace" flag
set -euo pipefail

action="$1"
kind_cluster_config_file_path="$2"

if [[ ! -f $kind_cluster_config_file_path ]]; then
    echo "Error: Kind cluster configuration in ${kind_cluster_config_file_path} was not found"
    exit 1
fi

PATH="$(pwd)/tools/:${PATH}"
export PATH

echo "=> KIND_VERSION: $(kind version)"
echo "=> CLUSTER_YAML: ${kind_cluster_config_file_path}"
echo "---------------Begin Cluster file---------------"
cat "${kind_cluster_config_file_path}"
echo "----------------End Cluster file----------------"

case "${action}" in
create)
    echo "=> Creating cluster"

    ctlptl apply -f "${kind_cluster_config_file_path}"
    ;;
delete)
    echo "=> Deleting cluster"

    ctlptl delete --ignore-not-found -f "${kind_cluster_config_file_path}"
    ;;
reset)
    echo "=> Reseting cluster"

    ctlptl delete --ignore-not-found -f "${kind_cluster_config_file_path}"
    ctlptl apply -f "${kind_cluster_config_file_path}"
    ;;
esac
