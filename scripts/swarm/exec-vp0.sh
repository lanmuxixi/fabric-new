#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
container_id="$("${SCRIPT_DIR}/get-service-container.sh" vp0)"

docker exec -it "${container_id}" /bin/sh
