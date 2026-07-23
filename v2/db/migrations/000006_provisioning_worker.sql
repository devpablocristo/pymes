-- Forward-only installation of the provisioning-only outbox projection.
-- Keeping the worker on a filtered view prevents it from leasing unrelated
-- IAM commands from the shared platform outbox table.

CREATE OR REPLACE VIEW app.organization_provisioning_outbox_messages
WITH (security_invoker = true)
AS
SELECT *
FROM public.platform_outbox_messages
WHERE topic = 'iam.organization.provision.requested.v1'
WITH LOCAL CHECK OPTION;

REVOKE ALL ON TABLE app.organization_provisioning_outbox_messages FROM PUBLIC;

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT USAGE ON SCHEMA app TO pymes_backend;
        GRANT SELECT, UPDATE
        ON TABLE app.organization_provisioning_outbox_messages
        TO pymes_backend;
    END IF;
END
$grant$;
