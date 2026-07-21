-- 0010_dashboard.up.sql
-- Dashboard widgets catálogo + layouts personalizados por usuario.
--
-- Consolida: 0021_dashboard_personalizable + 0044_dashboard_services_widget.
-- Las tablas legacy (dashboard_layouts, dashboard_default_layouts,
-- dashboard_configs) NO se recrean — fueron dropped en 0047/0048.

CREATE TABLE IF NOT EXISTS dashboard_widgets_catalog (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    widget_key text NOT NULL UNIQUE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    domain text NOT NULL,
    kind text NOT NULL,
    default_width integer NOT NULL,
    default_height integer NOT NULL,
    min_width integer NOT NULL DEFAULT 2,
    min_height integer NOT NULL DEFAULT 2,
    max_width integer NOT NULL DEFAULT 12,
    max_height integer NOT NULL DEFAULT 8,
    allowed_roles_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    required_scopes_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    supported_contexts_json jsonb NOT NULL DEFAULT '[]'::jsonb,
    settings_schema_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    data_endpoint text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_dashboard_widgets_catalog_updated_at
    BEFORE UPDATE ON dashboard_widgets_catalog
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS user_dashboard_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    user_actor text NOT NULL,
    context text NOT NULL,
    layout_version integer NOT NULL DEFAULT 1,
    items_json jsonb NOT NULL,
    last_applied_default_layout_key text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_dashboard_layouts_actor_context_uniq UNIQUE (user_actor, context)
);
CREATE INDEX IF NOT EXISTS idx_user_dashboard_layouts_user_id
    ON user_dashboard_layouts(user_id);

CREATE TRIGGER trg_user_dashboard_layouts_updated_at
    BEFORE UPDATE ON user_dashboard_layouts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed inicial del catálogo de widgets. Se mantiene ON CONFLICT DO UPDATE
-- para que pueda re-correrse y actualizar metadata.
INSERT INTO dashboard_widgets_catalog (
    widget_key, title, description, domain, kind,
    default_width, default_height, min_width, min_height, max_width, max_height,
    allowed_roles_json, required_scopes_json, supported_contexts_json, settings_schema_json,
    data_endpoint, is_active
) VALUES
    ('sales.summary', 'Ventas del periodo', 'Resumen operativo de ventas completadas del periodo actual.',
     'control-plane', 'metric', 4, 2, 3, 2, 6, 3,
     '["owner","admin","member","service"]'::jsonb, '[]'::jsonb,
     '["home","commercial","control"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/sales-summary', true),
    ('cashflow.summary', 'Cashflow resumido', 'Ingresos, egresos y balance del periodo.',
     'control-plane', 'metric', 4, 2, 3, 2, 6, 3,
     '["owner","admin","member","service"]'::jsonb, '[]'::jsonb,
     '["home","operations","control"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/cashflow-summary', true),
    ('quotes.pipeline', 'Presupuestos abiertos', 'Pipeline de presupuestos por estado.',
     'control-plane', 'metric', 4, 2, 3, 2, 6, 3,
     '["owner","admin","member"]'::jsonb, '[]'::jsonb,
     '["home","commercial"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/quotes-pipeline', true),
    ('inventory.low_stock', 'Alertas de stock', 'Productos por debajo del minimo definido.',
     'control-plane', 'list', 6, 3, 4, 3, 8, 5,
     '["owner","admin","member"]'::jsonb, '[]'::jsonb,
     '["home","operations","control"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/low-stock', true),
    ('sales.recent', 'Ventas recientes', 'Ultimas ventas registradas en el tenant.',
     'control-plane', 'feed', 6, 3, 4, 3, 8, 6,
     '["owner","admin","member"]'::jsonb, '[]'::jsonb,
     '["home","commercial"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/recent-sales', true),
    ('products.top', 'Top productos', 'Productos con mayor facturacion en el periodo.',
     'control-plane', 'list', 6, 3, 4, 3, 8, 6,
     '["owner","admin","member"]'::jsonb, '[]'::jsonb,
     '["home","commercial","operations"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/top-products', true),
    ('billing.subscription', 'Estado del plan', 'Plan actual, estado de facturacion y limites.',
     'control-plane', 'status', 4, 2, 3, 2, 6, 4,
     '["owner","admin"]'::jsonb, '[]'::jsonb,
     '["home","control"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/billing-status', true),
    ('audit.activity', 'Actividad reciente', 'Ultimos eventos relevantes del tenant.',
     'control-plane', 'feed', 6, 3, 4, 3, 8, 6,
     '["owner","admin","member","service"]'::jsonb, '[]'::jsonb,
     '["home","control","operations"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/audit-activity', true),
    ('services.top', 'Top servicios', 'Servicios con mayor facturacion en el periodo.',
     'control-plane', 'list', 6, 3, 4, 3, 8, 6,
     '["owner","admin","member"]'::jsonb, '[]'::jsonb,
     '["home","commercial","operations"]'::jsonb, '{}'::jsonb,
     '/v1/dashboard-data/top-services', true)
ON CONFLICT (widget_key) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    domain = EXCLUDED.domain,
    kind = EXCLUDED.kind,
    default_width = EXCLUDED.default_width,
    default_height = EXCLUDED.default_height,
    min_width = EXCLUDED.min_width,
    min_height = EXCLUDED.min_height,
    max_width = EXCLUDED.max_width,
    max_height = EXCLUDED.max_height,
    allowed_roles_json = EXCLUDED.allowed_roles_json,
    required_scopes_json = EXCLUDED.required_scopes_json,
    supported_contexts_json = EXCLUDED.supported_contexts_json,
    settings_schema_json = EXCLUDED.settings_schema_json,
    data_endpoint = EXCLUDED.data_endpoint,
    is_active = EXCLUDED.is_active,
    updated_at = now();
