-- Rollback: re-crea schema sin data (la data, si existe en Nexus, se queda
-- ahí; manualmente exportar de Nexus /v1/policies?org_id=... e importar
-- cada row si se quiere recuperar).

CREATE TABLE IF NOT EXISTS procurement_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
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

CREATE INDEX IF NOT EXISTS idx_procurement_policies_org ON procurement_policies(tenant_id);
