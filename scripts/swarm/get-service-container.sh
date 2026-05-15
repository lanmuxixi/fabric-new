#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
ENV_FILE="${ENV_FILE:-${REPO_ROOT}/deploy/swarm/.env}"

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

STACK_NAME="${STACK_NAME:-fabricdns}"
SERVICE_NAME="${1:-vp0}"
FULL_SERVICE_NAME="${STACK_NAME}_${SERVICE_NAME}"

container_id="$(docker ps --filter "label=com.docker.swarm.service.name=${FULL_SERVICE_NAME}" --format '{{.ID}}' | head -n 1)"
if [[ -z "${container_id}" ]]; then
  echo "No running container found for service ${FULL_SERVICE_NAME}" >&2
  exit 1
fi

printf '%s\n' "${container_id}"
