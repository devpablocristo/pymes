DELETE FROM audit_log WHERE org_id NOT IN (SELECT id FROM orgs);

ALTER TABLE audit_log
    ADD CONSTRAINT fk_audit_log_org FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE;
