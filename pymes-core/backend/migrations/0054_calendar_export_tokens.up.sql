-- Tokens de export de calendario.
--
-- Cada usuario interno puede emitir tokens largos y opacos para suscribir su
-- agenda desde apps externas (Apple Calendar, Google Calendar, Outlook,
-- Thunderbird) vía URL pública. El cliente externo no requiere auth Clerk: la
-- propia URL contiene un token con suficiente entropía como para ser tratado
-- como secret.
--
-- En DB sólo se guarda el HASH del token (sha256), nunca el plaintext. La
-- única vez que el plaintext aparece es en la respuesta del POST de creación;
-- después el dueño tiene que copiarlo o regenerarlo. Mismo modelo que GitHub
-- personal access tokens.
CREATE TABLE IF NOT EXISTS calendar_export_tokens (
    id            uuid PRIMARY KEY,
    org_id        uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    -- created_by es el actor del request que generó el token (UUID interno o
    -- external_id de Clerk). Mismo patrón que scheduling_bookings.created_by:
    -- text libre, no FK estricto, para evitar fricción cuando el actor viene
    -- por API key. Permite filtrar "mis tokens" por igualdad de string.
    created_by    text NOT NULL DEFAULT '',
    -- Etiqueta libre para que el usuario distinga "iPhone personal" de
    -- "Mac de la oficina" en la UI.
    name          text NOT NULL DEFAULT '',
    -- token_hash es sha256(plaintext). Búsqueda por hash en el endpoint
    -- público es O(log n) gracias al índice único.
    token_hash    text NOT NULL,
    -- scopes deja la puerta abierta a futuros recortes (ej: 'bookings_only',
    -- 'events_only'). Por ahora sólo 'all' está implementado, pero el campo
    -- existe para no requerir migración cuando se agregue.
    scopes        text NOT NULL DEFAULT 'all',
    last_used_at  timestamptz,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT calendar_export_tokens_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_calendar_export_tokens_org_creator
    ON calendar_export_tokens (org_id, created_by)
    WHERE revoked_at IS NULL;
