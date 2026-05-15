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

BIND_CONFIG_DIR="${BIND_CONFIG_DIR:-/opt/fabric-dns/bind/config}"
BIND_CACHE_DIR="${BIND_CACHE_DIR:-/opt/fabric-dns/bind/cache}"
BIND_RECORDS_DIR="${BIND_RECORDS_DIR:-/opt/fabric-dns/bind/records}"
BIND_RUN_UID="${BIND_RUN_UID:-}"
BIND_RUN_GID="${BIND_RUN_GID:-}"

mkdir -p "${BIND_CONFIG_DIR}" "${BIND_CACHE_DIR}" "${BIND_RECORDS_DIR}"

cp "${REPO_ROOT}/deploy/swarm/bind/config/named.conf" "${BIND_CONFIG_DIR}/named.conf"
cp "${REPO_ROOT}/deploy/swarm/bind/config/named.conf.options" "${BIND_CONFIG_DIR}/named.conf.options"
cp "${REPO_ROOT}/deploy/swarm/bind/config/named.conf.local" "${BIND_CONFIG_DIR}/named.conf.local"
cp "${REPO_ROOT}/deploy/swarm/bind/records/db.com" "${BIND_RECORDS_DIR}/db.com"
cp "${REPO_ROOT}/deploy/swarm/bind/records/db.cn" "${BIND_RECORDS_DIR}/db.cn"

chmod 755 "${BIND_CONFIG_DIR}"
chmod 644 "${BIND_CONFIG_DIR}/named.conf" "${BIND_CONFIG_DIR}/named.conf.options" "${BIND_CONFIG_DIR}/named.conf.local"

if [[ -n "${BIND_RUN_UID}" && -n "${BIND_RUN_GID}" ]]; then
  chown -R "${BIND_RUN_UID}:${BIND_RUN_GID}" "${BIND_CACHE_DIR}" "${BIND_RECORDS_DIR}"
  chmod 775 "${BIND_CACHE_DIR}" "${BIND_RECORDS_DIR}"
  chmod 664 "${BIND_RECORDS_DIR}/db.com" "${BIND_RECORDS_DIR}/db.cn"
else
  # Fall back to permissive lab-friendly permissions so Bind9 can create journal files.
  chmod 777 "${BIND_CACHE_DIR}" "${BIND_RECORDS_DIR}"
  chmod 666 "${BIND_RECORDS_DIR}/db.com" "${BIND_RECORDS_DIR}/db.cn"
fi

echo "Prepared Bind9 layout:"
echo "  config:  ${BIND_CONFIG_DIR}"
echo "  cache:   ${BIND_CACHE_DIR}"
echo "  records: ${BIND_RECORDS_DIR}"
if [[ -z "${BIND_RUN_UID}" || -z "${BIND_RUN_GID}" ]]; then
  echo "  ownership: permissive mode (set BIND_RUN_UID/BIND_RUN_GID to tighten permissions)"
fi
