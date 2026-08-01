# Validación y spikes descartables

Los spikes viven fuera de cualquier repositorio, por ejemplo en
`/tmp/pymes-v3-spikes`, y se eliminan después de documentar el resultado. No
consumen certificados reales ni llaman producción ARCA.

| Spike | Montaje | Aceptación |
|---|---|---|
| ARCA falso | HTTP fake que devuelve CAE, rechazo y corte tras procesar | la misma solicitud/número es consultada y nunca reemitida; se preserva snapshot. |
| Posteo con respuesta perdida | accounting fake persiste y corta conexión | reintento con igual clave devuelve mismo `journal_entry_id`; no duplica líneas. |
| Dos organizaciones | dos schemas y tokens con org distinta | ninguna consulta o comando puede observar/mutar la otra organización. |
| Identidad interna | JWKS local + KMS falso con CRC32C, versión explícita y overlap; tokens expirados, audiencia/rol incorrecto | producción rechaza semillas y aliases; se valida firma al arrancar, se rechazan claims ausentes o dispares y se propagan request/correlation/actor sin cruce tenant. |
| Caída/recuperación | apagar fiscal y contable durante publicaciones | outbox sobrevive, lease vence sin doble trabajo, backoff y reconciliación convergen. |

## Tabla de aceptación de contratos

| Caso | Fiscal | Contable |
|---|---|---|
| A/B/C | tipo, receptor, IVA y número explícito válidos | venta separa neto/IVA/total |
| NC/ND | asocia original y conserva referencia | reversa/ajuste enlaza asiento fuente |
| IVA y redondeo | importe fijo en snapshot, sin float | débitos = créditos por moneda; línea de redondeo explícita |
| Moneda extranjera | cotización y fecha inmovilizadas | importe transaccional y funcional; diferencia FX explícita |
| Período bloqueado | puede autorizarse pero no postear fuera de política | devuelve `PERIOD_LOCKED` sin mutación |
| Pago parcial | no aplica | reduce partida por importe, deja saldo, idempotente |

Antes del MVP se ejecutarán las suites del fork OA y del SDK en CI reproducible,
más las pruebas de Pymes v3 que implementen estos casos. Se prohíbe reutilizar
código GPL de LedgerSMB; sólo se transcriben casos expresados como requisitos.

## Resultado ejecutado

Los cinco spikes iniciales pasaron y sus invariantes se trasladaron a suites
durables. `make db-integration` valida RLS, numeración concurrente,
provisionamiento, Clerk, PostgreSQL Fiscal y el boundary contable por schema.
`make fiscal-e2e` y `make accounting-e2e` prueban los clientes reales contra
los servicios privados. `make backup-restore-smoke` construye tres bases fuente
y tres destinos descartables, restaura datos tenant reales —incluido el schema
headless de Accounting—, reaplica dos veces las migraciones y demuestra que el
worker one-shot recupera una respuesta contable perdida sin duplicar el asiento.

`make observability-e2e` valida endpoints y el heartbeat JSON sin PII;
`make monitoring-config-check` genera métricas, políticas y dashboard de STG y
PRD sin llamar a GCP; `make replay-smoke` aplica todas las migraciones en una
base descartable, mueve una DLQ, repite el comando como no-op y demuestra que
su auditoría no admite mutación.

`make security` ejecuta `govulncheck` sobre Pymes y el runtime headless
contable, además de `npm audit` sobre Fiscal. El gate quedó en cero
vulnerabilidades alcanzables después de actualizar Go 1.26.5, `pgx`, `x/text`
y `go-jose`; la incorporación del cliente KMS actualizó además `grpc` a una
versión sin la vulnerabilidad alcanzable detectada por `govulncheck`.

La suite completa histórica de Open Accounting conserva fallos ajenos al
runtime headless en SmartAccounts/cutover y en un enlace documental a
`.mailmap`. Las suites relevantes del binario headless, accounting, tenant,
boundary arquitectónico y la integración PostgreSQL pasan; esos fallos
heredados no se incorporan al gate de Pymes.
