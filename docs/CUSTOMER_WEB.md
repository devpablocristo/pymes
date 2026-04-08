# Web clientes — superficie pública del producto

> **Propósito.** Inventario de todo lo que ven y usan los **clientes finales** de
> las PyMEs (los clientes del comerciante), separado de lo que usa el dueño o
> los empleados en la consola interna.
>
> **Para qué sirve este documento.**
> 1. Tener un mapa mental compartido para no confundir "el calendario del dueño"
>    con "el calendario del cliente" (o "el chat del operador" con "el chat del
>    cliente final"), que es la fuente de la mayoría de las regresiones de UX.
> 2. Servir como lista cuando más adelante decidamos partir el frontend en dos
>    apps (`apps/console` para el dueño / empleados, `apps/customer-web` para
>    el cliente final), o cuando agreguemos un portal del cliente con login.
> 3. Mantener visible el **backlog cliente-facing** (sección 6) para que no se
>    pierda entre features de operación interna.
>
> **Lo que NO es este documento.** No es un refactor, no propone mover código,
> no duplica nada de lo que ya está en `docs/PYMES_CORE.md`, `docs/AUTH.md` ni
> `pymes-core/backend/docs/SCHEDULING_BACKEND.md`. Es solo un índice cruzado.

---

## Definición operativa

Una funcionalidad es **cliente-facing** si cumple **todas** estas condiciones:

1. La usa una persona que **no tiene cuenta en la consola interna** (no es
   dueño ni empleado de la PyME).
2. Accede **sin login**, o con identidad mínima (token de acción, teléfono,
   email), nunca con sesión Clerk / RBAC del producto.
3. Representa al **cliente del comerciante**, no al comerciante mismo.

Si una funcionalidad la dispara el dueño pero el resultado lo consume el
cliente (ej. `wa.me/` link generado desde la consola), **no** es cliente-facing
en este sentido — es una herramienta del dueño con salida hacia afuera. Esos
casos van listados en la sección 5 ("Frontera dueño → cliente") solo como
referencia.

---

## 1. Reservas / agenda pública (scheduling)

Es el módulo cliente-facing más completo hoy. Permite que un cliente reserve,
confirme o cancele turnos sin tener cuenta.

**Backend.** Montado bajo `/v1/public/:org_id` con rate limit 30 req/min y
body máx 64KB en
[pymes-core/backend/wire/bootstrap.go:279-284](../pymes-core/backend/wire/bootstrap.go#L279-L284).

Endpoints (documentados también en
[pymes-core/backend/docs/SCHEDULING_BACKEND.md:217-226](../pymes-core/backend/docs/SCHEDULING_BACKEND.md#L217-L226)):

| Método | Path | Descripción |
|---|---|---|
| GET | `/v1/public/:org_id/info` | Info del negocio (nombre, dirección, horarios) |
| GET | `/v1/public/:org_id/catalog/services` | Catálogo público de servicios reservables |
| GET | `/v1/public/:org_id/scheduling/services` | Idem desde el namespace `scheduling` |
| GET | `/v1/public/:org_id/scheduling/availability` | Slots disponibles para una fecha/servicio |
| POST | `/v1/public/:org_id/scheduling/book` | Reservar un turno |
| GET | `/v1/public/:org_id/scheduling/my-bookings` | Mis reservas (identidad por teléfono/email) |
| GET | `/v1/public/:org_id/scheduling/queues` | Listado de colas activas |
| POST | `/v1/public/:org_id/scheduling/queues/:id/tickets` | Sacar ticket de cola |
| GET | `…/queues/:id/tickets/:ticket_id/position` | Mi posición en la cola |
| POST | `/v1/public/:org_id/scheduling/waitlist` | Sumarme a la lista de espera |
| POST | `/v1/public/:org_id/scheduling/bookings/actions/confirm` | Confirmar via magic link |
| POST | `/v1/public/:org_id/scheduling/bookings/actions/cancel` | Cancelar via magic link |

Los magic links se construyen en
[pymes-core/backend/internal/scheduler/usecases.go:427](../pymes-core/backend/internal/scheduler/usecases.go#L427)
(`/v1/public/{orgSlug}/scheduling/bookings/actions/{action}?token={token}`).

**Frontend / componentes compartidos.**

- [modules/scheduling/ts/src/PublicSchedulingFlow.tsx](../../modules/scheduling/ts/src/PublicSchedulingFlow.tsx) —
  el flujo real que ve el cliente: catálogo → fecha → slot → confirmación.
- [modules/scheduling/ts/src/QueueOperatorBoard.tsx](../../modules/scheduling/ts/src/QueueOperatorBoard.tsx) —
  contiene tanto la vista del operador como el lado cliente de la cola; en
  un futuro split conviene partirlo en dos componentes.
- [modules/scheduling/ts/src/SchedulingCalendarBoard.tsx](../../modules/scheduling/ts/src/SchedulingCalendarBoard.tsx) —
  hoy se renderiza dentro de la consola del dueño en
  [frontend/src/pages/CalendarPage.tsx](../frontend/src/pages/CalendarPage.tsx),
  pero su flujo de booking restringido (branch → service → resource → slot)
  está pensado para el cliente final, no para el dueño. Es el origen de la
  confusión que motivó este documento — ver nota al final.

**Vista preview en la consola del dueño.**

[frontend/src/pages/PublicPreviewPage.tsx](../frontend/src/pages/PublicPreviewPage.tsx)
renderiza `PublicSchedulingFlow` dentro de la consola autenticada para que el
dueño vea cómo le aparece a sus clientes. Es owner-side por hosting pero
expone literalmente la UX cliente-facing — útil tenerlo en cuenta cuando
cambiemos estilos del flujo público.

---

## 2. Chat con IA público (agentes conversacionales)

Servicio FastAPI en [`ai/`](../ai/), routers públicos sin auth de sesión:

| Router | Endpoint | Descripción |
|---|---|---|
| [ai/src/api/public_router.py:21](../ai/src/api/public_router.py#L21) | `POST /v1/public/{org_slug}/chat` | Chat genérico con el agente del negocio |
| [ai/src/api/public_router.py](../ai/src/api/public_router.py) | `POST /v1/public/{org_slug}/chat/identify` | Identificar al cliente por teléfono/email |
| [ai/src/api/public_sales_router.py:36](../ai/src/api/public_sales_router.py#L36) | `POST /v1/public/{org_slug}/sales-agent/chat` | Agente comercial (presupuestos, links de pago) |
| [ai/src/api/public_sales_router.py:77](../ai/src/api/public_sales_router.py#L77) | `POST /v1/public/{org_slug}/sales-agent/contracts` | Generación de contrato al cliente |
| [ai/src/domains/professionals/teachers/public_router.py](../ai/src/domains/professionals/teachers/public_router.py) | `POST /v1/professionals/teachers/public/{org_slug}/chat` | Agente vertical profesores |
| [ai/src/domains/workshops/auto_repair/public_router.py](../ai/src/domains/workshops/auto_repair/public_router.py) | `POST /v1/workshops/auto-repair/public/{org_slug}/chat` | Agente vertical taller mecánico |
| [ai/src/domains/workshops/bike_shop/public_router.py](../ai/src/domains/workshops/bike_shop/public_router.py) | `POST /v1/workshops/bike-shop/public/{org_slug}/chat` | Agente vertical bicicletería |

Todos los agentes públicos comparten herramientas: ver disponibilidad, agendar
turnos (`scheduling.book`, `scheduling.reschedule`), consultar catálogo, armar
presupuestos, generar links de pago. Las descripciones de las herramientas
están en español (cliente-facing) en
[ai/src/tools/review_policy.py:54-55](../ai/src/tools/review_policy.py#L54-L55).

---

## 3. WhatsApp inbound (cliente escribiendo al negocio)

Webhooks de Meta en
[pymes-core/backend/internal/whatsapp/inbound.go](../pymes-core/backend/internal/whatsapp/inbound.go),
montados sin auth (rate limit 240/min, body máx 256KB) — ver
`docs/WHATSAPP_SETUP.md` y `CLAUDE.md §12`.

| Método | Path | Descripción |
|---|---|---|
| GET | `/v1/webhooks/whatsapp` | Verificación inicial Meta |
| POST | `/v1/webhooks/whatsapp` | Mensajes entrantes + status (delivered/read) |

El cliente nunca toca directamente estos endpoints — escribe al WhatsApp del
negocio, Meta reenvía acá, y el AI agent toma la conversación. Por eso es
cliente-facing aunque el cliente vea solo la UI de WhatsApp.

**Opt-in.** Antes de poder enviar mensajes salientes a un cliente, debe
existir registro en `whatsapp_opt_ins`. La gestión la hace el dueño desde la
consola, pero el cliente tiene que haber dado consentimiento previo (LATAM
compliance). Ver `CLAUDE.md §12.6`.

---

## 4. Pagos externos (cliente paga)

[pymes-core/backend/internal/paymentgateway/handler.go:73-75](../pymes-core/backend/internal/paymentgateway/handler.go#L73-L75)
registra rutas externas bajo `/v1/public/:org_id`:

| Método | Path | Descripción |
|---|---|---|
| GET | `/v1/public/:org_id/quote/:id/payment-link` | Obtener el link de pago de un presupuesto |

Hoy esto es **una sola ruta**: el cliente abre el link que recibió por
WhatsApp/email y el endpoint resuelve el redirect al gateway (Mercado Pago u
otro). Todo el resto del checkout (formulario, captura de tarjeta, callback)
ocurre en el gateway externo. Cuando agreguemos métodos de pago locales o
checkout propio, el inventario crece acá.

Documentación de fraude / trazabilidad de cobros:
[pymes-core/docs/FRAUD_PREVENTION.md](../pymes-core/docs/FRAUD_PREVENTION.md).

---

## 5. Frontera dueño → cliente (no es web clientes, pero genera salida hacia el cliente)

Lista de cosas que el **dueño** dispara desde la consola interna y cuyo
resultado le llega al cliente. **No** son web clientes según la definición
operativa, pero es útil tenerlas mapeadas en el mismo lugar para no mezclarlas.

| Feature | Endpoint owner-side | Sale al cliente como |
|---|---|---|
| Link `wa.me` de presupuesto | `GET /v1/whatsapp/quote/:id` ([whatsapp/handler.go:80](../pymes-core/backend/internal/whatsapp/handler.go#L80)) | Mensaje de WhatsApp con texto preformado |
| Link `wa.me` de comprobante de venta | `GET /v1/whatsapp/sale/:id/receipt` ([whatsapp/handler.go:81](../pymes-core/backend/internal/whatsapp/handler.go#L81)) | Idem |
| Mensaje libre a cliente | `GET /v1/whatsapp/customer/:id/message` | Idem |
| Envío directo por WhatsApp Cloud API | `POST /v1/whatsapp/send/{text,template,media,interactive}` | Mensaje real desde el número del negocio |
| Confirmación / cancelación de booking por email | Generado por `scheduler.usecases` | Magic link → consume rutas de la sección 1 |
| Generación de PDF de presupuesto / factura | `pdfgenHandler` (auth) | Adjunto en mensaje |

Todas estas rutas están detrás de auth + RBAC en
[pymes-core/backend/wire/bootstrap.go:286-320](../pymes-core/backend/wire/bootstrap.go#L286-L320).
El cliente nunca las toca directamente.

---

## 6. Backlog cliente-facing (lo que **no existe todavía**)

Cosas que tendrían sentido como features del cliente final pero hoy no están
implementadas. No es un compromiso, es una lista de candidatos para discutir.

### Portal del cliente con login (mediano plazo)
- Login del cliente final (magic link por email/SMS, sin password).
- "Mis turnos" persistente, no por token efímero como hoy.
- Historial de compras / presupuestos / facturas del cliente con esa PyME.
- Re-booking en un click ("repetir el último turno").
- Cancelar / reprogramar sin necesidad de buscar el email original.

### Tracking de orden de trabajo (workshops + verticales con OT)
- URL pública con token: `"tu auto está en diagnóstico / esperando repuesto / listo para retirar"`.
- Notificaciones push (PWA) o WhatsApp automáticas en cada cambio de estado.
- Foto del trabajo terminado, presupuesto adicional con aprobación 1-click.
- Equivalente para `professionals` (entrega de informes), `beauty` (recordatorio + foto post servicio), `restaurants` (estado del pedido en mesa).

### Recibo público
- URL única `/v1/public/:org_id/receipts/:id?token=...` con la venta detallada,
  imprimible y compartible.
- Hoy el comprobante viaja como PDF adjunto por WhatsApp; falta el equivalente web.

### Reseñas / NPS
- Pedido de reseña automatizado al cerrar la OT o el turno.
- Página pública de reseñas del negocio (cliente-facing) o integración con
  Google Business Profile.
- Hoy hay infraestructura de `review-notifications` en el lado dueño
  ([ai/src/api/review_callback.py](../ai/src/api/review_callback.py)) pero no
  superficie para que el cliente final deje la reseña.

### Auto-servicio de cuenta corriente
- "¿Cuánto me debo con este negocio?" — saldo, vencimientos, link de pago.
- Especialmente relevante para verticales con cuenta corriente
  (`professionals`, `workshops` con clientes recurrentes).

### Loyalty / fidelización
- "Tenés N visitas, en la próxima te toca el descuento" visible para el cliente.
- Cupones / códigos canjeables.

### Catálogo público navegable (no solo en chat IA)
- Versión web del menú / servicios / productos sin tener que entrar al chat.
- Especialmente relevante para `restaurants` (menú QR) y `beauty` (vidriera de
  servicios con fotos y precios).

### Reservas grupales / eventos
- Reservar para varias personas al mismo tiempo (eventos, clases grupales).
- Hoy `scheduling` modela 1 cliente = 1 booking.

---

## Nota sobre el calendario interno (origen de este documento)

El documento nace porque [frontend/src/pages/CalendarPage.tsx](../frontend/src/pages/CalendarPage.tsx)
renderiza
[`SchedulingCalendar`](../../modules/scheduling/ts/src/SchedulingCalendarBoard.tsx)
del módulo compartido, que está **diseñado para el flujo cliente-facing**
(branch → service → resource → slot, resize bloqueado, validación de
disponibilidad). El dueño en su consola interna esperaba el comportamiento
"estilo Google Calendar" libre que existió hasta el commit `1d3abb6`
(2026-04-03), donde se reemplazó la implementación local por el módulo
compartido como parte del refactor "consume shared runtime and modules
primitives" ([45d343c](../../)).

### Solución implementada

Después de inspeccionar el módulo upstream se descubrió que el modelo
`BlockedRange` (kinds `holiday | manual | maintenance | leave`) ya existía
en [`modules/scheduling/go/domain/entities.go`](../../modules/scheduling/go/domain/entities.go),
con repo, usecase, endpoints `GET/POST` y, lo más importante, **ya descontaba
los bloqueos del cálculo de availability** (`generateSlotsForResource`).
Solo faltaba completar el CRUD y renderizarlos en el frontend.

Cambios en el módulo (no en pymes):

- **Backend `modules/scheduling/go` v0.2.0** — `UpdateBlockedRange` y
  `DeleteBlockedRange` en repo, usecase y handler HTTP
  (`PATCH /scheduling/blocked-ranges/:id` con `scheduling:update`,
  `DELETE /scheduling/blocked-ranges/:id` con `scheduling:delete`),
  audit logs `scheduling.blocked_range.{updated,deleted}`, validación
  extraída a `validateBlockedRangeFields` (DRY con Create), tests del
  validador.
- **Frontend `modules/scheduling/ts` v0.2.0** — tipos `BlockedRange`,
  `BlockedRangePayload`; cliente con `listBlockedRanges`, `createBlockedRange`,
  `updateBlockedRange`, `deleteBlockedRange`; componente `BlockedRangeModal`
  (~230 LOC); botón "Bloquear horario" en el aside del calendario; query
  `blockedRangesQuery` que se invalida junto con el resto de la agenda;
  bloqueos renderizados como eventos grises en FullCalendar con
  `extendedProps.blockedRange` para discriminarlos de los bookings; soporte
  drag/resize de bloqueos (los bookings siguen restringidos); tests del
  cliente y del flujo de creación end-to-end.

Cambios en pymes (mínimos):

- `go.mod`: `replace github.com/devpablocristo/modules/scheduling/go => ../modules/scheduling/go`
  (temporal hasta publicar v0.2.0).
- `frontend/src/pages/CalendarPage.tsx`: **cero cambios** — el componente
  ahora soporta bloqueos out of the box.
- Esta nota.

El dueño ahora puede crear, editar, mover, redimensionar y borrar bloqueos
desde su calendario interno sin pasar por el flujo de booking restringido
(que sigue siendo el correcto para el cliente final). Los bloqueos afectan
automáticamente la disponibilidad que ve el cliente en
`PublicSchedulingFlow`, sin código adicional.

### Drag/resize libres en bookings (F2 — implementado)

Segundo paso de la misma línea de trabajo: además de los bloqueos, el dueño
ahora puede **arrastrar y redimensionar bookings** libremente desde la consola
interna. Antes el resize estaba bloqueado y el drag exigía soltar en un slot
pre-calculado del servicio.

Cambios en el módulo (no en pymes):

- **Backend `modules/scheduling/go` v0.3.0** —
  - `RescheduleBookingInput` extendido con `EndAt *time.Time` opcional.
  - `RescheduleBooking` usecase: si `EndAt` viene, salta el slot lookup y entra
    a modo "custom duration", validando contra availability rules + blocked
    ranges + booking overlaps via nuevo helper privado
    `validateBookingRangeFits`. Si `EndAt` no viene, comportamiento idéntico
    al anterior (compatibilidad total con el flujo público y con consumidores
    que no necesitan custom duration).
  - DTO `RescheduleBookingRequest.EndAt *string` y handler que parsea
    `end_at` opcional con `parseRFC3339Ptr`.
  - Audit log y `scheduling.booking.rescheduled` event ahora incluyen `end_at`
    para que los downstream (notificaciones al cliente, integraciones) vean
    la nueva duración.
  - Tests: `TestRangeFitsAnyWindow` con 6 casos (inside, exact, lunch break,
    before/after, partially-out).

- **Frontend `modules/scheduling/ts` v0.3.0** —
  - `RescheduleBookingPayload.end_at?: string` en types.
  - `handleEventAllow` simplificado: ahora deja pasar cualquier rango no
    vacío para bookings y blocks. La validación dura está en el backend.
  - `handleEventResize` para bookings: ya no muestra "duration locked";
    ahora persiste como reschedule custom-duration con confirmación previa
    (`resizeBookingTitle` / `resizeBookingDescription`).
  - `handleCalendarEventDrop` para bookings: además de `start_at` ahora
    manda también `end_at`, así el backend respeta la duración del evento
    arrastrado (relevante si el dueño ya había hecho un resize previo).
  - Helper compartido `persistBookingReschedule(info, sourceBooking, copy)`
    para no duplicar la rama de drop y resize.
  - El test `confirms and persists calendar event drag reschedules` actualizado
    para verificar que `client.rescheduleBooking` recibe `end_at`.
  - Test nuevo `persists calendar event resize as a custom-duration reschedule`.
  - Bug encontrado en el harness de tests: `calendarSurfaceMocks.last` era un
    global del módulo que dejaba closures stale entre tests. Solucionado
    reseteándolo en `renderCalendar`.

Cambios en pymes:

- `go.mod` replace actualizado al pin de v0.3.0.
- Esta nota.

### Decisiones de producto que entraron en F2

1. **Revalidación dura**: cuando el dueño mueve o redimensiona un booking,
   el backend revalida contra `availability_rules`, `blocked_ranges` y
   bookings existentes del mismo recurso. Si el destino no encaja, devuelve
   `409 conflict` y el frontend hace `info.revert()`. Razón: garantiza que la
   agenda interna del dueño y la disponibilidad que ven los clientes finales
   en `PublicSchedulingFlow` siempre coincidan.
2. **Notificación automática al cliente**: el flujo reusa el evento
   `scheduling.booking.rescheduled` que ya emitía el módulo (downstream:
   notification port), así que cualquier reprogramación dispara la
   notificación que ya existía para el flujo público.
3. **No se permite "forzar"** un horario sin availability rule. Si el dueño
   quiere abrir un día especial, crea una availability rule nueva. Si quiere
   anotar algo libre, usa un bloqueo (F1) o un turno ad-hoc (F3).

### Turnos ad-hoc — servicio catch-all (F3 — implementado)

Tercer paso. El dueño quería poder anotar turnos sin tener que crear un
servicio nuevo del catálogo cada vez. Decisión de producto: en vez de
modificar el modelo `Booking` para hacer `service_id` nullable (cambio
upstream invasivo), agregamos un servicio especial **"Turno general"** con
duración corta (15min) que el dueño selecciona como comodín; la duración real
se ajusta arrastrando el evento (F2).

Cambios en el módulo:

- **Backend `modules/scheduling/go` v0.4.0** —
  - Nuevo seed `seeds/0002_catchall_service.sql`: crea el servicio
    `general_appointment` ("Turno general", 15min, sin buffers, schedule mode,
    `metadata = {"catchall": true}`) y lo linkea a todos los recursos activos
    de la org. Idempotente vía `uuid_generate_v5` y `ON CONFLICT DO NOTHING`.
  - `seeds/runner.go`: ahora aplica los seeds en orden via `demoFiles[]`
    (0001 demo + 0002 catch-all). 0002 depende de los recursos creados por
    0001.

Cambios en pymes:

- **`pymes-core/backend/internal/publicapi/repository.go`**: nuevo helper
  `isCatchAllService(metadata)` que detecta el flag `metadata.catchall = true`
  (acepta bool y string para tolerar variantes en jsonb). `listSchedulingPublicServices`
  filtra estos servicios del catálogo público — el cliente final que reserva
  por `PublicSchedulingFlow` nunca ve "Turno general".
- **`pymes-core/backend/internal/publicapi/repository_test.go`**: `TestIsCatchAllService`
  con 9 casos cubriendo nil, vacío, bool true/false, string true/TRUE/false,
  tipo no soportado, key ausente.
- **`pymes/scripts/seeds/core-06-scheduling.sh`**: ahora ejecuta
  `0001_demo.sql` + `0002_catchall_service.sql` cuando se corre `make seed`.
- `go.mod` replace pin actualizado a v0.4.0.
- Esta nota.

Lo que el dueño puede hacer ahora (acumulado F1+F2+F3):

1. Bloqueos one-shot (F1).
2. Drag/resize libres en bookings (F2).
3. **Crear un turno ad-hoc**: en el calendario interno, abre el modal de
   reserva, elige el servicio "Turno general" (15min default), elige el
   recurso, guarda. Después arrastra el evento para ajustar la duración y/o
   moverlo. Cero impacto en el catálogo público.

### Limitaciones aceptadas de F3

1. **No se autoprovisiona en orgs nuevas de producción.** El seed corre solo
   durante `make seed` (dev/demo). Para una PyME real recién onboardeada, el
   dueño tiene que crear el servicio "Turno general" manualmente vía la UI
   de admin, **o** correr el SQL del seed durante el provisionamiento. Si en
   uso real esto resulta fricción, el siguiente paso es agregar un
   `EnsureCatchAllService` usecase que se llame desde el flow de onboarding
   o lazy desde la primera carga del calendario.
2. **Recursos nuevos no se linkean automáticamente.** El seed linkea el
   catch-all a los recursos que existían al momento de correrlo. Si la
   PyME agrega un recurso nuevo después, tiene que linkearlo manualmente al
   catch-all desde la UI de servicios. Mismo workaround si se vuelve molesto:
   un trigger o un usecase de re-link.

### Lo que no entró en este cambio

1. Mostrar el motivo del bloqueo en `PublicSchedulingFlow` (F4 — opcional,
   hoy solo se ve la ausencia de slots).
2. Unificación calendario ↔ `AvailabilityRule` para editar reglas
   recurrentes desde el calendario (F5 — opcional).
