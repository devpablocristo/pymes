-- Conexiones de sync con calendarios externos (Google, Outlook, etc.).
--
-- Cada usuario interno puede conectar UNA cuenta externa por proveedor.
-- La conexión guarda el refresh_token encriptado (AES-GCM, mismo Crypto que
-- usa paymentgateway). El access_token también se cachea encriptado para
-- evitar refreshes innecesarios entre syncs frecuentes.
--
-- sync_token (Google) / delta_link (Outlook) son punteros opacos del proveedor
-- para hacer pulls incrementales. Si están vacíos, el próximo pull es full.
CREATE TABLE IF NOT EXISTS calendar_sync_connections (
    id                       uuid PRIMARY KEY,
    org_id                   uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    -- created_by sigue el patrón text del resto del sistema (ver
    -- scheduling_bookings, calendar_export_tokens). UUID interno o external_id.
    created_by               text NOT NULL DEFAULT '',
    provider                 text NOT NULL,                 -- 'google' | 'microsoft'
    provider_account_email   text NOT NULL DEFAULT '',     -- para mostrar "conectado como foo@gmail.com"
    provider_calendar_id     text NOT NULL DEFAULT '',     -- ID del calendario remoto al que sincronizamos
    provider_calendar_name   text NOT NULL DEFAULT '',
    scopes                   text NOT NULL DEFAULT '',     -- scopes concedidos (space-separated)
    refresh_token_encrypted  text NOT NULL,
    access_token_encrypted   text NOT NULL DEFAULT '',
    access_token_expires_at  timestamptz,
    sync_token               text NOT NULL DEFAULT '',     -- delta sync token del proveedor
    last_sync_at             timestamptz,
    last_sync_error          text NOT NULL DEFAULT '',
    revoked_at               timestamptz,
    created_at               timestamptz NOT NULL DEFAULT now(),
    updated_at               timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT calendar_sync_connections_provider_check
        CHECK (provider IN ('google', 'microsoft'))
);

-- Una conexión activa por (org, creator, provider). Un mismo usuario no puede
-- conectar dos cuentas Google a la vez; debe revocar la anterior primero.
CREATE UNIQUE INDEX IF NOT EXISTS uidx_calendar_sync_connections_active
    ON calendar_sync_connections (org_id, created_by, provider)
    WHERE revoked_at IS NULL;

-- Estado del flujo OAuth: cada `state` random emitido por StartConnect se
-- guarda hasta que llegue el callback. TTL corto (15 min). Se borran los
-- expirados al iniciar un nuevo flow.
CREATE TABLE IF NOT EXISTS calendar_sync_oauth_states (
    state       text PRIMARY KEY,
    org_id      uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    created_by  text NOT NULL DEFAULT '',
    provider    text NOT NULL,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_calendar_sync_oauth_states_expiry
    ON calendar_sync_oauth_states (expires_at);
