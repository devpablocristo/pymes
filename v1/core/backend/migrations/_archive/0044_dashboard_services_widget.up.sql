INSERT INTO dashboard_widgets_catalog (
    widget_key, title, description, domain, kind,
    default_width, default_height, min_width, min_height, max_width, max_height,
    allowed_roles_json, required_scopes_json, supported_contexts_json, settings_schema_json,
    data_endpoint, is_active
) VALUES (
    'services.top',
    'Top servicios',
    'Servicios con mayor facturacion en el periodo.',
    'control-plane',
    'list',
    6, 3, 4, 3, 8, 6,
    '["owner","admin","member"]'::jsonb,
    '[]'::jsonb,
    '["home","commercial","operations"]'::jsonb,
    '{}'::jsonb,
    '/v1/dashboard-data/top-services',
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
