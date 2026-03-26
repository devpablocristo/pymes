-- Notificaciones in-app por usuario (bandeja + contexto para el asistente).
-- Reemplaza cualquier borrador previo con otro esquema (p. ej. actor_id) para evitar índices rotos.
DROP TABLE IF EXISTS in_app_notifications CASCADE;

CREATE TABLE in_app_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title text NOT NULL,
    body text NOT NULL,
    kind text NOT NULL DEFAULT 'system',
    entity_type text NOT NULL DEFAULT '',
    entity_id text NOT NULL DEFAULT '',
    chat_context jsonb NOT NULL DEFAULT '{}'::jsonb,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_in_app_notifications_user_created
    ON in_app_notifications (user_id, created_at DESC);

CREATE INDEX idx_in_app_notifications_org_created
    ON in_app_notifications (org_id, created_at DESC);
