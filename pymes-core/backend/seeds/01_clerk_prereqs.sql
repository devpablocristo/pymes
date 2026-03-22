-- Prerrequisitos cuando la org ya existe (Clerk: external_id = org_...).
-- El placeholder __SEED_ORG_ID__ se reemplaza en Go por el UUID interno (no se toca external_id).
-- API key demo: hash único global; ON CONFLICT reasigna la clave a esta org en dev.

INSERT INTO tenant_settings (org_id, plan_code)
VALUES ('__SEED_ORG_ID__', 'starter')
ON CONFLICT (org_id) DO NOTHING;

INSERT INTO org_api_keys (id, org_id, name, api_key_hash, key_prefix, created_by)
VALUES (
    '00000000-0000-0000-0000-000000000004',
    '__SEED_ORG_ID__',
    'local-dev-key',
    '91678ad136f46807fd001e50281fcc842e4b40388a83a85c5ea069c4383e739a',
    'psk_local_adm',
    'seed'
)
ON CONFLICT (api_key_hash) DO UPDATE SET
    org_id = EXCLUDED.org_id,
    name = EXCLUDED.name,
    key_prefix = EXCLUDED.key_prefix;

INSERT INTO org_api_key_scopes (id, api_key_id, scope) VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:read'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:write')
ON CONFLICT (api_key_id, scope) DO NOTHING;
