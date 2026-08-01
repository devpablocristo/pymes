#!/usr/bin/env bash
set -euo pipefail

# Explicit, tenant-scoped replay. The operator must confirm the failure code so
# a stale or already-replayed event cannot be moved accidentally.
: "${PYMES_DATABASE_URL:?PYMES_DATABASE_URL is required}"
: "${PYMES_REPLAY_ORGANIZATION_ID:?PYMES_REPLAY_ORGANIZATION_ID is required}"
: "${PYMES_REPLAY_EVENT_ID:?PYMES_REPLAY_EVENT_ID is required}"
: "${PYMES_REPLAY_FAILURE_CODE:?PYMES_REPLAY_FAILURE_CODE is required}"
: "${PYMES_REPLAY_ACTOR_REF:?PYMES_REPLAY_ACTOR_REF is required (opaque, never a name or email)}"
: "${PYMES_REPLAY_CHANGE_REF:?PYMES_REPLAY_CHANGE_REF is required}"

if [[ ! "$PYMES_REPLAY_EVENT_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]]; then
  echo "PYMES_REPLAY_EVENT_ID must be a canonical UUID" >&2
  exit 2
fi
if [[ ! "$PYMES_REPLAY_ORGANIZATION_ID" =~ ^[A-Za-z0-9_-]{1,255}$ ]]; then
  echo "PYMES_REPLAY_ORGANIZATION_ID has an invalid shape" >&2
  exit 2
fi
if [[ ! "$PYMES_REPLAY_FAILURE_CODE" =~ ^[A-Z0-9_]{1,64}$ ]]; then
  echo "PYMES_REPLAY_FAILURE_CODE must be an uppercase stable code" >&2
  exit 2
fi
if [[ ! "$PYMES_REPLAY_ACTOR_REF" =~ ^[A-Za-z0-9:_./-]{1,255}$ ]]; then
  echo "PYMES_REPLAY_ACTOR_REF must be an opaque non-PII reference" >&2
  exit 2
fi
if [[ ! "$PYMES_REPLAY_CHANGE_REF" =~ ^[A-Za-z0-9:_./-]{1,255}$ ]]; then
  echo "PYMES_REPLAY_CHANGE_REF has an invalid shape" >&2
  exit 2
fi

psql "$PYMES_DATABASE_URL" \
  -X \
  -qAt \
  -v ON_ERROR_STOP=1 \
  -v organization_id="$PYMES_REPLAY_ORGANIZATION_ID" \
  -v event_id="$PYMES_REPLAY_EVENT_ID" \
  -v failure_code="$PYMES_REPLAY_FAILURE_CODE" \
  -v actor_ref="$PYMES_REPLAY_ACTOR_REF" \
  -v change_ref="$PYMES_REPLAY_CHANGE_REF" <<'SQL'
BEGIN;
SELECT set_config('app.org_id', :'organization_id', true) AS tenant_context
\gset
SELECT
  pg_advisory_xact_lock(
    hashtextextended(:'organization_id' || ':' || :'event_id', 0)
  ),
  true AS replay_lock
\gset

SELECT EXISTS (
  SELECT 1
  FROM app.outbox_dead_letter_replays audit
  WHERE audit.org_id = :'organization_id'
    AND audit.event_id = :'event_id'::uuid
    AND audit.failure_code = :'failure_code'
    AND audit.actor_ref = :'actor_ref'
    AND audit.change_ref = :'change_ref'
    AND NOT EXISTS (
      SELECT 1
      FROM app.outbox_dead_letters current_failure
      WHERE current_failure.org_id = audit.org_id
        AND current_failure.id = audit.event_id
    )
) AS already_replayed
\gset

\if :already_replayed
  COMMIT;
  \echo 'replay-noop'
\else
  WITH source AS MATERIALIZED (
    SELECT
      id, org_id, topic, payload, payload_hash, idempotency_key,
      request_id, actor_ref, source_version, snapshot_digest,
      correlation_id, failed_at, failure_code
    FROM app.outbox_dead_letters
    WHERE org_id = :'organization_id'
      AND id = :'event_id'::uuid
      AND failure_code = :'failure_code'
    FOR UPDATE
  ),
  recorded AS (
    INSERT INTO app.outbox_dead_letter_replays (
      org_id, event_id, failed_at, failure_code,
      replayed_at, actor_ref, change_ref
    )
    SELECT
      org_id, id, failed_at, failure_code,
      now(), :'actor_ref', :'change_ref'
    FROM source
    ON CONFLICT (org_id, event_id, failed_at) DO NOTHING
    RETURNING org_id, event_id, failed_at
  ),
  moved AS (
    DELETE FROM app.outbox_dead_letters dead_letter
    USING source, recorded
    WHERE dead_letter.org_id = source.org_id
      AND dead_letter.id = source.id
      AND recorded.org_id = source.org_id
      AND recorded.event_id = source.id
      AND recorded.failed_at = source.failed_at
    RETURNING
      dead_letter.id, dead_letter.org_id, dead_letter.topic,
      dead_letter.payload, dead_letter.payload_hash,
      dead_letter.idempotency_key, dead_letter.request_id,
      dead_letter.actor_ref, dead_letter.source_version,
      dead_letter.snapshot_digest, dead_letter.correlation_id,
      dead_letter.failed_at
  ),
  replayed AS (
    INSERT INTO app.outbox (
      id, org_id, topic, payload, payload_hash, idempotency_key,
      request_id, actor_ref, source_version, snapshot_digest,
      correlation_id, available_at, attempts, lease_token,
      lease_expires_at, published_at, created_at
    )
    SELECT
      id, org_id, topic, payload, payload_hash, idempotency_key,
      request_id, actor_ref, source_version, snapshot_digest,
      correlation_id, now(), 0, NULL, NULL, NULL, failed_at
    FROM moved
    RETURNING id
  )
  SELECT EXISTS (SELECT 1 FROM replayed) AS replayed
  \gset

  \if :replayed
    COMMIT;
    \echo 'replay-created'
  \else
    ROLLBACK;
    \echo 'event was not replayed: verify tenant, UUID, failure code and prior replay state'
    \quit 3
  \endif
\endif
SQL
