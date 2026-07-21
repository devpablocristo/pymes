DROP INDEX IF EXISTS medical.idx_oh_exams_tags;
DROP INDEX IF EXISTS medical.idx_oh_exams_tenant_favorite;

ALTER TABLE medical.occupational_health_exams
    DROP COLUMN IF EXISTS image_urls,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS is_favorite,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS client_name;
