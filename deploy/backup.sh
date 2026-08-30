#!/usr/bin/env bash
set -Eeuo pipefail

readonly ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly ENV_FILE="${ROOT_DIR}/.env.production"
readonly COMPOSE_FILE="${ROOT_DIR}/deploy/docker-compose.prod.yml"
readonly BACKUP_DIR="${OPORA_BACKUP_DIR:-${ROOT_DIR}/backups}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing ${ENV_FILE}." >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a
umask 077
mkdir -p "$BACKUP_DIR"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
target="${BACKUP_DIR}/opora-postgres-${timestamp}.dump"
temporary="${target}.tmp"
trap 'rm -f "$temporary"' EXIT

docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-acl >"$temporary"
mv "$temporary" "$target"
trap - EXIT
chmod 600 "$target"
echo "$target"
