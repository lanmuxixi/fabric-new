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
CHAINCODE_CTOR="${CHAINCODE_CTOR:-{\"Function\":\"init\",\"Args\":[\"com:10.92.2.140:53\",\"cn:10.92.2.140:53\"]}}"
CHAINCODE_DEPLOY_LOG="${CHAINCODE_DEPLOY_LOG:-${REPO_ROOT}/deploy/swarm/last-chaincode-deploy.log}"
CHAINCODE_ID_FILE="${CHAINCODE_ID_FILE:-${REPO_ROOT}/deploy/swarm/last-chaincode-id.txt}"

record_env_value() {
  local key="$1"
  local value="$2"

  touch "${ENV_FILE}"
  if grep -q "^${key}=" "${ENV_FILE}"; then
    sed -i.bak "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
  else
    printf '\n%s=%s\n' "${key}" "${value}" >> "${ENV_FILE}"
  fi
}

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
  record_env_value DNS_CHAINCODEID "${chaincode_id}"
  echo "Recorded DNS_CHAINCODEID in ${ENV_FILE}"
  echo "Run scripts/swarm/deploy-stack.sh again so vp0 receives CORE_DNS_CHAINCODEID."
else
  echo "Warning: could not extract a chaincode name automatically." >&2
  echo "Review ${CHAINCODE_DEPLOY_LOG} and write the returned chaincode name into ${CHAINCODE_ID_FILE}." >&2
fi
