# Disposición de módulos

| Origen | Conservar/adaptar | Podar o no integrar | Uso como referencia |
|---|---|---|---|
| Open Accounting | núcleo accounting, modelos de diario/cuentas/períodos, decimal, repositorios y reportes, tras auditoría | UI Svelte, auth pública, tenants HTTP, invoicing comercial, Estonia tax/KMD, payroll, banking, inventory, assets, plugins, webhooks, cutover y entrypoints monolíticos | límites y tests de aislamiento |
| arca-facturacion | transporte WSAA/WSFE y operaciones ARCA; `crearFactura(InvoiceRequest)` explícito | `crearFacturaAuto`, `facturar`, `facturarConAsociado` y cualquier autonumeración | implementación de WS y tipado |
| Pymes v2 | semántica de snapshot/lease/outbox, consulta incierta, KMS y plan fiscal-contable | copiar runtime/API/modelos de v2 | A/B/C, NC/ND, redondeos, workers y tests |
| pyafipws | ninguno en producción | librería como dependencia/segundo emisor | fixtures, catálogos WSAA/WSFE/padrón/FCE/WSFEX/CAEA |
| LedgerSMB | ninguno en producción | cualquier copia, por GPLv2 | casos de diario, pagos parciales, reversas y cierre |

La extracción de OA será un fork mínimo mantenido por el equipo. Si la poda no
permite aislar en dos sprints el núcleo sin imports de módulos comerciales/UI,
se conserva sólo el modelo y se implementa el servicio contable nuevo; no se
arrastra OA completo por conveniencia.
