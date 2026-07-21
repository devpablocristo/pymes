-- 0002_audit_and_governance.down.sql

DROP TRIGGER IF EXISTS trg_protected_resources_updated_at ON protected_resources;

DROP TABLE IF EXISTS webhook_events_clerk;
DROP TABLE IF EXISTS org_invitations;
DROP TABLE IF EXISTS restore_evidence;
DROP TABLE IF EXISTS protected_resources;
DROP TABLE IF EXISTS audit_log;
