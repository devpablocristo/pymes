DO $grant$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pymes_backend') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE
        ON TABLE public.platform_outbox_messages
        TO pymes_backend;
    END IF;
END
$grant$;
