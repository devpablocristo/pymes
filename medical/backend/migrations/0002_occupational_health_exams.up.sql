CREATE TABLE IF NOT EXISTS medical.occupational_health_exams (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    patient_name text NOT NULL,
    patient_document text NOT NULL DEFAULT '',
    employer_name text NOT NULL DEFAULT '',
    exam_type text NOT NULL DEFAULT 'pre_employment',
    status text NOT NULL DEFAULT 'pending',
    scheduled_at timestamptz NULL,
    completed_at timestamptz NULL,
    result text NOT NULL DEFAULT '',
    notes text NOT NULL DEFAULT '',
    created_by text NOT NULL DEFAULT '',
    updated_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz NULL,
    CONSTRAINT chk_oh_exam_type CHECK (exam_type IN ('pre_employment', 'periodic', 'return_to_work', 'exit', 'other')),
    CONSTRAINT chk_oh_exam_status CHECK (status IN ('pending', 'scheduled', 'completed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_oh_exams_tenant_status
    ON medical.occupational_health_exams (org_id, status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_oh_exams_tenant_scheduled
    ON medical.occupational_health_exams (org_id, scheduled_at DESC NULLS LAST)
    WHERE deleted_at IS NULL;

