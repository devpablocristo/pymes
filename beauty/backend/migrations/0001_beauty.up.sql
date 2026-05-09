-- 0001_beauty.up.sql (vertical Beauty — squashed)
-- Schema isolado en `beauty.*`. Las tablas legacy (salon_services,
-- staff_members) fueron dropped en 0004/0005 — el squash arranca con un
-- estado mínimo. Si más adelante el vertical agrega features nuevas, van
-- en 0002_beauty_*, 0003_beauty_*, etc.

CREATE SCHEMA IF NOT EXISTS beauty;

-- TODO(beauty-vertical): post-squash el schema queda vacío. El módulo Go
-- (`beauty/backend/internal/`) no debe asumir tablas dropeadas. Si se
-- reactivan stylists / salons / etc., agregar tablas aquí o en migración
-- siguiente respetando convenciones (org_id, archived_at, ON DELETE
-- explícito, set_updated_at trigger).
