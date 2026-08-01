BEGIN;

ALTER TABLE app.notification_settings
  ADD COLUMN IF NOT EXISTS pergo_channel text,
  ADD COLUMN IF NOT EXISTS pergo_sender_identity text;

ALTER TABLE app.notification_settings
  DROP CONSTRAINT IF EXISTS notification_settings_pergo_channel_check,
  DROP CONSTRAINT IF EXISTS notification_settings_pergo_sender_check,
  DROP CONSTRAINT IF EXISTS notification_settings_pergo_route_pair_check;

ALTER TABLE app.notification_settings
  ADD CONSTRAINT notification_settings_pergo_channel_check CHECK (
    pergo_channel IS NULL OR
    pergo_channel IN ('whatsapp','whatsapp_cloud','whatsapp_mock')
  ),
  ADD CONSTRAINT notification_settings_pergo_sender_check CHECK (
    pergo_sender_identity IS NULL OR
    char_length(pergo_sender_identity) BETWEEN 1 AND 255
  ),
  ADD CONSTRAINT notification_settings_pergo_route_pair_check CHECK (
    (pergo_channel IS NULL) = (pergo_sender_identity IS NULL)
  );

ALTER TABLE app.notifications
  ADD COLUMN IF NOT EXISTS delivery_channel text,
  ADD COLUMN IF NOT EXISTS sender_identity text;

ALTER TABLE app.notifications
  DROP CONSTRAINT IF EXISTS notifications_delivery_channel_check,
  DROP CONSTRAINT IF EXISTS notifications_sender_identity_check,
  DROP CONSTRAINT IF EXISTS notifications_delivery_route_pair_check;

ALTER TABLE app.notifications
  ADD CONSTRAINT notifications_delivery_channel_check CHECK (
    delivery_channel IS NULL OR
    delivery_channel IN ('whatsapp','whatsapp_cloud','whatsapp_mock')
  ),
  ADD CONSTRAINT notifications_sender_identity_check CHECK (
    sender_identity IS NULL OR char_length(sender_identity) BETWEEN 1 AND 255
  ),
  ADD CONSTRAINT notifications_delivery_route_pair_check CHECK (
    (delivery_channel IS NULL) = (sender_identity IS NULL)
  );

COMMIT;
