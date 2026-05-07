ALTER TABLE medical.occupational_health_exams
    ADD COLUMN IF NOT EXISTS client_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_method text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_favorite boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS tags text[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS image_urls text[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_oh_exams_tenant_favorite
    ON medical.occupational_health_exams (tenant_id, is_favorite)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_oh_exams_tags
    ON medical.occupational_health_exams USING gin (tags);
