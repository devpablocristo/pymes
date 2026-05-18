ALTER TABLE IF EXISTS public.services RENAME TO catalog_services;
ALTER TABLE IF EXISTS public.system_services RENAME TO services;

ALTER INDEX IF EXISTS idx_services_org_code RENAME TO idx_catalog_services_org_code;
ALTER INDEX IF EXISTS idx_services_org RENAME TO idx_catalog_services_org;
ALTER INDEX IF EXISTS idx_services_org_name RENAME TO idx_catalog_services_org_name;
