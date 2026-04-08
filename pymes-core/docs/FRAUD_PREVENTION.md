# Prevención de fraude y robos internos

**Prioridad del producto:** la trazabilidad de dinero, stock y acciones sensibles es un requisito de confianza para dueños de PyMEs. Este documento es la referencia canónica de **qué hace el sistema hoy**, **cómo consultarlo** y **qué extender** sin duplicar dominio fuera de `pymes-core`.

---

## 1. Objetivo y alcance

- **Objetivo:** reducir la *oportunidad* de fraude (controles de acceso, segregación rudimentaria) y dejar **evidencia auditable** (quién hizo qué, cuándo, sobre qué recurso) para investigaciones y conciliaciones.
- **Alcance:** backend transversal `pymes-core/backend` (auditoría, ventas, cobros, caja, inventario, RBAC, etc.). No sustituye políticas internas, arqueos físicos, CCTV ni procesos legales/laborales.

---

## 2. Modelo de amenazas (resumido)

Escenarios típicos que el producto puede **atenuar** con datos y permisos:

| Riesgo | Mecanismo en producto |
|--------|------------------------|
| Cobro no registrado o mal registrado | Tabla `payments` + movimiento de caja + evento **`payment.created`** en `audit_log` |
| Venta anulada para encubrir faltante | Evento **`sale.voided`** en auditoría + reversión de stock/caja según lógica de dominio |
| Ajuste de stock opaco | Evento **`inventory.adjusted`** en auditoría |
| Exceso de privilegios | RBAC por recurso/acción; roles semilla `cashier`, `seller`, `accountant`, `warehouse` (ver migración `0013_rbac_seed.up.sql` + `0051_english_role_names.up.sql`) |
| Repudio | Cadena de hash en `audit_log` (`prev_hash`, `hash`) por org |

---

## 3. Auditoría (`audit_log`)

### 3.1 Comportamiento

- Cada entrada lleva `org_id`, `action`, `resource_type`, `resource_id`, `payload` (JSON), `created_at`, actor (legado + campos enriquecidos cuando aplica).
- Las filas se encadenan con **hash** respecto a la entrada anterior de la misma org (integridad frente a borrados/alteraciones simples a nivel aplicación).

Código principal: `pymes-core/backend/internal/audit/` (repositorio, use cases, handler HTTP).

### 3.2 API (autenticado, por permisos)

| Método | Ruta | Uso |
|--------|------|-----|
| `GET` | `/v1/audit` | Listado reciente (límite definido en backend) |
| `GET` | `/v1/audit/export?format=csv` | Export CSV para análisis externo |

Los permisos efectivos dependen del rol RBAC (ej. rol semilla **accountant** incluye `audit:read` y `audit:export` en el seed de referencia).

### 3.3 Consola (frontend)

- **Administración → Registro de auditoría** (`AdminPage`): tabla de eventos y botón **Descargar CSV**.
- Los cobros aparecen como acción **`payment.created`** cuando se confirma un `POST /v1/sales/:id/payments` exitoso.

---

## 4. Cobros por venta y evento `payment.created`

### 4.1 Por qué es crítico

El dinero cobrado debe ser **reconciliable** con ventas y caja. Sin registro en auditoría, un dueño solo ve tablas dispersas; con **`payment.created`** queda un **evento explícito** por cada cobro con actor y montos.

### 4.2 Contrato del evento

Tras un cobro exitoso (`internal/payments` → repositorio en transacción), se emite:

| Campo | Valor |
|-------|--------|
| `action` | `payment.created` |
| `resource_type` | `payment` |
| `resource_id` | UUID del pago |
| Actor (string legado) | `created_by` del pago (= actor autenticado en el handler) |
| `payload` | `sale_id`, `amount`, `method`, `received_at` (RFC3339); `notes` si no vacío |

Código: `pymes-core/backend/internal/payments/usecases.go` (inyección de `AuditPort`, llamada post-`CreateSalePayment` exitoso). Cableado: `pymes-core/backend/wire/bootstrap.go` (`payments.NewUsecases(paymentsRepo, auditUC)`).

### 4.3 API de cobros

| Método | Ruta | Permiso típico |
|--------|------|----------------|
| `GET` | `/v1/sales/:id/payments` | `payments:read` |
| `POST` | `/v1/sales/:id/payments` | `payments:create` |

Validación de método de pago y montos: ver `internal/payments/usecases.go` y repositorio (tope al saldo pendiente de la venta).

---

## 5. Otros eventos relevantes (no exhaustivo)

Para investigaciones cruzadas, conviene filtrar CSV/export por `action` / `resource_type`:

- Ventas: `sale.created`, `sale.voided`
- Caja: `cashflow.created`, `cashflow.sale_income`, `cashflow.sale_void` (según flujo)
- Inventario: `inventory.adjusted`
- Cotizaciones / compras / clientes / productos: ver grep de `audit.Log` en `internal/*/usecases.go`
- RBAC: `rbac.role.*`, `rbac.role.assigned`, `rbac.role.unassigned`
- Procurement: acciones sobre `procurement_request` / `procurement_policy`

---

## 6. RBAC y segregación de funciones (SoD)

El producto no impone automáticamente “dos personas” en cada acción; ofrece **permisos granulares** para que la org configure **roles mínimos**:

- **Cajero (semilla):** `sales` read/create, `payments` read/create, `cashflow`, etc. — **no** incluye `sales:void` (la anulación exige permiso explícito `sales` + acción `void` en `POST /v1/sales/:id/void`).
- **Contador (semilla):** lectura de auditoría, export, reportes, lectura de ventas/cobros.
- **Almacenero:** inventario/productos; no cobros por defecto en seed.

**Recomendación:** documentar internamente por cliente una **matriz rol × acción** y revisarla al incorporar personal.

---

## 7. Buenas prácticas operativas

1. Revisión periódica del CSV de auditoría: agrupar por `actor` y contar `payment.created`, `sale.voided`, `inventory.adjusted`.
2. Conciliar **suma de cobros** (`payments` + eventos) con **caja** y arqueos físicos.
3. Identidad clara: preferir usuarios humanos con JWT (actor trazable); API keys solo para integraciones, con scopes mínimos.
4. En PyMEs con poca gente: **controles compensatorios** (firma del dueño en anulaciones, muestras semanales).

---

## 8. Evolución recomendada (backlog técnico)

Implementaciones futuras coherentes con arquitectura (todo en `pymes-core` si es transversal):

- Motor de **reglas / excepciones** (umbrales: voids/día, montos, horarios) → señales en DB o nuevas filas de auditoría.
- **Doble aprobación** para `sale.voided` o ajustes de stock sobre umbral.
- Jobs batch: **perfiles por usuario** (desvíos respecto a media histórica) para priorizar revisión humana.

---

## 9. Mantenimiento del documento

Quien agregue:

- nuevas acciones auditadas, o  
- cambios en permisos de rutas sensibles, o  
- nuevos flujos de dinero,

debe **actualizar esta guía** y, si aplica, el seed RBAC en migraciones nuevas (nunca editar migraciones ya aplicadas).

---

## 10. Referencias cruzadas

- Índice y mapa de puertos/servicios: [docs/README.md](../../docs/README.md)
- Arquitectura y deployables: [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md)
- Core transversal: [docs/PYMES_CORE.md](../../docs/PYMES_CORE.md)
- Identidad y variables de API: [docs/AUTH.md](../../docs/AUTH.md)
- Integración `core` + AI comercial: [docs/CORE_INTEGRATION.md](../../docs/CORE_INTEGRATION.md)
- Reglas del repo (seguridad genérica): [CLAUDE.md](../../CLAUDE.md) § Seguridad
