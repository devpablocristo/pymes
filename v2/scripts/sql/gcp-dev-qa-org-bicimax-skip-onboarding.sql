-- =============================================================================
-- QA dev Pymes: saltear onboarding para URLs tipo /bicimax/services
--
-- SOLO ejecutar contra la base `pymes` de Cloud SQL del proyecto Pymes
-- (ej. instancia pymes-dev-db en pymes-dev-352318). NO usar en Ponti ni otros tenants.
--
-- Requisitos previos:
-- 1) En Clerk (mismo instance que el frontend dev): creá una Organization para QA
--    o usá una existente y copiá su Organization ID (empieza con org_...).
-- 2) Invitá al usuario de QA a esa organización (Admin/Developer según necesiten).
-- 3) El usuario inicia sesión UNA VEZ en https://pymes-dev-352318.web.app con esa
--    organización activa en Clerk. Eso crea en BD la fila `orgs` (external_id = org_...)
--    y `org_settings` mínimos vía auto-provision.
-- 4) Sustituí ORG_ID_LITERAL abajo por ese org_... y ejecutá este script con psql.
--
-- Efecto: onboarding marcado como completo, nombre comercial "Bicimax" (slug URL bicimax),
-- vertical taller (workshops) orientado a servicios. El perfil local en el navegador se
-- hidrata en el próximo GET /v1/admin/tenant-settings.
--
-- Si el navegador tenía perfil viejo: borrar datos del sitio o la clave localStorage
-- del namespace pymes-ui antes de reintentar.
-- =============================================================================

BEGIN;

UPDATE orgs
SET
  name = 'Bicimax',
  slug = 'bicimax',
  updated_at = now()
WHERE external_id = 'ORG_ID_LITERAL';

UPDATE org_settings
SET
  business_name = 'Bicimax',
  team_size = 'small',
  sells = 'services',
  payment_method = 'mixed',
  vertical = 'workshops',
  client_label = 'clientes',
  uses_billing = false,
  scheduling_enabled = true,
  currency = 'ARS',
  onboarding_completed_at = now(),
  updated_at = now()
WHERE org_id = (SELECT id FROM orgs WHERE external_id = 'ORG_ID_LITERAL');

COMMIT;
