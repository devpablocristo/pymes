CREATE SCHEMA IF NOT EXISTS fiscal;

CREATE TABLE IF NOT EXISTS fiscal.requests (
  organization_id text NOT NULL,
  request_id text NOT NULL,
  idempotency_key text NOT NULL,
  payload_hash char(64) NOT NULL,
  request jsonb NOT NULL,
  result jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (organization_id, request_id),
  UNIQUE (organization_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS fiscal.mock_authorizations (
  voucher_key text PRIMARY KEY,
  decision jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
