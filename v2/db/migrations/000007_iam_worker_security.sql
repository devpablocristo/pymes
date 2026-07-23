-- The process holding Clerk's secret gets a narrower database role than the
-- public API. RLS on the shared outbox makes the provisioning projection a
-- real authorization boundary instead of only a query convenience.

ALTER TABLE public.platform_outbox_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.platform_outbox_messages FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS pymes_outbox_application_access
ON public.platform_outbox_messages;
CREATE POLICY pymes_outbox_application_access
ON public.platform_outbox_messages
AS PERMISSIVE
FOR ALL
TO PUBLIC
USING (current_user IN ('pymes_backend', 'pymes_iam_worker'))
WITH CHECK (current_user IN ('pymes_backend', 'pymes_iam_worker'));

DROP POLICY IF EXISTS pymes_outbox_worker_topic_boundary
ON public.platform_outbox_messages;
CREATE POLICY pymes_outbox_worker_topic_boundary
ON public.platform_outbox_messages
AS RESTRICTIVE
FOR ALL
TO PUBLIC
USING (
    current_user <> 'pymes_iam_worker'
    OR topic = 'iam.organization.provision.requested.v1'
)
WITH CHECK (
    current_user <> 'pymes_iam_worker'
    OR topic = 'iam.organization.provision.requested.v1'
);

DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_iam_worker') THEN
        GRANT USAGE ON SCHEMA app, iam TO pymes_iam_worker;

        GRANT SELECT, UPDATE
        ON TABLE public.platform_outbox_messages
        TO pymes_iam_worker;

        GRANT SELECT, UPDATE
        ON TABLE
            app.organization_provisioning_requests,
            app.organization_provisioning_outbox_messages
        TO pymes_iam_worker;

        GRANT SELECT, UPDATE
        ON TABLE iam.organizations
        TO pymes_iam_worker;

        GRANT SELECT, INSERT, UPDATE
        ON TABLE iam.invitations
        TO pymes_iam_worker;
    END IF;
END
$grant$;
