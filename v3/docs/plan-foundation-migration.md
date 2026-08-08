# Plan definitivo: Foundation como núcleo único del ecosistema

Fecha de decisión: 2026-08-08

## 1. Objetivo

Consolidar en `foundation` toda capacidad realmente compartida por los productos
del ecosistema y retirar por completo el repositorio histórico `platform` de las
generaciones activas.

Pymes v3 continuará siendo el producto SaaS, BFF, orquestador y fuente de verdad
de su dominio. Foundation aportará bibliotecas privadas, componentes frontend y
servicios reutilizables que cada producto instanciará de manera independiente.

La migración continúa desde el baseline actual. No se rehace el trabajo ya
validado de Pymes v3, Open Accounting, ARCA ni PerGo: se extrae, estabiliza,
publica y adopta en el nuevo destino.

## 2. Decisiones cerradas

- `foundation` será la única fuente activa de librerías Go, paquetes frontend,
  SDKs técnicos y servicios reutilizables.
- El repositorio `github.com/devpablocristo/platform` queda deprecado y no
  recibirá funcionalidad nueva.
- El namespace interno `github.com/devpablocristo/foundation/platform/...` se
  conserva: abandonar el repositorio Platform no significa eliminar el
  directorio `platform/` de Foundation.
- Pymes, Foundation, Axis, MedMory y Ponti seguirán siendo proyectos
  independientes. No habrá imports, llamadas, bases, despliegues ni dependencias
  cruzadas entre productos salvo el consumo explícito de artefactos publicados
  por Foundation.
- Axis y MedMory se usan sólo como referencia de cómo consumir acciones y
  dependencias privadas. No se copia su dominio ni se crea contacto técnico con
  Pymes.
- Las generaciones congeladas permanecen inmutables: Pymes v1/v2, Axis v1/v2 y
  MedMory v1 pueden conservar referencias históricas a Platform mientras sigan
  en modo lectura.
- El gate “cero Platform” se aplica a las generaciones activas, no a árboles
  históricos congelados.
- No se introducirá un broker en el MVP. La coordinación seguirá usando HTTP
  interno, outbox, inbox, leases, idempotencia y reconciliación.
- Todos los repositorios propios terminarán privados. Los artefactos compartidos
  se distribuirán desde Foundation mediante canales privados y versiones
  inmutables.
- Los servicios Foundation ya existentes (`identity`, `parties` y
  `notifications`) no cuentan como servicios nuevos de esta migración.
- Pymes no trasladará su dominio `parties` a Foundation: continuará siendo
  dueño de sus clientes, proveedores y referencias comerciales.
- `identity` se limitará a primitivas técnicas compartidas; Clerk y las
  membresías locales conservarán el ownership definido para Pymes.

## 3. Modelo objetivo de Foundation

Foundation se organiza en tres categorías claramente separadas.

### 3.1 Librerías Go privadas

Se publicarán como módulos versionados desde el repositorio privado Foundation:

- autenticación y validación de Clerk;
- PostgreSQL, contexto tenant y utilidades transaccionales;
- errores estables y transporte HTTP;
- observabilidad;
- validación;
- seguridad e identidad interna;
- clientes técnicos de Google Calendar;
- núcleo puro de algoritmos de agenda;
- SDKs o clientes de los servicios Foundation.

Las librerías no serán dueñas de datos de producto, no conocerán rutas de Pymes
y no incluirán modelos comerciales de ningún consumidor.

### 3.2 Paquetes frontend privados

Se publicarán en GitHub Packages bajo Foundation:

- componentes del design system;
- Calendar Board genérico;
- helpers de infraestructura web que sean realmente compartidos;
- componentes neutrales ya utilizados por más de un producto.

Los paquetes no contendrán branding, copy, permisos, rutas ni reglas propias de
Pymes. Los formatters triviales y wrappers de producto permanecerán locales.

### 3.3 Servicios reutilizables

Foundation tendrá servicios completos, desplegables e instanciables:

| Servicio Foundation | Origen | Responsabilidad |
|---|---|---|
| `identity` | existente | identidad técnica compartida y contratos de integración |
| `parties` | existente | capacidades neutrales de actores cuando un producto decida consumirlas |
| `accounting` | Open Accounting headless | cuentas, diarios, períodos, posteos, reversas, partidas y reportes |
| `fiscal-arca` | Fiscal Adapter + `arca-facturacion` | credenciales tenant, WSAA, WSFE, autorización y consulta exacta |
| `communications` | núcleo reutilizable de PerGo | proveedores, credenciales, entrega y estados de mensajería |
| `notifications` | existente | email e in-app; no absorbe WhatsApp en el primer corte |

Cada producto desplegará su propia instancia por entorno. Compartir el código no
implica compartir bases, secretos, identidades, colas ni datos.

Los únicos servicios nuevos creados por este plan son `accounting`,
`fiscal-arca` y `communications`. `identity`, `parties` y `notifications` sólo
podrán ampliarse mediante cambios separados y compatibles; su existencia no
autoriza a moverles dominio de Pymes.

## 4. Ownership final

| Información o capacidad | Fuente de verdad |
|---|---|
| Usuarios y organizaciones externas | Clerk |
| Membresías, permisos y estado del tenant | Pymes |
| Parties, documentos, pagos y numeración | Pymes |
| Sucursales, servicios, recursos y turnos | Pymes Scheduling |
| OAuth y mappings de calendarios | adapters de Pymes Scheduling |
| Eventos de Google y Meet | proyecciones externas de Pymes Scheduling |
| Intenciones y reglas comerciales de notificación | Pymes Notifications |
| Credenciales y entrega de WhatsApp | instancia Foundation Communications |
| Credenciales, tickets y autorización fiscal | instancia Foundation Fiscal ARCA / ARCA |
| Asientos, períodos y partidas | instancia Foundation Accounting |
| Algoritmos genéricos de agenda | librería Foundation Scheduling, sin persistencia |
| Componentes visuales genéricos | Foundation Design System |
| Axis y MedMory | ninguna responsabilidad dentro de Pymes |

## 5. Arquitectura de consumo en Pymes

Pymes conservará su monolito modular Go y la estructura vertical consumer-owned
ya definida. Foundation no reemplaza los bounded contexts ni invade el dominio.

Reglas obligatorias:

- las interfaces pertenecen al consumidor;
- cada integración vive detrás de un adapter local;
- cada adapter mantiene archivo raíz, `models/` y `helpers/`;
- los tipos Foundation se traducen en el límite del adapter;
- ningún tipo externo entra a `usecases/domain`;
- la construcción concreta ocurre exclusivamente en `wire`;
- `cmd` sólo carga configuración y controla el ciclo de vida;
- el navegador sólo llama al BFF de Pymes;
- Pymes no accede a bases de servicios Foundation;
- Foundation no importa Pymes ni conoce sus rutas públicas.

Adapters finales principales:

```text
commerce/accounting.go
commerce/accounting/models/
commerce/accounting/helpers/

commerce/fiscal.go
commerce/fiscal/models/
commerce/fiscal/helpers/

scheduling/foundation_scheduling.go
scheduling/foundation_scheduling/models/
scheduling/foundation_scheduling/helpers/

scheduling/google_calendar.go
scheduling/google_calendar/models/
scheduling/google_calendar/helpers/

notifications/communications.go
notifications/communications/models/
notifications/communications/helpers/
```

## 6. Interfaces y contratos comunes

Foundation publicará contratos versionados y clientes generados, pero no una
biblioteca de dominio común.

Todo comando interno sensible incluirá:

- `organization_id`;
- `command_id` o `request_id`;
- `idempotency_key`;
- `correlation_id`;
- `source_id` y `source_version`;
- `snapshot_digest`;
- actor y request ID de auditoría.

La identidad workload-to-workload será JWT Ed25519 de hasta cinco minutos con
`iss`, `aud`, `org_id`, scopes, actor, `jti` y correlación. Cada servicio
validará audiencia y tenant; en producción se combinará con identidades de
workload y secretos separados por entorno.

Los servicios publicarán:

- imagen privada por digest;
- OpenAPI versionado;
- cliente/SDK privado cuando corresponda;
- manifiesto con commit, digest, hash del contrato y versión de migración;
- SBOM y provenance verificables.

Pymes fijará versiones y digests, nunca ramas flotantes ni checkouts de código
de otro repositorio.

## 7. Agenda: decisión de dominio

Agenda seguirá dentro de Pymes porque sus datos, permisos, workflows y estados
son parte del producto.

Foundation recibirá sólo el núcleo puro y sin estado de:

- cálculo de slots;
- buffers previos y posteriores;
- intersección de disponibilidad;
- bloqueos y excepciones;
- capacidad de sesiones;
- elegibilidad de recursos;
- operaciones temporales y zonas horarias.

No se creará un servicio Foundation Scheduling en esta etapa. Pymes conservará
PostgreSQL, RLS, holds, reservas concurrentes, recurrencia materializada,
waitlist, cola, auditoría y API pública. El frontend consumirá el Calendar Board
privado de Foundation mediante un wrapper local.

## 8. Accounting

El runtime headless validado de Open Accounting se moverá a
`foundation/services/accounting` conservando comportamiento y contratos.

Se trasladan:

- provisionamiento tenant explícito;
- plan de cuentas y diarios;
- períodos;
- posteos balanceados e inmutables;
- reversas;
- partidas abiertas y aplicaciones;
- reportes;
- decimal exacto;
- idempotencia y recibos canónicos;
- aislamiento por organización;
- migraciones y readiness.

No se trasladan UI, autenticación pública, facturación comercial, payroll,
banking, inventario ni módulos regionales ajenos al núcleo contable.

La extracción debe preservar las invariantes ya corregidas: identidad de cuenta
y party en partidas, reversas con aplicaciones, aging histórico, comandos
semánticos, exactitud `numeric(28,8)` y migración fail-closed.

Tras la adopción por Pymes, Open Accounting quedará privado, en modo histórico y
archivado. No se eliminará hasta probar equivalencia contractual, migraciones,
backup/restore y corte por digest.

## 9. Fiscal ARCA

`foundation/services/fiscal-arca` absorberá el servicio Fiscal Adapter y las
partes necesarias del SDK `arca-facturacion`.

El servicio será multi-tenant y tendrá:

- generación de clave privada y CSR por organización;
- carga y validación de certificado;
- vault fiscal cifrado;
- tickets WSAA;
- WSFEv1;
- autorización con numeración explícita;
- consulta exacta ante resultado incierto;
- A/B/C y NC/ND del MVP;
- errores estables y reconciliación.

Pymes seguirá reservando punto de venta, tipo y número. Fiscal nunca
autonumerará. No existirá CUIT ni certificado global de Pymes; cada cliente
configurará sus credenciales y ambientes.

La lógica de bajo nivel de `arca-facturacion` podrá quedar como paquete interno
del servicio si no tiene consumidores independientes reales. No se publicará
una biblioteca por inercia. Después del corte, el repositorio ARCA quedará
privado y archivado.

Padrón, FCE, WSFEX y CAEA siguen fuera del primer cierre.

## 10. Communications y PerGo

El núcleo reusable de PerGo se extraerá a
`foundation/services/communications`.

Foundation Communications será dueño de:

- credenciales por tenant;
- selección del proveedor;
- transporte;
- external message ID;
- estados de entrega;
- firma y validación de webhooks;
- deduplicación técnica.

Pymes será dueño de la intención comercial, destinatario, template, versión,
variables, momento de envío y estado de negocio. La caída de Communications no
bloqueará turnos ni transacciones comerciales.

El MVP del servicio será PostgreSQL-only. NATS quedará opcional y fuera del
camino obligatorio. La dependencia `whatsmeow` y sus obligaciones MPL-2.0 se
documentarán y revisarán antes de redistribuir binarios.

PerGo es actualmente un fork público y no puede convertirse directamente en
privado conservando su red de forks. Primero se solicitará el detach en GitHub;
si no es viable, se creará un mirror privado verificado, se preservarán historia
y tags, y se retirará el fork público después del corte.

## 11. Google Calendar y Meet como adapters de Scheduling

Google Calendar y Google Meet no serán un bounded context ni un servicio propio.
Son proveedores externos del contexto `scheduling`: Calendar proyecta un turno
en un calendario externo y Meet agrega una conferencia a esa misma proyección.

Los puertos `CalendarProvider` y `MeetingProvider` pertenecerán a
`scheduling/usecases.go`. El adapter `scheduling/google_calendar.go` implementará
ambos usando el SDK técnico de bajo nivel publicado por Foundation. Sus payloads
y traducciones permanecerán en `scheduling/google_calendar/models` y
`scheduling/google_calendar/helpers`.

El contexto transversal `calendars` actual se absorberá en `scheduling` durante
la migración. OAuth, conexiones tenant, mappings, outbox y reconciliación
seguirán en Pymes como infraestructura del adapter, sin convertirse en dominio.

El MVP mantiene:

- sincronización Pymes → Google;
- calendario secundario “Pymes”;
- FreeBusy opcional;
- Meet opcional;
- IDs determinísticos;
- `requestId` independiente para Meet;
- ETags e `If-Match`;
- reconciliación de `409`, `412` y timeouts;
- tokens cifrados con KMS por entorno y AAD tenant-aware.

Google caído no bloqueará reservas: Scheduling confirmará el turno localmente y
sincronizará mediante outbox. Sincronización bidireccional, Outlook y Teams
permanecen fuera.

## 12. Distribución privada y workflows

Los workflows de Axis y MedMory se estudiarán sólo como patrón. Pymes y el resto
de consumidores tendrán configuración propia y no referenciarán esos proyectos.

### 12.1 Go privado

- tags semánticos inmutables en Foundation;
- una credencial de lectura diferente por repositorio consumidor;
- secreto consumidor `FOUNDATION_DEPLOY_KEY`;
- `GOPRIVATE=github.com/devpablocristo/foundation`;
- acción compuesta privada de bootstrap fijada por SHA completo;
- `git config` temporal y `known_hosts` fijado;
- BuildKit SSH para resolver módulos durante Docker build;
- prueba de instalación desde caché fría sin checkout local de Foundation.

### 12.2 Frontend privado

- GitHub Packages como registry;
- `packages: read` en consumidores y `packages: write` sólo en releases;
- `.npmrc` generado durante CI, nunca con token commiteado;
- BuildKit secret para npm;
- paquetes publicados sin `public` access;
- lockfiles con versiones exactas y prueba de instalación desde caché fría.

### 12.3 Servicios privados

- imágenes en Artifact Registry privado;
- contratos y manifests firmados;
- pin por digest en cada entorno;
- migrador separado del runtime;
- ninguna dependencia `replace`, `file:`, `link:`, `workspace:` o ruta local;
- ninguna acción reusable fijada con `@main`.

## 13. Migración de consumidores

### 13.1 Pymes v3

Se migrarán todas las dependencias activas de Platform:

- PostgreSQL → módulo Foundation equivalente;
- observabilidad → Foundation;
- Clerk → Foundation;
- Google Calendar/Meet → adapters de Scheduling sobre el SDK Foundation;
- errores y jobs indirectos → Foundation;
- scheduling puro → nuevo módulo Foundation Scheduling;
- `platform-browser` → eliminar si no tiene consumo real;
- helpers npm de scheduling → mantener local si son sólo formatters;
- Calendar Board → Foundation Design System.

Después se sustituirán las integraciones temporales:

- pin/checkout de Open Accounting → instancia Foundation Accounting;
- dependencia npm de `arca-facturacion` y servicio fiscal embebido → instancia
  Foundation Fiscal ARCA;
- contrato directo con PerGo → instancia Foundation Communications.

El gate de arquitectura rechazará nuevas referencias a Platform histórico,
dependencias locales, checkouts de otros repositorios y tipos Foundation dentro
del dominio.

### 13.2 Ponti

En la generación activa se migrarán autenticación, errores, HTTP/Gin,
notificaciones, seguridad y validación. El acceso GORM se separará de las
abstracciones PostgreSQL modernas en lugar de trasladarlo sin diseño.

Los helpers UI neutrales pasarán a Foundation. El cliente específico de
Governance de Axis permanecerá local en Ponti. Formularios y filtros genéricos
podrán pasar al design system; AI Console seguirá siendo producto.

### 13.3 Axis v3 y MedMory v2

Ya consumen Foundation. Sólo se alinearán distribución privada, pines por SHA,
instalación con caché fría y controles para impedir secretos dentro de imágenes.
No se modifica su dominio por esta migración.

## 14. Privacidad de repositorios

Orden obligatorio:

1. preparar distribución privada en Foundation;
2. demostrar instalación privada desde cada consumidor activo;
3. migrar Pymes y Ponti;
4. comprobar cero Platform en generaciones activas;
5. convertir Pymes y Ponti en privados;
6. convertir o reemplazar Open Accounting y ARCA por históricos privados;
7. resolver el fork PerGo mediante detach o mirror privado;
8. hacer privado y archivar Platform;
9. verificar que CI, builds y deploys funcionan desde caché fría.

Volver privado un repositorio no elimina código que ya fue público ni copias en
caches o proxies. Se revisarán secretos históricos y se rotará cualquier
credencial que alguna vez haya sido commiteada.

Las protecciones de rama conservarán checks automáticos, conversaciones
resueltas e historial lineal, pero no exigirán aprobación de un tercero al dueño
único del repositorio.

## 15. GCP y aislamiento operativo

La migración se validará primero en código, CI y entorno local. Luego se
desplegará en STG y finalmente en PRD.

Para Pymes:

- se mantiene `pymes-dev-352318` y `us-central1` mientras no exista una decisión
  posterior explícita;
- cada instancia Foundation tendrá service account, base lógica, usuario SQL,
  secretos, clave KMS, migrador, backups y alertas propios;
- STG y PRD permanecerán separados aunque compartan proyecto para reducir
  costos;
- no se eliminarán ni reducirán grants de Axis, v2 u otros productos;
- sólo se modificarán recursos cuyo nombre, identidad y ownership sean de
  Pymes/Foundation para Pymes;
- los cambios IAM amplios quedan prohibidos salvo auditoría y decisión
  específica independiente.

Feature flags por organización controlarán el corte:

- `foundation_accounting_enabled`;
- `foundation_fiscal_enabled`;
- `foundation_communications_enabled`;
- `foundation_scheduling_library_enabled`;
- `google_calendar_enabled`;
- `google_meet_enabled`.

## 16. Orden de ejecución

### F0. Congelar evidencia y preparar la migración

- registrar SHAs, tags, licencias, CI y estado limpio de todos los repositorios;
- actualizar `AGENTS.md` para reemplazar la política Platform por Foundation;
- actualizar los documentos y ADRs activos que todavía ordenen consumir
  Platform; los ADRs históricos incompatibles se marcarán como reemplazados y
  enlazarán la nueva decisión, sin reescribir su historia;
- documentar la matriz de ownership y el inventario de consumidores activos;
- crear gates que distingan generaciones activas de árboles congelados;
- registrar procedencia, commits, copyright y licencias de cada bloque extraído;
- no mezclar esta fase con cambios de comportamiento.

### F1. Cadena privada de Foundation

- crear release workflows de módulos Go, paquetes frontend y servicios;
- publicar acciones privadas reutilizables fijables por SHA;
- configurar GitHub Packages y Artifact Registry;
- agregar manifests, SBOM, provenance y verificación de digest;
- demostrar consumo desde un repositorio privado de prueba y caché fría.

### F2. Migrar librerías Platform a Foundation

- mover y estabilizar los módulos requeridos por Pymes y Ponti;
- crear Foundation Scheduling puro;
- portar Calendar Board al design system;
- publicar versiones semánticas;
- marcar Platform como deprecado y bloquear publicaciones nuevas.

### F3. Crear Foundation Accounting

- trasladar el runtime headless validado;
- preservar migraciones e invariantes;
- publicar imagen, OpenAPI, cliente y manifest;
- probar equivalencia contra Open Accounting;
- desplegar una instancia Pymes STG y migrar el adapter Pymes.

### F4. Crear Foundation Fiscal ARCA

- integrar el SDK de bajo nivel y Fiscal Adapter;
- mantener mock y real detrás del mismo contrato;
- implementar vault y onboarding por tenant;
- publicar imagen, contratos y manifest;
- validar homologación con un cliente piloto.

### F5. Crear Foundation Communications

- extraer el núcleo reusable de PerGo;
- publicar contrato e imagen;
- integrar fake contractual y webhook firmado;
- adoptar desde Pymes sin bloquear el dominio ante fallos.

### F6. Migrar Pymes v3 completamente

- sustituir todas las dependencias Platform por Foundation;
- adoptar servicios Foundation por adapter;
- eliminar checkouts, pins y builds de fuentes externas;
- actualizar web y lockfiles;
- ejecutar CI completo local y remoto;
- desplegar STG por flags y verificar reconciliación/rollback.

### F7. Migrar el resto del ecosistema activo

- Ponti;
- endurecimiento privado de Axis v3 y MedMory v2;
- gate global de cero Platform activo y cero dependencias locales.

### F8. Privatizar y retirar repositorios históricos

- convertir consumidores activos en privados;
- resolver PerGo fork/mirror;
- archivar Open Accounting, ARCA y Platform;
- conservar tags, bundles y trazabilidad;
- rotar secretos históricos y verificar accesos.

### F9. Despliegue y pilotos

- Accounting en STG y PRD;
- Communications con número controlado;
- Google/Meet con cuenta controlada;
- Fiscal homologación y producción por cliente;
- backups/restores y reconciliadores;
- corte progresivo por organización;
- no operar v2 y v3 simultáneamente sobre el mismo punto de venta.

## 17. Pruebas y gates

### Foundation

- unitarias, race, lint, vulnerabilidades y licencias;
- integración PostgreSQL por servicio;
- compatibilidad OpenAPI y clientes generados;
- builds reproducibles por digest;
- instalación Go/npm desde repositorio privado y caché fría;
- migraciones repetibles, upgrade y rollback documentado;
- SBOM y provenance verificables.

### Pymes

- `make architecture-check`;
- `make api-check`;
- `make db-integration`;
- `make scheduling-e2e`;
- `make notifications-e2e`;
- `make accounting-e2e`;
- `make fiscal-real-contract`;
- `make web-ci`;
- `make ci`.

### Escenarios obligatorios

- aislamiento estricto entre dos tenants;
- reservas concurrentes, recursos múltiples, capacidad, DST y waitlist;
- Accounting: replay, respuesta perdida, reversas, partidas y aging histórico;
- Fiscal: autorizado, rechazado, incierto, consulta exacta, certificado vencido
  y tenant cruzado;
- Communications: timeout antes/después, duplicado, caída y webhook repetido;
- adapters Google de Scheduling: OAuth inválido, token revocado, `409`, `412`,
  Meet pendiente y timeout;
- backup y restore por servicio;
- despliegue sin checkout de Foundation ni secreto dentro de la imagen;
- búsqueda global sin Platform en generaciones activas;
- búsqueda global sin `replace`, `file:`, `link:`, `workspace:` o rutas locales;
- workflows sin acciones privadas flotantes en `@main`.

Los tests determinísticos usarán fakes. Google real, ARCA homologación y pruebas
operativas se ejecutarán como jobs protegidos y separados.

## 18. Rollout y reversión

- cada adopción se hace por consumidor y por capacidad, nunca como big bang;
- primero se publica Foundation, después se actualiza el adapter consumidor;
- los contratos mantendrán compatibilidad durante una ventana de transición;
- los servicios se despliegan por digest antes de habilitar el flag;
- se ejecuta shadow/read-only cuando sea posible;
- el rollback deshabilita el flag y vuelve al digest anterior, sin revertir datos
  incompatibles;
- las migraciones destructivas quedan prohibidas hasta finalizar la ventana de
  convivencia;
- los repositorios históricos no se archivan hasta probar restore y rollback.

## 19. Riesgos que deben controlarse

- convertir Foundation en un monolito de dependencias sin límites claros;
- trasladar dominio de producto a una librería “genérica”;
- mezclar datos de productos por ahorrar infraestructura;
- romper builds privados por depender de credenciales personales;
- perder trazabilidad al mover historias de repositorios;
- obligaciones de licencias transitivas, especialmente MPL-2.0;
- mantener dos implementaciones activas indefinidamente;
- modificar IAM de otros proyectos o productos dentro del proyecto GCP
  compartido;
- afirmar cierre sólo porque CI está verde sin pilotos, restore y evidencia
  operativa.

Cada riesgo tendrá owner, evidencia, rollback y criterio de salida en el roadmap
de ejecución.

## 20. Criterio de finalización al 100%

El plan sólo estará completo cuando se demuestre simultáneamente que:

- Foundation es la única fuente activa de código compartido;
- sus módulos Go, paquetes frontend y servicios se publican de forma privada,
  reproducible e inmutable;
- Pymes v3 no consume Platform, Open Accounting, ARCA ni PerGo como fuentes o
  dependencias directas;
- Foundation Accounting funciona con todas las invariantes contables ya
  validadas;
- Foundation Fiscal ARCA funciona por organización con numeración explícita;
- Foundation Communications entrega WhatsApp sin acoplar el dominio de Pymes;
- Agenda continúa completa y tenant-safe dentro de Pymes usando sólo el núcleo
  algorítmico Foundation;
- Google Calendar/Meet funciona como adapters de Scheduling mediante el SDK
  Foundation y sin un servicio o contexto Calendars separado;
- Ponti activo fue migrado;
- Axis v3 y MedMory v2 consumen Foundation privado de forma inmutable;
- todas las generaciones activas pasan el gate cero Platform;
- no existen dependencias locales ni acciones privadas flotantes;
- los repositorios propios activos son privados;
- Platform, Open Accounting, ARCA y PerGo histórico están privados/retirados o
  reemplazados conforme a su caso;
- CI local y remoto pasa desde caché fría;
- STG y PRD están desplegados con identidades, secretos, KMS y bases separados;
- backups y restores fueron probados;
- los pilotos de Agenda, Communications, Google y Fiscal fueron realizados;
- la documentación, manifests y auditoría no conservan decisiones abiertas.

## 21. Fuera de este plan

- migración de datos de Pymes v2;
- cambios funcionales en generaciones congeladas;
- FullCalendar Premium o v7;
- sincronización bidireccional de Google;
- Outlook y Teams;
- padrón, FCE, WSFEX y CAEA;
- videollamadas propias;
- broker obligatorio;
- un servicio Foundation Scheduling con persistencia;
- compartir bases, secretos o instancias entre productos.
