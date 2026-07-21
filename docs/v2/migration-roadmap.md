# Roadmap de implementación

1. Congelar y verificar v1.
2. Publicar primitives faltantes de `platform`: Money, transacciones, tenancy,
   SaaS PostgreSQL, idempotencia, outbox y transporte web por instancia.
3. Crear runtime técnico v2, OpenAPI, web shell y base PostgreSQL nueva.
4. Implementar identidad/organizaciones y pruebas cross-org.
5. Entregar parties, catálogo, precios, inventario y secuencias.
6. Entregar presupuestos, ventas, pagos, compras y devoluciones.
7. Incorporar ledger genérico, reglas contables y fiscalidad ARCA.
8. Endurecer auditoría, observabilidad, accesibilidad, backups y E2E antes de
   habilitar CD.

Cada capacidad se divide en dominio/migración, persistencia/API y web. Un PR no
mezcla extracción de plataforma con adopción en Pymes.
