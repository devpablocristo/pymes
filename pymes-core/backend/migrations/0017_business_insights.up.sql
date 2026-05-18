-- 0017_business_insights.up.sql
-- Candidatos de business insights generados por análisis (anomalías,
-- recordatorios, oportunidades). Consume el dossier AI + audit_log + KPIs.
-- Consolida: 0057_business_insight_candidates.

CREATE TABLE IF NOT EXISTS pymes_business_insight_candidates (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    kind text NOT NULL,
    event_type text NOT NULL,
    entity_type text NOT NULL,
    entity_id text NOT NULL DEFAULT '',
    fingerprint text NOT NULL,
    severity text NOT NULL DEFAULT 'info'
        CONSTRAINT pymes_business_insight_candidates_severity_check
        CHECK (severity IN ('info','warning','critical')),
    status text NOT NULL DEFAULT 'new'
        CONSTRAINT pymes_business_insight_candidates_status_check
        CHECK (status IN ('new','notified','acknowledged','resolved','dismissed')),
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
    CONSTRAINT pymes_business_insight_candidates_org_fingerprint_uniq
        UNIQUE (org_id, fingerprint)
);
CREATE INDEX IF NOT EXISTS idx_pymes_biz_insights_org_status
    ON pymes_business_insight_candidates(org_id, status, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_pymes_biz_insights_org_entity
    ON pymes_business_insight_candidates(org_id, entity_type, entity_id);

CREATE TRIGGER trg_pymes_business_insight_candidates_updated_at
    BEFORE UPDATE ON pymes_business_insight_candidates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
