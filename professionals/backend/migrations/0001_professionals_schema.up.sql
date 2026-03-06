CREATE SCHEMA IF NOT EXISTS professionals;

-- Professional profiles linked to a party in the control-plane
CREATE TABLE professionals.professional_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id          UUID NOT NULL,
    party_id        UUID NOT NULL,
    public_slug     TEXT NOT NULL,
    bio             TEXT NOT NULL DEFAULT '',
    headline        TEXT NOT NULL DEFAULT '',
    is_public       BOOLEAN NOT NULL DEFAULT FALSE,
    is_bookable     BOOLEAN NOT NULL DEFAULT FALSE,
    accepts_new_clients BOOLEAN NOT NULL DEFAULT TRUE,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, public_slug)
);
CREATE INDEX idx_professional_profiles_org_id ON professionals.professional_profiles (org_id);

-- Specialties catalog per organization
CREATE TABLE professionals.specialties (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    code        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, code)
);
CREATE INDEX idx_specialties_org_id ON professionals.specialties (org_id);

-- Join table: professional <-> specialty
CREATE TABLE professionals.professional_specialties (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL,
    profile_id    UUID NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    specialty_id  UUID NOT NULL REFERENCES professionals.specialties(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, profile_id, specialty_id)
);

-- Links between professional profiles and products in the control-plane catalog
CREATE TABLE professionals.professional_service_links (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id              UUID NOT NULL,
    profile_id          UUID NOT NULL REFERENCES professionals.professional_profiles(id) ON DELETE CASCADE,
    product_id          UUID NOT NULL,
    public_description  TEXT NOT NULL DEFAULT '',
    display_order       INT NOT NULL DEFAULT 0,
    is_featured         BOOLEAN NOT NULL DEFAULT FALSE,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_service_links_org_id ON professionals.professional_service_links (org_id);

-- Intake forms / questionnaires
CREATE TABLE professionals.intakes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID NOT NULL,
    appointment_id    UUID,
    profile_id        UUID NOT NULL REFERENCES professionals.professional_profiles(id),
    customer_party_id UUID,
    product_id        UUID,
    status            TEXT NOT NULL DEFAULT 'draft',
    payload           JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_intakes_org_id ON professionals.intakes (org_id);
CREATE INDEX idx_intakes_org_status ON professionals.intakes (org_id, status);

-- Sessions: one per appointment
CREATE TABLE professionals.sessions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id            UUID NOT NULL,
    appointment_id    UUID NOT NULL,
    profile_id        UUID NOT NULL REFERENCES professionals.professional_profiles(id),
    customer_party_id UUID,
    product_id        UUID,
    status            TEXT NOT NULL DEFAULT 'scheduled',
    started_at        TIMESTAMPTZ,
    ended_at          TIMESTAMPTZ,
    summary           TEXT NOT NULL DEFAULT '',
    metadata          JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, appointment_id)
);
CREATE INDEX idx_sessions_org_id ON professionals.sessions (org_id);
CREATE INDEX idx_sessions_org_status ON professionals.sessions (org_id, status);

-- Clinical / session notes
CREATE TABLE professionals.session_notes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    session_id  UUID NOT NULL REFERENCES professionals.sessions(id) ON DELETE CASCADE,
    note_type   TEXT NOT NULL DEFAULT 'general',
    title       TEXT NOT NULL DEFAULT '',
    body        TEXT NOT NULL,
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_session_notes_session_id ON professionals.session_notes (session_id);
