-- Employees: entidad transversal (F1) con los campos base de persona-empleado.
-- Cada vertical podrá embeber y extender (satélite o ALTER TABLE) sin tocar este core.

CREATE TABLE IF NOT EXISTS employees (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    first_name  text NOT NULL DEFAULT '',
    last_name   text NOT NULL DEFAULT '',
    email       text NOT NULL DEFAULT '',
    phone       text NOT NULL DEFAULT '',
    position    text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','inactive','terminated')),
    hire_date   date,
    end_date    date,
    user_id     uuid REFERENCES users(id),
    notes       text NOT NULL DEFAULT '',
    is_favorite boolean NOT NULL DEFAULT false,
    tags        text[]  NOT NULL DEFAULT '{}',
    created_by  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_employees_org            ON employees(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_employees_org_status     ON employees(org_id, status);
CREATE INDEX IF NOT EXISTS idx_employees_org_email      ON employees(org_id, email) WHERE email <> '';
CREATE INDEX IF NOT EXISTS idx_employees_org_deleted_at ON employees(org_id, deleted_at);
