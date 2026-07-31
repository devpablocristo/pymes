# Plan de ejecución de Pymes v3

## Objetivo

Construir el primer corte vertical de Pymes v3:

> Documento comercial → outbox → contabilización idempotente → consulta del
> estado desde el BFF. La autorización ARCA se incorpora en una etapa posterior.

El desarrollo funcional, la orquestación, el BFF y los workers viven en
`pymes/v3`. Open Accounting y `arca-facturacion` permanecen detrás de contratos
privados y no reciben lógica comercial de Pymes.

## Alcance del MVP

Incluido:

- organizaciones, membresías y autorización con Clerk;
- parties mínimos para clientes y proveedores;
- documentos comerciales, compras, pagos y aplicaciones parciales;
- asientos, reversas, períodos, partidas abiertas y aplicaciones parciales;
- outbox/inbox, idempotencia, auditoría y reconciliación operativa.

Fuera de la etapa actual: toda integración ARCA (WSAA, WSFEv1, certificados,
KMS/secret manager fiscal, FCE, WSFEX, CAEA y padrón), inventario, payroll,
conciliación bancaria automática, migración masiva desde v2 y una UI contable
completa. ARCA se abrirá como iniciativa específica después de estabilizar el
corte comercial-contable.

## Reglas de ejecución

- Cada PR debe ser desplegable, reversible y tener pruebas propias.
- Ningún navegador llama directamente a Fiscal o Accounting.
- Ningún servicio comparte tablas o credenciales de base con otro.
- Todo importe viaja como string decimal; no se usa punto flotante.
- Las integraciones se desarrollan primero contra fakes deterministas.
- No se consume código GPL de LedgerSMB ni se copia código de pyafipws.
- v1 y v2 permanecen en sólo lectura durante la construcción de v3.
- El contrato OpenAPI se cambia antes que productores y consumidores.

## Estructura objetivo

```text
pymes/v3/
├── backend/
│   ├── cmd/                 # api, worker y provision-org
│   ├── wire/                # composición de dependencias
│   └── internal/<recurso>/  # domain, handler, usecases, repository, companion/access
├── db/                      # migraciones y pruebas PostgreSQL
├── contracts/               # OpenAPI y eventos versionados
├── fiscal-adapter/          # diferido; conserva sólo contrato/fake local
├── scripts/                 # CI, generación y E2E
└── docs/
```

El servicio contable se despliega desde el fork privado de Open Accounting. En
`pymes/v3` viven su cliente, contrato, tests de compatibilidad y toda la
orquestación. Los únicos cambios permitidos en el fork son extracción/poda,
tenancy fail-closed y la implementación de la API contable privada.

## Secuencia de entregas

### Hito 0 — Fundación ejecutable

#### PR 0.1 — Esqueleto y calidad

Crear los módulos, Docker Compose local, configuración tipada, health checks y
pipeline de formato, test, análisis estático y build.

Aceptación:

- `make test`, `make build` y `make up` funcionan desde `pymes/v3`;
- API, worker, fiscal fake y PostgreSQL levantan localmente;
- no hay secretos o `.env` versionados;
- cada servicio expone readiness y liveness independientes.

#### PR 0.2 — Organización e identidad interna

Validar sesión Clerk en el BFF y emitir una credencial interna corta para los
workers. Resolver una única organización activa por request y propagar
`organization_id`, `actor_id`, roles, request ID y correlation ID.

Aceptación:

- audiencia, firma, expiración, rol y organización se validan fail-closed;
- un token de una organización no accede a otra;
- Fiscal y Accounting sólo aceptan identidades de servicio;
- logs de rechazo no incluyen token ni PII.

#### PR 0.3 — Outbox, inbox e idempotencia

Agregar outbox transaccional a Pymes, leases recuperables en el worker e inbox
para los consumidores. Guardar clave, hash de payload y resultado durable.

Aceptación:

- caída después del commit no pierde el evento;
- dos workers no ejecutan la misma lease simultáneamente;
- repetir una clave y el mismo payload devuelve el resultado original;
- repetir la clave con otro payload devuelve `IDEMPOTENCY_KEY_REUSED`;
- métricas muestran pendientes, edad, reintentos y DLQ.

#### PR 0.4 — Provisionamiento y observabilidad

Crear el registro de servicios por organización y el flujo explícito para
provisionar Accounting. Incorporar trazas, auditoría redactada y alertas base.

Aceptación:

- una organización no provisionada recibe `ORG_NOT_PROVISIONED`;
- nunca se deriva un schema como fallback;
- el provisionamiento es repetible e idempotente;
- una operación se puede seguir por correlation ID entre los tres servicios.

**Puerta H0:** pasan los spikes de identidad, dos organizaciones, caída y replay
sin servicios reales ni credenciales ARCA.

### Hito 1 — Servicio contable privado

#### PR 1.1 — Corte headless de Open Accounting

Crear una rama de integración del fork, aislar los paquetes de plan de cuentas,
diario, períodos y reportes, y eliminar de su grafo de ejecución UI, auth
pública, invoicing, payroll, módulos Estonia y handlers monolíticos.

Aceptación:

- el binario contable no importa módulos podados;
- sólo expone rutas `/internal/v1` y health;
- la suite contable original relevante sigue pasando;
- licencia y atribuciones MIT quedan preservadas.

Si este aislamiento no puede demostrarse en dos iteraciones de PR, se abandona
el runtime de OA y se implementa un servicio nuevo usando únicamente su modelo
y sus comportamientos como referencia. Esta es una puerta de decisión objetiva,
no una extensión indefinida de la poda.

#### PR 1.2 — Tenancy y plan de cuentas

Implementar schemas provisionados, migraciones versionadas, cuentas activas y
resolución obligatoria de organización.

Aceptación:

- pruebas con dos organizaciones demuestran aislamiento de escritura y lectura;
- migrar una organización no altera otra;
- schema ausente falla cerrado;
- backups y restore conservan el mapa organización/schema.

#### PR 1.3 — PostingCommand e inmutabilidad

Implementar el contrato `PostingCommand`, validación decimal, balance por moneda,
inbox idempotente y `AccountingEvent`.

Aceptación:

- débitos y créditos deben balancear exactamente;
- un posteo confirmado no se edita ni elimina;
- respuesta perdida y reintento devuelven el mismo `journal_entry_id`;
- una fuente/version no se contabiliza dos veces;
- redondeo y diferencia de cambio son líneas explícitas.

#### PR 1.4 — Períodos, reversas y partidas abiertas

Agregar bloqueo de períodos, reversa enlazada, cuentas por cobrar/pagar y
aplicaciones parciales.

Aceptación:

- período bloqueado rechaza sin mutación;
- reversa conserva vínculo y no modifica el asiento original;
- un pago parcial deja el saldo exacto;
- repetir la aplicación no reduce dos veces el saldo;
- trial balance, mayor y aging reconcilian con los asientos.

**Puerta H1:** todos los casos contables pasan para dos organizaciones, moneda
funcional y extranjera, período bloqueado, reversa y pago parcial.

### Hito 2 — Servicio fiscal ARCA

**Estado actual:** sólo el transporte mock está activo. Las credenciales, WSAA,
WSFEv1 y el SDK real quedan diferidos hasta iniciar explícitamente la integración
ARCA. El mock usa los mismos puertos y contrato que deberá implementar el
companion real.

#### PR 2.1 — Adaptador y transporte falso

Crear `fiscal-adapter` detrás del OpenAPI actual y un puerto `FiscalAuthority`.
El companion mock reproduce CAE, rechazo, timeout antes de procesar y respuesta
perdida después de procesar; no instala ni importa `arca-facturacion`.

Aceptación:

- ningún SDK ARCA aparece en el runtime activo, handlers o dominio;
- el request contiene número explícito y snapshot digest;
- helpers de autonumeración están ausentes y bloqueados por tests;
- decisiones y fallos del mock se normalizan a códigos estables.

#### PR 2.2 — Credenciales y WSAA

**Diferida.**

Resolver `credential_ref` mediante KMS/secret manager, cachear tickets por
organización/ambiente/servicio y alertar expiraciones.

Aceptación:

- clave, certificado, ticket y XML nunca llegan a logs o DB de Pymes;
- producción y homologación no comparten credenciales ni tickets;
- tickets vencidos se renuevan con exclusión concurrente;
- certificado inválido devuelve `CERTIFICATE_UNAVAILABLE` sin reintento ciego.

#### PR 2.3 — Reserva y autorización WSFEv1

**Diferida en su conexión real; la numeración y snapshots se prueban con mock.**

Pymes reserva en una transacción `(organización, ambiente, punto de venta,
tipo, número)`, congela el documento y publica `FiscalAuthorizationRequested`.

Aceptación:

- concurrencia no reserva dos veces el mismo número;
- A/B/C y NC/ND envían importes exactos y asociación correcta;
- rechazo ARCA es definitivo y auditable;
- un documento congelado no cambia durante la autorización.

#### PR 2.4 — Incertidumbre y reconciliación

**Implementada contra mock; reconciliación ARCA real diferida.**

Implementar `uncertain`, consulta exacta y reconciliador periódico. No se emite
otro comprobante hasta obtener un resultado definitivo.

Aceptación:

- respuesta perdida termina en CAE recuperado por consulta;
- `not_found` sólo es definitivo según la política documentada de ARCA;
- ningún reintento incrementa o cambia el número;
- divergencias generan `ReconciliationRequired` y alerta operativa.

**Puerta H2 mock:** las pruebas A/B/C, NC/ND, idempotencia, aislamiento, moneda
extranjera, timeout y respuesta perdida pasan sin SDK real. La suite del SDK se
incorporará a CI cuando comience la etapa ARCA real.

### Hito 3 — Corte vertical del producto

#### PR 3.1 — Parties y documentos

Implementar customers/suppliers, factura y nota, sus snapshots y transiciones.
El BFF nunca permite saltar estados ni alterar un documento fiscal congelado.

#### PR 3.2 — Orquestación CAE → contabilidad

Al autorizarse el comprobante, guardar el resultado y publicar la intención
contable en la misma transacción. El worker produce `PostingCommand` y persiste
el resultado del servicio contable.

#### PR 3.3 — Cobros, pagos y aplicaciones

Confirmar cobros/pagos, contabilizarlos y aplicar partidas de forma idempotente;
incorporar reversa comercial y contable coordinada por saga.

Aceptación conjunta H3:

- una factura A/B/C completa el flujo CAE → asiento;
- NC/ND referencia comprobante y asiento original;
- una caída de Fiscal o Accounting converge al restaurar el servicio;
- no se contabiliza antes del CAE en documentos que lo requieren;
- pago parcial y reversa conservan saldos exactos;
- todas las vistas y comandos respetan organización y roles.

### Hito 4 — Producción controlada

#### PR 4.1 — Operación y reconciliación

Dashboards, alertas, runbooks, replays administrados, consultas de divergencias,
expiración de certificados y métricas SLO.

#### PR 4.2 — Migraciones, backup y disaster recovery

Ensayar migraciones por schema, rollback compatible, backup cifrado y restore
en un entorno vacío.

#### PR 4.3 — Piloto de homologación

Ejecutar un piloto con organizaciones de prueba, credenciales de homologación y
doble revisión de resultados fiscales/contables.

**Puerta H4:** restore probado, cero divergencias sin explicación, runbooks
ejecutados y corte vertical estable antes de habilitar producción.

## Matriz de responsabilidad técnica

| Área | Responsable primario | Revisión obligatoria |
|---|---|---|
| Dominio comercial, BFF e IAM | equipo Pymes v3 | seguridad y producto |
| Workers, outbox y sagas | plataforma Pymes v3 | Accounting/Fiscal |
| Fork Open Accounting | responsable contable | equipo Pymes v3 y licencias |
| Adaptador ARCA | responsable fiscal | seguridad y equipo Pymes v3 |
| Contratos internos | equipo Pymes v3 | productor y consumidor |
| Infra, KMS, backups y SLO | plataforma | seguridad y owners de servicio |

Son roles, no nombres: antes de iniciar cada hito se asigna una persona concreta
por rol y un reemplazo para revisiones críticas.

## Riesgos que bloquean una salida

| Riesgo | Control | Condición de bloqueo |
|---|---|---|
| Acoplamiento excesivo de OA | puerta de dos iteraciones y grafo de imports | módulos podados siguen en runtime |
| Doble emisión ARCA | reserva única + `uncertain → consult` | existe un camino de reemisión ciega |
| Cruce de organizaciones | org claim + path + schema + pruebas negativas | cualquier lectura/escritura cruza org |
| Duplicación contable | inbox y hash de payload | respuesta perdida crea dos asientos |
| Filtración de secretos/PII | KMS, redacción y tests de logs | material sensible llega a log o DB Pymes |
| Contratos divergentes | OpenAPI en CI y consumer tests | productor/consumidor no validan misma versión |
| Licencias | inventario SBOM y revisión | aparece código GPL/LGPL copiado en el producto |

## Orden inmediato

El backend del MVP y el servicio contable headless están implementados. El
único hito funcional deliberadamente abierto es ARCA real: no debe iniciarse
hasta disponer de credenciales de homologación, estrategia KMS/secret manager,
paquete publicado de `arca-facturacion` y una organización piloto. El estado
comando por comando queda en `07-estado-implementacion.md`.
