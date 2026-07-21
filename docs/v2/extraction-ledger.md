# Ledger de extracción v1 → v2

Cada capacidad debe registrar aquí su decisión antes de implementarse.

| Capacidad v1 | Decisión | Destino | Condición de aceptación |
| --- | --- | --- | --- |
| Identidad y organizaciones | Reconstruir | v2 + SaaS platform | Sesión verificada y aislamiento cross-org |
| Customers y suppliers | Unificar | v2 Party con roles | Una identidad puede asumir ambos roles |
| Productos y servicios | Reconstruir | v2 Catalog | Money exacto y lifecycle canónico |
| Presupuestos, ventas y pagos | Reconstruir | v2 Commercial | Transacción única, idempotencia y outbox |
| Ledger | Generalizar motor | platform + reglas v2 | Asientos exactos, balanceados e inmutables |
| ARCA | Generalizar SDK | platform SDK + reglas v2 | Homologación y cero comprobantes duplicados |
| Workshops/professionals/restaurants/beauty/medical | Conservar | v1 | Fuera del núcleo horizontal inicial |
| Expo mobile | Conservar | v1 | Reconsiderar después de estabilizar APIs |
| Assistant, agentes, WhatsApp y gateways | Diferir | v1 | Requiere RFC y evidencia de producto |

Cada fila reconstruida se ampliará con rutas de referencia, tests de aceptación,
dependencias y PR reemplazante. Una referencia histórica nunca es una
dependencia de runtime.
