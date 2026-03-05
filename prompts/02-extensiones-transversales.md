# Prompt 02 — Extensiones Transversales

## Contexto

Este prompt extiende el **control-plane** (Prompt 00) y el **core de negocio** (Prompt 01) con funcionalidades transversales que toda pyme necesita sin importar el vertical. Son las piezas que completan la experiencia de producto.

**Prerequisitos**: Prompts 00 y 01 implementados y funcionales.

**Regla fundamental**: estos modulos viven dentro de `control-plane/backend/internal/` porque comparten la misma DB, el mismo auth, el mismo Lambda, y el mismo tenant (`org_id`). NO son servicios separados.

---

## Modulos a implementar

| # | Modulo | Descripcion | Prioridad |
|---|--------|-------------|-----------|
| 1 | `rbac` | Roles y permisos granulares | 1 |
| 2 | `purchases` | Compras a proveedores | 2 |
| 3 | `accounts` | Cuentas corrientes (fiado, deudas clientes/proveedores) | 3 |
| 4 | `payments` | Pagos parciales y multiples medios por venta | 4 |
| 5 | `returns` | Devoluciones parciales y notas de credito | 5 |
| 6 | `discounts` | Descuentos por item, por venta, y promociones | 6 |
| 7 | `pricelists` | Listas de precios (mayorista, minorista, custom) | 7 |
| 8 | `recurring` | Gastos recurrentes (alquiler, servicios, sueldos) | 8 |
| 9 | `appointments` | Turnos, citas y reservas | 9 |
| 10 | `dataio` | Import/Export CSV y Excel | 10 |
| 11 | `attachments` | Archivos adjuntos (S3 + presigned URLs) | 11 |
| 12 | `pdfgen` | Generacion de PDFs (recibos, presupuestos) | 12 |
| 13 | `timeline` | Activity timeline por entidad | 13 |
| 14 | `outwebhooks` | Webhooks salientes para integraciones | 14 |
| 15 | `whatsapp` | Integracion WhatsApp (links + templates) | 15 |
| 16 | `dashboard` | Dashboard con KPIs configurables | 16 |
| 17 | `scheduler` | Tareas programadas (cron) | 17 |

---

## 1. RBAC — Roles y Permisos Granulares

### Problema

Hoy el sistema tiene `role` (admin/member) del JWT de Clerk y `scopes` en API keys, pero no hay control granular. Toda pyme con mas de 2 empleados necesita: "el vendedor puede crear ventas pero no ver reportes", "el cajero no puede anular ventas".

### Entidades de dominio

```go
type Role struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Name        string       // "vendedor", "cajero", "contador", "admin"
    Description string
    IsSystem    bool         // roles del sistema no se pueden borrar
    Permissions []Permission
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type Permission struct {
    Resource string // "customers", "products", "sales", "inventory", "cashflow", "reports", "admin", "billing"
    Actions  []string // "read", "create", "update", "delete", "void", "export"
}

type UserRole struct {
    UserID    uuid.UUID
    OrgID     uuid.UUID
    RoleID    uuid.UUID
    AssignedBy string
    AssignedAt time.Time
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_system boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource text NOT NULL,
    action text NOT NULL,
    UNIQUE(role_id, resource, action)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions(role_id);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by text,
    assigned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, org_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_org ON user_roles(org_id);
```

### API

```
GET    /v1/roles                    — Listar roles de la org
POST   /v1/roles                    — Crear rol custom
GET    /v1/roles/:id                — Detalle con permisos
PUT    /v1/roles/:id                — Actualizar permisos
DELETE /v1/roles/:id                — Eliminar (solo custom, no system)
POST   /v1/roles/:id/assign/:user_id — Asignar rol a usuario
DELETE /v1/roles/:id/assign/:user_id — Remover rol de usuario
GET    /v1/users/:user_id/permissions — Permisos efectivos del usuario
```

### Roles del sistema (seed)

| Rol | Permisos |
|-----|----------|
| `admin` | Todo: `*:*` |
| `vendedor` | customers:read,create,update / products:read / sales:read,create / quotes:read,create,update / inventory:read |
| `cajero` | sales:read,create / cashflow:read,create / customers:read |
| `contador` | reports:read / cashflow:read / sales:read / billing:read / audit:read,export |
| `almacenero` | inventory:read,create,update / products:read |

### Middleware de autorizacion

```go
// internal/shared/handlers/rbac_middleware.go
func (m *RBACMiddleware) RequirePermission(resource, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        orgID := c.GetString("org_id")
        actor := c.GetString("actor")

        // 1. Si auth method es JWT y role es "org:admin" de Clerk -> permitir todo
        // 2. Si auth method es API key -> verificar scopes (ya existente)
        // 3. Si no -> buscar user_roles + role_permissions en DB (cacheable)
        // 4. Si no tiene permiso -> 403 Forbidden

        if !m.rbacUC.HasPermission(c.Request.Context(), orgID, actor, resource, action) {
            c.AbortWithStatusJSON(403, gin.H{"error": "forbidden", "required": resource + ":" + action})
            return
        }
        c.Next()
    }
}
```

### Cache

Los permisos de un usuario no cambian frecuentemente. Cachear en memoria con TTL de 5 minutos. En Lambda, cada instancia tiene su cache — aceptable porque Lambda escala a pocas instancias concurrentes para pymes.

```go
type permCache struct {
    mu    sync.RWMutex
    items map[string]cacheEntry // key: "orgID:actor"
}

type cacheEntry struct {
    permissions map[string]map[string]bool // resource -> action -> allowed
    expiresAt   time.Time
}
```

### Reglas de negocio

- Cada usuario tiene exactamente **un rol por org**. Si no tiene rol asignado, es `member` (solo read basico).
- Los roles `admin` e `is_system = true` no se pueden eliminar ni modificar.
- Solo `admin` puede gestionar roles y asignarlos.
- El primer usuario de una org (creador) recibe automaticamente el rol `admin`.
- Los permisos se verifican en el middleware ANTES de llegar al handler.
- API keys mantienen su sistema de scopes actual — RBAC aplica solo a usuarios JWT.

### Integracion con handlers existentes

Los handlers existentes de Prompt 01 necesitan agregar el middleware. Ejemplo:

```go
// internal/customers/handler.go
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, rbac *handlers.RBACMiddleware) {
    g := rg.Group("/customers")
    g.GET("", rbac.RequirePermission("customers", "read"), h.List)
    g.POST("", rbac.RequirePermission("customers", "create"), h.Create)
    g.GET("/:id", rbac.RequirePermission("customers", "read"), h.Get)
    g.PUT("/:id", rbac.RequirePermission("customers", "update"), h.Update)
    g.DELETE("/:id", rbac.RequirePermission("customers", "delete"), h.Delete)
}
```

---

## 2. Purchases — Compras a Proveedores

### Problema

Toda pyme compra mercaderia. No hay modulo de compras. Es tan fundamental como ventas: compra a proveedor → entrada de stock → deuda con proveedor → pago. La ferreteria compra tornillos, la verduleria compra frutas, el taller compra repuestos, el profe online compra licencias.

### Entidades de dominio

```go
type Purchase struct {
    ID            uuid.UUID
    OrgID         uuid.UUID
    Number        string         // "CPA-00001"
    SupplierID    *uuid.UUID
    SupplierName  string
    Status        string         // "draft" | "received" | "partial" | "voided"
    PaymentStatus string         // "pending" | "partial" | "paid"
    Items         []PurchaseItem
    Subtotal      decimal.Decimal
    TaxTotal      decimal.Decimal
    Total         decimal.Decimal
    Currency      string
    Notes         string
    ReceivedAt    *time.Time
    CreatedBy     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type PurchaseItem struct {
    ID          uuid.UUID
    PurchaseID  uuid.UUID
    ProductID   *uuid.UUID
    Description string
    Quantity    decimal.Decimal
    UnitCost    decimal.Decimal
    TaxRate     decimal.Decimal
    Subtotal    decimal.Decimal
    SortOrder   int
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS purchases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    supplier_id uuid REFERENCES suppliers(id),
    supplier_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'received', 'partial', 'voided')),
    payment_status text NOT NULL DEFAULT 'pending' CHECK (payment_status IN ('pending', 'partial', 'paid')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    received_at timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS purchase_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id uuid NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_cost numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_purchases_org ON purchases(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_purchases_supplier ON purchases(supplier_id) WHERE supplier_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_purchases_org_status ON purchases(org_id, status);
```

### API

```
GET    /v1/purchases              — Listar (paginado, filtro por status/supplier/fecha)
POST   /v1/purchases              — Crear compra
GET    /v1/purchases/:id          — Detalle con items
PUT    /v1/purchases/:id          — Actualizar (solo draft)
DELETE /v1/purchases/:id          — Eliminar (solo draft)
POST   /v1/purchases/:id/receive  — Marcar como recibida (ingresa stock)
POST   /v1/purchases/:id/void     — Anular compra
```

### Reglas de negocio

- `number` secuencial: `CPA-{5 digitos}`. Configurable via `tenant_settings.purchase_prefix`.
- Al **recibir** (`receive`): genera movimientos de stock `type = 'in'`, `reason = 'purchase'` para cada item con `track_stock = true`.
- Al **recibir**: genera movimiento de caja `type = 'expense'`, `category = 'purchase'` SI el pago es inmediato. Si es a credito, solo genera la deuda en cuenta corriente del proveedor (ver modulo accounts).
- Al **anular**: revierte stock (`type = 'out'`, `reason = 'void'`) si ya fue recibida.
- `payment_status` se actualiza automaticamente cuando se registran pagos parciales (ver modulo payments).
- Solo se pueden editar/eliminar compras en estado `draft`.
- Totales se recalculan server-side igual que en ventas.
- Puede tener recepcion parcial: `status = 'partial'` cuando se recibe parte de los items (futuro, v1 es todo o nada).

### Extension de tenant_settings

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS purchase_prefix text NOT NULL DEFAULT 'CPA',
    ADD COLUMN IF NOT EXISTS next_purchase_number int NOT NULL DEFAULT 1;
```

### Interacciones

```
Purchase (recibir)
  ├── Inventory: stock_movement(type=in, reason=purchase) por cada item
  ├── Cashflow: cash_movement(type=expense) SI pago inmediato
  ├── Accounts: crear deuda con proveedor SI pago a credito
  ├── Products: actualizar cost_price si cambio
  ├── Audit: audit_log entry
  └── Timeline: entry en supplier

Purchase (anular)
  ├── Inventory: stock_movement(type=out, reason=void) reverso
  ├── Cashflow: cash_movement reverso SI ya se habia pagado
  ├── Accounts: cancelar deuda pendiente
  └── Audit: audit_log entry
```

---

## 3. Accounts — Cuentas Corrientes (Fiado)

### Problema

EL feature mas LATAM que existe. El cuadernito del kiosquero. "Juan me debe $5000 de la semana pasada". "Le debo $30k al proveedor de bebidas". Toda verduleria, ferreteria, kiosco, taller, comercio de barrio vive de fiar. Sin esto, el sistema no sirve para el 80% de las pymes LATAM.

### Entidades de dominio

```go
type Account struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Type        string         // "receivable" | "payable"
    EntityType  string         // "customer" | "supplier"
    EntityID    uuid.UUID
    EntityName  string         // denormalized para queries rapidos
    Balance     decimal.Decimal // positivo = le deben a la org (receivable), negativo = la org debe (payable)
    Currency    string
    CreditLimit decimal.Decimal // limite de fiado (0 = sin limite)
    UpdatedAt   time.Time
}

type AccountMovement struct {
    ID          uuid.UUID
    AccountID   uuid.UUID
    OrgID       uuid.UUID
    Type        string         // "charge" | "payment" | "adjustment" | "void"
    Amount      decimal.Decimal // positivo = aumenta deuda, negativo = reduce deuda
    Balance     decimal.Decimal // saldo despues del movimiento (running balance)
    Description string
    ReferenceType string       // "sale", "purchase", "payment", "manual"
    ReferenceID *uuid.UUID
    CreatedBy   string
    CreatedAt   time.Time
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('receivable', 'payable')),
    entity_type text NOT NULL CHECK (entity_type IN ('customer', 'supplier')),
    entity_id uuid NOT NULL,
    entity_name text NOT NULL DEFAULT '',
    balance numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    credit_limit numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_accounts_org ON accounts(org_id, type);
CREATE INDEX IF NOT EXISTS idx_accounts_entity ON accounts(org_id, entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_accounts_balance ON accounts(org_id) WHERE balance != 0;

CREATE TABLE IF NOT EXISTS account_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('charge', 'payment', 'adjustment', 'void')),
    amount numeric(15,2) NOT NULL,
    balance numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_account_movements_account ON account_movements(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_account_movements_org ON account_movements(org_id, created_at DESC);
```

### API

```
GET    /v1/accounts                          — Listar cuentas (filtro por tipo, con saldo != 0)
GET    /v1/accounts/receivable               — Solo cuentas a cobrar (clientes que deben)
GET    /v1/accounts/payable                  — Solo cuentas a pagar (deudas con proveedores)
GET    /v1/accounts/:entity_type/:entity_id  — Cuenta de un cliente/proveedor especifico
GET    /v1/accounts/:id/movements            — Historial de movimientos de una cuenta
POST   /v1/accounts/:id/payment              — Registrar pago (reduce saldo)
POST   /v1/accounts/:id/charge               — Registrar cargo manual (aumenta saldo)
POST   /v1/accounts/:id/adjust               — Ajuste manual (correccion)
PUT    /v1/accounts/:id/credit-limit         — Configurar limite de fiado
GET    /v1/accounts/summary                  — Resumen: total a cobrar, total a pagar, neto
```

### Flujo "Venta fiada"

1. Se crea una venta con `payment_method = 'credit'` (nuevo valor)
2. El usecase de sales detecta que es fiada
3. Se crea/obtiene la cuenta `receivable` del cliente
4. Se registra un `charge` en la cuenta por el total de la venta
5. NO se genera movimiento de caja (la plata no entro todavia)
6. Cuando el cliente paga: `POST /v1/accounts/:id/payment` → genera movimiento de caja

### Flujo "Compra a credito"

1. Se recibe una compra con pago diferido
2. Se crea/obtiene la cuenta `payable` del proveedor
3. Se registra un `charge` por el total de la compra
4. Cuando se paga al proveedor: `POST /v1/accounts/:id/payment` → genera movimiento de caja

### Reglas de negocio

- Las cuentas se crean automaticamente al primer cargo (lazy creation).
- El `balance` se actualiza atomicamente con cada movimiento (running balance en la tabla `account_movements`).
- Los movimientos son **inmutables** — nunca se editan ni borran.
- `credit_limit`: si > 0, la venta fiada se rechaza si `balance + venta.total > credit_limit`. Si = 0, sin limite (fiado libre).
- El resumen (`/summary`) muestra: total a cobrar de todos los clientes, total a pagar a todos los proveedores, neto.
- Al registrar pago: se genera `cash_movement(type=income)` si es receivable, `cash_movement(type=expense)` si es payable.
- La cuenta muestra el estado de cuenta completo: fecha, concepto, debe, haber, saldo. Como el cuadernito pero digital.

---

## 4. Payments — Pagos Parciales y Multiples Medios

### Problema

"$5000 en efectivo y $3000 con tarjeta". Hoy una venta tiene UN solo `payment_method`. En la realidad se mezclan medios. Ademas, una venta fiada se puede pagar en cuotas: hoy $2000, la semana que viene $3000.

### Entidades de dominio

```go
type Payment struct {
    ID            uuid.UUID
    OrgID         uuid.UUID
    ReferenceType string         // "sale" | "purchase"
    ReferenceID   uuid.UUID
    Method        string         // "cash" | "card" | "transfer" | "check" | "other"
    Amount        decimal.Decimal
    Notes         string
    ReceivedAt    time.Time      // cuando se recibio el pago
    CreatedBy     string
    CreatedAt     time.Time
}
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    reference_type text NOT NULL CHECK (reference_type IN ('sale', 'purchase')),
    reference_id uuid NOT NULL,
    method text NOT NULL DEFAULT 'cash' CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other')),
    amount numeric(15,2) NOT NULL,
    notes text NOT NULL DEFAULT '',
    received_at timestamptz NOT NULL DEFAULT now(),
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payments_reference ON payments(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payments_org ON payments(org_id, created_at DESC);
```

### API

```
GET    /v1/sales/:id/payments       — Pagos de una venta
POST   /v1/sales/:id/payments       — Registrar pago a una venta
GET    /v1/purchases/:id/payments   — Pagos de una compra
POST   /v1/purchases/:id/payments   — Registrar pago a una compra
```

### Cambios al modelo de ventas

El campo `payment_method` de `sales` pasa a ser informativo (metodo principal o "mixed" si hay multiples). El detalle real esta en la tabla `payments`.

Agregar a `sales`:
```sql
ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS amount_paid numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS payment_status text NOT NULL DEFAULT 'paid'
        CHECK (payment_status IN ('pending', 'partial', 'paid'));
```

### Flujo de venta con pago inmediato (sin cambio)

1. Se crea la venta
2. Se registra automaticamente un `Payment` por el total con el `payment_method` indicado
3. `amount_paid = total`, `payment_status = 'paid'`
4. Se genera `cash_movement(type=income)` como antes

### Flujo de venta fiada

1. Se crea la venta con `payment_method = 'credit'`
2. NO se registra Payment
3. `amount_paid = 0`, `payment_status = 'pending'`
4. Se carga en cuenta corriente del cliente
5. Cuando el cliente paga (parcial o total):
   - `POST /v1/sales/:id/payments` con `{"method": "cash", "amount": 3000}`
   - Se registra Payment, se actualiza `amount_paid`, se recalcula `payment_status`
   - Se genera `cash_movement(type=income)`
   - Se registra `account_movement(type=payment)` en la cuenta del cliente

### Flujo de pago con multiples medios

1. Se crea la venta SIN pago automatico
2. Se registran N payments:
   - `{"method": "cash", "amount": 5000}`
   - `{"method": "card", "amount": 3000}`
3. Cada payment genera su propio `cash_movement`
4. `payment_status` se actualiza: si `amount_paid >= total` → `paid`, si `amount_paid > 0` → `partial`, si `0` → `pending`

### Reglas de negocio

- Payments son **inmutables** — no se editan ni borran.
- No se puede pagar mas del saldo pendiente: `amount <= total - amount_paid`.
- Cada payment genera un `cash_movement` individual (para saber que entro en efectivo vs tarjeta).
- `method = 'check'` es comun en LATAM para pagos a proveedores.
- Los payments tienen `received_at` que puede ser distinto a `created_at` (registrar un pago de ayer).

---

## 5. Returns — Devoluciones y Notas de Credito

### Problema

El cliente devuelve 1 de 3 productos. Hoy solo hay `void` que anula toda la venta. Se necesita devolucion parcial + nota de credito o reembolso. Pasa en todos los comercios: la ferreteria, el kiosco, la tienda online.

### Entidades de dominio

```go
type Return struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Number      string         // "DEV-00001"
    SaleID      uuid.UUID
    Reason      string         // "defective", "wrong_item", "changed_mind", "other"
    Items       []ReturnItem
    Subtotal    decimal.Decimal
    TaxTotal    decimal.Decimal
    Total       decimal.Decimal
    RefundMethod string        // "cash" | "credit_note" | "original_method"
    Status      string         // "completed" | "voided"
    Notes       string
    CreatedBy   string
    CreatedAt   time.Time
}

type ReturnItem struct {
    ID          uuid.UUID
    ReturnID    uuid.UUID
    SaleItemID  uuid.UUID      // referencia al item original de la venta
    ProductID   *uuid.UUID
    Description string
    Quantity    decimal.Decimal // cantidad devuelta (no puede exceder la vendida)
    UnitPrice   decimal.Decimal
    TaxRate     decimal.Decimal
    Subtotal    decimal.Decimal
}

type CreditNote struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Number      string         // "NC-00001"
    CustomerID  uuid.UUID
    ReturnID    uuid.UUID
    Amount      decimal.Decimal
    UsedAmount  decimal.Decimal // cuanto se ha aplicado a ventas futuras
    Balance     decimal.Decimal // amount - used_amount
    ExpiresAt   *time.Time
    Status      string         // "active" | "used" | "expired" | "voided"
    CreatedAt   time.Time
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS returns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    sale_id uuid NOT NULL REFERENCES sales(id),
    reason text NOT NULL DEFAULT 'other' CHECK (reason IN ('defective', 'wrong_item', 'changed_mind', 'other')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    refund_method text NOT NULL DEFAULT 'cash' CHECK (refund_method IN ('cash', 'credit_note', 'original_method')),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'voided')),
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS return_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    return_id uuid NOT NULL REFERENCES returns(id) ON DELETE CASCADE,
    sale_item_id uuid NOT NULL REFERENCES sale_items(id),
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS credit_notes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    customer_id uuid NOT NULL REFERENCES customers(id),
    return_id uuid NOT NULL REFERENCES returns(id),
    amount numeric(15,2) NOT NULL,
    used_amount numeric(15,2) NOT NULL DEFAULT 0,
    balance numeric(15,2) NOT NULL,
    expires_at timestamptz,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'used', 'expired', 'voided')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE INDEX IF NOT EXISTS idx_returns_org ON returns(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_returns_sale ON returns(sale_id);
CREATE INDEX IF NOT EXISTS idx_credit_notes_customer ON credit_notes(org_id, customer_id) WHERE status = 'active';
```

### API

```
POST   /v1/sales/:id/return         — Crear devolucion parcial o total
GET    /v1/returns                   — Listar devoluciones
GET    /v1/returns/:id               — Detalle
POST   /v1/returns/:id/void          — Anular devolucion

GET    /v1/credit-notes              — Listar notas de credito
GET    /v1/credit-notes/:id          — Detalle
GET    /v1/customers/:id/credit-notes — Notas de credito de un cliente
POST   /v1/sales/:id/apply-credit    — Aplicar nota de credito a una venta
```

### Flujo de devolucion

1. `POST /v1/sales/:id/return` con items y cantidades a devolver
2. Valida que las cantidades no excedan lo vendido (menos devoluciones previas)
3. Crea el `Return` con items
4. Segun `refund_method`:
   - `cash`: genera `cash_movement(type=expense, category=return)` + reembolso directo
   - `credit_note`: genera `CreditNote` con saldo a favor del cliente
   - `original_method`: genera `cash_movement` con el metodo de pago original de la venta
5. Si los productos tienen `track_stock = true`: genera `stock_movement(type=in, reason=return)` (devuelve stock)
6. Actualiza `amount_paid` y `payment_status` de la venta original si aplica

### Reglas de negocio

- No se puede devolver mas de lo vendido por item.
- Las devoluciones son **inmutables** (solo se pueden anular, creando un contra-movimiento).
- `number` secuencial: `DEV-{5 digitos}`. Notas de credito: `NC-{5 digitos}`.
- Las notas de credito pueden tener vencimiento (`expires_at`). Default: sin vencimiento.
- Al aplicar credito a una venta nueva: se reduce el `balance` de la nota y se genera un `Payment` especial con `method = 'credit_note'`.
- Devoluciones generan audit log y timeline en el customer.

### Extension de tenant_settings

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS return_prefix text NOT NULL DEFAULT 'DEV',
    ADD COLUMN IF NOT EXISTS credit_note_prefix text NOT NULL DEFAULT 'NC',
    ADD COLUMN IF NOT EXISTS next_return_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_credit_note_number int NOT NULL DEFAULT 1;
```

---

## 6. Discounts — Descuentos

### Problema

Toda pyme da descuentos. "10% por pago en efectivo", "2x1 en tal producto", "$500 de descuento al total". Hoy el modelo de ventas y presupuestos no tiene campo de descuento.

### Diseno

NO es un modulo separado con su propio directorio. Son **campos adicionales** en los modelos existentes de `sales`, `quotes`, y `products`.

### Cambios al modelo

**Sale / Quote items** — agregar descuento por item:
```sql
ALTER TABLE sale_items
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE quote_items
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0;
```

**Sales / Quotes** — agregar descuento global:
```sql
ALTER TABLE sales
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS discount_total numeric(15,2) NOT NULL DEFAULT 0;

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS discount_type text NOT NULL DEFAULT 'none'
        CHECK (discount_type IN ('none', 'percentage', 'fixed')),
    ADD COLUMN IF NOT EXISTS discount_value numeric(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS discount_total numeric(15,2) NOT NULL DEFAULT 0;
```

### Calculo de totales (actualizado)

```
Por cada item:
  item_subtotal = quantity * unit_price
  item_discount = discount_type == 'percentage' ? item_subtotal * discount_value / 100 : discount_value
  item_net = item_subtotal - item_discount
  item_tax = item_net * tax_rate / 100

Totales de la venta/presupuesto:
  subtotal = SUM(item_net)
  discount_total = (global) discount_type == 'percentage' ? subtotal * discount_value / 100 : discount_value
  subtotal_after_discount = subtotal - discount_total
  tax_total = SUM(item_tax) ajustado proporcionalmente si hay descuento global
  total = subtotal_after_discount + tax_total
```

### Reglas de negocio

- Descuento por item: aplica ANTES del impuesto (descuento sobre precio neto).
- Descuento global: aplica sobre el subtotal (suma de items netos).
- Se pueden combinar: descuento por item + descuento global.
- `discount_value` nunca puede hacer que el total sea negativo.
- Los descuentos se persisten en la venta (snapshot) — no dependen de reglas activas.
- El `subtotal` de la venta refleja el monto ANTES de descuentos. `discount_total` muestra cuanto se desconto. `total` es el monto final.

---

## 7. Price Lists — Listas de Precios

### Problema

La ferreteria le cobra $100 el tornillo al particular y $70 al electricista que compra siempre. El mayorista tiene lista distinta al minorista. "Lista A", "Lista B". Es universalmente comun en comercios LATAM.

### Entidades de dominio

```go
type PriceList struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Name        string         // "Minorista", "Mayorista", "Empleados"
    Description string
    IsDefault   bool           // la lista que se aplica si no se especifica otra
    Markup      decimal.Decimal // porcentaje sobre precio base (ej: -30 para 30% descuento)
    IsActive    bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type PriceListItem struct {
    PriceListID uuid.UUID
    ProductID   uuid.UUID
    Price       decimal.Decimal // precio especifico para este producto en esta lista (override)
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS price_lists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_default boolean NOT NULL DEFAULT false,
    markup numeric(5,2) NOT NULL DEFAULT 0,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);

CREATE TABLE IF NOT EXISTS price_list_items (
    price_list_id uuid NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    price numeric(15,2) NOT NULL,
    PRIMARY KEY (price_list_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_price_lists_org ON price_lists(org_id) WHERE is_active = true;
```

### API

```
GET    /v1/price-lists                          — Listar listas de precios
POST   /v1/price-lists                          — Crear lista
GET    /v1/price-lists/:id                      — Detalle con items
PUT    /v1/price-lists/:id                      — Actualizar (nombre, markup, activa)
DELETE /v1/price-lists/:id                      — Eliminar (solo si no es default)
PUT    /v1/price-lists/:id/items                — Actualizar precios de productos (bulk)
GET    /v1/products/:id/prices                  — Precio del producto en todas las listas
```

### Asignacion de lista a clientes

Agregar a `customers`:
```sql
ALTER TABLE customers
    ADD COLUMN IF NOT EXISTS price_list_id uuid REFERENCES price_lists(id);
```

### Resolucion de precio

Cuando se crea una venta o presupuesto, el precio se resuelve asi:

1. Si el cliente tiene `price_list_id` → buscar en esa lista
2. Si la lista tiene `price_list_items` para el producto → usar ese precio (override)
3. Si no hay override → aplicar `markup` de la lista sobre `products.price`
4. Si el cliente no tiene lista → usar `products.price` (lista default)

```go
func (uc *Usecases) ResolvePrice(ctx context.Context, orgID, productID, customerID uuid.UUID) (decimal.Decimal, error) {
    // 1. Buscar lista del cliente
    // 2. Buscar override en price_list_items
    // 3. Si no hay override, aplicar markup
    // 4. Fallback: product.price
}
```

### Reglas de negocio

- Solo puede haber UNA lista `is_default = true` por org.
- `markup` puede ser negativo (descuento) o positivo (recargo). Ej: -30 = 30% menos, +10 = 10% mas.
- Los overrides por producto (`price_list_items`) tienen prioridad sobre el markup general.
- Al crear venta/presupuesto, el precio se snapshot (se guarda en `sale_items.unit_price`). Si la lista cambia despues, las ventas anteriores no se afectan.
- La lista default no se puede eliminar.

---

## 8. Recurring — Gastos Recurrentes

### Problema

Alquiler, luz, agua, internet, sueldos, cuota del monotributo, pago del sistema. Todos los meses. Hoy hay que cargarlos manualmente uno por uno en caja. Toda pyme tiene gastos fijos.

### Entidades de dominio

```go
type RecurringExpense struct {
    ID            uuid.UUID
    OrgID         uuid.UUID
    Description   string
    Amount        decimal.Decimal
    Currency      string
    Category      string         // "rent", "utilities", "salary", "tax", "insurance", "subscription", "other"
    PaymentMethod string         // "cash" | "card" | "transfer" | "debit" | "other"
    Frequency     string         // "monthly" | "biweekly" | "weekly" | "quarterly" | "yearly"
    DayOfMonth    int            // dia del mes en que se paga (1-28)
    SupplierID    *uuid.UUID     // proveedor asociado (opcional)
    IsActive      bool
    NextDueDate   time.Time
    LastPaidDate  *time.Time
    Notes         string
    CreatedBy     string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS recurring_expenses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    description text NOT NULL,
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    payment_method text NOT NULL DEFAULT 'transfer',
    frequency text NOT NULL DEFAULT 'monthly'
        CHECK (frequency IN ('weekly', 'biweekly', 'monthly', 'quarterly', 'yearly')),
    day_of_month int NOT NULL DEFAULT 1 CHECK (day_of_month BETWEEN 1 AND 28),
    supplier_id uuid REFERENCES suppliers(id),
    is_active boolean NOT NULL DEFAULT true,
    next_due_date date NOT NULL,
    last_paid_date date,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recurring_expenses_org ON recurring_expenses(org_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_recurring_expenses_due ON recurring_expenses(next_due_date) WHERE is_active = true;
```

### API

```
GET    /v1/recurring-expenses              — Listar (activos, proximos a vencer)
POST   /v1/recurring-expenses              — Crear
GET    /v1/recurring-expenses/:id          — Detalle
PUT    /v1/recurring-expenses/:id          — Actualizar
DELETE /v1/recurring-expenses/:id          — Eliminar
POST   /v1/recurring-expenses/:id/pay      — Marcar como pagado (genera movimiento de caja)
POST   /v1/recurring-expenses/:id/skip     — Saltar periodo actual sin pagar
GET    /v1/recurring-expenses/upcoming      — Proximos vencimientos (proximos 30 dias)
```

### Flujo de pago

1. `POST /v1/recurring-expenses/:id/pay`
2. Genera `cash_movement(type=expense, category=<category>, reference_type=recurring)`
3. Si tiene `supplier_id`: genera `account_movement` en la cuenta del proveedor (si paga a credito)
4. Actualiza `last_paid_date = today`
5. Calcula `next_due_date` segun `frequency`
6. Genera audit log

### Scheduler task (automatico)

Agregar al scheduler:
```go
{
    Name:     "recurring_expenses_reminder",
    Interval: 24 * time.Hour,
    Handler: func(ctx) {
        // 1. Buscar recurring_expenses WHERE next_due_date <= today + 3 days AND is_active
        // 2. Para cada uno, generar notificacion al admin: "El alquiler vence en 3 dias"
        // 3. Si next_due_date < today: marcar como vencido (solo notificar, no pagar auto)
    },
}
```

### Reglas de negocio

- Los gastos recurrentes **NO se pagan automaticamente**. Solo se recuerda y facilita el registro.
- `day_of_month` hasta 28 (para evitar problemas con febrero).
- `next_due_date` se calcula automaticamente al pagar o crear.
- El gasto recurrente puede tener monto variable (ej: luz). El `amount` es el estimado, al pagar se puede cambiar el monto real.
- Al pagar, el body puede incluir `{"amount": 15000}` para override del monto (ej: la boleta de luz vino mas cara).
- `skip` avanza `next_due_date` sin generar movimiento (ej: mes de vacaciones, no se pago).
- Dashboard widget "upcoming" muestra gastos por vencer en los proximos 7 dias.

---

## 9. Appointments — Turnos, Citas y Reservas

### Problema

El taller agenda turnos para recibir autos, el profe agenda clases, el profesional agenda consultas, el salon de belleza agenda turnos. No todos los negocios lo usan (el kiosco no), pero es tan comun en negocios de servicio que es transversal. Lo que cambia por vertical es el NOMBRE (turno, cita, clase, reserva) pero la mecanica es la misma.

### Entidades de dominio

```go
type Appointment struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    CustomerID  *uuid.UUID
    CustomerName string       // si no hay customer registrado
    CustomerPhone string      // para confirmar/recordar
    Title       string        // "Revision auto", "Clase de piano", "Consulta"
    Description string
    Status      string        // "scheduled" | "confirmed" | "in_progress" | "completed" | "cancelled" | "no_show"
    StartAt     time.Time
    EndAt       time.Time
    Duration    int           // minutos
    Location    string        // "Sucursal Centro", "Online", "Domicilio"
    AssignedTo  string        // empleado/profesional asignado
    Color       string        // para UI calendario (#FF5733)
    Notes       string
    Metadata    map[string]any // extension por vertical
    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type AppointmentSlot struct {
    DayOfWeek   int    // 0=domingo, 1=lunes, ... 6=sabado
    StartTime   string // "09:00"
    EndTime     string // "18:00"
    SlotMinutes int    // duracion de cada slot (30, 60, etc.)
    MaxPerSlot  int    // cuantos turnos simultaneos (default 1)
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS appointments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    customer_id uuid REFERENCES customers(id),
    customer_name text NOT NULL DEFAULT '',
    customer_phone text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'confirmed', 'in_progress', 'completed', 'cancelled', 'no_show')),
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    duration int NOT NULL DEFAULT 60,
    location text NOT NULL DEFAULT '',
    assigned_to text NOT NULL DEFAULT '',
    color text NOT NULL DEFAULT '#3B82F6',
    notes text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_appointments_org_date ON appointments(org_id, start_at);
CREATE INDEX IF NOT EXISTS idx_appointments_org_status ON appointments(org_id, status, start_at);
CREATE INDEX IF NOT EXISTS idx_appointments_customer ON appointments(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_appointments_assigned ON appointments(org_id, assigned_to, start_at) WHERE assigned_to != '';

CREATE TABLE IF NOT EXISTS appointment_slots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    day_of_week int NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    start_time time NOT NULL,
    end_time time NOT NULL,
    slot_minutes int NOT NULL DEFAULT 60,
    max_per_slot int NOT NULL DEFAULT 1,
    UNIQUE(org_id, day_of_week, start_time)
);
```

### API

```
GET    /v1/appointments                     — Listar (filtro por fecha, status, assigned_to)
POST   /v1/appointments                     — Crear turno
GET    /v1/appointments/:id                 — Detalle
PUT    /v1/appointments/:id                 — Actualizar (reprogramar, cambiar status)
DELETE /v1/appointments/:id                 — Cancelar
POST   /v1/appointments/:id/confirm         — Confirmar turno
POST   /v1/appointments/:id/complete        — Marcar como completado
POST   /v1/appointments/:id/no-show         — Marcar como no-show
GET    /v1/appointments/calendar             — Vista calendario (por semana/mes)
GET    /v1/appointments/available-slots      — Horarios disponibles para una fecha
GET    /v1/customers/:id/appointments       — Turnos de un cliente

# Configuracion de horarios
GET    /v1/appointment-slots                — Horarios de atencion
PUT    /v1/appointment-slots                — Configurar horarios (bulk)
```

### Vista calendario

`GET /v1/appointments/calendar?from=2026-03-01&to=2026-03-31`

Retorna turnos agrupados por dia, con la info minima para renderizar un calendario:

```json
{
    "days": {
        "2026-03-05": [
            {"id": "uuid", "title": "Juan - Revision", "start": "09:00", "end": "10:00", "status": "confirmed", "color": "#3B82F6"}
        ]
    }
}
```

### Slots disponibles

`GET /v1/appointments/available-slots?date=2026-03-10&duration=60`

1. Lee `appointment_slots` para ese dia de la semana
2. Lee turnos existentes para esa fecha
3. Retorna slots libres:
```json
{
    "date": "2026-03-10",
    "slots": ["09:00", "10:00", "11:00", "14:00", "15:00", "16:00", "17:00"]
}
```

### Scheduler tasks

```go
{
    Name:     "appointment_reminders",
    Interval: 1 * time.Hour,
    Handler: func(ctx) {
        // 1. Buscar appointments WHERE start_at BETWEEN now+23h AND now+25h AND status = 'confirmed'
        // 2. Para cada uno con customer_phone: generar link WhatsApp de recordatorio
        // 3. Notificar al admin/profesional asignado
    },
},
{
    Name:     "appointment_no_show",
    Interval: 1 * time.Hour,
    Handler: func(ctx) {
        // Marcar como no_show turnos con start_at < now - 30min AND status = 'confirmed'
    },
}
```

### Reglas de negocio

- No se puede crear turno fuera del horario configurado (validar contra `appointment_slots`).
- No se puede crear turno si el slot ya esta lleno (`max_per_slot` alcanzado).
- Turnos cancelados no liberan el slot retroactivamente (para evitar race conditions).
- `assigned_to` es texto libre (nombre del profesional/empleado). En futuro podria ser `user_id`.
- `metadata` permite extension por vertical: el taller guarda `patente`, el medico guarda `motivo_consulta`.
- `duration` en minutos. Default segun `slot_minutes` del horario.
- Vista calendario soporta filtro por `assigned_to` (cada profesional ve sus turnos).
- Completar un turno puede generar una venta asociada (ej: consulta = venta del servicio). Esto es manual, no automatico.

### Extension de tenant_settings

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS appointments_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS appointment_label text NOT NULL DEFAULT 'Turno',
    ADD COLUMN IF NOT EXISTS appointment_reminder_hours int NOT NULL DEFAULT 24;
```

`appointments_enabled`: el kiosco no lo usa, el taller si. Se habilita por org.
`appointment_label`: "Turno", "Cita", "Clase", "Reserva" segun el negocio.

---

## 10. Data I/O — Import/Export CSV y Excel (ex-2)

### Dependencia

```bash
go get github.com/xuri/excelize/v2
```

`excelize` es la libreria estandar para leer/escribir .xlsx en Go.

### Arquitectura

```
internal/dataio/
  usecases.go              — logica de import/export
  handler.go               — endpoints HTTP
  handler/dto/dto.go       — DTOs
  usecases/domain/
    entities.go            — ImportJob, ExportRequest, ImportPreview
  parsers/
    csv.go                 — lectura CSV (encoding/csv)
    xlsx.go                — lectura/escritura Excel (excelize)
```

### Entidades de dominio

```go
type ImportPreview struct {
    FileName    string
    Format      string           // "csv" | "xlsx"
    TotalRows   int
    ValidRows   int
    ErrorRows   int
    Columns     []string         // columnas detectadas
    SampleRows  []map[string]string // primeras 5 filas
    Errors      []ImportError    // errores de validacion
}

type ImportError struct {
    Row     int    `json:"row"`
    Column  string `json:"column"`
    Value   string `json:"value"`
    Message string `json:"message"`
}

type ImportResult struct {
    TotalRows   int
    Created     int
    Updated     int
    Skipped     int
    Errors      []ImportError
}

type ExportRequest struct {
    Entity  string            // "customers", "products", "suppliers", "sales", "cashflow"
    Format  string            // "csv" | "xlsx"
    Filters map[string]string // filtros opcionales (date range, status, etc.)
}
```

### API

```
# Import
POST   /v1/import/:entity/preview   — Sube archivo, retorna preview con validacion
POST   /v1/import/:entity/confirm   — Confirma import despues del preview
GET    /v1/import/templates/:entity  — Descarga template vacio (XLSX con headers)

# Export
GET    /v1/export/:entity            — Exporta a XLSX (default) o CSV (?format=csv)
```

Donde `:entity` es: `customers`, `products`, `suppliers`, `sales`, `cashflow`.

### Flujo de importacion

1. **Upload + Preview** (`POST /v1/import/customers/preview`)
   - Acepta `multipart/form-data` con archivo CSV o XLSX
   - Parsea el archivo, valida cada fila contra las reglas del entity
   - Retorna `ImportPreview` con sample rows, errores, y conteos
   - El archivo se guarda temporalmente en `/tmp` (Lambda tiene 512MB en `/tmp`)
   - Se genera un `preview_id` (uuid) que referencia al archivo temporal

2. **Confirmacion** (`POST /v1/import/customers/confirm`)
   - Body: `{"preview_id": "uuid", "mode": "create_only" | "upsert"}`
   - `create_only`: solo inserta nuevos (ignora duplicados por email/tax_id)
   - `upsert`: actualiza si existe (match por email o tax_id)
   - Inserta en batch (100 filas por transaccion)
   - Retorna `ImportResult`
   - Genera entrada en audit log

3. **Template** (`GET /v1/import/templates/customers`)
   - Retorna XLSX vacio con headers correctos y una fila de ejemplo
   - Headers para customers: `name, type, email, phone, tax_id, address_street, address_city, address_state, address_zip_code, address_country, notes, tags`

### Columnas por entidad

**Customers**:
```
name*, type (person|company), email, phone, tax_id, address_street, address_city, address_state, address_zip_code, address_country, notes, tags (comma-separated)
```

**Products**:
```
name*, type (product|service), sku, price*, cost_price, unit, tax_rate, track_stock (true|false), description, tags (comma-separated)
```

**Suppliers**:
```
name*, email, phone, tax_id, contact_name, address_street, address_city, address_state, address_zip_code, address_country, notes, tags (comma-separated)
```

`*` = obligatorio.

### Flujo de exportacion

1. Query a la DB con filtros del usuario
2. Genera XLSX en memoria con `excelize`
3. Headers en negrita, columnas auto-width
4. Retorna como `Content-Disposition: attachment; filename="customers_2026-03-05.xlsx"`
5. Content-Type: `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`

Para CSV: `text/csv; charset=utf-8` con BOM UTF-8 (para que Excel en Windows lo abra bien con acentos).

### Exportacion de ventas y caja (para el contador)

El export de `sales` incluye columnas extra:
```
number, date, customer_name, payment_method, subtotal, tax_total, total, status, items_summary
```

El export de `cashflow` incluye:
```
date, type, amount, category, description, payment_method, reference_type
```

Estos dos exports aceptan filtro de fecha obligatorio: `?from=2026-01-01&to=2026-03-31`.

### Reglas de negocio

- Limite de archivo: 5MB (suficiente para ~50k filas).
- Limite de filas: 10,000 por import. Para mas, pedir que dividan el archivo.
- Encoding: detectar UTF-8 y Latin-1 automaticamente para CSV (muchos Excel en LATAM exportan Latin-1).
- El preview NO modifica datos — es idempotente y seguro.
- Solo `admin` y roles con permiso `<entity>:import` pueden importar.
- Solo roles con permiso `<entity>:export` pueden exportar.

---

## 11. Attachments — Archivos Adjuntos (S3)

### Dependencia

```bash
go get github.com/aws/aws-sdk-go-v2/service/s3
```

### Entidad de dominio

```go
type Attachment struct {
    ID            uuid.UUID
    OrgID         uuid.UUID
    AttachableType string    // "customer", "product", "sale", "quote", "supplier"
    AttachableID  uuid.UUID
    FileName      string    // nombre original del archivo
    ContentType   string    // MIME type
    SizeBytes     int64
    StorageKey    string    // key en S3: "orgs/{org_id}/{type}/{id}/{uuid}.{ext}"
    UploadedBy    string
    CreatedAt     time.Time
}

type UploadURL struct {
    UploadURL  string    // presigned PUT URL
    StorageKey string    // key que se asigno
    ExpiresAt  time.Time
}
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    attachable_type text NOT NULL,
    attachable_id uuid NOT NULL,
    file_name text NOT NULL,
    content_type text NOT NULL DEFAULT 'application/octet-stream',
    size_bytes bigint NOT NULL DEFAULT 0,
    storage_key text NOT NULL,
    uploaded_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attachments_entity ON attachments(org_id, attachable_type, attachable_id);
CREATE INDEX IF NOT EXISTS idx_attachments_org ON attachments(org_id, created_at DESC);
```

### API

```
POST   /v1/attachments/upload-url   — Solicitar presigned URL para subir
POST   /v1/attachments/confirm      — Confirmar upload exitoso (registra en DB)
GET    /v1/attachments/:id/url      — Obtener presigned URL para descargar
DELETE /v1/attachments/:id          — Eliminar archivo
GET    /v1/:entity/:id/attachments  — Listar attachments de una entidad
```

### Flujo de upload (presigned URL)

1. Frontend llama `POST /v1/attachments/upload-url` con:
   ```json
   {
       "attachable_type": "customer",
       "attachable_id": "uuid",
       "file_name": "foto.jpg",
       "content_type": "image/jpeg",
       "size_bytes": 204800
   }
   ```

2. Backend valida:
   - Tipo de archivo permitido (images, pdf, docs — no ejecutables)
   - Tamanio <= 10MB por archivo
   - Quota de storage por org (segun plan: starter 500MB, growth 5GB, enterprise 50GB)

3. Backend genera presigned PUT URL (expira en 15 min):
   ```go
   key := fmt.Sprintf("orgs/%s/%s/%s/%s%s", orgID, attachableType, attachableID, uuid.New(), ext)
   ```

4. Frontend sube directo a S3 con la presigned URL (no pasa por Lambda).

5. Frontend llama `POST /v1/attachments/confirm` con `storage_key` para registrar en DB.

### Flujo de download

1. `GET /v1/attachments/:id/url`
2. Backend genera presigned GET URL (expira en 1 hora)
3. Frontend redirige o usa la URL directamente

### Storage backend

```go
type StoragePort interface {
    GenerateUploadURL(ctx context.Context, key, contentType string, sizeBytes int64) (string, time.Time, error)
    GenerateDownloadURL(ctx context.Context, key string) (string, time.Time, error)
    DeleteObject(ctx context.Context, key string) error
}
```

Dos implementaciones:
- `S3Storage` — produccion, usa AWS S3 con presigned URLs
- `LocalStorage` — dev local, guarda en `/tmp/attachments/` y sirve via endpoint directo

### Variables de entorno

```env
STORAGE_BACKEND=local          # "s3" | "local"
S3_BUCKET=pymes-attachments
S3_REGION=us-east-1
```

### Reglas de negocio

- Tipos permitidos: `image/jpeg`, `image/png`, `image/webp`, `application/pdf`, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `text/csv`
- Tamanio maximo por archivo: 10MB
- Quota total por org segun plan (verificar antes de generar presigned URL)
- Al eliminar un attachment: borrar de S3 + borrar registro de DB
- Al soft-delete una entidad (customer, product), sus attachments persisten (pueden recuperarse)
- Presigned URLs nunca se cachean en frontend — siempre pedir una nueva

---

## 12. PDF Generation — Recibos y Presupuestos

### Dependencia

```bash
go get github.com/go-pdf/fpdf
```

`fpdf` es la libreria mas simple y madura para PDFs en Go. No requiere templates HTML ni headless browser.

### Arquitectura

```
internal/pdfgen/
  usecases.go              — logica de generacion
  handler.go               — endpoints HTTP
  templates/
    quote.go               — template de presupuesto
    receipt.go             — template de recibo/comprobante de venta
    common.go              — header, footer, estilos comunes
```

### API

```
GET    /v1/quotes/:id/pdf        — Generar PDF de presupuesto
GET    /v1/sales/:id/receipt      — Generar PDF de comprobante de venta
```

### Estructura del PDF

**Header** (comun):
- Logo de la org (si tiene attachment de tipo "org_logo") o nombre de la org
- Datos de la org: nombre, direccion, tax_id, telefono, email
- Se leen de `tenant_settings` (agregar campos en migracion)

**Presupuesto**:
```
┌────────────────────────────────────────┐
│  [LOGO]  NOMBRE DE LA EMPRESA          │
│          Direccion / Tel / Email        │
├────────────────────────────────────────┤
│  PRESUPUESTO N° PRE-00042             │
│  Fecha: 05/03/2026                     │
│  Valido hasta: 20/03/2026             │
├────────────────────────────────────────┤
│  Cliente: Juan Perez                   │
│  Email: juan@example.com              │
├────────────────────────────────────────┤
│  # │ Descripcion   │ Cant │ P.Unit │ Subtotal │
│  1 │ Producto A    │  2   │ $100   │ $200     │
│  2 │ Servicio B    │  1   │ $500   │ $500     │
├────────────────────────────────────────┤
│                      Subtotal: $700    │
│                      IVA 21%:  $147    │
│                      TOTAL:    $847    │
├────────────────────────────────────────┤
│  Notas: Incluye instalacion           │
└────────────────────────────────────────┘
```

**Comprobante de venta**: misma estructura pero con "COMPROBANTE DE VENTA" y metodo de pago.

### Extension de tenant_settings

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS business_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_tax_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_address text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_phone text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_email text NOT NULL DEFAULT '';
```

### Reglas de negocio

- PDFs se generan on-demand, no se almacenan (se pueden regenerar siempre).
- Response: `Content-Type: application/pdf`, `Content-Disposition: inline; filename="PRE-00042.pdf"`.
- Formato de moneda segun `tenant_settings.currency`: `$` para ARS/USD/MXN/CLP/COP, `R$` para BRL, `S/` para PEN.
- Formato de fecha segun locale: `DD/MM/YYYY` para LATAM.
- Si la org no tiene datos de negocio configurados, el PDF se genera igual pero con campos vacios (no bloquear).

---

## 13. Timeline — Activity Timeline por Entidad

### Problema

El audit log es tecnico. El dueno de la pyme necesita ver "que paso con este cliente": cuando se creo, que ventas tuvo, que presupuestos, que notas se agregaron. Un timeline de negocio legible.

### Entidad de dominio

```go
type TimelineEntry struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    EntityType  string     // "customer", "product", "sale", "quote", "supplier"
    EntityID    uuid.UUID
    EventType   string     // "created", "updated", "sale_completed", "quote_sent", "note_added", "attachment_added"
    Title       string     // "Venta VTA-00042 por $847"
    Description string     // detalle opcional
    Actor       string
    Metadata    map[string]any
    CreatedAt   time.Time
}
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS timeline_entries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    event_type text NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    actor text,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_timeline_entity ON timeline_entries(org_id, entity_type, entity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_timeline_org ON timeline_entries(org_id, created_at DESC);
```

### API

```
GET    /v1/:entity/:id/timeline    — Timeline de una entidad (paginado)
POST   /v1/:entity/:id/notes       — Agregar nota manual al timeline
```

### Eventos automaticos

Los usecases existentes generan timeline entries automaticamente:

| Evento | Title | Entity |
|--------|-------|--------|
| Crear cliente | "Cliente creado" | customer |
| Actualizar cliente | "Datos actualizados" | customer |
| Venta a cliente | "Venta VTA-00042 por $847" | customer |
| Presupuesto a cliente | "Presupuesto PRE-00015 por $500" | customer |
| Crear producto | "Producto creado" | product |
| Ajuste de stock | "Stock ajustado: +20 unidades" | product |
| Venta de producto | "Vendido: 3 unidades en VTA-00042" | product |

### Timeline port

```go
type TimelinePort interface {
    Record(ctx context.Context, entry TimelineEntry) error
}
```

Se inyecta en los usecases de customers, products, sales, quotes. Es nil-safe: si nil, no registra.

### Reglas de negocio

- Timeline entries son **inmutables** — no se editan ni borran.
- Las notas manuales (`POST .../notes`) si pueden tener `description` largo (hasta 2000 chars).
- El timeline se muestra en orden cronologico inverso (mas reciente primero).
- Paginacion con cursor, default 50 entries.

---

## 14. Outgoing Webhooks — Webhooks Salientes

### Problema

La pyme necesita conectar con sus propias herramientas: Zapier, n8n, bots de WhatsApp, ERPs externos. Los webhooks salientes permiten notificar a URLs externas cuando ocurren eventos.

### Entidades de dominio

```go
type WebhookEndpoint struct {
    ID        uuid.UUID
    OrgID     uuid.UUID
    URL       string
    Secret    string       // para firmar payloads (HMAC-SHA256)
    Events    []string     // "sale.created", "sale.voided", "customer.created", "quote.accepted", etc.
    IsActive  bool
    CreatedBy string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type WebhookDelivery struct {
    ID          uuid.UUID
    EndpointID  uuid.UUID
    EventType   string
    Payload     map[string]any
    StatusCode  int
    ResponseBody string
    Attempts    int
    NextRetry   *time.Time
    DeliveredAt *time.Time
    CreatedAt   time.Time
}
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    url text NOT NULL,
    secret text NOT NULL,
    events text[] NOT NULL DEFAULT '{}',
    is_active boolean NOT NULL DEFAULT true,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_org ON webhook_endpoints(org_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id uuid NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    status_code int,
    response_body text NOT NULL DEFAULT '',
    attempts int NOT NULL DEFAULT 0,
    next_retry timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint ON webhook_deliveries(endpoint_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(next_retry) WHERE delivered_at IS NULL AND attempts < 5;
```

### API

```
GET    /v1/webhooks                   — Listar endpoints de la org
POST   /v1/webhooks                   — Crear endpoint
GET    /v1/webhooks/:id               — Detalle
PUT    /v1/webhooks/:id               — Actualizar (URL, events, active)
DELETE /v1/webhooks/:id               — Eliminar endpoint
GET    /v1/webhooks/:id/deliveries    — Historial de envios
POST   /v1/webhooks/:id/test          — Enviar evento de prueba
```

### Eventos disponibles

```
customer.created, customer.updated, customer.deleted
supplier.created, supplier.updated, supplier.deleted
product.created, product.updated, product.deleted
sale.created, sale.voided
quote.created, quote.accepted, quote.rejected
cashflow.created
inventory.adjusted, inventory.low_stock
```

### Firma del payload

Cada delivery se firma con HMAC-SHA256 usando el `secret` del endpoint:

```
X-Webhook-Signature: sha256={hmac}
X-Webhook-Event: sale.created
X-Webhook-ID: {delivery_id}
X-Webhook-Timestamp: {unix_timestamp}
```

Mensaje firmado: `{webhook_id}.{timestamp}.{body}`

### Disparo y reintentos

```go
type WebhookDispatcher interface {
    Dispatch(ctx context.Context, orgID uuid.UUID, eventType string, payload map[string]any) error
}
```

**En Lambda**: el dispatch es sincronico pero con timeout corto (5 segundos por endpoint). Si falla, se registra el delivery con `next_retry` y se reintenta en el proximo request que toque el scheduler (ver modulo 9).

**Reintentos**: hasta 5 intentos con backoff exponencial (1min, 5min, 30min, 2hr, 12hr). Despues de 5 fallos, se marca como failed y se desactiva el endpoint si acumula 10 fallos consecutivos.

### Reglas de negocio

- Maximo 5 endpoints por org en plan starter, 20 en growth, ilimitado en enterprise.
- El `secret` se genera automaticamente al crear el endpoint (32 bytes hex).
- El `secret` se muestra una sola vez al crear (como las API keys).
- Los deliveries se retienen 30 dias.
- El payload incluye siempre: `event`, `org_id`, `timestamp`, `data` (la entidad completa).
- Solo `admin` puede gestionar webhooks.

---

## 15. WhatsApp — Integracion Basica

### Problema

En LATAM, WhatsApp es el canal principal de comunicacion con clientes. No es necesario integrar la API de WhatsApp Business (compleja y costosa) — con links `wa.me` y mensajes prearmados se cubre el 80% del caso de uso.

### Funcionalidad

No es un modulo con DB propia. Es una utilidad que se integra en los modulos existentes.

### API

```
GET    /v1/whatsapp/quote/:id         — Genera link de WhatsApp con presupuesto
GET    /v1/whatsapp/sale/:id/receipt   — Genera link de WhatsApp con comprobante
GET    /v1/whatsapp/customer/:id/message — Genera link de WhatsApp con mensaje custom
```

### Respuesta

```json
{
    "whatsapp_url": "https://wa.me/5491112345678?text=Hola%20Juan%2C%20te%20enviamos...",
    "phone": "+5491112345678",
    "message": "Hola Juan, te enviamos el presupuesto PRE-00042 por $847. Podes verlo en: https://app.pymes.com/q/abc123"
}
```

### Templates de mensajes

Configurables por org en `tenant_settings`:

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS wa_quote_template text NOT NULL DEFAULT 'Hola {customer_name}, te enviamos el presupuesto {number} por {total}.',
    ADD COLUMN IF NOT EXISTS wa_receipt_template text NOT NULL DEFAULT 'Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!',
    ADD COLUMN IF NOT EXISTS wa_default_country_code text NOT NULL DEFAULT '54';
```

### Logica

```go
func BuildWhatsAppURL(phone, countryCode, message string) string {
    // 1. Limpiar telefono: remover espacios, guiones, parentesis
    // 2. Si no empieza con "+", agregar countryCode
    // 3. URL encode del mensaje
    // 4. Retornar "https://wa.me/{phone}?text={encoded_message}"
}
```

### Reglas de negocio

- Si el cliente no tiene telefono, retornar error 422 con mensaje claro.
- El template se interpola server-side con los datos reales.
- El link abre WhatsApp en el dispositivo del usuario (no envia automaticamente).
- `wa_default_country_code` se usa cuando el telefono no tiene codigo de pais.

---

## 16. Dashboard — KPIs Configurables

### Problema

El dueno de la pyme necesita ver de un vistazo: ventas del dia, plata en caja, stock bajo, presupuestos pendientes. El dashboard actual es un placeholder.

### API

```
GET    /v1/dashboard                  — KPIs del dashboard
GET    /v1/dashboard/config           — Configuracion de widgets
PUT    /v1/dashboard/config           — Actualizar configuracion
```

### Respuesta de `/v1/dashboard`

```json
{
    "period": "today",
    "kpis": {
        "sales_today": {
            "value": 15400.00,
            "count": 8,
            "currency": "ARS",
            "trend": 12.5
        },
        "sales_month": {
            "value": 285000.00,
            "count": 142,
            "currency": "ARS",
            "trend": -3.2
        },
        "cashflow_balance": {
            "income": 285000.00,
            "expense": 180000.00,
            "balance": 105000.00,
            "currency": "ARS"
        },
        "pending_quotes": {
            "count": 5,
            "total_value": 42000.00
        },
        "low_stock_products": {
            "count": 3,
            "items": [
                {"name": "Producto X", "quantity": 2, "min_quantity": 10}
            ]
        },
        "top_products_month": [
            {"name": "Producto A", "quantity_sold": 45, "revenue": 90000.00}
        ],
        "recent_sales": [
            {"number": "VTA-00142", "customer": "Juan", "total": 2500.00, "time": "14:30"}
        ]
    }
}
```

### Widgets disponibles

| Widget | Descripcion | Query |
|--------|-------------|-------|
| `sales_today` | Ventas del dia (monto + cantidad) | `sales WHERE created_at >= today AND status = completed` |
| `sales_month` | Ventas del mes | `sales WHERE created_at >= first_of_month` |
| `sales_trend` | Comparacion con periodo anterior (%) | comparar con mes/dia anterior |
| `cashflow_balance` | Balance de caja del mes | `SUM(income) - SUM(expense)` |
| `pending_quotes` | Presupuestos en estado draft/sent | `quotes WHERE status IN (draft, sent)` |
| `low_stock_products` | Productos con stock bajo | `stock_levels WHERE quantity <= min_quantity` |
| `top_products_month` | Top 5 productos mas vendidos | `sale_items GROUP BY product_id ORDER BY SUM(quantity) DESC LIMIT 5` |
| `recent_sales` | Ultimas 10 ventas | `sales ORDER BY created_at DESC LIMIT 10` |
| `customers_month` | Clientes nuevos del mes | `customers WHERE created_at >= first_of_month` |

### Configuracion

```sql
CREATE TABLE IF NOT EXISTS dashboard_configs (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    widgets jsonb NOT NULL DEFAULT '["sales_today","sales_month","cashflow_balance","pending_quotes","low_stock_products","top_products_month","recent_sales"]',
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

El usuario elige que widgets ver y en que orden. Default: todos habilitados.

### Reglas de negocio

- El dashboard es **una sola query compuesta** (o pocas queries paralelas). No N+1.
- `trend` se calcula comparando el periodo actual vs el anterior (ej: ventas hoy vs ayer, mes actual vs mes anterior).
- Los KPIs se calculan en tiempo real, no se cachean (la DB es rapida para estos aggregates con indices correctos).
- El dashboard respeta permisos RBAC: un `vendedor` no ve `cashflow_balance`.

---

## 17. Scheduler — Tareas Programadas

### Problema

Hay tareas que deben ejecutarse periodicamente:
- Expirar presupuestos vencidos (`valid_until < now()`)
- Reintentar webhook deliveries fallidos
- Alertas de stock bajo
- Limpiar archivos temporales de import
- Limpiar webhook deliveries viejos (>30 dias)

### Arquitectura

En Lambda no hay cron nativo. Se usa **EventBridge Scheduler** que invoca un endpoint dedicado.

### API

```
POST   /v1/internal/scheduler/run    — Ejecuta tareas pendientes
```

Este endpoint esta protegido: solo acepta requests de EventBridge (verificar header `X-Scheduler-Secret` que viene como env var).

### Entidad de dominio

```go
type ScheduledTask struct {
    Name     string
    Interval time.Duration
    Handler  func(ctx context.Context) error
}
```

### Tasks

```go
var tasks = []ScheduledTask{
    {
        Name:     "expire_quotes",
        Interval: 1 * time.Hour,
        Handler:  func(ctx) { quotesUC.ExpireOverdue(ctx) },
    },
    {
        Name:     "retry_webhooks",
        Interval: 5 * time.Minute,
        Handler:  func(ctx) { outwebhooksUC.RetryPending(ctx) },
    },
    {
        Name:     "low_stock_alerts",
        Interval: 6 * time.Hour,
        Handler:  func(ctx) { inventoryUC.SendLowStockAlerts(ctx) },
    },
    {
        Name:     "cleanup_temp_files",
        Interval: 24 * time.Hour,
        Handler:  func(ctx) { dataioUC.CleanupTempFiles(ctx) },
    },
    {
        Name:     "cleanup_old_deliveries",
        Interval: 24 * time.Hour,
        Handler:  func(ctx) { outwebhooksUC.CleanupOldDeliveries(ctx, 30) },
    },
}
```

### Tabla de control

```sql
CREATE TABLE IF NOT EXISTS scheduler_runs (
    task_name text NOT NULL,
    last_run_at timestamptz NOT NULL DEFAULT now(),
    next_run_at timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'ok',
    error_message text NOT NULL DEFAULT '',
    PRIMARY KEY (task_name)
);
```

### Logica

1. EventBridge invoca `POST /v1/internal/scheduler/run` cada 5 minutos.
2. El handler itera las tasks, consulta `scheduler_runs` para ver si toca ejecutar.
3. Si `now() >= next_run_at`, ejecuta la task y actualiza `last_run_at` y `next_run_at`.
4. Usa `SELECT ... FOR UPDATE` para evitar ejecucion concurrente (multiples Lambdas).

### Infra (Terraform)

```hcl
resource "aws_scheduler_schedule" "cron" {
  name       = "${var.project}-scheduler"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = "rate(5 minutes)"

  target {
    arn      = aws_lambda_function.api.arn
    role_arn = aws_iam_role.scheduler.arn

    input = jsonencode({
      httpMethod = "POST"
      path       = "/v1/internal/scheduler/run"
      headers    = { "X-Scheduler-Secret" = var.scheduler_secret }
    })
  }
}
```

### Reglas de negocio

- El scheduler NO es user-facing. No tiene UI.
- Cada task tiene su propio intervalo. El trigger de EventBridge es cada 5 min, pero cada task decide si le toca ejecutar.
- Si una task falla, logea el error y continua con las demas. No bloquea.
- Timeout total del scheduler: 60 segundos (las tasks deben ser rapidas).

---

## Migraciones SQL

### `0008_transversal_core.up.sql` — Tablas de negocio transversal

Crea las tablas nuevas de negocio:

- `purchases` + `purchase_items`
- `accounts` + `account_movements`
- `payments`
- `returns` + `return_items` + `credit_notes`
- `price_lists` + `price_list_items`
- `recurring_expenses`
- `appointments` + `appointment_slots`

Y los ALTERs a tablas existentes:

- `sales`: agregar `amount_paid`, `payment_status`
- `sales` + `quotes`: agregar `discount_type`, `discount_value`, `discount_total`
- `sale_items` + `quote_items`: agregar `discount_type`, `discount_value`
- `customers`: agregar `price_list_id`

### `0009_transversal_infra.up.sql` — Tablas de infraestructura transversal

- `roles` + `role_permissions` + `user_roles`
- `attachments`
- `timeline_entries`
- `webhook_endpoints` + `webhook_deliveries`
- `dashboard_configs`
- `scheduler_runs`

### `0010_tenant_settings_ext.up.sql` — Extension de tenant_settings

```sql
ALTER TABLE tenant_settings
    -- Purchases
    ADD COLUMN IF NOT EXISTS purchase_prefix text NOT NULL DEFAULT 'CPA',
    ADD COLUMN IF NOT EXISTS next_purchase_number int NOT NULL DEFAULT 1,
    -- Returns
    ADD COLUMN IF NOT EXISTS return_prefix text NOT NULL DEFAULT 'DEV',
    ADD COLUMN IF NOT EXISTS credit_note_prefix text NOT NULL DEFAULT 'NC',
    ADD COLUMN IF NOT EXISTS next_return_number int NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS next_credit_note_number int NOT NULL DEFAULT 1,
    -- Business info (PDF)
    ADD COLUMN IF NOT EXISTS business_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_tax_id text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_address text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_phone text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS business_email text NOT NULL DEFAULT '',
    -- WhatsApp
    ADD COLUMN IF NOT EXISTS wa_quote_template text NOT NULL DEFAULT 'Hola {customer_name}, te enviamos el presupuesto {number} por {total}.',
    ADD COLUMN IF NOT EXISTS wa_receipt_template text NOT NULL DEFAULT 'Hola {customer_name}, tu comprobante de compra {number} por {total}. Gracias por tu compra!',
    ADD COLUMN IF NOT EXISTS wa_default_country_code text NOT NULL DEFAULT '54',
    -- Appointments
    ADD COLUMN IF NOT EXISTS appointments_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS appointment_label text NOT NULL DEFAULT 'Turno',
    ADD COLUMN IF NOT EXISTS appointment_reminder_hours int NOT NULL DEFAULT 24;
```

### `0011_rbac_seed.up.sql` — Roles del sistema y seed data

```sql
-- Roles del sistema para el org de desarrollo local
-- admin, vendedor, cajero, contador, almacenero
-- con sus permisos correspondientes (incluyendo purchases, accounts, returns, appointments)
-- Lista de precios default "Minorista" para el org local
```

---

## Dependencias nuevas

```bash
go get github.com/xuri/excelize/v2
go get github.com/go-pdf/fpdf
go get github.com/aws/aws-sdk-go-v2/service/s3
```

---

## Variables de entorno nuevas

```env
# ── Storage (Attachments) ──
STORAGE_BACKEND=local              # "s3" | "local"
S3_BUCKET=pymes-attachments
S3_REGION=us-east-1

# ── Scheduler ──
SCHEDULER_SECRET=change-me-in-prod
```

---

## Interacciones entre modulos

```
Sale (crear, pago inmediato)
  ├── Inventory: stock_movement(out) (existente)
  ├── Payments: registrar Payment automatico
  ├── Cashflow: cash_movement(income) por cada payment
  ├── Price Lists: resolver precio segun lista del cliente
  ├── Discounts: aplicar descuentos por item y global
  ├── Audit: audit_log entry (existente)
  ├── Timeline: entry en customer + cada product
  └── Webhooks: dispatch "sale.created"

Sale (crear, fiada/credito)
  ├── Inventory: stock_movement(out)
  ├── Accounts: charge en cuenta corriente del cliente
  ├── NO cashflow (la plata no entro)
  ├── Audit + Timeline + Webhooks
  └── payment_status = 'pending'

Sale (pago parcial posterior)
  ├── Payments: registrar Payment
  ├── Accounts: payment en cuenta corriente del cliente
  ├── Cashflow: cash_movement(income)
  └── Actualizar amount_paid y payment_status

Purchase (recibir, pago inmediato)
  ├── Inventory: stock_movement(in, reason=purchase)
  ├── Payments: registrar Payment automatico
  ├── Cashflow: cash_movement(expense)
  ├── Products: actualizar cost_price si cambio
  ├── Audit + Timeline + Webhooks
  └── payment_status = 'paid'

Purchase (recibir, a credito)
  ├── Inventory: stock_movement(in, reason=purchase)
  ├── Accounts: charge en cuenta corriente del proveedor
  ├── NO cashflow
  └── payment_status = 'pending'

Return (devolucion)
  ├── Inventory: stock_movement(in, reason=return)
  ├── Cashflow: cash_movement(expense) SI reembolso directo
  ├── Credit Notes: crear nota de credito SI refund_method=credit_note
  ├── Accounts: ajustar cuenta corriente si venta era fiada
  ├── Audit + Timeline
  └── Webhooks: dispatch "return.created"

Recurring Expense (pagar)
  ├── Cashflow: cash_movement(expense)
  ├── Accounts: movement en cuenta del proveedor (si aplica)
  └── Actualizar next_due_date

Appointment (completar)
  ├── Timeline: entry en customer
  ├── (Opcionalmente genera venta manual del servicio)
  └── Webhooks: dispatch "appointment.completed"

Quote (aceptar → venta)
  ├── Sale: crea venta con items + descuentos del presupuesto
  ├── (la venta dispara sus propios efectos)
  ├── Timeline: entry en customer + quote
  └── Webhooks: dispatch "quote.accepted"

Scheduler (periodico)
  ├── Expire quotes: quotes vencidos → status=expired
  ├── Recurring reminders: alertar gastos proximos a vencer
  ├── Appointment reminders: notificar turnos de manana
  ├── Appointment no-show: marcar turnos pasados sin asistir
  ├── Retry webhooks: reintentar deliveries fallidos
  ├── Low stock alerts: notificar productos bajo minimo
  └── Cleanup: temp files, old deliveries
```

---

## Integracion en bootstrap.go

```go
// ── Nuevos repos ──
rbacRepo := rbac.NewRepository(db)
purchasesRepo := purchases.NewRepository(db)
accountsRepo := accounts.NewRepository(db)
paymentsRepo := payments.NewRepository(db)
returnsRepo := returns.NewRepository(db)
pricelistsRepo := pricelists.NewRepository(db)
recurringRepo := recurring.NewRepository(db)
appointmentsRepo := appointments.NewRepository(db)
attachmentRepo := attachments.NewRepository(db)
timelineRepo := timeline.NewRepository(db)
outwebhookRepo := outwebhooks.NewRepository(db)
dashboardRepo := dashboard.NewRepository(db)

// Storage
storage := attachments.NewStorage(cfg.StorageBackend, cfg.S3Bucket, cfg.S3Region)

// ── Nuevos usecases ──
rbacUC := rbac.NewUsecases(rbacRepo, auditUC)
accountsUC := accounts.NewUsecases(accountsRepo, cashflowUC, auditUC)
paymentsUC := payments.NewUsecases(paymentsRepo, accountsUC, cashflowUC, auditUC)
pricelistsUC := pricelists.NewUsecases(pricelistsRepo, auditUC)
returnsUC := returns.NewUsecases(returnsRepo, inventoryUC, cashflowUC, accountsUC, auditUC)
recurringUC := recurring.NewUsecases(recurringRepo, cashflowUC, accountsUC, auditUC)
appointmentsUC := appointments.NewUsecases(appointmentsRepo, auditUC)

// Actualizar sales y purchases para usar accounts, payments, pricelists
salesUC := sales.NewUsecases(salesRepo, inventoryUC, cashflowUC, accountsUC, paymentsUC, pricelistsUC, auditUC)
purchasesUC := purchases.NewUsecases(purchasesRepo, inventoryUC, cashflowUC, accountsUC, paymentsUC, auditUC)

attachmentUC := attachments.NewUsecases(attachmentRepo, storage, auditUC)
timelineUC := timeline.NewUsecases(timelineRepo)
outwebhookUC := outwebhooks.NewUsecases(outwebhookRepo)
dashboardUC := dashboard.NewUsecases(salesRepo, cashflowRepo, inventoryRepo, quotesRepo, customersRepo, accountsRepo, appointmentsRepo, recurringRepo)
dataioUC := dataio.NewUsecases(customersRepo, productsRepo, suppliersRepo, salesRepo, cashflowRepo, auditUC)
pdfgenUC := pdfgen.NewUsecases(quotesRepo, salesRepo, adminRepo)
waUC := whatsapp.NewUsecases(customersRepo, quotesRepo, salesRepo, adminRepo)

// RBAC middleware
rbacMiddleware := handlers.NewRBACMiddleware(rbacUC)

// Inyectar timeline y webhooks en usecases existentes (setter injection, nil-safe)
salesUC.SetTimeline(timelineUC)
salesUC.SetWebhookDispatcher(outwebhookUC)
purchasesUC.SetTimeline(timelineUC)
purchasesUC.SetWebhookDispatcher(outwebhookUC)
customersUC.SetTimeline(timelineUC)
customersUC.SetWebhookDispatcher(outwebhookUC)
productsUC.SetTimeline(timelineUC)
appointmentsUC.SetTimeline(timelineUC)
returnsUC.SetWebhookDispatcher(outwebhookUC)

// ── Nuevos handlers ──
rbacHandler := rbac.NewHandler(rbacUC)
purchasesHandler := purchases.NewHandler(purchasesUC)
accountsHandler := accounts.NewHandler(accountsUC)
paymentsHandler := payments.NewHandler(paymentsUC)
returnsHandler := returns.NewHandler(returnsUC)
pricelistsHandler := pricelists.NewHandler(pricelistsUC)
recurringHandler := recurring.NewHandler(recurringUC)
appointmentsHandler := appointments.NewHandler(appointmentsUC)
attachmentHandler := attachments.NewHandler(attachmentUC)
timelineHandler := timeline.NewHandler(timelineUC)
outwebhookHandler := outwebhooks.NewHandler(outwebhookUC)
dashboardHandler := dashboard.NewHandler(dashboardUC)
dataioHandler := dataio.NewHandler(dataioUC)
pdfgenHandler := pdfgen.NewHandler(pdfgenUC)
waHandler := whatsapp.NewHandler(waUC)
schedulerHandler := scheduler.NewHandler(cfg.SchedulerSecret, quotesUC, outwebhookUC, inventoryUC, dataioUC, recurringUC, appointmentsUC)

// ── Registrar rutas (con RBAC middleware) ──
rbacHandler.RegisterRoutes(authGroup)
purchasesHandler.RegisterRoutes(authGroup, rbacMiddleware)
accountsHandler.RegisterRoutes(authGroup, rbacMiddleware)
paymentsHandler.RegisterRoutes(authGroup, rbacMiddleware)
returnsHandler.RegisterRoutes(authGroup, rbacMiddleware)
pricelistsHandler.RegisterRoutes(authGroup, rbacMiddleware)
recurringHandler.RegisterRoutes(authGroup, rbacMiddleware)
appointmentsHandler.RegisterRoutes(authGroup, rbacMiddleware)
attachmentHandler.RegisterRoutes(authGroup, rbacMiddleware)
dashboardHandler.RegisterRoutes(authGroup, rbacMiddleware)
dataioHandler.RegisterRoutes(authGroup, rbacMiddleware)
pdfgenHandler.RegisterRoutes(authGroup)
waHandler.RegisterRoutes(authGroup)
outwebhookHandler.RegisterRoutes(authGroup)

// Scheduler (ruta interna, no auth JWT — protegida por secret)
schedulerHandler.RegisterRoutes(v1)
```

---

## Orden de implementacion recomendado

1. Migraciones SQL (`0008`, `0009`, `0010`, `0011`)
2. `rbac` — roles, permisos, middleware (impacta todos los handlers)
3. `accounts` — cuentas corrientes (dependency de purchases, sales fiadas, returns)
4. `payments` — pagos parciales y multiples medios (dependency de sales y purchases)
5. `discounts` — descuentos en sales y quotes (ALTERs + logica de calculo)
6. `pricelists` — listas de precios (dependency de sales al resolver precio)
7. `purchases` — compras a proveedores (usa accounts, payments, inventory)
8. `returns` — devoluciones y notas de credito (usa accounts, inventory)
9. `recurring` — gastos recurrentes
10. `appointments` — turnos y citas
11. `dataio` — import/export CSV y Excel
12. `attachments` — archivos (dependency de pdfgen y verticales)
13. `pdfgen` — PDFs de presupuestos y ventas
14. `timeline` — activity timeline
15. `outwebhooks` — webhooks salientes
16. `whatsapp` — links de WhatsApp
17. `dashboard` — KPIs (depende de todos los demas para mostrar datos)
18. `scheduler` — tareas programadas (depende de quotes, webhooks, recurring, appointments)
19. Actualizar handlers existentes de Prompt 01 para usar RBAC middleware + payments + discounts + pricelists
20. Tests unitarios + E2E
21. Actualizar Terraform (EventBridge, S3 bucket)

---

## Criterios de exito

- [ ] `go build ./...` compila sin errores
- [ ] `go test ./...` todos los tests pasan
- [ ] RBAC: admin puede todo, vendedor solo puede crear ventas, cajero no puede ver reportes
- [ ] Purchases: crear compra → recibir → stock ingresa → deuda con proveedor generada
- [ ] Accounts: venta fiada → saldo en cuenta del cliente → pago parcial → saldo actualizado
- [ ] Accounts: compra a credito → saldo con proveedor → pago → saldo en 0
- [ ] Payments: venta con 2 medios de pago → 2 payments → 2 cash_movements
- [ ] Returns: devolucion parcial → stock devuelto → nota de credito generada
- [ ] Returns: aplicar nota de credito a venta nueva → saldo descontado
- [ ] Discounts: venta con descuento por item + descuento global → total correcto
- [ ] Price Lists: cliente con lista mayorista → precio resuelto automaticamente al crear venta
- [ ] Recurring: crear gasto mensual → pagar → cash_movement generado → next_due_date actualizado
- [ ] Appointments: crear turno → confirmar → completar → timeline del cliente actualizado
- [ ] Appointments: verificar slots disponibles → no permitir doble booking
- [ ] Import: subir CSV de clientes → preview → confirmar → clientes creados
- [ ] Export: descargar XLSX de ventas del mes con filtro de fecha
- [ ] Attachments: subir foto a un producto via presigned URL, descargar via presigned URL
- [ ] PDF: generar PDF de presupuesto con datos de la org y items
- [ ] Timeline: ver historial de un cliente (creacion, ventas, pagos, notas)
- [ ] Webhooks: crear endpoint → crear venta → delivery registrado
- [ ] WhatsApp: generar link wa.me con presupuesto para un cliente con telefono
- [ ] Dashboard: ver KPIs (ventas hoy, caja, deudas, stock bajo, presupuestos pendientes, turnos del dia)
- [ ] Scheduler: expirar presupuestos, recordar gastos, recordar turnos automaticamente
- [ ] Tests E2E cubren: compras, fiado, pagos parciales, devoluciones, descuentos, listas de precio, turnos
