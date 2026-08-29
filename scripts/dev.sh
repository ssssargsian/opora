#!/bin/zsh
set -euo pipefail

project_root="${0:A:h:h}"
cd "$project_root"

if [[ -f .env ]]; then
  set -a
  source .env
  set +a
fi

export DATABASE_URL="${DATABASE_URL:-postgres://opora:opora_local_postgres_change_me@localhost:5432/opora?sslmode=disable}"

docker compose up -d

for attempt in {1..30}; do
  if docker compose exec -T postgres pg_isready -U "${POSTGRES_USER:-opora}" -d "${POSTGRES_DB:-opora}" >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "30" ]]; then
    echo "[opora] PostgreSQL did not become ready in time." >&2
    exit 1
  fi
  sleep 1
done

echo "[opora] infrastructure ready"

(cd apps/api && go run github.com/pressly/goose/v3/cmd/goose@v3.27.3 -dir db/migrations postgres "$DATABASE_URL" up)
echo "[opora] database migrations applied"

(cd apps/api && go run ./cmd/api) &
api_pid=$!
(pnpm --dir apps/web dev) &
web_pid=$!

cleanup() {
  trap - INT TERM EXIT
  kill "$api_pid" "$web_pid" 2>/dev/null || true
  wait "$api_pid" "$web_pid" 2>/dev/null || true
  echo "[opora] local API and frontend stopped; Docker volumes are preserved"
}
trap cleanup INT TERM EXIT

echo "[api] starting on http://localhost:8080"
echo "[web] starting on http://localhost:3000"

while kill -0 "$api_pid" 2>/dev/null && kill -0 "$web_pid" 2>/dev/null; do
  sleep 1
done

exit 1
