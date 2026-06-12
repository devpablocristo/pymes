DELETE FROM audit_log WHERE tenant_id NOT IN (SELECT id FROM tenants);

ALTER TABLE audit_log
    ADD CONSTRAINT fk_audit_log_org FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;
