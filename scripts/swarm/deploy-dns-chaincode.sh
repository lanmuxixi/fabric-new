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
: "${VP0_ENDPOINT:?Set VP0_ENDPOINT to the published gRPC endpoint of vp0, for example 10.161.34.8:7051.}"

CHAINCODE_PATH="${CHAINCODE_PATH:-github.com/hyperledger/fabric/examples/chaincode/go/chaincode_dns_reslover}"
CHAINCODE_CTOR="${CHAINCODE_CTOR:-{\"Function\":\"init\",\"Args\":[\"com:10.161.34.51:53\",\"cn:10.161.34.51:53\"]}}"
CHAINCODE_DEPLOY_LOG="${CHAINCODE_DEPLOY_LOG:-${REPO_ROOT}/deploy/swarm/last-chaincode-deploy.log}"
CHAINCODE_ID_FILE="${CHAINCODE_ID_FILE:-${REPO_ROOT}/deploy/swarm/last-chaincode-id.txt}"

echo "Deploying ${CHAINCODE_PATH} against ${VP0_ENDPOINT}"
echo "Ctor: ${CHAINCODE_CTOR}"
echo "Save the returned chaincode name for later queries such as TopLevelGetAll."

deploy_output="$(
docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  -v "${REPO_ROOT}:/opt/gopath/src/github.com/hyperledger/fabric" \
  -w /opt/gopath/src/github.com/hyperledger/fabric \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode deploy \
    -p "${CHAINCODE_PATH}" \
    -c "${CHAINCODE_CTOR}"
)"

printf '%s\n' "${deploy_output}" | tee "${CHAINCODE_DEPLOY_LOG}"

chaincode_id="$(printf '%s\n' "${deploy_output}" | sed -n 's/^Deploy chaincode: //p' | tail -n 1 | tr -d '\r')"
if [[ -n "${chaincode_id}" ]]; then
  printf '%s\n' "${chaincode_id}" > "${CHAINCODE_ID_FILE}"
  echo "Recorded chaincode name in ${CHAINCODE_ID_FILE}"
else
  echo "Warning: could not extract a chaincode name automatically." >&2
  echo "Review ${CHAINCODE_DEPLOY_LOG} and write the returned chaincode name into ${CHAINCODE_ID_FILE}." >&2
fi
