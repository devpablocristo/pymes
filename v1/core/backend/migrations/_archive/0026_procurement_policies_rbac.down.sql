-- Reverso de 0026_procurement_policies_rbac.up.sql
-- Quita los permisos RBAC del recurso procurement_policies.

DELETE FROM role_permissions
WHERE resource = 'procurement_policies'
  AND action IN ('read', 'create', 'update', 'delete');
