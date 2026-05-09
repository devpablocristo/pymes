-- 0003_notifications.up.sql
-- Notification preferences (per user), notification log (org-scoped envío
-- de email/sms/etc.), in-app notifications.
--
-- Schema saas como source of truth. Pymes-core/0036/0037 (rename a
-- pymes_notification_*) desaparecen porque ya no hay colisión con la lib.

CREATE TABLE IF NOT EXISTS notification_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT notification_preferences_user_type_channel_uniq
        UNIQUE (user_id, notification_type, channel)
);
CREATE INDEX IF NOT EXISTS idx_notification_prefs_user
    ON notification_preferences(user_id);

CREATE TABLE IF NOT EXISTS notification_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    notification_type text NOT NULL,
    channel text NOT NULL DEFAULT 'email',
    recipient text NOT NULL,
    subject text NOT NULL,
    status text NOT NULL DEFAULT 'sent'
        CONSTRAINT notification_log_status_check
        CHECK (status IN ('queued','sent','delivered','failed','bounced')),
    dedup_key text,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_notification_log_org_created
    ON notification_log(org_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_log_dedup_key
    ON notification_log(dedup_key) WHERE dedup_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS in_app_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor_id text NOT NULL DEFAULT '',
    type text NOT NULL,
    title text NOT NULL,
    body text NOT NULL DEFAULT '',
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_inapp_notif_org_unread
    ON in_app_notifications(org_id, read_at) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_inapp_notif_actor_created
    ON in_app_notifications(actor_id, created_at DESC);

CREATE TRIGGER trg_notification_preferences_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
