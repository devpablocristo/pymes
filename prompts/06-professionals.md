# Prompt 06 — Vertical Professionals

## Contexto

Este prompt crea la primera vertical autónoma del ecosistema `pymes`: `professionals`.

La vertical vive en el mismo monorepo, pero separada de `control-plane`:

```text
professionals/
├── backend/
├── frontend/
├── infra/
└── ai/
```

La decisión de arquitectura es doble:

1. `professionals` es una vertical autónoma por servicio/Lambda
2. `professionals` NO puede repetir ninguna capacidad que ya exista en `control-plane`

**Prerequisitos**: Prompts 00, 01, 02, 03, 04 y 05 implementados y funcionales.

**Regla fundamental**: la separación por servicio no habilita duplicación de dominio. Si una capacidad ya existe en `control-plane`, la vertical la consume por HTTP y no la vuelve a modelar localmente.

**Regla de runtime**: todo backend de este repo se diseña como Lambda desde el inicio. Eso incluye `control-plane/backend`, `professionals/backend` y cualquier otro backend futuro. El mismo criterio aplica al servicio de AI cuando se despliega como backend de la plataforma.

---

## Alcance obligatorio

Todo lo definido en este prompt forma parte del alcance requerido para `professionals`:

- backend vertical propio
- frontend vertical propio
- infra propia
- AI propia de la vertical
- contratos HTTP claros con `control-plane`
- ownership de datos solo sobre el delta vertical
- perfiles profesionales
- especialidades
- relación profesional <-> servicios del core
- intake y notas operativas
- sesiones/atenciones
- experiencia pública y privada específica de la vertical
- observabilidad, testing y documentación

Nada de esto debe interpretarse como opcional. La vertical debe nacer bien separada, pero apoyada sobre la base transversal ya existente.

---

## Qué es `professionals`

`professionals` es una vertical para prestadores de servicios individuales o microestudios cuyo negocio gira alrededor de:

- agenda y turnos
- clientes
- servicios vendibles
- intake previo
- seguimiento operativo liviano
- registro de atención o sesión
- cobro por servicio

Ejemplos que sí entran:

- psicólogos
- coaches
- consultores
- nutricionistas independientes
- entrenadores personales
- profesores particulares
- abogados independientes
- contadores independientes
- freelancers de servicios profesionales

Ejemplos que no entran en este prompt:

- clínicas u hospitales
- educación formal
- talleres mecánicos
- retail o kioscos
- operaciones con inventario/logística pesada

La vertical debe ser concreta: amplia en casos de uso, pero sin transformarse en un segundo core transversal.

---

## Regla central: no repetir base transversal

Antes de crear cualquier tabla, endpoint, módulo o tool en `professionals`, hay que clasificar la capacidad en una de estas dos categorías:

### 1. Reutiliza desde `control-plane`

Esto ya existe o debe existir en la base transversal, por lo tanto la vertical lo consume por HTTP:

- organizaciones y tenant settings
- auth e identidad
- API keys
- users y memberships
- party model
- customers
- products como catálogo de productos y servicios vendibles
- appointments como motor de agenda, disponibilidad y reservas
- quotes
- sales
- payments
- WhatsApp
- notificaciones base
- billing
- admin
- RBAC/base de permisos
- runtime/base común de AI

### 2. Crea nuevo en `professionals`

Esto sí pertenece al dominio específico de la vertical:

- perfil profesional como extensión sobre `party`
- especialidades
- relación entre profesional y servicios del catálogo transversal
- configuración vertical de atención
- intake previo a una atención
- registro de sesión/atención
- notas internas de sesión
- UX pública y privada específica
- prompts, policies y tools específicas de la vertical

### Prohibiciones explícitas

Este prompt NO debe crear en `professionals`:

- un catálogo paralelo a `products`
- un motor propio de disponibilidad paralelo a `appointments`
- una tabla propia de bookings/reservas si la reserva real vive en `appointments`
- customers paralelos
- quotes o sales paralelos
- payment links paralelos
- auth paralela
- otra implementación base de AI que replique la ya existente

---

## Objetivo de producto

La vertical debe permitir que un profesional o microestudio opere desde una experiencia enfocada, sin exponer toda la complejidad del producto transversal:

1. presentar profesionales y especialidades
2. mostrar servicios ya existentes del catálogo base, pero curados para la vertical
3. permitir reservar usando el motor de turnos del core
4. capturar intake previo cuando aplique
5. registrar sesiones/atenciones realizadas
6. agregar notas operativas internas
7. emitir presupuestos o links de pago usando la base transversal
8. ofrecer asistencia AI específica del rubro

La vertical agrega contexto y flujo. No reemplaza las capacidades base del sistema.

---

## Decisión arquitectónica

### Estructura objetivo

```text
pymes/
├── ai/
├── control-plane/
│   ├── backend/
│   ├── infra/
│   └── shared/
├── professionals/
│   ├── backend/
│   └── infra/
├── pkgs/
└── go.work
```

### Responsabilidad por componente

- `professionals/backend`: API vertical, composición de datos verticales y consumo del core
- `frontend`: experiencia web unificada con módulos específicos de la vertical
- `professionals/infra`: despliegue de Lambdas, frontend, variables, secretos, monitoreo
- `ai`: runtime conversacional unificado con módulo específico para `professionals`

### Frontera con `control-plane`

`control-plane` conserva ownership de:

- identidad y autenticación
- `party`
- `customers`
- `products`
- `appointments`
- `quotes`
- `sales`
- `payments`
- `whatsapp`
- notificaciones
- settings de organización
- billing
- administración
- RBAC y capacidades base de plataforma

`professionals` conserva ownership de:

- `professional_profiles`
- `specialties`
- `professional_service_links`
- `intakes`
- `sessions`
- `session_notes`
- presentaciones, listados y flujos UX de la vertical
- prompts, tools y policy específica de la vertical dentro del `ai/` unificado

---

## Ownership de datos

### Regla obligatoria

Cada servicio es dueño solo de sus tablas. No se permiten escrituras directas sobre tablas ajenas.

### Recomendación inicial

Usar la misma instancia PostgreSQL en una primera etapa, pero con separación lógica:

- `control_plane.*` para datos transversales
- `professionals.*` para datos verticales

`professionals` puede referenciar IDs del core, pero no asumir ownership sobre esas entidades.

### Tablas mínimas de la vertical

```sql
CREATE SCHEMA IF NOT EXISTS professionals;

CREATE TABLE professionals.professional_profiles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    party_id uuid NOT NULL,
    public_slug text NOT NULL,
    bio text NOT NULL DEFAULT '',
    headline text NOT NULL DEFAULT '',
    is_public boolean NOT NULL DEFAULT true,
    is_bookable boolean NOT NULL DEFAULT true,
    accepts_new_clients boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, party_id),
    UNIQUE (org_id, public_slug)
);

CREATE TABLE professionals.specialties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    code text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, code),
    UNIQUE (org_id, name)
);

CREATE TABLE professionals.professional_specialties (
    profile_id uuid NOT NULL,
    specialty_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, specialty_id)
);

CREATE TABLE professionals.professional_service_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    profile_id uuid NOT NULL,
    product_id uuid NOT NULL,
    public_description text NOT NULL DEFAULT '',
    display_order integer NOT NULL DEFAULT 0,
    is_featured boolean NOT NULL DEFAULT false,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, profile_id, product_id)
);

CREATE TABLE professionals.intakes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    appointment_id uuid,
    profile_id uuid NOT NULL,
    customer_party_id uuid,
    product_id uuid,
    status text NOT NULL DEFAULT 'draft',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE professionals.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    appointment_id uuid NOT NULL,
    profile_id uuid NOT NULL,
    customer_party_id uuid,
    product_id uuid,
    status text NOT NULL DEFAULT 'completed',
    started_at timestamptz,
    ended_at timestamptz,
    summary text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, appointment_id)
);

CREATE TABLE professionals.session_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    session_id uuid NOT NULL,
    note_type text NOT NULL DEFAULT 'internal',
    title text NOT NULL DEFAULT '',
    body text NOT NULL,
    created_by text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
```

### Interpretación obligatoria

- `party_id` sigue siendo la identidad del profesional
- `product_id` sigue apuntando al catálogo de `control-plane`
- `appointment_id` sigue apuntando al motor de turnos del core
- `customer_party_id` sigue siendo party/customer del core

La vertical guarda solamente su contexto extra.

---

## Backend — `professionals/backend`

### Objetivo

Exponer la API vertical y orquestar el cruce entre:

- datos propios de `professionals`
- capacidades transversales de `control-plane`

### Stack recomendado

| Capa | Tecnología |
|------|-----------|
| Lenguaje | Go 1.24 |
| HTTP | Gin |
| Persistencia | GORM + PostgreSQL |
| Migraciones | golang-migrate |
| Runtime | AWS Lambda |
| Integración | HTTP con `control-plane` |

### Regla de runtime

`professionals/backend` se implementa como Lambda:

```text
professionals/backend/
├── cmd/
│   ├── lambda/
│   │   └── main.go
│   └── local/
│       └── main.go
```

No se diseña como proceso long-running asumido ni como módulo embebido dentro de otro backend.

### Estructura sugerida

```text
professionals/backend/
├── cmd/
│   ├── lambda/
│   └── local/
├── internal/
│   ├── professional_profiles/
│   ├── specialties/
│   ├── service_links/
│   ├── intakes/
│   ├── sessions/
│   ├── public/
│   └── shared/
│       ├── app/
│       ├── config/
│       ├── store/
│       ├── handlers/
│       ├── controlplane/
│       └── audit/
├── migrations/
├── wire/
├── go.mod
└── go.sum
```

### Módulos obligatorios

#### `professional_profiles`

Extensión vertical sobre `party`:

- bio
- headline
- slug público
- flags de visibilidad
- flags de disponibilidad comercial

#### `specialties`

Especialidades de la vertical:

- psicología
- coaching
- nutrición
- inglés
- consultoría

#### `service_links`

Relación entre perfil profesional y `products` del core:

- qué servicios ofrece cada profesional
- orden de visualización
- descripción pública específica de la vertical
- servicios destacados

No crea productos. Solo vincula y enriquece productos ya existentes.

#### `intakes`

Ficha previa a la atención:

- motivos de consulta
- información operativa previa
- respuestas de formulario
- consentimiento operativo simple

No reemplaza historia clínica ni expediente regulado.

#### `sessions`

Registro de atención efectivamente realizada:

- inicio/fin
- estado
- resumen operativo
- notas internas

### Endpoints internos autenticados

```text
GET    /v1/professionals
POST   /v1/professionals
GET    /v1/professionals/:id
PUT    /v1/professionals/:id

GET    /v1/specialties
POST   /v1/specialties
PUT    /v1/specialties/:id

GET    /v1/professionals/:id/services
PUT    /v1/professionals/:id/services

GET    /v1/intakes/:id
POST   /v1/intakes
PUT    /v1/intakes/:id
POST   /v1/intakes/:id/submit

GET    /v1/sessions
POST   /v1/sessions
GET    /v1/sessions/:id
POST   /v1/sessions/:id/complete
POST   /v1/sessions/:id/notes
```

### Endpoints públicos o verticales de fachada

La vertical puede exponer endpoints propios para UX y AI, pero solo como composición sobre el core:

```text
GET /v1/public/:org_slug/professionals
GET /v1/public/:org_slug/professionals/:slug
GET /v1/public/:org_slug/catalog
```

Si expone disponibilidad o reserva en endpoints propios, esos endpoints deben delegar al motor real de `appointments` en `control-plane`. No se permite persistir una reserva paralela en tablas locales.

### Reglas de negocio obligatorias

- toda tabla propia es multi-tenant por `org_id`
- no se duplica identidad: el profesional real vive en `party`
- no se duplica catálogo: el servicio vendible real vive en `products`
- no se duplica agenda: la reserva real vive en `appointments`
- un `session` puede referenciar un único `appointment_id`
- el intake no reemplaza customer ni party del core
- toda operación sensible genera evento de auditoría

---

## Contratos HTTP con `control-plane`

### Regla obligatoria

La comunicación entre servicios se hace por clientes HTTP propios y contratos explícitos.

### Capacidades mínimas a consumir

`professionals/backend` necesita poder consumir:

- bootstrap de organización y usuario
- settings transversales
- lookup de `party` y `customers`
- lookup de `products`
- creación/consulta de `appointments`
- creación/consulta de `quotes`
- creación/consulta de `sales`
- creación/consulta de `payment-links`
- notificaciones o canales transversales si el core los expone

### Cliente interno sugerido

```text
internal/shared/controlplane/
├── client.go
├── auth.go
├── organizations.go
├── parties.go
├── customers.go
├── products.go
├── appointments.go
├── quotes.go
├── sales.go
├── payments.go
└── notifications.go
```

### Principios para los contratos

- timeout corto por request
- retries solo para lecturas idempotentes
- idempotency key en escrituras remotas
- `request_id` propagado entre servicios
- errores tipados y mapeados
- autenticación servicio-a-servicio con token interno
- no consumir endpoints pensados solo para UI

### Contratos esperados

```text
GET  /internal/v1/orgs/:org_id/bootstrap
GET  /internal/v1/orgs/:org_id/settings
GET  /internal/v1/parties/:party_id
POST /internal/v1/customers/resolve
GET  /internal/v1/products
POST /internal/v1/appointments
GET  /internal/v1/appointments/:id
POST /internal/v1/quotes
POST /internal/v1/sales
POST /internal/v1/payment-links
GET  /internal/v1/payment-links/:id
```

Si un contrato no existe todavía, debe crearse en `control-plane` como API interna estable. No debe "simularse" localmente dentro de la vertical.

---

## Frontend — `professionals/frontend`

### Objetivo

Entregar una experiencia específica para la vertical, pero apoyada sobre la base ya existente.

### Stack recomendado

| Capa | Tecnología |
|------|-----------|
| Framework | React 18 |
| Lenguaje | TypeScript |
| Bundler | Vite |
| Estado async | TanStack Query |
| Auth | Clerk |
| HTTP | cliente común desde `pkgs/ts-pkg` |

### Principio de diseño

`professionals/frontend` no debe reconstruir en paralelo:

- billing
- admin
- customers genéricos
- catálogo genérico
- pagos genéricos
- reportes genéricos

Eso sigue siendo responsabilidad de la base o se consume desde ella.

La vertical debe enfocarse en:

- perfil profesional
- especialidades
- oferta visible por profesional
- intake
- sesiones
- agenda del día consumiendo turnos del core
- experiencia pública de consulta/reserva

### Pantallas privadas mínimas

- dashboard de agenda del día
- perfiles profesionales
- especialidades
- asignación de servicios del catálogo base
- intake pendientes
- sesiones realizadas
- notas operativas

### Pantallas públicas mínimas

- landing del profesional o estudio
- listado de profesionales
- catálogo vertical curado a partir de `products`
- acceso a reserva usando el motor del core
- intake previo cuando corresponda
- confirmación y próximos pasos

### Regla de UX

Si una pantalla es transversal, no se duplica en la vertical. Se linkea, se embebe o se consume desde la plataforma base.

---

## AI — módulo `professionals` dentro de `ai/`

### Objetivo

Tener una AI específica de la vertical sin volver a crear otra app de AI separada.

### Regla fundamental

El módulo de `professionals` dentro de `ai/` no debe reimplementar desde cero:

- auth
- rate limiting base
- observabilidad base
- contrato general de tools
- capas comunes de policy/guardrails
- primitives comunes del runtime AI

Si alguna de esas piezas hoy vive solo en un módulo puntual del `ai/` unificado, primero debe extraerse a una capa compartida antes de copiarla.

### Flujo recomendado

```text
frontend -> ai(professionals) -> professionals/backend -> control-plane/backend
```

### Ownership real de la AI vertical

La AI de `professionals` sí es dueña de:

- prompts específicos del rubro
- tools que leen/escriben `intakes`
- tools que leen/escriben `sessions`
- tools que consultan perfiles y especialidades
- composición conversacional sobre appointments/products/quotes/payments del core
- políticas de lenguaje y escalamiento propias de la vertical

### Tools mínimas

- `get_professional_profiles`
- `get_professional_catalog`
- `create_or_update_intake`
- `get_session_summary`
- `get_today_schedule`
- `book_appointment` delegando al core a través del backend vertical
- `prepare_quote` delegando al core a través del backend vertical
- `get_payment_link` delegando al core a través del backend vertical

### Restricciones obligatorias

- no diagnosticar
- no actuar como sustituto profesional
- no prometer resultados
- no inventar precios ni disponibilidad
- no crear reservas fuera del motor de appointments
- no exponer información privada de otros clientes

---

## Infra — `professionals/infra`

### Objetivo

Desplegar la vertical con independencia operativa real.

### Responsabilidades

- Lambda de `professionals/backend`
- configuración del módulo de `professionals` dentro del `ai/` unificado
- frontend estático
- API Gateway propio o rutas dedicadas
- secretos y variables de entorno
- monitoreo y alarmas
- despliegue desacoplado de `control-plane`

### Estructura sugerida

```text
professionals/infra/
├── main.tf
├── variables.tf
├── outputs.tf
├── terraform.tfvars.example
└── modules/
```

### Requisitos mínimos

- despliegue independiente por servicio
- configuración separada
- URLs claras para backend y AI
- secreto de comunicación interna con `control-plane`
- logs y métricas separados
- tagging por vertical y entorno

---

## Auth y multi-tenant

### Regla obligatoria

La experiencia debe ser coherente con la plataforma, aunque los servicios estén separados.

### Requisitos

- misma autoridad de identidad
- JWTs válidos en toda la plataforma
- soporte de API keys cuando aplique
- `org_id` siempre resuelto y validado
- auditoría con actor real o actor servicio
- autorización por rol/permiso antes de ejecutar acciones internas

### Interpretación correcta

`professionals` no crea auth nueva. Consume auth de plataforma.

Si una pieza compartida es necesaria para validar JWT/API key en varios servicios, debe extraerse a `pkgs/` o a la infraestructura compartida. No debe copiarse manualmente servicio por servicio.

---

## Observabilidad

Todo componente de `professionals` debe incluir:

- logging estructurado
- `request_id`
- `org_id`
- `service_name`
- latencia por endpoint
- métricas de intakes y sesiones
- métricas de uso de appointments consumidos desde el core
- tracing entre `professionals` y `control-plane`

### KPIs iniciales

- reservas consumidas por día
- ratio de confirmación
- ratio de cancelación
- sesiones completadas
- sesiones con intake previo
- conversión de reserva a pago cuando aplique

---

## Testing obligatorio

### Backend

- unit tests table-driven para reglas de perfiles, intakes y sessions
- tests de cliente HTTP a `control-plane` con mocks
- tests que validen que no existe persistencia paralela de appointments/products/customers

### Frontend

- tests de componentes clave
- tests del flujo público vertical
- tests de intake

### AI

- tests de tools verticales
- tests de policy/guardrails
- tests de composición con dependencias del core mockeadas

### Integración

- flujo E2E de reserva pública usando `appointments` del core
- flujo interno de creación de sesión
- flujo de preparación de presupuesto o link de pago
- validación de comunicación `professionals -> control-plane`

---

## Orden de implementación recomendado

Implementar en este orden:

1. contratos HTTP faltantes en `control-plane`
2. `professionals/backend`
3. `professionals/frontend`
4. módulo `professionals` dentro de `ai/`
5. `professionals/infra`

### Justificación

Antes de construir la vertical, hay que asegurar que la base transversal expone lo necesario por contrato. La vertical no debe "inventar localmente" capacidades que el core todavía no abrió correctamente.

---

## MVP recomendado

El MVP de la vertical debe incluir:

- perfiles profesionales
- especialidades
- asignación de servicios del catálogo base
- consumo del motor de appointments del core
- intake básico
- registro de sesión
- notas internas de sesión
- integración con quotes y links de pago del core
- AI pública y privada especializada

No incluir en MVP:

- catálogo paralelo de servicios
- motor propio de agenda
- customers paralelos
- reservas paralelas
- pagos paralelos
- historia clínica
- expedientes regulados

---

## Resultado esperado

Al finalizar este prompt, `pymes` debe tener:

- una primera vertical autónoma por servicio/Lambda
- una vertical que no duplica capacidades ya resueltas en `control-plane`
- contratos HTTP claros entre plataforma transversal y dominio vertical
- una experiencia usable para profesionales independientes y microestudios
- un patrón replicable para futuras verticales hermanas

`professionals` debe ser la primera vertical de referencia, no un pseudo-core alternativo ni un segundo backend generalista.
