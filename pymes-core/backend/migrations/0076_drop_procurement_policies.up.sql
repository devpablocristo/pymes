-- Drop tabla local de policies CEL.
--
-- Después de la refactorización "gobernanza siempre Nexus, IA siempre
-- Companion", las policies de procurement viven en Nexus como source of
-- truth, scopeadas por tenant. Pymes las consume vía governanceclient (HTTP)
-- y ya no embebe motor ni almacena policies localmente.
--
-- IMPORTANTE: antes de aplicar esta migración, exportar las rows existentes
-- y POSTear cada una a Nexus /v1/policies con scope de tenant.
-- Ver scripts/migrate_procurement_policies_to_nexus.sh --apply.
--
-- Si la tabla está vacía (clusters dev sin data), el drop es no-op.

DROP TABLE IF EXISTS procurement_policies;
