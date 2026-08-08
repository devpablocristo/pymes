# ADR 0001: Pymes orquesta; contabilidad y fiscal son privados

**Estado:** parcialmente reemplazada por ADR 0014 respecto del origen de
Accounting y Fiscal; se conserva la separación por red y base.
**Decisión:** Pymes v3 conserva documentos, IAM, organización, numeración y
estado fiscal. Accounting conserva exclusivamente el libro y sus reportes.
Fiscal encapsula protocolo ARCA y secretos, pero no toma propiedad del
comprobante comercial.

**Consecuencia:** no hay acceso de navegador, tablas compartidas ni APIs OA
actuales expuestas. Se paga el coste de outbox/inbox para obtener límites claros.
