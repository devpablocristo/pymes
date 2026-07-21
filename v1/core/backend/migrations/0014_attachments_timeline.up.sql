-- 0014_attachments_timeline.up.sql
-- Files (S3-style attachable_type/id) + entity timeline (audit-style feed).
-- Consolida: 0011_transversal_infra (attachments + timeline_entries).

CREATE TABLE IF NOT EXISTS attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    attachable_type text NOT NULL,
    attachable_id uuid NOT NULL,
    file_name text NOT NULL,
    content_type text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes bigint NOT NULL DEFAULT 0,
    storage_key text NOT NULL,
    uploaded_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_attachments_entity
    ON attachments(org_id, attachable_type, attachable_id);
CREATE INDEX IF NOT EXISTS idx_attachments_org
    ON attachments(org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS timeline_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    event_type text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    actor text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_timeline_entity
    ON timeline_entries(org_id, entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_timeline_org
    ON timeline_entries(org_id, created_at DESC);
