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

version=${1:-}
os_name=$(uname -s | tr '[:upper:]' '[:lower:]')
os_arch=$(uname -m)

if [[ "$os_arch" == "x86_64" ]]; then
    os_arch="amd64"
fi

readonly os_arch

if [[ -x tools/kind && $(tools/kind version) == *"${version}"* ]]; then
    echo "Kind v${version} already installed"
    exit 0
fi

readonly tar_package_name="kind-${os_name}-${os_arch}"
readonly tar_package_url="https://github.com/kubernetes-sigs/kind/releases/download/v${version}/${tar_package_name}"

mkdir -p tools/

echo "Downloading Kind package from URL ${tar_package_url}"
curl -sSfL "${tar_package_url}" -o "tools/kind"
chmod +x tools/kind

tools/kind version
