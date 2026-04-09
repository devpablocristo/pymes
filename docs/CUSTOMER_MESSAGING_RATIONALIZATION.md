# Customer Messaging Rationalization

Diseño para racionalizar WhatsApp en `pymes`, aprovechar lo que ya existe, eliminar solapamientos y extraer solo lo verdaderamente reusable hacia `core` y `modules`.

## Objetivo

Pasar de un módulo centrado en el proveedor `WhatsApp` a una capacidad de producto llamada `customer messaging`, donde:

- `WhatsApp` es un canal/adaptador, no el dominio principal
- `wa.me` y Cloud API dejan de verse como features paralelos
- inbox, conversaciones y campañas quedan separadas de la integración Meta
- las piezas reusable reales viven en `core` y `modules`
- la integración específica de Meta permanece en `pymes-core`

## Problemas actuales

### 1. Mezcla de responsabilidades

Hoy `pymes-core/backend/internal/whatsapp` mezcla:

- conexión al proveedor
- webhook e inbound
- lógica de negocio de contacto al cliente
- campañas
- inbox/conversaciones
- bridge hacia AI
- links `wa.me`

### 2. Solapamiento funcional

Existen dos caminos para “contactar al cliente por WhatsApp”:

- `wa.me` con texto prearmado
- envío oficial por Cloud API

Ambos resuelven la misma intención de negocio pero están modelados como cosas distintas.

### 3. Documentación duplicada o desalineada

La verdad funcional está repartida entre:

- [WHATSAPP_SETUP.md](./WHATSAPP_SETUP.md)
- [CLAUDE.md](../CLAUDE.md)
- [DEUDA_TECNICA.md](./DEUDA_TECNICA.md)

### 4. Bounded context mal nombrado

El nombre `whatsapp` hace que:

- el canal domine el diseño
- otras superficies queden acopladas a Meta
- el futuro agregado de otros canales sea más costoso

## Decisiones

### D1. No extraer WhatsApp entero a `core`

No mover a `core`:

- `MetaClient`
- webhook verify/signature de Meta
- `phone_number_id`, `waba_id`, tokens
- templates/campaigns/consent específicos de WhatsApp

Motivo:

- siguen pegados al producto y al proveedor
- `core` hoy no tiene otro consumidor real de esa integración
- `core/docs/EXTRACTION-SOURCES.md` ya marca esta pieza como no lista para extracción

### D2. Crear el bounded context `customer_messaging` dentro de `pymes-core`

`WhatsApp` pasa a ser un adapter del contexto.

### D3. Conservar `wa.me`, pero degradarlo a un modo de entrega

No eliminar `wa.me`.

Sí dejar de tratarlo como feature separado. Debe quedar como:

- `share_link`

frente a:

- `official_channel`

### D4. Extraer solo las piezas reusable reales

A `core`:

- eventos de mensajería
- timeline / metadata común
- handoff interno reutilizable

A `modules`:

- inbox UI
- board o feed reusable para conversaciones
- actions de asignación, leído, resolver

## Arquitectura objetivo

### Backend en `pymes-core`

```text
pymes-core/backend/internal/customer_messaging/
├── domain/
│   ├── entities.go
│   ├── conversation.go
│   ├── delivery_mode.go
│   └── events.go
├── application/
│   ├── send_message.go
│   ├── generate_share_link.go
│   ├── handle_inbound.go
│   ├── manage_conversations.go
│   ├── manage_campaigns.go
│   └── manage_consents.go
├── ports/
│   ├── timeline.go
│   ├── event_bus.go
│   ├── ai_bridge.go
│   └── channel.go
├── adapters/http/
│   ├── handler.go
│   └── dto/
└── channels/whatsapp/
    ├── meta_client.go
    ├── webhook.go
    ├── connection_repository.go
    ├── template_repository.go
    ├── consent_repository.go
    └── mapper.go
```

### Separación conceptual

- `customer_messaging`: intención de negocio
- `channels/whatsapp`: adapter de proveedor
- `ai bridge`: collaborator del contexto, no centro del módulo
- `quotes/sales/customers`: productores de contenido o destinatarios

## Modelo funcional unificado

### Capacidad canónica: contactar cliente

Casos actuales:

- presupuesto
- comprobante
- mensaje libre
- campaña
- respuesta automática

Todos deben pasar por una intención común:

```text
ContactCustomer
├── target
├── content
├── channel = whatsapp
└── delivery_mode = share_link | official_channel
```

### Delivery modes

`share_link`

- abre `wa.me`
- no requiere conexión oficial
- no persiste mensaje como envío oficial
- útil para handoff humano/manual

`official_channel`

- usa Cloud API
- requiere conexión activa
- requiere opt-in
- persiste mensaje y estado operativo

## Qué se queda en `pymes-core`

Debe quedarse en `pymes-core` porque es producto o proveedor específico:

- conexión WhatsApp por org
- tablas `whatsapp_connections`, `whatsapp_templates`, `whatsapp_opt_ins`
- campaigns y recipients
- webhook público de Meta
- bridge inbound -> AI -> reply
- links comerciales ligados a `quotes` y `sales`

## Qué se extrae a `core`

### 1. Event contracts

Usar o ampliar [core/eventing/go](../../core/eventing/go/envelope.go) con eventos como:

- `customer_message.received`
- `customer_message.sent`
- `customer_message.delivery_updated`
- `conversation.assigned`
- `conversation.resolved`
- `campaign.dispatched`

Esto permite desacoplar:

- timeline
- métricas
- handoffs
- futuros consumidores

### 2. Timeline y metadata común

Reusar [core/activity/go](../../core/activity/go) para registrar actividad operativa de mensajería sin depender del proveedor.

### 3. Inbox de notificaciones no-chat

Reusar [core/notifications/go/inbox](../../core/notifications/go/inbox) para avisos internos, por ejemplo:

- “nueva conversación sin asignar”
- “campaña completada”
- “error de envío masivo”

No usarlo como storage de chat. Solo para avisos operativos.

## Qué se extrae a `modules`

### 1. Inbox UI reusable

Crear un módulo UI reusable, por ejemplo:

```text
modules/inbox/ts/
├── src/ConversationInbox.tsx
├── src/types.ts
├── src/actions.ts
└── src/styles.css
```

Apoyarse en:

- [modules/ui/notification-feed/ts](../../modules/ui/notification-feed/ts/src/NotificationFeed.tsx)
- [modules/kanban/board/ts](../../modules/kanban/board/ts/src/StatusKanbanBoard.tsx)

### 2. No extraer una UI WhatsApp-specific

`WhatsAppInboxPage` y `WhatsAppCampaignsPage` en `frontend` deben quedar como composición/configuración de módulos reusable, no como lógica bespoke.

## Qué eliminar o deprecar

### Deprecar a nivel conceptual

- “módulo WhatsApp” como dominio completo

Reemplazar por:

- `customer messaging`
- `whatsapp channel`
- `share links`

### Deprecar duplicación documental

Dejar una sola fuente de verdad por tipo:

- `docs/CUSTOMER_MESSAGING_RATIONALIZATION.md`: diseño
- `docs/WHATSAPP_SETUP.md`: setup operativo de Meta
- `docs/DEUDA_TECNICA.md`: gaps reales de implementación

[CLAUDE.md](../CLAUDE.md) debe quedar solo como resumen breve y puntero.

### Deprecar endpoints internos demasiado acoplados al canal

Actual:

- `/v1/internal/v1/whatsapp/send-text`

Objetivo:

- `/v1/internal/v1/customer-messaging/send-text`

Mantener compatibilidad transitoria con el path actual y marcarlo como legacy.

## API target

### API externa owner-side

Mantener en transición:

- `/v1/whatsapp/...`

Objetivo canónico:

- `/v1/customer-messaging/connections/whatsapp`
- `/v1/customer-messaging/messages`
- `/v1/customer-messaging/conversations`
- `/v1/customer-messaging/campaigns`
- `/v1/customer-messaging/consents/whatsapp`
- `/v1/customer-messaging/share-links`

No migrar todo de golpe. Primero reorganizar internamente; luego exponer aliases o v2.

### API interna service-to-service

Canónica:

- `POST /v1/internal/v1/customer-messaging/send-text`

Opcional futuro:

- `POST /v1/internal/v1/customer-messaging/send`

con payload:

- `channel`
- `delivery_mode`
- `party_id`
- `body`
- `template`
- `media`

## Reglas de dominio

### R1. El dominio no sabe de Meta

El dominio conoce:

- canal
- modo de entrega
- consent
- conversación
- campaña

No conoce:

- `X-Hub-Signature-256`
- `waba_id`
- `phone_number_id`

### R2. `wa.me` y Cloud API son modos, no dominios

La intención de negocio es la misma: contactar al cliente.

### R3. Chat e inbox no son lo mismo

- `conversation storage`: producto
- `notification inbox`: reusable para avisos internos

### R4. Estado documentado debe coincidir con estado implementado

Hoy el tracking de `statuses` de Meta está modelado pero no totalmente conectado en el webhook actual.

Hasta implementarlo:

- documentarlo como parcial
- no venderlo como cerrado

## Plan de migración

### Fase 0. Documentación y lenguaje

- crear este documento
- reducir duplicación entre docs
- adoptar el nombre `customer messaging`

### Fase 1. Refactor interno sin romper API

- crear `internal/customer_messaging`
- mover use cases y entidades
- dejar `internal/whatsapp` como facade/adaptador temporal

### Fase 2. Separar canal de dominio

- mover webhook y client Meta a `channels/whatsapp`
- dejar campañas/consents/conversations en el contexto nuevo

### Fase 3. Extraer reusables

- eventos a `core/eventing`
- inbox UI a `modules/inbox/ts`
- timeline común a `core/activity` donde aplique

### Fase 4. Alias y deprecaciones HTTP

- introducir rutas canónicas `customer-messaging`
- mantener rutas legacy `/v1/whatsapp/*`
- marcar legacy en docs y OpenAPI

### Fase 5. Completar gaps reales

- procesar `statuses` del webhook
- decidir si campañas siguen siendo WhatsApp-only o se generalizan
- consolidar métricas de mensajes por evento

## Mapa mover / dejar / extraer

### Dejar

- `pymes-core/backend/internal/whatsapp/clients.go`
- `pymes-core/backend/internal/whatsapp/inbound.go`
- migraciones `whatsapp_*`
- setup de env `WHATSAPP_*`

### Reorganizar dentro de `pymes-core`

- `handler.go`
- `usecases.go`
- `repository.go`
- `usecases/domain/entities.go`

### Extraer a `modules`

- composición visual de bandeja
- primitives de card/list/action para conversaciones

### Integrar con `core`

- event envelopes
- timeline
- inbox de avisos internos

## Estado objetivo

Al terminar la racionalización:

- `WhatsApp` deja de ser el nombre del dominio
- el dominio pasa a ser `customer messaging`
- el proveedor Meta queda encapsulado
- `wa.me` y Cloud API quedan unificados bajo una sola intención
- la UI de inbox deja de estar acoplada al proveedor
- `core` y `modules` reciben solo reutilización real, no integración prematura
