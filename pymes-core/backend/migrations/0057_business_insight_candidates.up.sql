CREATE TABLE IF NOT EXISTS pymes_business_insight_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    kind text NOT NULL,
    event_type text NOT NULL,
    entity_type text NOT NULL,
    entity_id text NOT NULL DEFAULT '',
    fingerprint text NOT NULL,
    severity text NOT NULL DEFAULT 'info',
    status text NOT NULL DEFAULT 'new',
    title text NOT NULL,
    body text NOT NULL,
    evidence_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    occurrence_count integer NOT NULL DEFAULT 1,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    first_notified_at timestamptz,
    last_notified_at timestamptz,
    resolved_at timestamptz,
    last_actor text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pymes_business_insight_candidates_fingerprint_uniq UNIQUE (org_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_pymes_business_insight_candidates_org_status
    ON pymes_business_insight_candidates (org_id, status, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_pymes_business_insight_candidates_org_entity
    ON pymes_business_insight_candidates (org_id, entity_type, entity_id);
