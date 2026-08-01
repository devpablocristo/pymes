# Inventario y evidencia

Fecha del estudio: 2026-07-30. Los SHAs son una fotografía, no un requisito de
despliegue. Pymes v1/v2 se inspeccionaron en sólo lectura; el worktree de Pymes
estaba sucio y no fue alterado.

| Proyecto | Fotografía | Stack / licencia | Hallazgo y uso decidido |
|---|---|---|---|
| Pymes | `9226cd778878` | Go, Postgres, Clerk; licencia no declarada en raíz | v3 es producto nuevo. v2 aporta semántica ARCA, outbox, leases y storage seguro. |
| Open Accounting | `8f0d27ef9ee9` | Go 1.26, Chi/GORM/pgx, Postgres 16, Svelte; MIT | Núcleo candidato para contabilidad, pero su API, UI y tenancy pública no se exponen. |
| arca-facturacion | `a53f58c` | TypeScript, Vitest; MIT | SDK candidato para WSAA/WSFE y servicios ARCA; se encapsula. |
| pyafipws | `d595b07` | Python; LGPLv3+ con excepción comercial | Oráculo de compatibilidad, fixtures y catálogo de casos. No segundo emisor. |
| LedgerSMB | `2645029d6` | Perl/Postgres; GPLv2 | Referencia de reglas de diario, pagos, cierre y reversas. No se copia código. |

Se indexaron los cinco repositorios para el estudio. Los índices registraron,
respectivamente, 15.735 nodos Pymes, 18.481 Open Accounting, 364 ARCA SDK,
4.620 pyafipws y 5.691 LedgerSMB.

## Evidencia comprobada

### Pymes v2

El flujo fiscal de v2 mantiene una solicitud persistida, toma una lease para el
worker, conserva un snapshot exacto y distingue la respuesta perdida de un
rechazo. En una incertidumbre consulta el comprobante por el mismo número antes
de volver a emitir (`TestProcessorRecoversExactUncertainNumberWithoutReauthorizing`).
La finalización autorizada persiste la intención de contabilizar en la misma
unidad de trabajo (`TestFinalizeAuthorizedPersistsAccountingIntentInSameUnit`).

Su catálogo actual cubre A/B/C, NC/ND, IVA 0/2,5/5/10,5/21/27 y ARS/USD/EUR.
También tiene WSAA, validación de respuestas WSFE y un plan contable posterior
al CAE con snapshots y redondeo funcional explícito.

Se ejecutó, con éxito:

```text
cd pymes/v2/backend && go test ./internal/fiscal/... ./internal/fiscalaccounting/...
ok .../internal/fiscal
ok .../internal/fiscal/ar
ok .../internal/fiscal/ar/artifacts
ok .../internal/fiscal/ar/authority
ok .../internal/fiscal/ar/wsaa
ok .../internal/fiscal/ar/wsfev1
```

### Open Accounting

Tiene un dominio contable amplio (plan, diarios, períodos, movimientos y
reportes), utiliza decimales y Postgres, y sigue un modelo schema-por-tenant.
Sin embargo, contiene producto Estonia-específico, UI y superficies públicas
grandes (incluidos entrypoints/handlers monolíticos). Su función de resolución
de schema tiene un fallback a `tenant_{id}` si no encuentra el tenant: **no es
aceptable** para v3; el provisionamiento debe ser explícito y fallar cerrado.

Se ejecutó, con éxito:

```text
cd open-accounting && go test ./internal/accounting/... ./internal/tenant/...
ok github.com/HMB-research/open-accounting/internal/accounting
ok github.com/HMB-research/open-accounting/internal/tenant
```

### ARCA SDK y referencias

`Arca.crearFactura(InvoiceRequest)` acepta un `InvoiceRequest` con número de
comprobante explícito y lo transmite al pedido WSFE. Los helpers
`crearFacturaAuto`, `facturar` y `facturarConAsociado` obtienen el siguiente
número: quedan prohibidos para v3. Esto resuelve la condición esencial de que
Pymes controle la reserva y la recuperación exacta del número.

El script `npm test` del SDK invoca `vitest run`, pero el checkout no tiene
`vitest` instalado (`sh: 1: vitest: not found`). No se instaló nada ni se cambió
el fork; antes de adoptarlo se debe ejecutar su suite en un entorno reproducible.
LedgerSMB y pyafipws se inspeccionaron como especificaciones de comportamiento:
registran pagos, reversas, períodos cerrados y conciliación, pero sus licencias
impiden copiar su código a este producto.

## Comparativa ponderada

Escala 1--5; peso entre paréntesis. La puntuación es para el rol indicado, no
una afirmación de calidad general.

| Opción / rol | Función (25) | Integridad (20) | Seguridad (15) | Adaptación (15) | Operación (10) | Test (10) | Licencia (5) | Total / 5 | Decisión |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| OA como núcleo contable | 5 | 4 | 3 | 2 | 3 | 4 | 5 | 3,85 | Adaptar y podar detrás de API nueva |
| OA sin cambios como API pública | 4 | 3 | 2 | 1 | 2 | 3 | 5 | 2,75 | Descartar |
| arca-facturacion encapsulado | 4 | 3 | 3 | 4 | 4 | 3 | 5 | 3,65 | Adaptar |
| pyafipws como emisor | 5 | 4 | 3 | 2 | 2 | 4 | 3 | 3,55 | No adoptar; usar como oráculo |
| LedgerSMB como código | 5 | 5 | 4 | 1 | 2 | 4 | 1 | 3,50 | Prohibido por licencia; referencia |
| Pymes v2 como runtime | 4 | 4 | 4 | 3 | 3 | 4 | 4 | 3,80 | Referencia selectiva, no base de v3 |

La evaluación no deja una decisión pendiente: se extrae/adapta el núcleo de OA
en el fork público mantenido por el equipo y se despliega únicamente su runtime
headless como servicio privado; no se expone ni se despliega OA completo.
