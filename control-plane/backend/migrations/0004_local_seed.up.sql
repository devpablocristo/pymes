-- Seed data for local development only.
-- API key: psk_local_admin (SHA256: 91678ad136f46807fd001e50281fcc842e4b40388a83a85c5ea069c4383e739a)

INSERT INTO orgs (id, external_id, name, slug)
VALUES ('00000000-0000-0000-0000-000000000001', 'org_local', 'Local Dev Org', 'local-dev')
ON CONFLICT (id) DO NOTHING;

INSERT INTO users (id, external_id, email, name)
VALUES ('00000000-0000-0000-0000-000000000002', 'local-admin', 'admin@local.dev', 'Local Admin')
ON CONFLICT (id) DO NOTHING;

INSERT INTO org_members (id, org_id, user_id, role)
VALUES (
    '00000000-0000-0000-0000-000000000003',
    '00000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000002',
    'admin'
)
ON CONFLICT (org_id, user_id) DO NOTHING;

INSERT INTO tenant_settings (org_id, plan_code)
VALUES ('00000000-0000-0000-0000-000000000001', 'starter')
ON CONFLICT (org_id) DO NOTHING;

INSERT INTO org_api_keys (id, org_id, name, key_hash, key_prefix, created_by)
VALUES (
    '00000000-0000-0000-0000-000000000004',
    '00000000-0000-0000-0000-000000000001',
    'local-dev-key',
    '91678ad136f46807fd001e50281fcc842e4b40388a83a85c5ea069c4383e739a',
    'psk_local_adm',
    'seed'
)
ON CONFLICT (key_hash) DO NOTHING;

INSERT INTO org_api_key_scopes (id, key_id, scope) VALUES
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:read'),
    (gen_random_uuid(), '00000000-0000-0000-0000-000000000004', 'admin:console:write')
ON CONFLICT (key_id, scope) DO NOTHING;
