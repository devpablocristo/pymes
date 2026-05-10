ALTER TABLE IF EXISTS public.services RENAME TO system_services;
ALTER TABLE IF EXISTS public.catalog_services RENAME TO services;

ALTER INDEX IF EXISTS idx_catalog_services_org_code RENAME TO idx_services_org_code;
ALTER INDEX IF EXISTS idx_catalog_services_org RENAME TO idx_services_org;
ALTER INDEX IF EXISTS idx_catalog_services_org_name RENAME TO idx_services_org_name;
