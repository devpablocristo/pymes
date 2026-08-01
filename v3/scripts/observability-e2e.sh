#!/usr/bin/env sh
set -eu

root_dir=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root_dir"
command -v jq >/dev/null

docker compose up -d --build --wait

curl -fsS http://127.0.0.1:18080/healthz >/dev/null
curl -fsS http://127.0.0.1:18080/readyz >/dev/null
curl -fsS http://127.0.0.1:18081/healthz >/dev/null
curl -fsS http://127.0.0.1:18081/readyz >/dev/null
curl -fsS http://127.0.0.1:18082/healthz >/dev/null
curl -fsS http://127.0.0.1:18082/readyz >/dev/null

docker compose exec -T fiscal-fake node -e '
fetch("http://worker:8080/metrics")
  .then(async (response) => {
    const body = await response.text();
    const expected = [
      "pymes_outbox_pending",
      "pymes_outbox_retrying",
      "pymes_outbox_dead_letters",
      "pymes_outbox_oldest_age_seconds",
      "pymes_fiscal_uncertain",
      "pymes_dependency_circuit_open"
    ];
    if (!response.ok || !expected.every((name) => body.includes(name))) process.exit(1);
  })
  .catch(() => process.exit(1));
'

attempts=0
metrics_log=
while test -z "$metrics_log"; do
  metrics_log=$(docker compose logs --no-color --no-log-prefix --tail=200 worker |
    jq -Rrc 'fromjson? | select(.event == "worker_metrics" and .ready == true)' |
    tail -n 1)
  attempts=$((attempts + 1))
  if test "$attempts" -ge 30; then
    echo "worker did not emit a structured metric heartbeat" >&2
    exit 1
  fi
  if test -z "$metrics_log"; then
    sleep 1
  fi
done

printf '%s\n' "$metrics_log" | jq -e '
  (.outbox_pending | type == "number") and
  (.outbox_dead_letters | type == "number") and
  (.outbox_oldest_age_seconds | type == "number") and
  (.fiscal_uncertain | type == "number") and
  (.dependency_circuits_open | type == "number") and
  (has("organization_id") | not) and
  (has("actor") | not) and
  (has("cuit") | not) and
  (has("tax_identifier") | not) and
  (has("token") | not) and
  (has("credential") | not) and
  (has("certificate") | not) and
  (has("payload") | not) and
  (has("xml") | not)
' >/dev/null
