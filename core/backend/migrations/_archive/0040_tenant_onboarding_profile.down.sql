-- Reverso de 0040_tenant_onboarding_profile.up.sql
-- WARNING: borra todas las columnas de perfil de onboarding del tenant.
-- Cualquier dato de onboarding (vertical elegida, team_size, etc.) se pierde.

ALTER TABLE tenant_settings DROP COLUMN IF EXISTS onboarding_completed_at;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS vertical;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS payment_method;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS uses_billing;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS client_label;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS sells;
ALTER TABLE tenant_settings DROP COLUMN IF EXISTS team_size;
