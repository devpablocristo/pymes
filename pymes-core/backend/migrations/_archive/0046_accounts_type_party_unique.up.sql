CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_org_type_party_unique
    ON accounts(tenant_id, type, party_id);
