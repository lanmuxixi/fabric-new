#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)

SOURCE_IMAGE="${SOURCE_IMAGE:-}"
TARGET_IMAGE="${TARGET_IMAGE:-${FABRIC_PEER_IMAGE:-fabric-dns-peer:swarm-exp3}}"

cd "${REPO_ROOT}"
make peer-image

if [[ -z "${SOURCE_IMAGE}" ]]; then
  SOURCE_IMAGE=$(docker images hyperledger/fabric-peer --format '{{.Repository}}:{{.Tag}}' | head -n 1)
fi

if [[ -z "${SOURCE_IMAGE}" ]]; then
  echo "Failed to locate the image produced by 'make peer-image'." >&2
  exit 1
fi

docker tag "${SOURCE_IMAGE}" "${TARGET_IMAGE}"

echo "Tagged ${SOURCE_IMAGE} as ${TARGET_IMAGE}."
echo "Distribute ${TARGET_IMAGE} to every Swarm node or push it to a registry before deploying the stack."
