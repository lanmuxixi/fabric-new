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
: "${VP0_ENDPOINT:?Set VP0_ENDPOINT to the published gRPC endpoint of vp0, for example 10.92.2.138:7051.}"

CHAINCODE_PATH="${CHAINCODE_PATH:-github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover}"
CHAINCODE_CTOR="${CHAINCODE_CTOR:-{\"Function\":\"init\",\"Args\":[\"com:10.92.2.140\",\"cn:10.92.2.140\"]}}"

docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  -v "${REPO_ROOT}:/opt/gopath/src/github.com/hyperledger/fabric" \
  -w /opt/gopath/src/github.com/hyperledger/fabric \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode deploy \
    -p "${CHAINCODE_PATH}" \
    -c "${CHAINCODE_CTOR}"
