-- Reverso de 0025_procurement_requests.up.sql
-- Elimina solicitudes de compra/gasto + políticas CEL.
-- WARNING: borra datos. Asegurate de no necesitarlos antes de correr down.

DELETE FROM role_permissions
WHERE resource = 'procurement_requests'
  AND action IN ('read', 'create', 'update', 'submit', 'approve', 'reject');

DROP INDEX IF EXISTS idx_procurement_policies_org;
DROP TABLE IF EXISTS procurement_policies;

DROP INDEX IF EXISTS idx_procurement_request_lines_request;
DROP TABLE IF EXISTS procurement_request_lines;

DROP INDEX IF EXISTS idx_procurement_requests_archived;
DROP INDEX IF EXISTS idx_procurement_requests_status;
DROP INDEX IF EXISTS idx_procurement_requests_org;
DROP TABLE IF EXISTS procurement_requests;
