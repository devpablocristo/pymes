-- 0001_professionals.up.sql (vertical Professionals — squashed)
-- Schema isolado en `professionals.*` con FK a orgs(id) en pymes-core.
-- Consolida: 0001..0008 actuales (post `0008_complete_tenant_schema_rename`).

CREATE SCHEMA IF NOT EXISTS professionals;

CREATE TABLE IF NOT EXISTS professionals.professional_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_id uuid NOT NULL,
    public_slug text NOT NULL,
    bio text NOT NULL DEFAULT '',
    headline text NOT NULL DEFAULT '',
    is_public boolean NOT NULL DEFAULT false,
    is_bookable boolean NOT NULL DEFAULT false,
    accepts_new_clients boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT professional_profiles_org_slug_uniq UNIQUE (org_id, public_slug)
);
CREATE INDEX IF NOT EXISTS idx_professional_profiles_org
    ON professionals.professional_profiles(org_id);
CREATE INDEX IF NOT EXISTS idx_professional_profiles_deleted_at
    ON professionals.professional_profiles(org_id, deleted_at);

CREATE TRIGGER trg_professional_profiles_updated_at
    BEFORE UPDATE ON professionals.professional_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS professionals.specialties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT specialties_org_code_uniq UNIQUE (org_id, code)
);
CREATE INDEX IF NOT EXISTS idx_specialties_org ON professionals.specialties(org_id);
CREATE INDEX IF NOT EXISTS idx_specialties_deleted_at
    ON professionals.specialties(org_id, deleted_at);

CREATE TRIGGER trg_specialties_updated_at
    BEFORE UPDATE ON professionals.specialties
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS professionals.professional_specialties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    specialty_id uuid NOT NULL REFERENCES professionals.specialties(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT professional_specialties_org_profile_specialty_uniq
        UNIQUE (org_id, profile_id, specialty_id)
);

CREATE TABLE IF NOT EXISTS professionals.professional_service_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    profile_id uuid NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    product_id uuid,
    service_id uuid,
    public_description text NOT NULL DEFAULT '',
    display_order int NOT NULL DEFAULT 0,
    is_featured boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_service_links_org
    ON professionals.professional_service_links(org_id);
CREATE INDEX IF NOT EXISTS idx_service_links_org_service
    ON professionals.professional_service_links(org_id, service_id) WHERE service_id IS NOT NULL;

CREATE TRIGGER trg_professional_service_links_updated_at
    BEFORE UPDATE ON professionals.professional_service_links
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS professionals.intakes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    booking_id uuid,
    profile_id uuid NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    customer_party_id uuid,
    product_id uuid,
    service_id uuid,
    status text NOT NULL DEFAULT 'draft',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    is_favorite boolean NOT NULL DEFAULT false,
    tags text[] NOT NULL DEFAULT '{}'::text[],
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_intakes_org ON professionals.intakes(org_id);
CREATE INDEX IF NOT EXISTS idx_intakes_org_status ON professionals.intakes(org_id, status);
CREATE INDEX IF NOT EXISTS idx_intakes_deleted_at
    ON professionals.intakes(org_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_intakes_org_service
    ON professionals.intakes(org_id, service_id) WHERE service_id IS NOT NULL;

CREATE TRIGGER trg_intakes_updated_at
    BEFORE UPDATE ON professionals.intakes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS professionals.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    booking_id uuid NOT NULL,
    profile_id uuid NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    customer_party_id uuid,
    product_id uuid,
    service_id uuid,
    status text NOT NULL DEFAULT 'scheduled',
    started_at timestamptz,
    ended_at timestamptz,
    summary text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sessions_org_booking_uniq UNIQUE (org_id, booking_id)
);
CREATE INDEX IF NOT EXISTS idx_sessions_org ON professionals.sessions(org_id);
CREATE INDEX IF NOT EXISTS idx_sessions_org_status ON professionals.sessions(org_id, status);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at
    ON professionals.sessions(org_id, deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_org_service
    ON professionals.sessions(org_id, service_id) WHERE service_id IS NOT NULL;

CREATE TRIGGER trg_sessions_updated_at
    BEFORE UPDATE ON professionals.sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS professionals.session_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES professionals.sessions(id) ON DELETE CASCADE,
    note_type text NOT NULL DEFAULT 'general',
    title text NOT NULL DEFAULT '',
    body text NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_session_notes_org
    ON professionals.session_notes(org_id);
CREATE INDEX IF NOT EXISTS idx_session_notes_session
    ON professionals.session_notes(session_id);
