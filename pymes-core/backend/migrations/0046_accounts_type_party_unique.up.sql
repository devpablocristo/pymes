CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_org_type_party_unique
    ON accounts(org_id, type, party_id);
