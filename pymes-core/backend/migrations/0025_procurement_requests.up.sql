-- Solicitudes internas de compra/gasto + políticas CEL (governance) por org.

CREATE TABLE IF NOT EXISTS procurement_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    requester_actor text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    category text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft',
    estimated_total numeric(18, 4) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    evaluation_json jsonb,
    purchase_id uuid REFERENCES purchases(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_procurement_requests_org ON procurement_requests(org_id);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_status ON procurement_requests(org_id, status);
CREATE INDEX IF NOT EXISTS idx_procurement_requests_archived ON procurement_requests(org_id, archived_at);

CREATE TABLE IF NOT EXISTS procurement_request_lines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id uuid NOT NULL REFERENCES procurement_requests(id) ON DELETE CASCADE,
    description text NOT NULL DEFAULT '',
    product_id uuid,
    quantity numeric(18, 4) NOT NULL DEFAULT 1,
    unit_price_estimate numeric(18, 4) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_procurement_request_lines_request ON procurement_request_lines(request_id);

CREATE TABLE IF NOT EXISTS procurement_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    expression text NOT NULL,
    effect text NOT NULL,
    priority int NOT NULL DEFAULT 100,
    mode text NOT NULL DEFAULT 'enforce',
    enabled boolean NOT NULL DEFAULT true,
    action_filter text NOT NULL DEFAULT 'procurement.submit',
    system_filter text NOT NULL DEFAULT 'pymes',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_procurement_policies_org ON procurement_policies(org_id);

-- Permisos para roles de demo (solo si existen: el seed RBAC vive en seeds/03_rbac.sql)
DO $$
DECLARE
    r_almacenero uuid := '21000000-0000-0000-0000-000000000005';
    r_contador uuid := '21000000-0000-0000-0000-000000000004';
BEGIN
    IF EXISTS (SELECT 1 FROM roles WHERE id = r_almacenero) THEN
        INSERT INTO role_permissions (id, role_id, resource, action)
        VALUES
            (gen_random_uuid(), r_almacenero, 'procurement_requests', 'read'),
            (gen_random_uuid(), r_almacenero, 'procurement_requests', 'create'),
            (gen_random_uuid(), r_almacenero, 'procurement_requests', 'update'),
            (gen_random_uuid(), r_almacenero, 'procurement_requests', 'submit')
        ON CONFLICT (role_id, resource, action) DO NOTHING;
    END IF;
    IF EXISTS (SELECT 1 FROM roles WHERE id = r_contador) THEN
        INSERT INTO role_permissions (id, role_id, resource, action)
        VALUES
            (gen_random_uuid(), r_contador, 'procurement_requests', 'read'),
            (gen_random_uuid(), r_contador, 'procurement_requests', 'approve'),
            (gen_random_uuid(), r_contador, 'procurement_requests', 'reject')
        ON CONFLICT (role_id, resource, action) DO NOTHING;
    END IF;
END $$;
