# Disposición de módulos

| Origen | Conservar/adaptar | Podar o no integrar | Uso como referencia |
|---|---|---|---|
| Open Accounting | núcleo accounting, modelos de diario/cuentas/períodos, decimal, repositorios y reportes, tras auditoría | UI Svelte, auth pública, tenants HTTP, invoicing comercial, Estonia tax/KMD, payroll, banking, inventory, assets, plugins, webhooks, cutover y entrypoints monolíticos | límites y tests de aislamiento |
| arca-facturacion | transporte WSAA/WSFE y operaciones ARCA; `crearFactura(InvoiceRequest)` explícito | `crearFacturaAuto`, `facturar`, `facturarConAsociado` y cualquier autonumeración | implementación de WS y tipado |
| Pymes v2 | semántica de snapshot/lease/outbox, consulta incierta, KMS y plan fiscal-contable | copiar runtime/API/modelos de v2 | A/B/C, NC/ND, redondeos, workers y tests |
| pyafipws | ninguno en producción | librería como dependencia/segundo emisor | fixtures, catálogos WSAA/WSFE/padrón/FCE/WSFEX/CAEA |
| LedgerSMB | ninguno en producción | cualquier copia, por GPLv2 | casos de diario, pagos parciales, reversas y cierre |

La adaptación de OA quedó fusionada en el fork mantenido por el equipo. El
commit canónico `6647992c75bee76bb70a6baafdb6b0d94fc0acab` conserva el runtime
headless aislado y sus tres targets privados; Pymes fija ese commit. En OA
están verdes CI remota, unitarias/race/vet/lint, build, integración PostgreSQL
y Docker; Pymes pasó `make accounting-test`, `make accounting-e2e` y `make ci`.
El required check remoto volvió a pasar para el pin canónico y el baseline de
Pymes quedó integrado mediante su PR #52.
El repositorio sigue siendo público, pero el servicio contable se despliega
privado y no expone UI ni superficies comerciales de OA.
