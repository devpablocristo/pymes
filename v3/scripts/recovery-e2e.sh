#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"

wait_http_up() {
  url=$1
  attempts=0
  until curl -fsS "$url" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if test "$attempts" -ge 60; then
      echo "endpoint did not recover: $url" >&2
      return 1
    fi
    sleep 1
  done
}

wait_http_down() {
  url=$1
  attempts=0
  while curl -fsS "$url" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if test "$attempts" -ge 30; then
      echo "endpoint remained ready: $url" >&2
      return 1
    fi
    sleep 1
  done
}

worker_ready() {
  docker compose exec -T fiscal-fake node -e '
fetch("http://worker:8080/readyz")
  .then((response) => process.exit(response.ok ? 0 : 1))
  .catch(() => process.exit(1));
' >/dev/null 2>&1
}

wait_worker_up() {
  attempts=0
  until worker_ready; do
    attempts=$((attempts + 1))
    if test "$attempts" -ge 60; then
      echo "worker did not recover" >&2
      return 1
    fi
    sleep 1
  done
}

wait_worker_down() {
  attempts=0
  while worker_ready; do
    attempts=$((attempts + 1))
    if test "$attempts" -ge 30; then
      echo "worker remained ready without PostgreSQL" >&2
      return 1
    fi
    sleep 1
  done
}

docker compose up -d --build --wait
wait_http_up http://127.0.0.1:18080/readyz
wait_http_up http://127.0.0.1:18081/readyz
wait_http_up http://127.0.0.1:18082/readyz
wait_worker_up

docker compose stop fiscal-fake
wait_http_down http://127.0.0.1:18081/readyz
wait_http_up http://127.0.0.1:18080/readyz
wait_http_up http://127.0.0.1:18082/readyz
docker compose up -d --wait fiscal-fake
wait_http_up http://127.0.0.1:18081/readyz

docker compose stop accounting
wait_http_down http://127.0.0.1:18082/readyz
wait_http_up http://127.0.0.1:18080/readyz
wait_http_up http://127.0.0.1:18081/readyz
docker compose up -d --wait accounting
wait_http_up http://127.0.0.1:18082/readyz

docker compose stop fiscal-postgres
wait_http_down http://127.0.0.1:18081/readyz
wait_http_up http://127.0.0.1:18080/readyz
wait_http_up http://127.0.0.1:18082/readyz
docker compose up -d --wait fiscal-postgres
wait_http_up http://127.0.0.1:18081/readyz

docker compose stop accounting-postgres
wait_http_down http://127.0.0.1:18082/readyz
wait_http_up http://127.0.0.1:18080/readyz
wait_http_up http://127.0.0.1:18081/readyz
docker compose up -d --wait accounting-postgres
wait_http_up http://127.0.0.1:18082/readyz

docker compose stop postgres
wait_http_down http://127.0.0.1:18080/readyz
wait_worker_down
wait_http_up http://127.0.0.1:18081/readyz
wait_http_up http://127.0.0.1:18082/readyz
docker compose up -d --wait postgres
wait_http_up http://127.0.0.1:18080/readyz
wait_worker_up

docker compose restart worker
wait_worker_up
