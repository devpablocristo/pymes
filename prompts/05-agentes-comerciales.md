# Prompt 05 — Agentes Comerciales para Pymes

## Contexto

Este prompt agrega una nueva capacidad estratégica al producto: **agentes comerciales especializados** que operan dentro de `pymes` para vender mejor, asistir a usuarios internos y preparar el terreno para interacción estructurada entre agentes.

No se trata de "sumar otro chatbot". Se trata de implementar agentes de negocio **seguros, acotados por políticas, auditables y conectados al backend de `pymes` como única fuente de verdad**.

**Prerequisitos**: Prompts 00, 01, 02, 03 y 04 implementados y funcionales.

**Regla fundamental**: el agente NO define precios, stock, descuentos, aprobaciones, condiciones comerciales ni reglas operativas por su cuenta. Toda decisión sensible debe apoyarse en datos, políticas y validaciones del backend Go.

**Decisión de producto**: estos agentes forman parte de `pymes`. No son un producto separado. Pueden ejecutarse como módulos o servicios independientes dentro del mismo monorepo, pero siguen siendo una capacidad interna del sistema.

---

## Alcance obligatorio

Todo lo definido en este prompt forma parte del alcance requerido de la capacidad comercial asistida por IA:

- agente de ventas externo
- agente de ventas interno
- base del agente de compras interno
- contrato estructurado agente-a-agente
- política comercial verificable
- auditoría y trazabilidad
- seguridad y guardrails
- testing
- documentación

Nada de esto debe interpretarse como "nice to have" salvo que el prompt lo marque de forma explícita.

---

## Visión del producto

`pymes` debe evolucionar desde un sistema de gestión con asistencia conversacional a una plataforma que también pueda:

1. **Atender oportunidades comerciales externas**
2. **Asistir al equipo de ventas interno**
3. **Preparar compras y abastecimiento**
4. **Hablar con otros agentes bajo contratos estructurados**

La visión recomendada es esta:

```text
humano -> agente comprador externo -> interfaz comercial controlada de pymes -> backend de pymes
```

No se recomienda como primera implementación:

```text
humano -> LLM comprador -> LLM vendedor -> tools libres -> sistema
```

Porque eso aumenta riesgo operativo, costo, latencia y falta de control.

---

## Decisión arquitectónica

### El agente debe ser parte de `pymes`

Como producto, el agente comercial debe vivir dentro de `pymes` porque depende de:

- catálogo
- clientes
- productos
- precios
- stock
- presupuestos
- ventas
- pagos
- turnos
- políticas del tenant
- permisos por rol
- auditoría

Separarlo como producto independiente demasiado temprano obligaría a duplicar:

- autenticación
- autorización
- sincronización de datos
- políticas de negocio
- contratos de integración
- observabilidad
- auditoría

### Implementación recomendada

Elegir la mejor variante según el estado real del repo:

1. **Extender `ai/`** si la orquestación actual alcanza
2. **Crear submódulos especializados dentro de `ai/`** para `sales_agent`, `procurement_agent`, `policy_layer`, etc.
3. **Crear un servicio adicional dentro del monorepo** solo si la separación operativa realmente lo justifica

La preferencia por defecto es **reutilizar `ai/`** y extraer responsabilidades por módulo antes de crear nuevos servicios.

---

## Principios obligatorios

- `pymes` sigue siendo el producto principal
- el backend Go sigue siendo la única fuente de verdad
- el LLM nunca define la verdad de negocio
- no se permiten compromisos comerciales fuera de política
- las políticas críticas viven en backend o en una capa verificable
- toda acción sensible debe quedar auditada
- las respuestas deben ser claras, profesionales y en español
- las acciones de escritura requieren confirmación o una política explícita
- no exponer información interna a canales externos
- no permitir negociación libre no estructurada entre agentes para compromisos reales

---

## Orden de implementación recomendado

Implementar en este orden:

1. agente de ventas externo
2. agente de ventas interno
3. agente de compras interno
4. contrato estructurado agente-a-agente
5. interacción controlada entre agente comprador externo y agente vendedor de `pymes`

### Justificación

El agente de ventas entrega valor más rápido porque:

- ya existe canal externo
- ya existen tools de ventas
- ya existen pagos y links de cobro
- ya existen presupuestos, clientes, productos y turnos

El agente de compras debe venir después porque requiere mayor madurez en:

- abastecimiento
- proveedores
- reposición
- lead time
- mínimos de compra
- aprobaciones internas

---

## 1. Agente de ventas externo

### Objetivo

Atender clientes externos o agentes compradores externos mediante un canal controlado, sin exponer datos internos del negocio.

### Capacidades obligatorias

- responder preguntas comerciales
- mostrar información pública del negocio
- listar servicios o productos públicos
- consultar disponibilidad
- orientar al cliente hacia la mejor opción
- preparar una cotización o presupuesto controlado
- iniciar reserva o intención de compra
- entregar link de pago cuando aplique
- escalar a humano cuando no tenga autorización o contexto suficiente

### Restricciones obligatorias

- no exponer información financiera interna
- no exponer deuda de clientes
- no exponer márgenes
- no exponer datos de terceros
- no prometer descuentos fuera de política
- no comprometer stock sin validación
- no cerrar operaciones complejas sin confirmación o policy layer

### Canales posibles

- chat web público
- WhatsApp
- embebido en landing o micrositio
- API comercial para agentes externos

### Herramientas permitidas

El agente externo debe operar con un allowlist reducido. Ejemplos:

- `get_business_info`
- `get_public_services`
- `check_availability`
- `book_appointment`
- `get_my_appointments`
- `get_payment_link`
- `get_public_quote`
- `create_lead`
- `request_quote`

### Lo que NO debe hacer

- modificar precios
- crear ventas definitivas sin reglas claras
- consultar datos internos del tenant
- navegar libremente por herramientas internas

---

## 2. Agente de ventas interno

### Objetivo

Asistir a usuarios internos con rol comercial para vender más rápido, con mejor contexto y sin violar políticas del negocio.

### Capacidades obligatorias

- buscar clientes
- buscar productos
- consultar stock
- consultar presupuestos
- crear presupuestos
- preparar ventas
- generar links de pago
- consultar estado de cobro
- sugerir upsell y cross-sell
- resumir actividad comercial

### Restricciones obligatorias

- respetar permisos por rol
- respetar módulos activos del tenant
- confirmar antes de acciones de escritura si la política lo exige
- no saltar validaciones del backend

### Roles y perfiles

Debe soportar al menos:

- `admin`
- `vendedor`
- `cajero`
- otros perfiles comerciales que existan en el sistema

La disponibilidad de tools debe depender de:

- rol
- módulos activos
- modo del canal
- política comercial

### Casos de uso obligatorios

- "armame un presupuesto para este cliente"
- "mostrame qué productos tengo para ofrecer"
- "qué puedo vender hoy con stock disponible"
- "generame el link de pago"
- "qué ventas tengo pendientes de cobro"

---

## 3. Agente de compras interno

### Objetivo

Asistir en abastecimiento, reposición y análisis de compras sin automatizar decisiones peligrosas en la primera etapa.

### Capacidades obligatorias

- detectar stock bajo
- sugerir reposición
- resumir riesgo de quiebre
- consultar compras recientes
- consultar proveedores
- comparar opciones
- preparar borradores de órdenes de compra
- sugerir cantidades recomendadas

### Restricciones obligatorias

- no emitir compras finales automáticamente en MVP
- no negociar libremente con proveedores en la primera etapa
- no asumir costos, plazos ni condiciones no verificadas

### Casos de uso obligatorios

- "qué tengo que reponer esta semana"
- "qué proveedor conviene para este producto"
- "preparame un borrador de orden de compra"
- "qué productos están por quedarse sin stock"

### Resultado esperado en MVP

Aunque no quede completo como ventas, debe quedar:

- diseñado
- con base de arquitectura
- con tools o adapters identificados
- con puntos de extensión listos

---

## 4. Contrato estructurado agente-a-agente

### Regla de diseño

La interacción entre agentes NO debe basarse en texto libre como contrato principal.

Debe existir un protocolo estructurado, validable y auditable.

### Intents mínimos

Implementar o diseñar mensajes estructurados como:

- `request_quote`
- `quote_response`
- `counter_offer`
- `offer_acceptance`
- `offer_rejection`
- `availability_request`
- `availability_response`
- `payment_request`
- `reservation_request`

### Campos mínimos

Todo mensaje agente-a-agente debe poder representar:

- `request_id`
- `org_id`
- `counterparty_id`
- `intent`
- `items`
- `quantities`
- `currency`
- `price_terms`
- `payment_terms`
- `delivery_terms`
- `valid_until`
- `metadata`
- `signature` o equivalente de autenticidad si aplica

### Validaciones obligatorias

- esquema estricto
- campos requeridos
- idempotencia
- timestamps
- replay protection si aplica
- validación de permisos
- validación de canal

### Regla operativa

El texto libre puede existir como UX o explicación, pero **la acción comprometible debe basarse en payload estructurado**.

---

## 5. Capa de políticas comerciales

### Problema

Un LLM no debe tener libertad para decidir:

- precios
- descuentos
- disponibilidad
- stock reservado
- condiciones de pago
- aprobación de operaciones

### Solución obligatoria

Crear una **policy layer** verificable, reutilizable y auditable.

### La policy layer debe decidir

- qué herramientas puede usar cada agente
- qué acciones requieren confirmación humana
- qué descuentos están permitidos
- si un presupuesto puede emitirse
- si una venta puede confirmarse
- si un link de pago puede generarse
- si una reserva puede tomarse
- qué datos se pueden mostrar según canal y rol

### Ubicación recomendada

Puede vivir en:

- backend Go
- capa interna del servicio AI
- adapters de autorización/política especializados

Pero la regla es: **debe ser lógica verificable en código, no solo una instrucción en prompt**.

---

## Seguridad y guardrails

### Protección obligatoria

- prompt injection desde usuarios externos
- prompt injection desde agentes externos
- fuga de datos internos
- uso indebido de tools
- bucles de tool-calling
- escalamiento de privilegios
- escrituras no confirmadas
- payloads inválidos de agente-a-agente

### Medidas mínimas

- allowlist de tools por modo
- allowlist de tools por rol
- allowlist de tools por canal
- límite de cantidad de tool calls
- timeout por tool
- timeout total por interacción
- sanitización de inputs
- separación fuerte entre external/internal
- auditoría de acciones sensibles

### Canales

Los canales externos deben ser siempre más restrictivos que los internos.

---

## Auditoría y trazabilidad

Toda acción relevante del agente debe registrar:

- quién inició la interacción
- canal
- org_id
- agente o modo utilizado
- tools ejecutadas
- resultado
- entidad afectada
- decisión automática o confirmada por humano

### Casos obligatorios de auditoría

- generación de presupuesto
- creación de venta
- generación de link de pago
- reserva de turno
- recomendación aprobada de compra
- cualquier decisión comercial comprometible

---

## UX y comportamiento

### Reglas obligatorias

- responder siempre en español
- lenguaje simple, profesional y directo
- no mostrar JSON al usuario final
- si no puede ejecutar algo, explicarlo con claridad
- si falta autorización, escalar o pedir intervención humana
- si la acción requiere confirmación, pedirla explícitamente

### Estilo recomendado

- útil antes que creativo
- claro antes que verboso
- comercial pero no invasivo
- nunca engañoso

---

## Integración con el código existente

Antes de crear nuevas capas, auditar y reutilizar:

- `ai/`
- `control-plane/backend`
- endpoints públicos ya existentes
- tool registry existente
- auth context actual
- circuit breaker
- quotas
- observability
- audit log
- módulos de pagos, ventas, presupuestos, inventario, turnos, compras y cuentas

No construir una plataforma abstracta sobrediseñada si el código actual ya resuelve parte del problema.

---

## MVP obligatorio

### MVP 1 — Agente de ventas externo

Debe incluir:

- endpoint o flujo de chat comercial externo
- allowlist segura de tools
- consulta de productos/servicios públicos
- consulta de disponibilidad
- capacidad de orientar al cliente
- posibilidad de preparar cotización o presupuesto controlado
- generación o entrega de link de pago si aplica
- fallback claro y seguro

### MVP 2 — Agente de ventas interno

Debe incluir:

- integración con rol comercial
- búsqueda de clientes
- búsqueda de productos
- consulta de stock
- creación de presupuesto
- preparación de venta
- link de pago
- consulta de estado de cobro

### MVP 3 — Base del agente de compras interno

Debe incluir al menos:

- diseño listo
- entry points claros
- tools de análisis o consulta mínimas
- puntos de extensión para proveedores, oferta y orden de compra

---

## Tests obligatorios

Agregar pruebas para:

- permisos por rol
- restricciones por modo `internal` vs `external`
- allowlist de tools
- confirmación previa a escritura
- timeouts
- máximo de tool calls
- errores del backend
- respuestas vacías del modelo
- payloads inválidos del contrato agente-a-agente
- exposición indebida de datos internos
- auditoría mínima de acciones sensibles

### Tipos de prueba recomendados

- unit tests
- integration tests
- tests de políticas
- tests de seguridad
- tests de contract/schema

---

## Criterios de éxito

El trabajo se considera exitoso si:

- el agente de ventas externo puede asistir comercialmente sin exponer información interna
- el agente de ventas interno acelera tareas comerciales reales
- el backend sigue siendo la única fuente de verdad
- no hay acciones comprometibles fuera de política
- la arquitectura queda lista para sumar agente de compras
- la arquitectura queda lista para interacción agente-a-agente estructurada
- la solución es auditable, extensible y mantenible
- todo queda validado con tests y chequeos reales

---

## Orden sugerido de ejecución

1. auditar `ai/`, `control-plane/backend` y endpoints públicos existentes
2. definir arquitectura final
3. implementar MVP del agente de ventas externo
4. implementar MVP del agente de ventas interno
5. dejar base del agente de compras interno
6. definir e implementar contrato estructurado agente-a-agente
7. agregar tests
8. documentar
9. validar con pruebas reales

---

## Output esperado del trabajo

Quiero que quien implemente este prompt entregue:

- resumen de arquitectura elegida
- justificación de por qué vive dentro de `pymes`
- cambios implementados
- tools y endpoints reutilizados o agregados
- riesgos detectados
- guardrails aplicados
- tests ejecutados
- estado del MVP
- próximos pasos concretos

---

## Nota final

La prioridad no es crear una "demo llamativa" de agentes hablando entre sí.

La prioridad es construir una capacidad comercial real para `pymes`:

- útil
- segura
- trazable
- incremental
- alineada con el negocio

Primero resolver bien ventas.
Después compras.
Finalmente interacción estructurada entre agentes.
