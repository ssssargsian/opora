#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.prod.yml"
readonly ENV_FILE="${ROOT_DIR}/.env.production"

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this script as root." >&2
  exit 1
fi
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing ${ENV_FILE}; run deploy/provision-ubuntu.sh first." >&2
  exit 1
fi

cd "$ROOT_DIR"
git fetch --prune origin
git pull --ff-only
export OPORA_VERSION="$(git rev-parse --short=12 HEAD)"

compose=(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE")
"${compose[@]}" config --quiet
"${compose[@]}" build --pull api web
"${compose[@]}" up -d postgres minio clamav onlyoffice
"${compose[@]}" up -d --wait --wait-timeout 1200 postgres minio clamav onlyoffice
"${compose[@]}" run --rm minio-init
"${compose[@]}" run --rm migrate
"${compose[@]}" up -d --remove-orphans api web caddy
"${compose[@]}" up -d --wait --wait-timeout 600 api web caddy

app_domain="$(awk -F= '$1 == "APP_DOMAIN" {print $2}' "$ENV_FILE")"
office_domain="$(awk -F= '$1 == "OFFICE_DOMAIN" {print $2}' "$ENV_FILE")"
expected_ip="${OPORA_EXPECTED_IP:-155.212.223.107}"

curl --fail --silent --show-error --retry 12 --retry-delay 5 "http://127.0.0.1/health/ready" \
  -H "Host: ${app_domain}" >/dev/null

app_ip="$(getent ahostsv4 "$app_domain" 2>/dev/null | awk '{print $1; exit}' || true)"
office_ip="$(getent ahostsv4 "$office_domain" 2>/dev/null | awk '{print $1; exit}' || true)"
if [[ "$app_ip" == "$expected_ip" ]]; then
  curl --fail --silent --show-error --retry 18 --retry-delay 10 "https://${app_domain}/health/ready" >/dev/null
else
  echo "Application HTTPS check skipped until ${app_domain} resolves to ${expected_ip}." >&2
fi
if [[ "$office_ip" == "$expected_ip" ]]; then
  curl --fail --silent --show-error --retry 18 --retry-delay 10 "https://${office_domain}/healthcheck" >/dev/null
else
  echo "ONLYOFFICE HTTPS check skipped until ${office_domain} resolves to ${expected_ip}." >&2
fi

"${compose[@]}" ps
