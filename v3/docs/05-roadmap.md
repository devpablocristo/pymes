# Roadmap y estado

| Etapa | Estado | Evidencia / próximo alcance |
|---|---|---|
| 0. Fundaciones | completa | Hexagonal, CI, Clerk, RLS, provisionamiento, JWT Ed25519, outbox, métricas y contratos generados. KMS fiscal pertenece a la fase ARCA diferida. |
| 1. Núcleo contable | completa | Binario OA headless, schema explícito, decimal exacto, períodos, posteos, reversas, partidas, aplicaciones y reportes. |
| 2. Fiscal mock | completa | PostgreSQL durable; A/B/C, NC/ND, IVA, moneda extranjera, timeout, respuesta perdida y consulta exacta. |
| 2. Fiscal ARCA real | diferida por decisión explícita | WSAA/WSFEv1, certificados/KMS, homologación y SDK publicado. |
| 3. Documentos v3 | completa para backend MVP | Parties, ventas/compras, numeración, NC/ND, cobros/pagos, aplicaciones y reversas. |
| 4. Operación local/CI | completa | Health/readiness, timeouts, circuit breakers, métricas, backups/restores y recuperación E2E. Despliegue productivo y piloto dependen de la fase ARCA. |
| 5. Extensiones | futura | Padrón, FCE, WSFEX, CAEA, cierres/FX avanzados y migración v2. |

No se empieza UI amplia ni migración de datos antes de completar las etapas 0--2.
La puerta entre etapas es evidencia automática, no una aprobación informal.
# Estado de integración ARCA

La homologación contra ARCA, carga de certificados reales y piloto de emisión
quedan diferidos a una fase posterior. El adaptador y sus contratos se
mantienen preparados, pero ningún despliegue de v3 puede emitir comprobantes
reales hasta contar con la organización piloto, credenciales y aprobación de
homologación. Esta decisión evita habilitar emisión productiva sólo con fakes.
