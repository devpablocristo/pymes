# Ledger de extracción v1 → v2

V1 es una referencia histórica, nunca una dependencia de runtime. Cada
capacidad se reconstruye a partir de comportamiento observable, reglas y tests;
no se copian composition roots, schemas, wrappers ni clientes monolíticos.

## Criterio de extracción

Cada PR funcional debe registrar:

1. las rutas y tests de v1 usados como evidencia;
2. los comportamientos que deben sobrevivir;
3. la implementación legacy que se descarta;
4. el contrato y los tests de aceptación de v2;
5. el release publicado de `platform`, cuando corresponda;
6. el PR que reemplaza la capacidad.

Las capacidades agnósticas se implementan y publican primero en `platform`.
Marca, copy, rutas y reglas comerciales permanecen en v2.

## Base horizontal

| Capacidad | Evidencia v1 | Decisión | Destino / PR | Aceptación |
| --- | --- | --- | --- | --- |
| Runtime local | `v1/docker-compose.yml`, `v1/Makefile` | Reconstruir el grafo PostgreSQL → migrador → backend → web; descartar Mailhog, verticales, Axis y seeds | v2 / `V2-OPS-02` | `make up` requiere sólo Docker y deja todos los servicios saludables |
| Configuración y proceso API | `v1/core/backend/wire/wire.go`, health endpoints del core | Conservar fail-fast, logs estructurados, shutdown y health/readiness; descartar el bootstrap monolítico | v2 backend | `/healthz` no consulta DB; `/readyz` falla y se recupera con PostgreSQL |
| Migraciones | runner y tests de `v1/core/backend/migrations` | Conservar orden y repetibilidad como conducta; descartar SQL, GORM y migración implícita durante el arranque | v2 db + `platform/databases/postgres` | job separado, base vacía, segunda ejecución sin cambios |
| Shell de producto | `v1/ui/src/components/Shell.tsx`, `v1/ui/src/shared/frontendShell.tsx` | Reconstruir marca, navegación activa, búsqueda, colapso, responsive y skip-link; descartar catálogo dinámico y rutas verticales | `platform` / `PLAT-UI-SHELL-02`, v2 / `V2-WEB-BASE-01` | un `<main>`, teclado completo, drawer móvil y sólo enlaces existentes |
| Tema e idioma | `v1/ui/src/lib/theme.ts`, `v1/ui/src/lib/i18n/index.tsx` y sus tests | Consumir las primitives publicadas; mensajes y paleta quedan en v2 | `platform-browser`, v2 web | light/dark y ES/EN persisten y actualizan el documento |
| Estados transversales | Error boundaries, spinners y empty states de `v1/ui/src` | Reconstruir loading, skeleton, vacío, error recuperable, fatal y 404 | v2 web | estados localizados, accesibles y testeados |
| Transporte HTTP | `v1/ui/src/lib/api.ts` | Conservar auth, errores y cancelación como criterios; descartar singleton y API client monolítico | `platform` / transporte por instancia, adopción v2 posterior | base URL, token, headers y `fetch` inyectables |
| Identidad | `v1/ui/src/shared/frontendAuth.tsx`, handlers y tests SaaS | Reconstruir Clerk, sesión y organización activa sin onboarding ni tenant en URL | `platform` SaaS + v2 IAM | token verificado, organización provisionada e aislamiento cross-org |

### Activos de marca

Los siguientes SVG son activos de producto y se promoverán a v2 sin modificar
el archivo histórico:

| Activo v1 | SHA-256 |
| --- | --- |
| `v1/ui/src/assets/favicon.svg` | `a164d4d62bde5662e535091c12f6318bd10ed288caea2b74ec2907a8add87b46` |
| `v1/ui/src/assets/iso.svg` | `e901358cb117d36071930650eef0d1302d961617b0533ccccbc2ff21b85fadf6` |
| `v1/ui/src/assets/logo.svg` | `78fd1d32d6b764746678ebbed95ff6f1e1394474e3854db98880518a31a960a0` |
| `v1/ui/src/assets/logo-dark.svg` | `905e66c357e2b298d61ac0e45f8fd685b57c58d6b4395df861fba9753fcc96c4` |

## Núcleo horizontal de negocio

| Orden | Capacidad y evidencia v1 | Decisión | Destino | Condición de aceptación |
| --- | --- | --- | --- | --- |
| 1 | Customers y suppliers: `internal/customers`, `internal/suppliers` | Unificar como `Party` persona/organización con roles; descartar módulos duplicados y `automated_agent` | v2 Party | org-scoped, roles combinables, normalización de email/tax ID/teléfono y archivado inmutable |
| 2 | Productos, servicios y precios: `internal/products`, `internal/services`, `internal/pricelists` | Reconstruir catálogo y listas; diferir scheduling, data URLs y metadata libre | v2 Catalog + Pricelist | Money exacto, product XOR service, snapshots y una lista default por organización |
| 3 | Stock: `internal/inventory` | Reconstruir niveles y movimientos inmutables; descartar actualizaciones separadas de la operación comercial | v2 Inventory | nivel y movimiento atómicos; sólo `track_stock` mueve existencias |
| 4 | Numeración usada por documentos comerciales | Reconstruir contador atómico por organización y tipo documental | v2 Sequence | concurrencia sin duplicados ni saltos por rollback |
| 5 | Presupuestos: `internal/quotes`, `fsm_test.go` | Conservar líneas, snapshots, totales y FSM; reemplazar conversión no atómica | v2 Quote | draft editable; conversión a venta transaccional e idempotente |
| 6 | Ventas y pagos: `internal/sales`, `internal/payments` | Conservar reglas comerciales; reemplazar stock/cashflow best-effort | v2 Sale + Payment | venta, stock, pago y outbox en una UoW; sin sobrepago |
| 7 | Compras: `internal/purchases` | Reconstruir proveedor, líneas, recepción y pago; descartar FSM totalmente libre | v2 Purchase | recepción, stock, ledger y pago atómicos |
| 8 | Devoluciones y créditos: `internal/returns` | Reconstruir límites, reposición, refund y reversas | v2 Return | nunca supera lo vendido; crédito sólo para la misma Party |
| 9 | Ledger: `internal/ledger`, `posting_test.go` | Generalizar motor exacto; cuentas y cashflow pasan a proyecciones | `platform` ledger + reglas v2 | débitos = créditos, asientos inmutables y reversas |
| 10 | Fiscal: `internal/fiscal`, `internal/fiscal/arca`, tests de emisión | Generalizar SDK y reconstruir reglas; no reutilizar mutex local | `platform` ARCA + reglas v2 | homologación vigente, unicidad DB, CAE/QR trazables y retry sin duplicados |

Invariantes comunes:

- toda persistencia tenant usa `org_id`, contexto transaccional y RLS;
- archivado implica que la entidad ya no es mutable;
- cantidades son positivas, precios no negativos y totales se calculan en servidor;
- las líneas conservan snapshots de descripción, precio, moneda e impuestos;
- documentos financieros se corrigen con reversas, nunca update/delete;
- comandos repetidos o concurrentes producen un único resultado;
- Money usa Decimal exacto y se serializa como string.

## Diferido o conservado

| Capacidad v1 | Decisión | Motivo |
| --- | --- | --- |
| Workshops, professionals, restaurants, beauty y medical | Conservar en v1 | Fuera del núcleo horizontal |
| Expo mobile | Conservar en v1 | Reconsiderar después de estabilizar las APIs |
| Assistant, agentes y WhatsApp | Diferir | Requiere RFC y evidencia de producto |
| Scheduling, calendario y branches | Diferir | Se evaluará después del flujo comercial base |
| Procurement/Nexus y aprobaciones | Diferir | Integración externa y FSM requieren diseño independiente |
| Gateways de pago | Diferir | Primero se estabiliza el dominio Payment |
| PDF, dashboard y reportes históricos | Diferir | Deben consumir fuentes de verdad nuevas |
| Billing SaaS y planes | Descartar de la base | No pertenece al primer núcleo de Pymes v2 |
| Onboarding | Descartar | Organizaciones creadas por administración o invitación |

## Implementaciones prohibidas

- imports o ejecución de código bajo `v1/`;
- migraciones o fallback de datos legacy;
- importes o cantidades monetarias con `float64`;
- tenant derivado de URL, `X-Pymes-Tenant-Slug` o `X-Org-ID`;
- API key local automática o bypass de autenticación;
- `replace`, `file:`, `link:`, `workspace:` o rutas absolutas;
- efectos de stock, pago, ledger u outbox fuera de la transacción principal;
- mutex de proceso como garantía de unicidad distribuida.
