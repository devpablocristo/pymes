# Prompt 07 — Dashboard Personalizable Transversal

## Contexto

Este prompt agrega una capacidad transversal nueva a `pymes`: un **dashboard personalizable por usuario**.

La premisa de producto es esta:

- todos los perfiles del sistema deben poder tener una home configurable
- cada usuario puede armar su dashboard según sus necesidades
- el motor del dashboard es transversal y vive en `control-plane`
- los widgets y datos de negocio siguen perteneciendo a cada dominio o vertical

Este prompt NO crea "el dashboard de una vertical". Crea la **base de plataforma** para que cualquier usuario tenga un dashboard configurable y para que cada dominio publique widgets reutilizables dentro de ese marco común.

**Prerequisitos**: Prompts 00, 01, 02, 03, 04, 05 y 06 implementados y funcionales.

**Regla fundamental**: si el dashboard configurable aplica a todos los perfiles y experiencias del sistema, su motor pertenece a `control-plane`. Las verticales solo agregan widgets y fuentes de datos propias.

---

## Alcance obligatorio

Todo lo definido en este prompt forma parte del alcance requerido:

- motor transversal de dashboard configurable
- layouts por usuario
- dashboards por defecto por contexto y/o rol
- catálogo de widgets
- widgets habilitados por rol o contexto
- persistencia de layout, visibilidad, tamaño y orden
- experiencia de personalización desde frontend
- endpoints y contratos HTTP claros
- ownership bien separado entre motor y widgets
- soporte para widgets del `control-plane` y de verticales como `professionals`
- testing
- documentación

Nada de esto debe considerarse opcional salvo que el prompt lo marque explícitamente.

---

## Visión de producto

`pymes` no debe obligar a todos los usuarios a entrar siempre a la misma home estática. El sistema debe permitir que cada usuario vea primero lo que más necesita para operar.

Ejemplos:

- owner/admin: ventas, cashflow, aprobaciones, alertas
- operador comercial: oportunidades, presupuestos, clientes recientes
- profesional: agenda de hoy, sesiones pendientes, pacientes recientes
- soporte interno: tickets, auditoría, actividad reciente

La experiencia recomendada es:

```text
login -> resolver contexto del usuario -> cargar dashboard base -> aplicar personalización -> mostrar widgets habilitados
```

La regla de arranque de la v1 es simple:

- todos los usuarios empiezan desde una **configuración base por defecto**
- esa configuración base debe ser siempre la misma para el mismo contexto inicial
- esa base no depende de decisiones previas del usuario
- recién después el usuario puede mover, ocultar, agregar o redimensionar widgets

No se busca un constructor de pantallas genérico e ilimitado. Se busca una **home configurable**, consistente, segura y orientada a productividad.

---

## Decisión arquitectónica

### Qué vive en `control-plane`

`control-plane` es dueño de la capacidad transversal de dashboard:

- catálogo transversal de widgets
- layouts por usuario
- dashboards por defecto
- reglas de visibilidad por rol/contexto
- composición y render contract
- preferencias de usuario

### Qué NO vive en `control-plane`

`control-plane` NO debe absorber la lógica de negocio específica de cada vertical o módulo.

Ejemplos que NO deben quedar hardcodeados en el motor:

- cómo se calcula la agenda del profesional
- cómo se calcula un resumen clínico
- cómo se arma un KPI específico de una vertical
- cómo se consulta una lista especializada de trabajo de un dominio

### Qué vive en cada dominio o vertical

Cada dominio sigue siendo dueño de:

- los datos que alimentan sus widgets
- la lógica de negocio de esos widgets
- la metadata específica del widget si corresponde
- los endpoints de datos

Ejemplos:

- `control-plane/backend`: widgets transversales de ventas, caja, pagos, auditoría, stock, billing
- `professionals/backend`: widgets de agenda del profesional, sesiones, intakes, pacientes recientes, notas pendientes

---

## Principios obligatorios

- el dashboard configurable es una capacidad transversal
- el layout es del usuario, no del dispositivo
- el backend Go sigue siendo la fuente de verdad
- un usuario solo puede ver widgets permitidos por su rol y contexto
- las verticales no duplican el motor de dashboard
- el motor no debe conocer detalles de negocio de cada widget
- la UI debe tener un modo normal y un modo edición
- debe existir siempre un dashboard por defecto utilizable
- debe existir una configuración base inicial, estable y predecible
- la configuración base inicial debe ser siempre la misma para todos los usuarios del mismo contexto de arranque
- si no hay personalización guardada, se usa el dashboard default
- si un widget deja de estar disponible, el sistema no debe romper
- toda preferencia persistida debe estar acotada por contratos claros

---

## Objetivo funcional

El sistema debe permitir:

1. listar widgets disponibles para el usuario autenticado
2. obtener el dashboard actual del usuario
3. mover widgets
4. cambiar tamaño de widgets
5. ocultar o mostrar widgets
6. agregar widgets desde un catálogo permitido
7. resetear al dashboard por defecto
8. soportar dashboards por contexto o vista

La v1 NO necesita:

- constructor visual arbitrario de páginas
- fórmulas custom del usuario
- widgets definidos por usuarios finales
- scripting libre
- drag-and-drop entre múltiples tabs avanzadas
- marketplace complejo

Eso puede venir después. La v1 debe ser simple, sólida y productiva.

### Regla de bootstrap inicial

La v1 debe implementar este comportamiento:

1. primer acceso del usuario -> cargar una configuración base única del dashboard
2. esa configuración base debe ser consistente y repetible
3. si el usuario guarda cambios, desde ese momento se usa su layout personalizado
4. si el usuario resetea, vuelve exactamente a la configuración base definida por el sistema

No hay personalización "vacía" al inicio. Siempre existe una base lista para usar.

---

## Modelo conceptual

Separar el problema en cuatro piezas:

### 1. Definición de widget

Describe qué widget existe en la plataforma.

Incluye:

- `widget_key`
- `title`
- `description`
- `domain`
- `kind`
- `default_size`
- `min_w`
- `min_h`
- `max_w`
- `max_h`
- `supported_contexts`
- `allowed_roles`
- `data_source`
- `settings_schema`
- `status`

### 2. Layout del usuario

Describe cómo está armado el dashboard real del usuario.

Incluye por widget instanciado:

- `widget_key`
- `instance_id`
- `x`
- `y`
- `w`
- `h`
- `visible`
- `settings`
- `pinned`
- `order_hint`

### 3. Dashboard default

Plantilla recomendada según:

- contexto de navegación

### Regla de default en v1

Para simplificar la primera versión:

- definir una sola configuración base por contexto
- no variar esa base por usuario
- no variar esa base por historial del usuario
- no personalizar automáticamente la base según comportamiento

Más adelante se puede extender a presets por rol, vertical o tipo de organización, pero la v1 debe arrancar con una base única y estable.

### 4. Datos del widget

Cada widget obtiene sus datos desde un endpoint específico del dominio dueño de esa información.

El motor del dashboard no recalcula métricas de negocio.

---

## Estructura recomendada

### Backend

```text
control-plane/backend/internal/dashboard/
├── handler.go
├── usecases.go
├── repository.go
├── handler/
│   └── dto/
│       └── dto.go
├── repository/
│   └── models/
│       └── models.go
└── usecases/
    └── domain/
        └── domain.go
```

### Frontend

```text
control-plane/frontend/src/
├── dashboard/
│   ├── components/
│   ├── registry/
│   ├── widgets/
│   ├── hooks/
│   ├── types/
│   └── utils/
└── pages/
    └── DashboardPage.tsx
```

### Verticales

Las verticales no recrean el motor. Solo agregan:

- widgets visuales propios en frontend
- adaptadores de datos propios
- endpoints de datos en su backend
- registro de metadata del widget si el catálogo es federado

---

## Ownership de datos

### `control-plane` es dueño de

- `dashboard_widgets_catalog`
- `dashboard_default_layouts`
- `user_dashboard_layouts`
- `user_dashboard_preferences`

### Cada dominio/vertical es dueño de

- los datos de negocio consumidos por sus widgets
- la semántica del widget
- cualquier configuración específica del widget no genérica

### Regla clave

Un widget puede estar registrado en el catálogo transversal, pero eso NO significa que `control-plane` sea dueño de sus datos de negocio.

---

## Tablas recomendadas

### `dashboard_widgets_catalog`

Catálogo de widgets disponibles.

Campos sugeridos:

- `id`
- `widget_key` unique
- `title`
- `description`
- `domain`
- `kind`
- `default_width`
- `default_height`
- `min_width`
- `min_height`
- `max_width`
- `max_height`
- `allowed_roles_json`
- `supported_contexts_json`
- `settings_schema_json`
- `data_endpoint`
- `is_active`
- `created_at`
- `updated_at`

### `dashboard_default_layouts`

Plantillas base por contexto.

Campos sugeridos:

- `id`
- `layout_key`
- `context`
- `name`
- `items_json`
- `is_active`
- `created_at`
- `updated_at`

### `user_dashboard_layouts`

Dashboard persistido por usuario.

Campos sugeridos:

- `id`
- `user_id`
- `context`
- `layout_version`
- `items_json`
- `last_applied_default_layout_key`
- `created_at`
- `updated_at`

### `user_dashboard_preferences`

Preferencias generales.

Campos sugeridos:

- `id`
- `user_id`
- `default_context`
- `preferences_json`
- `created_at`
- `updated_at`

---

## Contratos HTTP mínimos

### Backend transversal del dashboard

#### `GET /v1/dashboard`

Devuelve el dashboard efectivo del usuario para un contexto dado.

Query params sugeridos:

- `context`

Response:

```json
{
  "context": "home",
  "layout": {
    "source": "default",
    "items": [
      {
        "instance_id": "sales-summary-1",
        "widget_key": "sales.summary",
        "x": 0,
        "y": 0,
        "w": 6,
        "h": 3,
        "visible": true,
        "settings": {}
      }
    ]
  },
  "available_widgets": [
    {
      "widget_key": "sales.summary",
      "title": "Resumen de ventas",
      "domain": "control-plane",
      "default_size": { "w": 6, "h": 3 }
    }
  ]
}
```

Regla:

- si el usuario no tiene layout persistido, `source` debe ser `default`
- si el usuario ya personalizó su dashboard, `source` debe ser `user`

#### `PUT /v1/dashboard`

Persistir layout completo del usuario.

#### `POST /v1/dashboard/reset`

Resetear al layout default del contexto.

#### `GET /v1/dashboard/widgets`

Listar catálogo disponible para el usuario.

### Endpoints de datos por widget

Cada dominio expone sus endpoints propios, por ejemplo:

- `GET /v1/dashboard-data/sales-summary`
- `GET /v1/dashboard-data/cashflow-summary`
- `GET /v1/dashboard-data/professionals/today-agenda`
- `GET /v1/dashboard-data/professionals/pending-session-notes`

El motor de dashboard no debe mezclar estas consultas dentro del mismo módulo si pertenecen a dominios distintos.

---

## Contrato frontend recomendado

El frontend debe separar:

### 1. Shell del dashboard

Responsable de:

- cargar layout
- renderizar grid
- entrar/salir de modo edición
- persistir cambios
- resetear

### 2. Registry de widgets

Mapa `widget_key -> componente`.

Ejemplo conceptual:

```ts
type DashboardWidgetRegistryItem = {
  widgetKey: string;
  component: React.ComponentType<{ instanceId: string; settings: Record<string, unknown> }>;
};
```

### 3. Widget container

Responsable de:

- borde/card común
- loading/error state
- toolbar del widget
- acciones de ocultar/remover/configurar

### 4. Data adapters

Cada widget consulta su endpoint o hook propio. No conviene forzar un único endpoint gigante para todos.

---

## Dashboard default recomendado

### Owner / admin

- ventas del día
- cashflow resumido
- pagos pendientes
- actividad reciente
- stock bajo
- métricas de facturación

### Comercial

- presupuestos abiertos
- clientes recientes
- oportunidades o follow-ups
- ventas del período

### Profesional

- agenda de hoy
- próximas sesiones
- intakes pendientes
- pacientes/clientes recientes
- notas pendientes

### Regla de UX

El usuario debe ver un dashboard útil incluso si nunca configuró nada.

### Regla adicional para v1

Ese dashboard inicial debe salir de una única configuración base definida por el sistema. Luego cada usuario puede modificarla y guardar su propia versión.

---

## Regla de permisos

El catálogo de widgets visible para un usuario debe filtrarse por:

- autenticación
- rol
- scopes
- vertical o contexto activo
- disponibilidad real del módulo

Ejemplos:

- un profesional no debería ver widgets de administración global si no tiene permiso
- un usuario comercial no debería ver widgets clínicos
- si una vertical no está habilitada para el tenant, sus widgets no deben aparecer

---

## Estrategia de implementación recomendada

Implementar en este orden:

1. modelo y tablas del dashboard
2. configuración base única por contexto
3. catálogo base de widgets
4. endpoint `GET /v1/dashboard`
5. endpoint `PUT /v1/dashboard`
6. frontend shell con grid editable
7. widgets base de `control-plane`
8. integración de widgets de `professionals`
9. reset al layout default
10. testing y documentación

### Justificación

Este orden permite:

- entregar valor temprano
- validar el contrato del motor antes de sumar widgets complejos
- integrar verticales sin reescribir la base

---

## Testing obligatorio

### Backend

- test de resolución de dashboard default
- test de aplicación de layout del usuario sobre default
- test de filtrado de widgets por rol
- test de persistencia y lectura de layout
- test de reset a default
- test de rechazo de widgets no permitidos

### Frontend

- render con dashboard default
- render con dashboard personalizado
- ocultar/mostrar widget
- persistencia al mover o redimensionar
- manejo de widget no registrado
- manejo de widget con error de datos

### Integración

- login -> dashboard
- usuario sin layout guardado -> recibe default
- usuario con layout guardado -> recibe personalizado
- usuario sin permiso para un widget -> no lo ve aunque exista en catálogo

---

## Prohibiciones explícitas

Este prompt NO debe:

- crear un dashboard distinto por vertical desde cero
- duplicar drag-and-drop, layouts o preferencias en cada frontend
- mover lógica de negocio de widgets a `control-plane` si pertenece a otro dominio
- permitir widgets arbitrarios definidos por texto libre del usuario
- usar un único mega-endpoint que mezcle todas las consultas del sistema
- romper permisos por querer simplificar la UX

---

## Resultado esperado

Al finalizar este prompt, `pymes` debe tener:

- un motor transversal de dashboard configurable en `control-plane`
- dashboards default por tipo de usuario/contexto
- persistencia por usuario
- frontend listo para personalización
- catálogo de widgets con permisos
- capacidad de integrar widgets de verticales sin duplicar el motor

En una frase:

`control-plane` debe ser dueño del **framework del dashboard**, y cada dominio debe seguir siendo dueño del **contenido de sus widgets**.
