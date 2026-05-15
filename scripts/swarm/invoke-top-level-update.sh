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

if [[ "$#" -ne 6 ]]; then
  echo "Usage: $0 <domain> <ip> <record-type> <ttl> <owner> <signature>" >&2
  echo "Example: $0 xxx.google.com 2.2.2.2 A 86400 admin <signature>" >&2
  exit 1
fi

DOMAIN="$1"
IP="$2"
RECORD_TYPE="$3"
TTL="$4"
OWNER="$5"
SIGNATURE="$6"

CHAINCODE_ID_FILE="${CHAINCODE_ID_FILE:-${REPO_ROOT}/deploy/swarm/last-chaincode-id.txt}"
CHAINCODE_NAME="${CHAINCODE_NAME:-${ZZM:-${zzm:-}}}"
if [[ -z "${CHAINCODE_NAME}" && -f "${CHAINCODE_ID_FILE}" ]]; then
  CHAINCODE_NAME="$(tr -d '\r\n' < "${CHAINCODE_ID_FILE}")"
fi

: "${CHAINCODE_NAME:?Set CHAINCODE_NAME or CHAINCODE_ID_FILE before invoking.}"

docker run --rm \
  -e CORE_PEER_ADDRESS="${VP0_ENDPOINT}" \
  "${FABRIC_PEER_IMAGE}" \
  peer chaincode invoke \
    -n "${CHAINCODE_NAME}" \
    -c "{\"Function\":\"TopLevelUpdate\",\"Args\":[\"${DOMAIN}\",\"${IP}\",\"${RECORD_TYPE}\",\"${TTL}\",\"${OWNER}\",\"${SIGNATURE}\"]}"
