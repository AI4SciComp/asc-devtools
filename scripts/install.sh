#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail
project_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
exec python3 "${project_root}/scripts/manage_install.py" install "$@"
