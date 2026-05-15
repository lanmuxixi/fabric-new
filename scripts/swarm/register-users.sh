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

: "${FABRIC_PEER_IMAGE:?Set FABRIC_PEER_IMAGE in ${ENV_FILE} or export it before running this script.}"
: "${VP0_ENDPOINT:?Set VP0_ENDPOINT to the published gRPC endpoint of vp0.}"

CHAINCODE_ID_FILE="${CHAINCODE_ID_FILE:-${REPO_ROOT}/deploy/swarm/last-chaincode-id.txt}"
CHAINCODE_NAME="${CHAINCODE_NAME:-${ZZM:-${zzm:-${DNS_CHAINCODEID:-}}}}"
if [[ -z "${CHAINCODE_NAME}" && -f "${CHAINCODE_ID_FILE}" ]]; then
  CHAINCODE_NAME="$(tr -d '\r\n' < "${CHAINCODE_ID_FILE}")"
fi

: "${CHAINCODE_NAME:?Set CHAINCODE_NAME, DNS_CHAINCODEID, or CHAINCODE_ID_FILE before registering users.}"

docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  -e zzm="${CHAINCODE_NAME}" \
  -v "${REPO_ROOT}:/opt/gopath/src/github.com/hyperledger/fabric" \
  -w /opt/gopath/src/github.com/hyperledger/fabric \
  "${FABRIC_PEER_IMAGE}" \
  bash txscripts/register.sh
