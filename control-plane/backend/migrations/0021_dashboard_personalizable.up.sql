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

CREATE TABLE IF NOT EXISTS dashboard_default_layouts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    layout_key text NOT NULL UNIQUE,
    context text NOT NULL,
    name text NOT NULL,
    items_json jsonb NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_default_layouts_context_active
    ON dashboard_default_layouts(context)
    WHERE is_active = true;

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
    UNIQUE(user_actor, context)
);

CREATE INDEX IF NOT EXISTS idx_user_dashboard_layouts_user_id ON user_dashboard_layouts(user_id);

CREATE TABLE IF NOT EXISTS user_dashboard_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    user_actor text NOT NULL UNIQUE,
    default_context text NOT NULL DEFAULT 'home',
    preferences_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO dashboard_widgets_catalog (
    widget_key, title, description, domain, kind,
    default_width, default_height, min_width, min_height, max_width, max_height,
    allowed_roles_json, required_scopes_json, supported_contexts_json, settings_schema_json,
    data_endpoint, is_active
) VALUES
    (
        'sales.summary',
        'Ventas del periodo',
        'Resumen operativo de ventas completadas del periodo actual.',
        'control-plane',
        'metric',
        4, 2, 3, 2, 6, 3,
        '["owner","admin","member","service"]'::jsonb,
        '[]'::jsonb,
        '["home","commercial","control"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/sales-summary',
        true
    ),
    (
        'cashflow.summary',
        'Cashflow resumido',
        'Ingresos, egresos y balance del periodo.',
        'control-plane',
        'metric',
        4, 2, 3, 2, 6, 3,
        '["owner","admin","member","service"]'::jsonb,
        '[]'::jsonb,
        '["home","operations","control"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/cashflow-summary',
        true
    ),
    (
        'quotes.pipeline',
        'Presupuestos abiertos',
        'Pipeline de presupuestos por estado.',
        'control-plane',
        'metric',
        4, 2, 3, 2, 6, 3,
        '["owner","admin","member"]'::jsonb,
        '[]'::jsonb,
        '["home","commercial"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/quotes-pipeline',
        true
    ),
    (
        'inventory.low_stock',
        'Alertas de stock',
        'Productos por debajo del minimo definido.',
        'control-plane',
        'list',
        6, 3, 4, 3, 8, 5,
        '["owner","admin","member"]'::jsonb,
        '[]'::jsonb,
        '["home","operations","control"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/low-stock',
        true
    ),
    (
        'sales.recent',
        'Ventas recientes',
        'Ultimas ventas registradas en la organizacion.',
        'control-plane',
        'feed',
        6, 3, 4, 3, 8, 6,
        '["owner","admin","member"]'::jsonb,
        '[]'::jsonb,
        '["home","commercial"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/recent-sales',
        true
    ),
    (
        'products.top',
        'Top productos',
        'Productos con mayor facturacion en el periodo.',
        'control-plane',
        'list',
        6, 3, 4, 3, 8, 6,
        '["owner","admin","member"]'::jsonb,
        '[]'::jsonb,
        '["home","commercial","operations"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/top-products',
        true
    ),
    (
        'billing.subscription',
        'Estado del plan',
        'Plan actual, estado de facturacion y limites.',
        'control-plane',
        'status',
        4, 2, 3, 2, 6, 4,
        '["owner","admin"]'::jsonb,
        '[]'::jsonb,
        '["home","control"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/billing-status',
        true
    ),
    (
        'audit.activity',
        'Actividad reciente',
        'Ultimos eventos relevantes del tenant.',
        'control-plane',
        'feed',
        6, 3, 4, 3, 8, 6,
        '["owner","admin","member","service"]'::jsonb,
        '[]'::jsonb,
        '["home","control","operations"]'::jsonb,
        '{}'::jsonb,
        '/v1/dashboard-data/audit-activity',
        true
    )
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

INSERT INTO dashboard_default_layouts (layout_key, context, name, items_json, is_active) VALUES
    (
        'home.base.v1',
        'home',
        'Home base',
        '[
          {"instance_id":"sales-summary-1","widget_key":"sales.summary","x":0,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":0},
          {"instance_id":"cashflow-summary-1","widget_key":"cashflow.summary","x":4,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":1},
          {"instance_id":"quotes-pipeline-1","widget_key":"quotes.pipeline","x":8,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":false,"order_hint":2},
          {"instance_id":"inventory-low-stock-1","widget_key":"inventory.low_stock","x":0,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":3},
          {"instance_id":"sales-recent-1","widget_key":"sales.recent","x":6,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":4},
          {"instance_id":"top-products-1","widget_key":"products.top","x":0,"y":5,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":5},
          {"instance_id":"billing-status-1","widget_key":"billing.subscription","x":6,"y":5,"w":4,"h":2,"visible":true,"settings":{},"pinned":false,"order_hint":6},
          {"instance_id":"audit-activity-1","widget_key":"audit.activity","x":0,"y":8,"w":12,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":7}
        ]'::jsonb,
        true
    ),
    (
        'commercial.base.v1',
        'commercial',
        'Comercial base',
        '[
          {"instance_id":"sales-summary-1","widget_key":"sales.summary","x":0,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":0},
          {"instance_id":"quotes-pipeline-1","widget_key":"quotes.pipeline","x":4,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":1},
          {"instance_id":"sales-recent-1","widget_key":"sales.recent","x":0,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":2},
          {"instance_id":"top-products-1","widget_key":"products.top","x":6,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":3}
        ]'::jsonb,
        true
    ),
    (
        'operations.base.v1',
        'operations',
        'Operaciones base',
        '[
          {"instance_id":"cashflow-summary-1","widget_key":"cashflow.summary","x":0,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":0},
          {"instance_id":"inventory-low-stock-1","widget_key":"inventory.low_stock","x":0,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":1},
          {"instance_id":"top-products-1","widget_key":"products.top","x":6,"y":2,"w":6,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":2},
          {"instance_id":"audit-activity-1","widget_key":"audit.activity","x":0,"y":5,"w":12,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":3}
        ]'::jsonb,
        true
    ),
    (
        'control.base.v1',
        'control',
        'Control base',
        '[
          {"instance_id":"billing-status-1","widget_key":"billing.subscription","x":0,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":0},
          {"instance_id":"sales-summary-1","widget_key":"sales.summary","x":4,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":1},
          {"instance_id":"cashflow-summary-1","widget_key":"cashflow.summary","x":8,"y":0,"w":4,"h":2,"visible":true,"settings":{},"pinned":true,"order_hint":2},
          {"instance_id":"audit-activity-1","widget_key":"audit.activity","x":0,"y":2,"w":12,"h":3,"visible":true,"settings":{},"pinned":false,"order_hint":3}
        ]'::jsonb,
        true
    )
ON CONFLICT (layout_key) DO UPDATE SET
    context = EXCLUDED.context,
    name = EXCLUDED.name,
    items_json = EXCLUDED.items_json,
    is_active = EXCLUDED.is_active,
    updated_at = now();
