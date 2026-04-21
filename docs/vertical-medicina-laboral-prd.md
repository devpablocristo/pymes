# PRD — Vertical medicina / higiene laboral (ocupational health)

Documento de referencia para diseñar e implementar una **versión nueva** dentro del monorepo Pymes: mismos tipos de proceso de negocio que un sistema clásico de “sistema laboral” / QMS, **sin copiar** código, datos ni interfaz de terceros.

---

## 1. Objetivo

- Ofrecer a clientes PyME y prestadores una **consola** para gestionar **trabajadores**, **turnos**, **exámenes y prestaciones**, **ausentismos**, **accidentes** e **historia clínica laboral**, con **permisos por rol**, **trazabilidad** y **capacidades de IA** acopladas al servicio `ai/` (FastAPI).
- Integrarse con las reglas del repo: **hexagonal en Go**, **sin duplicar** lo que ya cubre `pymes-core`, **UI en español**, **código en inglés**.

---

## 2. Principios

1. **Greenfield**: modelo de datos y APIs propios; el menú de un sistema legado solo informa **requisitos funcionales**.
2. **Confidencialidad**: datos de salud laboral y personales — auditoría de accesos, minimización de datos hacia IA, retención acordada con el cliente.
3. **Multi-tenant**: un tenant = una organización cliente (p. ej. prestador); bajo ese tenant pueden existir **empresas contratantes** y **trabajadores** según el modelo que se defina en descubrimiento.

---

## 3. Inventario funcional (módulos de referencia)

Agrupación lógica para priorizar MVP vs fases posteriores.

### 3.1 Accesos / operación diaria

| Área | Notas |
|------|--------|
| Ausentismos | Registro y seguimiento de faltas / licencias según reglas de negocio. |
| Exámenes | Exámenes médicos u ocupacionales (ingreso, periódicos, egreso, etc.). |
| Asistencias médicas | Atenciones / episodios vinculados a agenda. |
| Prestaciones | Servicios o prestaciones otorgadas (según definición legal/operativa). |
| Accidentes | Registro de incidentes / accidentes laborales. |
| Trabajadores | Personas en relación de dependencia o equivalente bajo empresas cliente. |
| Historia clínica laboral | Expediente unificado por trabajador (documentos + evolución). |

### 3.2 Turnos

| Función | Notas |
|---------|--------|
| Reservar turnos | Alta de cita; puede alinearse con **appointments** de `pymes-core` si el contrato encaja. |
| Listar turnos | Consultas y filtros. |
| Control de turnos | Operativa del día (presente, cancelado, etc.). |
| Configurar turnos | Reglas, agendas, recursos (profesionales, boxes). |

### 3.3 Solicitudes

| Función | Notas |
|---------|--------|
| Control de ausentismo | Flujo solicitud → validación → registro. |
| Exámenes (solicitudes) | Pedido de examen diferenciado del “acceso directo” operativo. |

### 3.4 Cuenta y soporte

| Función | Notas |
|---------|--------|
| Datos personales / contraseña | Preferentemente **reutilizar** flujos de identidad y ajustes de `pymes-core` / frontend común. |
| Ayuda | FAQs, novedades; **auditoría** y **contabilidad** como informes o integraciones — no duplicar billing de `pymes-core` sin análisis explícito. |

### 3.5 Navegación tipo “módulos grandes”

- **Inicio**: dashboard y avisos legales / términos.
- **Registros**: vista operativa de altas y movimientos recientes (definir alcance en MVP).
- **Bases de datos**: mantenimiento maestro (empresas, obras, puestos, nomencladores).
- **Informes**: exportaciones y reportes; respetar RBAC y anti-fraude donde aplique.

---

## 4. Reutilización desde `pymes-core` vs código nuevo

Cada implementación debe listar explícitamente (convención del repo):

### 4.1 Reutilizar (no reimplementar)

- Autenticación, organización / tenant, bootstrap de org y **RBAC** según documentación vigente.
- **Citas / appointments** si el modelo coincide con turnos (validar con dominio).
- **Clientes / terceros** si “empresa contratante” encaja en el party model existente.
- **Facturación / cobros** solo si el vertical no redefine cobro propio; revisar `pymes-core` y documentación de fraude/auditoría.
- **IA**: invocación vía servicio `ai/` con políticas de datos y logs.

### 4.2 Nuevo en el vertical (delta de dominio)

- Entidades y reglas de: trabajador en contexto laboral, examen ocupacional, ausentismo, accidente, prestación, HC laboral, solicitudes, nomencladores médico-laborales.
- Pantallas y rutas del vertical en `frontend/` cuando `TenantProfile.vertical` incluya este vertical (requiere **extensión del producto**: nuevo valor de vertical acordado con el equipo).

---

## 5. IA (servicio `ai/`)

Capacidades orientativas (sujetas a cumplimiento y consentimiento):

- Resúmenes asistidos para el profesional (con registro de qué texto se envía al modelo).
- Extracción asistida desde PDFs (órdenes, estudios) hacia campos estructurados.
- Asistente de procedimiento / FAQ interna.
- Validaciones heurísticas (campos obligatorios, rangos de fechas, inconsistencias).

**Requisito**: no enviar a modelos externos más datos personales o clínicos de los estrictamente necesarios; definir retención y opciones por tenant.

---

## 6. Fases de entrega

| Fase | Contenido |
|------|-----------|
| **0 — Descubrimiento** | Roles, flujos críticos, MVP cerrado, integraciones (contabilidad, laboratorio, WhatsApp). |
| **1 — Fundaciones** | Nuevo backend vertical (estructura hexagonal), migraciones base, RBAC del vertical, “hola mundo” en UI. |
| **2 — MVP dominio** | Trabajadores + empresas (o equivalente), turnos o citas, un flujo de examen o ausentismo end-to-end. |
| **3 — Ampliación** | Accidentes, prestaciones, HC laboral, informes. |
| **4 — IA** | Endpoints y UI de asistentes / extracción con políticas acordadas. |
| **5 — Endurecimiento** | Auditoría, pruebas, revisión de seguridad y documentación de operación. |

---

## 7. Decisiones abiertas (completar antes de codificar el vertical)

1. **Nombre del vertical en código** (inglés): p. ej. `occupational_health` — alinear con `TenantProfile.vertical` y catálogo del frontend.
2. **MVP**: ¿qué subconjunto de la tabla del §3 entra en v1?
3. **Modelo de empresas**: ¿un tenant tiene muchas empresas cliente y muchos trabajadores cada una?
4. **Integraciones obligatorias** en v1 (facturación externa, laboratorio, etc.).

---

## 8. Referencias internas

- Reglas del monorepo: `CLAUDE.md`.
- Verticales sin duplicación: `.cursor/rules/verticals-no-duplication.mdc`.
- Anti-fraude / auditoría (cuando toque dinero o datos sensibles): `pymes-core/docs/FRAUD_PREVENTION.md` (si existe en el checkout).
