#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
ENV_FILE="${ENV_FILE:-${REPO_ROOT}/deploy/swarm/.env}"
STACK_FILE="${REPO_ROOT}/deploy/swarm/stack.yml"

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

STACK_NAME="${STACK_NAME:-fabricdns}"
SWARM_NETWORK="${SWARM_NETWORK:-fabric_dns}"

if [[ "$(docker info --format '{{.Swarm.LocalNodeState}}')" != "active" ]]; then
  echo "Docker Swarm is not active on this node." >&2
  exit 1
fi

: "${FABRIC_PEER_IMAGE:?Set FABRIC_PEER_IMAGE in ${ENV_FILE} or export it before running this script.}"

if ! docker network inspect "${SWARM_NETWORK}" >/dev/null 2>&1; then
  docker network create --driver overlay --attachable "${SWARM_NETWORK}"
fi

docker stack config --compose-file "${STACK_FILE}" >/dev/null
docker stack deploy --compose-file "${STACK_FILE}" "${STACK_NAME}"
docker stack services "${STACK_NAME}"
