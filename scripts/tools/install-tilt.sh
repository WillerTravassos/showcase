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

if [[ -x tools/tilt && $(tools/tilt version) == *"${version}"* ]]; then
    echo "Tilt v${version} already installed"
    exit 0
fi

tar_package_name="tilt.${version}.${os_name}.${os_arch}"
tar_package_url="https://github.com/tilt-dev/tilt/releases/download/v${version}/${tar_package_name}.tar.gz"
tarball_dir="$(mktemp)"

mkdir -p tools/

echo "Downloading Tilt package from URL ${tar_package_url}"
curl -sSfL "${tar_package_url}" -o "${tarball_dir}"
tar xf "${tarball_dir}" -C tools --strip-components 0 "tilt"
rm -rf "${tarball_dir}"

tools/tilt version
