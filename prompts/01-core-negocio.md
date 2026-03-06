# Prompt 01 — Core de Negocio para Pymes

## Contexto

Este prompt extiende el **control-plane** (Prompt 00) con los módulos de negocio que toda pyme necesita, sin importar el vertical. Son las entidades que hoy se gestionan con papel, Excel y WhatsApp.

**Prerequisito**: el control-plane ya está implementado y funcional (auth, billing, tenants, audit, notificaciones).

**Regla fundamental**: estos módulos viven dentro de `control-plane/backend/internal/` porque comparten la misma DB, el mismo auth, el mismo Lambda, y el mismo tenant (`org_id`). NO son un servicio separado.

## Alcance obligatorio

Todos los módulos, reglas de negocio, validaciones, transacciones, errores y tests definidos en este prompt son parte del alcance requerido. No deben reinterpretarse como backlog opcional ni como mejoras para "más adelante".

Si una parte parece más simple o más compleja, eso no altera su obligatoriedad. La implementación puede hacerse por dependencia técnica, pero el objetivo final sigue siendo completar **todo** este prompt.

---

## Módulos a implementar

| Módulo | Descripción | Prioridad |
|--------|-------------|-----------|
| `customers` | Clientes de la pyme | 1 |
| `suppliers` | Proveedores | 2 |
| `products` | Catálogo de productos y servicios | 3 |
| `inventory` | Stock por producto y depósito | 4 |
| `quotes` | Presupuestos / cotizaciones | 5 |
| `sales` | Ventas y comprobantes | 6 |
| `cashflow` | Caja: ingresos, egresos, cierre | 7 |
| `reports` | Reportes básicos (ventas, stock, clientes) | 8 |

---

## Tipos numéricos

En Go se usa `float64` para montos. En PostgreSQL se usa `numeric(15,2)` para garantizar precisión en la capa de persistencia. GORM convierte automáticamente entre ambos. Esta decisión prioriza la simplicidad y consistencia del código Go sobre la precisión absoluta en memoria — los errores de redondeo de float64 son despreciables para los montos de una pyme (hasta ~15 dígitos de precisión), y la DB es la fuente de verdad con `numeric`.

---

## Principios de diseño

1. **Multi-tenant**: toda tabla tiene `org_id`. Un cliente de la pyme A no es visible para la pyme B.
2. **Extensible por verticales**: los campos base son genéricos. Los verticales agregan campos via `metadata jsonb` o tablas de extensión. Ejemplo: salud agrega `obra_social` al cliente, talleres agrega `patente`.
3. **Soft delete**: clientes, proveedores y productos usan `deleted_at` (no se borran físicamente). Las ventas y movimientos de caja NUNCA se borran.
4. **Auditable**: operaciones CUD (create/update/delete) generan entrada en el audit log existente.
5. **Moneda**: todos los montos son `numeric(15,2)`. El campo `currency` (ISO 4217: ARS, USD, BRL, CLP, MXN, COP, PEN) se define a nivel de `tenant_settings`. No hay conversión de monedas; cada org opera en una sola moneda.
6. **Impuestos**: el sistema registra monto neto, impuesto y total. La lógica de cálculo de impuestos varía por país y se inyecta como configuración, NO como código hardcodeado. Para la base: solo un porcentaje de IVA configurable por org.
7. **Numeración de comprobantes**: secuencial por org, configurable (prefijo + número). No es factura electrónica — eso es del vertical o integración futura.
8. **Domain Errors**: cada módulo define sus errores específicos usando `apperror.Error` (ver Prompt 00, E1). Los handlers nunca formatean errores — usan `c.Error(err)`.
9. **Transacciones explícitas**: toda operación que impacta múltiples tablas (venta → stock → caja) usa `db.Transaction(ctx, fn)` (ver Prompt 00, E4).
10. **Validación en 2 capas**: binding tags en DTOs para formato + validaciones de negocio en usecases (ver Prompt 00, E3).

---

## Domain Errors (aplicados a todos los módulos)

```go
// Errores comunes de negocio reutilizables en todos los módulos del core
var (
    ErrPartyNotFound       = func(id string) *apperror.Error { return apperror.NewNotFound("party", id) }
    ErrProductNotFound     = func(id string) *apperror.Error { return apperror.NewNotFound("product", id) }
    ErrQuoteNotFound       = func(id string) *apperror.Error { return apperror.NewNotFound("quote", id) }
    ErrSaleNotFound        = func(id string) *apperror.Error { return apperror.NewNotFound("sale", id) }

    ErrQuoteNotDraft       = apperror.NewBusinessRule("Solo se pueden editar presupuestos en estado 'draft'")
    ErrSaleImmutable       = apperror.NewBusinessRule("Las ventas no se pueden editar, solo anular")
    ErrSaleAlreadyVoided   = apperror.NewBusinessRule("La venta ya está anulada")
    ErrStockInsufficient   = func(product string, available, requested float64) *apperror.Error {
        return apperror.NewBusinessRule(fmt.Sprintf("Stock insuficiente para '%s': disponible %.2f, solicitado %.2f", product, available, requested))
    }
    ErrTaxIDDuplicate      = func(taxID string) *apperror.Error { return apperror.NewConflict(fmt.Sprintf("Ya existe una entidad con CUIT/RUT '%s' en esta organización", taxID)) }
    ErrSKUDuplicate        = func(sku string) *apperror.Error { return apperror.NewConflict(fmt.Sprintf("Ya existe un producto con SKU '%s'", sku)) }
    ErrInvalidTotalCalc    = apperror.NewBusinessRule("Los totales no coinciden con el cálculo server-side")
)
```

---

## Arquitectura (misma que control-plane)

Cada módulo sigue la estructura hexagonal existente:

```
internal/<modulo>/
  usecases.go              — lógica de negocio
  handler.go               — Gin HTTP adapter
  repository.go            — GORM/PostgreSQL adapter
  handler/dto/dto.go       — DTOs HTTP
  usecases/domain/         — entidades de dominio
  repository/models/       — modelos GORM
```

Los handlers registran rutas en `wire/bootstrap.go` via `RegisterRoutes(authGroup)`.

---

## 1. Customers (Clientes) — via Party Model

### Concepto

**No existe tabla `customers`.** Un "cliente" es un `party` (persona u organización) con `party_role.role = 'customer'`. La API `/v1/customers` es un **alias de conveniencia** que internamente filtra parties por rol. Esto permite que un mismo registro sea cliente Y proveedor simultáneamente sin duplicación.

### Entidad de dominio (vista de negocio)

```go
// Customer es una vista de Party + PartyRole(customer)
type Customer struct {
    ID          uuid.UUID      // = party.id
    OrgID       uuid.UUID
    PartyType   string         // "person" | "organization"
    Name        string         // = party.display_name
    TaxID       string
    Email       string
    Phone       string
    Address     Address
    Notes       string
    Tags        []string
    Metadata    map[string]any
    // Extension según party_type
    Person       *PartyPerson
    Organization *PartyOrganization
    // Role-specific
    RoleMetadata map[string]any // price_list_id, credit_limit, etc.
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
}

type Address struct {
    Street  string `json:"street"`
    City    string `json:"city"`
    State   string `json:"state"`
    ZipCode string `json:"zip_code"`
    Country string `json:"country"`
}
```

### API

```
GET    /v1/customers              — Listar parties con rol 'customer' (paginado, filtro por name/email/tag/party_type, search)
POST   /v1/customers              — Crear party + asignar rol 'customer'
GET    /v1/customers/:id          — Detalle (party + extensión + roles)
PUT    /v1/customers/:id          — Actualizar party
DELETE /v1/customers/:id          — Soft delete del party
GET    /v1/customers/:id/sales    — Historial de ventas del cliente
# Futuro: GET /v1/customers/export (CSV), POST /v1/customers/import (CSV bulk)
```

### Implementación SQL

No hay tabla `customers`. La query es:

```sql
SELECT p.*, pr.metadata AS role_metadata
FROM parties p
JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id
WHERE pr.role = 'customer'
  AND pr.is_active = true
  AND p.org_id = $1
  AND p.deleted_at IS NULL
ORDER BY p.display_name;
```

### Reglas de negocio

- `display_name` es obligatorio, mínimo 2 caracteres.
- `tax_id` es único por org (si se provee). Validar formato según país es futuro.
- `email` no es obligatorio (muchas pymes LATAM no tienen email de sus clientes).
- Soft delete: `DELETE` setea `deleted_at`. Los queries filtran `WHERE deleted_at IS NULL`.
- La búsqueda (`?search=`) busca en `display_name`, `email`, `phone`, `tax_id` con `ILIKE`.
- Paginación con cursor (`?after=<uuid>&limit=20`).
- Al crear un customer, el handler: (1) crea el party con el `party_type` indicado, (2) crea la extensión (person/organization), (3) crea `party_role(role='customer')`.
- Si el party ya existe (por `tax_id` o `email`), se agrega el rol `customer` sin duplicar el party.
- **Backward compatibility**: el DTO de request/response de `/v1/customers` usa campos como `name` (mapeado a `display_name`), `type` (`person`/`company` → `party_type` `person`/`organization`) para que el frontend no necesite saber del Party Model.

---

## 2. Suppliers (Proveedores) — via Party Model

### Concepto

Igual que clientes, **no existe tabla `suppliers`**. Un proveedor es un `party` con `party_role.role = 'supplier'`. La API `/v1/suppliers` es un alias de conveniencia.

**Caso clave**: una empresa que es cliente Y proveedor simultáneamente (ej: el taller le compra repuestos al mismo negocio que le trae autos para reparar) tiene UN solo party con 2 roles. Su historial completo (compras + ventas) está centralizado.

### Entidad de dominio (vista de negocio)

```go
// Supplier es una vista de Party + PartyRole(supplier)
type Supplier struct {
    ID          uuid.UUID      // = party.id
    OrgID       uuid.UUID
    PartyType   string         // "person" | "organization"
    Name        string         // = party.display_name
    TaxID       string
    Email       string
    Phone       string
    Address     Address
    Notes       string
    Tags        []string
    Metadata    map[string]any
    // Role-specific
    RoleMetadata map[string]any // payment_terms, contact_name, etc.
    // Contactos vinculados (via party_relationships)
    Contacts    []PartyRelationship
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
}
```

### API

```
GET    /v1/suppliers              — Listar parties con rol 'supplier' (paginado, filtro, search)
POST   /v1/suppliers              — Crear party + asignar rol 'supplier'
GET    /v1/suppliers/:id          — Detalle (party + extensión + contactos)
PUT    /v1/suppliers/:id          — Actualizar party
DELETE /v1/suppliers/:id          — Soft delete
```

### Implementación SQL

```sql
SELECT p.*, pr.metadata AS role_metadata
FROM parties p
JOIN party_roles pr ON pr.party_id = p.id AND pr.org_id = p.org_id
WHERE pr.role = 'supplier'
  AND pr.is_active = true
  AND p.org_id = $1
  AND p.deleted_at IS NULL;
```

### Persona de contacto

El viejo campo `contact_name` se modela con **relaciones entre parties**:

1. El proveedor (organización) es un party con rol `supplier`
2. Su contacto es otro party (persona) con rol `contact`
3. Se vinculan con `party_relationships(type='contact_of', from=contacto, to=proveedor)`

Alternativamente, para pymes que solo necesitan un nombre de contacto, se guarda en `party_roles.metadata.contact_name` (más simple, sin crear otro party).

### Reglas de negocio

- Mismas reglas base que customers (display_name obligatorio, tax_id único por org, soft delete, búsqueda ILIKE).
- Si al crear un supplier el `tax_id` o `email` coincide con un party existente que ya es `customer`, se agrega el rol `supplier` al mismo party — **sin duplicar**.

---

## 3. Products (Productos y Servicios)

### Entidad de dominio

```go
type Product struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Type        string         // "product" | "service"
    SKU         string         // código interno (opcional)
    Name        string
    Description string
    Unit        string         // "unit", "kg", "hr", "m", "lt", etc.
    Price       float64        // precio de venta (sin impuestos)
    CostPrice   float64        // precio de costo (para margen)
    TaxRate     float64        // % IVA para este producto (override del default org)
    TrackStock  bool           // true para productos físicos, false para servicios
    Tags        []string
    Metadata    map[string]any
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
}
```

### API

```
GET    /v1/products               — Listar (paginado, filtro por type/tag, search)
POST   /v1/products               — Crear
GET    /v1/products/:id           — Detalle
PUT    /v1/products/:id           — Actualizar
DELETE /v1/products/:id           — Soft delete
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL DEFAULT 'product' CHECK (type IN ('product', 'service')),
    sku text,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    unit text NOT NULL DEFAULT 'unit',
    price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2),
    track_stock boolean NOT NULL DEFAULT true,
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_products_org_sku ON products(org_id, sku) WHERE deleted_at IS NULL AND sku IS NOT NULL AND sku != '';
CREATE INDEX IF NOT EXISTS idx_products_org ON products(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_products_org_name ON products(org_id, name) WHERE deleted_at IS NULL;
```

### Reglas de negocio

- `sku` es único por org (si se provee). Muchas pymes no usan SKU.
- `type = 'service'` implica `track_stock = false` siempre.
- `tax_rate` puede ser NULL; en ese caso se usa el IVA default del org (de `tenant_settings`).
- `unit` es texto libre pero se sugiere: "unit", "kg", "g", "lt", "ml", "hr", "m", "m2", "m3", "pack".

---

## 4. Inventory (Stock)

### Entidad de dominio

```go
type StockLevel struct {
    ProductID   uuid.UUID
    OrgID       uuid.UUID
    Quantity    float64
    MinQuantity float64   // alerta de stock bajo
    UpdatedAt   time.Time
}

type StockMovement struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    ProductID   uuid.UUID
    Type        string    // "in" | "out" | "adjustment"
    Quantity    float64   // positivo para in, negativo para out
    Reason      string    // "sale", "purchase", "return", "adjustment", "initial"
    ReferenceID *uuid.UUID // sale_id o purchase_id que originó el movimiento
    Notes       string
    CreatedBy   string
    CreatedAt   time.Time
}
```

### API

```
GET    /v1/inventory                      — Stock actual (paginado, filtro por stock bajo)
GET    /v1/inventory/:product_id          — Stock de un producto
POST   /v1/inventory/:product_id/adjust   — Ajuste manual de stock
GET    /v1/inventory/movements            — Historial de movimientos (paginado, filtro por producto/tipo/fecha)
GET    /v1/inventory/low-stock            — Productos con stock bajo
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS stock_levels (
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    quantity numeric(15,2) NOT NULL DEFAULT 0,
    min_quantity numeric(15,2) NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, product_id)
);

CREATE TABLE IF NOT EXISTS stock_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id),
    type text NOT NULL CHECK (type IN ('in', 'out', 'adjustment')),
    quantity numeric(15,2) NOT NULL,
    reason text NOT NULL DEFAULT '',
    reference_id uuid,
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stock_movements_org ON stock_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_movements_product ON stock_movements(org_id, product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_stock_low ON stock_levels(org_id) WHERE quantity <= min_quantity AND min_quantity > 0;
```

### Reglas de negocio

- `stock_levels` se crea automáticamente cuando se crea un producto con `track_stock = true`.
- Las ventas generan movimientos `type = 'out'` automáticamente.
- Los ajustes manuales son `type = 'adjustment'` con `notes` obligatorio.
- Stock puede ser negativo (la pyme decide si lo permite; default: sí, con warning).
- `stock_movements` son inmutables — nunca se editan ni borran.

---

## 5. Quotes (Presupuestos)

### Entidad de dominio

```go
type Quote struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Number      string        // secuencial: "PRE-00001"
    PartyID     *uuid.UUID    // party con rol customer (puede ser nil para presupuesto sin cliente registrado)
    PartyName   string        // nombre manual si no hay party_id (denormalizado)
    Status      string        // "draft" | "sent" | "accepted" | "rejected" | "expired"
    Items       []QuoteItem
    Subtotal    float64
    TaxTotal    float64
    Total       float64
    Currency    string
    Notes       string
    ValidUntil  *time.Time
    CreatedBy   string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type QuoteItem struct {
    ID          uuid.UUID
    QuoteID     uuid.UUID
    ProductID   *uuid.UUID    // puede ser item ad-hoc sin producto
    Description string
    Quantity    float64
    UnitPrice   float64
    TaxRate     float64
    Subtotal    float64
    SortOrder   int
}
```

### API

```
GET    /v1/quotes                — Listar (paginado, filtro por status/customer/fecha)
POST   /v1/quotes                — Crear
GET    /v1/quotes/:id            — Detalle con items
PUT    /v1/quotes/:id            — Actualizar (solo draft)
DELETE /v1/quotes/:id            — Eliminar (solo draft)
POST   /v1/quotes/:id/send      — Marcar como enviado
POST   /v1/quotes/:id/accept    — Aceptar (puede convertir a venta)
POST   /v1/quotes/:id/reject    — Rechazar
POST   /v1/quotes/:id/to-sale   — Convertir a venta
GET    /v1/quotes/:id/pdf       — Generar PDF (futuro, placeholder)
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS quotes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id),
    party_name text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'sent', 'accepted', 'rejected', 'expired')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    valid_until timestamptz,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS quote_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id uuid NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_quotes_org ON quotes(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_quotes_org_status ON quotes(org_id, status);
CREATE INDEX IF NOT EXISTS idx_quotes_party ON quotes(party_id) WHERE party_id IS NOT NULL;
```

### Reglas de negocio

- Solo se pueden editar/eliminar presupuestos en estado `draft`.
- `number` se genera automáticamente: `PRE-{secuencial 5 dígitos}`. Configurable por org via `tenant_settings.quote_prefix`. La numeración usa `SELECT next_quote_number FROM tenant_settings WHERE org_id = ? FOR UPDATE` dentro de una transacción para evitar race conditions, y luego `UPDATE ... SET next_quote_number = next_quote_number + 1`.
- `to-sale` copia los items a una nueva venta y marca el presupuesto como `accepted`.
- Totales se recalculan server-side: `subtotal = Σ(quantity × unit_price)`, `tax_total = Σ(subtotal_item × tax_rate / 100)`, `total = subtotal + tax_total`.
- `party_id` es opcional. Para presupuestos rápidos basta con `party_name`.

---

## 6. Sales (Ventas)

### Entidad de dominio

```go
type Sale struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    Number      string         // "VTA-00001"
    PartyID     *uuid.UUID     // party con rol customer
    PartyName   string         // denormalizado
    QuoteID     *uuid.UUID     // si se originó de un presupuesto
    Status      string         // "completed" | "voided"
    PaymentMethod string       // "cash" | "card" | "transfer" | "other"
    Items       []SaleItem
    Subtotal    float64
    TaxTotal    float64
    Total       float64
    Currency    string
    Notes       string
    CreatedBy   string
    CreatedAt   time.Time
}

type SaleItem struct {
    ID          uuid.UUID
    SaleID      uuid.UUID
    ProductID   *uuid.UUID
    Description string
    Quantity    float64
    UnitPrice   float64
    CostPrice   float64    // snapshot del costo al momento de la venta (para margen)
    TaxRate     float64
    Subtotal    float64
    SortOrder   int
}
```

### API

```
GET    /v1/sales                  — Listar (paginado, filtro por fecha/customer/payment_method)
POST   /v1/sales                  — Crear venta
GET    /v1/sales/:id              — Detalle con items
POST   /v1/sales/:id/void        — Anular venta
GET    /v1/sales/:id/receipt      — Generar recibo (futuro, placeholder)
```

### Tablas SQL

```sql
CREATE TABLE IF NOT EXISTS sales (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    number text NOT NULL,
    party_id uuid REFERENCES parties(id),
    party_name text NOT NULL DEFAULT '',
    quote_id uuid REFERENCES quotes(id),
    status text NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'voided')),
    payment_method text NOT NULL DEFAULT 'cash' CHECK (payment_method IN ('cash', 'card', 'transfer', 'other')),
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    tax_total numeric(15,2) NOT NULL DEFAULT 0,
    total numeric(15,2) NOT NULL DEFAULT 0,
    currency text NOT NULL DEFAULT 'ARS',
    notes text NOT NULL DEFAULT '',
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    voided_at timestamptz,
    UNIQUE(org_id, number)
);

CREATE TABLE IF NOT EXISTS sale_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id uuid NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id uuid REFERENCES products(id),
    description text NOT NULL,
    quantity numeric(15,2) NOT NULL DEFAULT 1,
    unit_price numeric(15,2) NOT NULL DEFAULT 0,
    cost_price numeric(15,2) NOT NULL DEFAULT 0,
    tax_rate numeric(5,2) NOT NULL DEFAULT 0,
    subtotal numeric(15,2) NOT NULL DEFAULT 0,
    sort_order int NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sales_org ON sales(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sales_org_date ON sales(org_id, created_at) WHERE status = 'completed';
CREATE INDEX IF NOT EXISTS idx_sales_party ON sales(party_id) WHERE party_id IS NOT NULL;
```

### DTOs con validación (binding tags)

```go
// handler/dto/dto.go
type CreateSaleRequest struct {
    PartyID       *uuid.UUID     `json:"party_id" binding:"omitempty"`
    PartyName     string         `json:"party_name" binding:"required_without=PartyID,max=200"`
    PaymentMethod string         `json:"payment_method" binding:"required,oneof=cash card transfer check other credit"`
    Items         []SaleItemReq  `json:"items" binding:"required,min=1,dive"`
    Notes         string         `json:"notes" binding:"max=2000"`
}

type SaleItemReq struct {
    ProductID   *uuid.UUID `json:"product_id" binding:"omitempty"`
    Description string     `json:"description" binding:"required,min=1,max=500"`
    Quantity    float64    `json:"quantity" binding:"required,gt=0"`
    UnitPrice   float64    `json:"unit_price" binding:"required,gte=0"`
    TaxRate     float64    `json:"tax_rate" binding:"gte=0,lte=100"`
}
```

### Transacción de creación de venta (patrón completo)

```go
func (uc *Usecases) CreateSale(ctx context.Context, orgID uuid.UUID, req CreateSaleRequest) (*domain.Sale, error) {
    // Validaciones de negocio pre-transacción
    if err := uc.validateSaleItems(ctx, orgID, req.Items); err != nil {
        return nil, err // retorna *apperror.Error
    }

    var sale *domain.Sale
    err := uc.db.Transaction(ctx, func(tx *gorm.DB) error {
        // 1. Número secuencial (atómico con FOR UPDATE)
        number, err := uc.repo.NextNumber(ctx, tx, orgID, "sale")
        if err != nil { return fmt.Errorf("next sale number: %w", err) }

        // 2. Calcular totales server-side
        calculated := uc.calculateTotals(req.Items)

        // 3. Crear venta
        sale, err = uc.repo.Create(ctx, tx, orgID, number, req, calculated)
        if err != nil { return fmt.Errorf("create sale: %w", err) }

        // 4. Stock (atómico)
        for _, item := range sale.Items {
            if item.ProductID != nil && item.TrackStock {
                if err := uc.inventoryPort.DeductStock(ctx, tx, orgID, *item.ProductID, item.Quantity, sale.ID); err != nil {
                    return err
                }
            }
        }

        // 5. Pago + caja (atómico)
        if req.PaymentMethod != "credit" {
            if err := uc.paymentsPort.RegisterAutoPayment(ctx, tx, orgID, sale.ID, sale.Total, req.PaymentMethod); err != nil {
                return err
            }
        } else if uc.accountsPort != nil {
            if err := uc.accountsPort.ChargeToAccount(ctx, tx, orgID, sale.PartyID, sale.Total, "sale", sale.ID); err != nil {
                return err
            }
        }

        return nil
    })
    if err != nil { return nil, err }

    // Side-effects best-effort (fuera de transacción, nil-safe)
    uc.auditPort.Log(ctx, orgID, "sale.created", "sale", sale.ID.String(), nil)
    uc.timelinePort.Record(ctx, domain.TimelineEntry{OrgID: orgID, EntityType: "party", EntityID: sale.PartyID, EventType: "sale_completed", Title: fmt.Sprintf("Venta %s por $%.2f", sale.Number, sale.Total)})
    uc.webhookPort.Dispatch(ctx, orgID, "sale.created", sale)

    return sale, nil
}
```

### Reglas de negocio

- Las ventas son **inmutables** una vez creadas. Solo se pueden anular (`void`), no editar. Intentar editar retorna `ErrSaleImmutable`.
- Al crear una venta con productos que tienen `track_stock = true`, se generan automáticamente movimientos de stock (`type = 'out'`) **dentro de la misma transacción**.
- Al anular una venta, se generan movimientos de stock reversos (`type = 'in'`, `reason = 'void'`) **dentro de una transacción**.
- Al crear una venta no fiada, se genera automáticamente un movimiento de caja (`type = 'income'`) **dentro de la misma transacción**.
- `cost_price` es un snapshot: se copia de `products.cost_price` al momento de la venta.
- `number` secuencial: `VTA-{5 dígitos}`. Configurable via `tenant_settings.sale_prefix`. Se obtiene con `SELECT ... FOR UPDATE` dentro de la transacción.
- Al anular, setear `voided_at = now()` además de `status = 'voided'`. Si la venta ya está anulada, retornar `ErrSaleAlreadyVoided`.

---

## 7. Cashflow (Caja)

### Entidad de dominio

```go
type CashMovement struct {
    ID            uuid.UUID
    OrgID         uuid.UUID
    Type          string     // "income" | "expense"
    Amount        float64
    Currency      string
    Category      string     // "sale", "purchase", "salary", "rent", "tax", "other"
    Description   string
    PaymentMethod string     // "cash" | "card" | "transfer" | "other"
    ReferenceType string     // "sale" | "quote" | "manual"
    ReferenceID   *uuid.UUID
    CreatedBy     string
    CreatedAt     time.Time
}

type CashSummary struct {
    OrgID         uuid.UUID
    PeriodStart   time.Time
    PeriodEnd     time.Time
    TotalIncome   float64
    TotalExpense  float64
    Balance       float64
    Currency      string
}
```

### API

```
GET    /v1/cashflow                       — Listar movimientos (paginado, filtro por tipo/categoría/fecha)
POST   /v1/cashflow                       — Crear movimiento manual
GET    /v1/cashflow/summary               — Resumen de caja (income/expense/balance por período)
GET    /v1/cashflow/summary/daily         — Resumen diario (últimos 30 días)
```

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS cash_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    type text NOT NULL CHECK (type IN ('income', 'expense')),
    amount numeric(15,2) NOT NULL,
    currency text NOT NULL DEFAULT 'ARS',
    category text NOT NULL DEFAULT 'other',
    description text NOT NULL DEFAULT '',
    payment_method text NOT NULL DEFAULT 'cash',
    reference_type text NOT NULL DEFAULT 'manual',
    reference_id uuid,
    created_by text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cash_movements_org ON cash_movements(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_type ON cash_movements(org_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cash_movements_org_date ON cash_movements(org_id, created_at);
```

### Reglas de negocio

- Los movimientos de caja son **inmutables** — nunca se editan ni borran.
- Las ventas crean movimientos automáticos `type = 'income'`, `category = 'sale'`, `reference_type = 'sale'`.
- Las anulaciones de ventas crean movimientos `type = 'expense'`, `category = 'sale'`, con nota de anulación.
- Movimientos manuales son `reference_type = 'manual'`.
- `summary` calcula `SUM(amount) WHERE type = 'income'` y `SUM(amount) WHERE type = 'expense'` en un rango de fechas.

---

## 8. Reports (Reportes)

### API

```
GET /v1/reports/sales-summary          — Ventas por período (total, cantidad, ticket promedio)
GET /v1/reports/sales-by-product       — Ranking de productos más vendidos
GET /v1/reports/sales-by-party         — Ranking de clientes (parties con rol customer) por monto
GET /v1/reports/sales-by-payment       — Ventas por método de pago
GET /v1/reports/inventory-valuation    — Valor del inventario (stock × costo)
GET /v1/reports/low-stock              — Productos con stock bajo
GET /v1/reports/cashflow-summary       — Resumen de caja por período
GET /v1/reports/profit-margin          — Margen de ganancia (venta - costo)
```

### Reglas

- Todos los reportes reciben `?from=2026-01-01&to=2026-03-31` (rango de fechas).
- Los reportes son **queries de lectura** directos a las tablas existentes — no hay tablas de reportes separadas.
- Los reportes respetan `org_id` del auth context.
- Formato de respuesta: JSON con los datos + totales. No genera PDF/Excel (eso es futuro).

---

## Migración SQL

Una sola migración `0005_core_business.up.sql` que crea todas las tablas:

- `products`
- `stock_levels`
- `stock_movements`
- `quotes` + `quote_items`
- `sales` + `sale_items`
- `cash_movements`

**NOTA**: NO crea tablas `customers` ni `suppliers` — estas son vistas lógicas sobre `parties` + `party_roles` (definidas en Prompt 00). Los FKs de `quotes.party_id`, `sales.party_id` apuntan a `parties(id)`.

Y `0005_core_business.down.sql` que las dropea en orden inverso (por foreign keys).

---

## Extensión de tenant_settings

Agregar a `tenant_settings` (migración `0006_tenant_business_settings.up.sql`):

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS currency text NOT NULL DEFAULT 'ARS',
    ADD COLUMN IF NOT EXISTS tax_rate numeric(5,2) NOT NULL DEFAULT 21.00,
    ADD COLUMN IF NOT EXISTS quote_prefix text NOT NULL DEFAULT 'PRE',
    ADD COLUMN IF NOT EXISTS sale_prefix text NOT NULL DEFAULT 'VTA',
    ADD COLUMN IF NOT EXISTS next_quote_number int NOT NULL DEFAULT 1,  -- usar SELECT ... FOR UPDATE al incrementar
    ADD COLUMN IF NOT EXISTS next_sale_number int NOT NULL DEFAULT 1,   -- usar SELECT ... FOR UPDATE al incrementar
    ADD COLUMN IF NOT EXISTS allow_negative_stock boolean NOT NULL DEFAULT true;
```

---

## Seed data adicional (migración `0007_core_seed.up.sql`)

Agregar datos de ejemplo al org de desarrollo local:

```sql
-- 3 parties con rol 'customer' (2 personas + 1 organización)
-- 2 parties con rol 'supplier' (1 persona + 1 organización)
-- 1 party que es TANTO customer como supplier (misma entidad, 2 roles)
-- 5 productos (3 productos físicos + 2 servicios)
-- Stock inicial para los 3 productos físicos
-- 1 presupuesto (aceptado) con 2 items
-- 2 ventas con items
-- Movimientos de stock y caja correspondientes
-- El party dual (customer+supplier) tiene ventas Y compras en su historial
```

---

## Paginación estándar

Todos los endpoints de listado usan el mismo patrón:

```
?limit=20            — cantidad por página (default 20, max 100)
?after=<uuid>        — cursor para página siguiente
?search=<texto>      — búsqueda full-text (ILIKE en campos relevantes)
?sort=created_at     — campo de ordenamiento
?order=desc          — dirección (asc/desc)
```

Respuesta:

```json
{
    "items": [...],
    "total": 150,
    "has_more": true,
    "next_cursor": "uuid-del-ultimo-item"
}
```

---

## Interacciones entre módulos

```
Sale (crear)
  ├── Inventory: stock_movement(type=out) por cada item con product.track_stock=true
  ├── Cashflow: cash_movement(type=income, category=sale)
  └── Audit: audit_log entry

Sale (anular)
  ├── Inventory: stock_movement(type=in, reason=void) reverso
  ├── Cashflow: cash_movement(type=expense, category=sale) reverso
  └── Audit: audit_log entry

Quote (aceptar → convertir a venta)
  ├── Sale: crea venta con items del presupuesto
  └── (la venta dispara sus propios efectos)

Product (crear con track_stock=true)
  └── Inventory: crea stock_level con quantity=0
```

Estas interacciones se orquestan en el **usecase** de ventas, NO en el handler ni en el repository. El usecase de `sales` recibe como dependencias los ports de `inventory` y `cashflow`.

---

## Reglas de implementación

1. Seguir la misma arquitectura hexagonal que los módulos existentes del control-plane.
2. Cada módulo define sus propios ports (interfaces) para las dependencias que necesita.
3. Los totales de quotes y sales se calculan server-side, nunca confiar en el frontend. Si el frontend envía totales, se ignoran y se recalculan.
4. `numeric(15,2)` en PostgreSQL, `float64` en Go. La precisión financiera se garantiza en la DB. No usar `shopspring/decimal`.
5. Todos los endpoints requieren auth (van en `authGroup`).
6. El `org_id` se extrae del auth context, NUNCA del path ni del body.
7. Registrar todas las rutas nuevas en `wire/bootstrap.go` usando Functional Options para dependencias opcionales.
8. **Domain Errors**: todo error de negocio usa `apperror.Error`. Los repositorios convierten `gorm.ErrRecordNotFound` → `apperror.NewNotFound`. Los handlers usan `c.Error(err)`.
9. **Transacciones**: crear venta y anular venta usan `db.Transaction`. Numeración secuencial con `FOR UPDATE`. Side-effects (audit, timeline, webhooks) fuera de la transacción.
10. **Validation**: todos los DTOs de request usan binding tags. Validaciones de negocio (stock suficiente, presupuesto en draft, tax_id único) en el usecase.
11. **Tests obligatorios**: unit tests table-driven para usecases de `sales` y `quotes` (interacciones complejas). Integration tests para repositorios con testcontainers-go.
12. Agregar tests E2E al script `scripts/e2e-test.sh` para los nuevos endpoints.

---

## Testing — Ejemplo para Sales

### Unit test del usecase (table-driven)

```go
func TestSalesUsecases_CreateSale(t *testing.T) {
    prodID := uuid.New()
    tests := []struct {
        name    string
        req     dto.CreateSaleRequest
        setup   func(m *mocks)
        wantErr apperror.Code
    }{
        {
            name: "venta cash con stock OK",
            req:  dto.CreateSaleRequest{PaymentMethod: "cash", Items: []dto.SaleItemReq{{ProductID: &prodID, Description: "Tornillo", Quantity: 5, UnitPrice: 100}}},
            setup: func(m *mocks) {
                m.repo.EXPECT().NextNumber(gomock.Any(), gomock.Any(), orgID, "sale").Return("VTA-00001", nil)
                m.repo.EXPECT().Create(gomock.Any(), gomock.Any(), orgID, "VTA-00001", gomock.Any(), gomock.Any()).Return(testSale, nil)
                m.inventory.EXPECT().DeductStock(gomock.Any(), gomock.Any(), orgID, prodID, float64(5), testSale.ID).Return(nil)
                m.payments.EXPECT().RegisterAutoPayment(gomock.Any(), gomock.Any(), orgID, testSale.ID, float64(500), "cash").Return(nil)
            },
        },
        {
            name: "venta fiada sin módulo accounts → error",
            req:  dto.CreateSaleRequest{PaymentMethod: "credit", PartyID: &partyID, Items: []dto.SaleItemReq{{Description: "Servicio", Quantity: 1, UnitPrice: 1000}}},
            setup: func(m *mocks) {
                m.repo.EXPECT().NextNumber(gomock.Any(), gomock.Any(), orgID, "sale").Return("VTA-00002", nil)
                m.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testSaleFiada, nil)
                // accountsPort es nil → no puede fiar
            },
            wantErr: apperror.CodeBusinessRule,
        },
        {
            name: "stock insuficiente → rollback transacción",
            req:  dto.CreateSaleRequest{PaymentMethod: "cash", Items: []dto.SaleItemReq{{ProductID: &prodID, Description: "Tornillo", Quantity: 9999, UnitPrice: 100}}},
            setup: func(m *mocks) {
                m.repo.EXPECT().NextNumber(gomock.Any(), gomock.Any(), orgID, "sale").Return("VTA-00003", nil)
                m.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(testSaleBig, nil)
                m.inventory.EXPECT().DeductStock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
                    Return(apperror.NewBusinessRule("stock insuficiente"))
            },
            wantErr: apperror.CodeBusinessRule,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            ctrl := gomock.NewController(t)
            m := newMocks(ctrl)
            tt.setup(m)
            uc := sales.NewUsecases(m.repo, m.db, logger,
                sales.WithInventory(m.inventory),
                sales.WithPayments(m.payments),
            )
            _, err := uc.CreateSale(context.Background(), orgID, tt.req)
            if tt.wantErr == "" {
                require.NoError(t, err)
            } else {
                var appErr *apperror.Error
                require.ErrorAs(t, err, &appErr)
                assert.Equal(t, tt.wantErr, appErr.Code)
            }
        })
    }
}
```

### Integration test del repository

```go
func TestSalesRepository_Integration(t *testing.T) {
    if testing.Short() { t.Skip("skipping integration test") }

    ctx := context.Background()
    db := testutil.SetupPostgresWithMigrations(ctx, t)
    repo := sales.NewRepository(db)

    t.Run("create and list sales", func(t *testing.T) {
        // Crear org y party de test
        // Crear venta
        // Verificar que se lista correctamente
        // Verificar paginación con cursor
    })

    t.Run("void sale", func(t *testing.T) {
        // Crear venta → anular → verificar status y voided_at
        // Intentar anular de nuevo → verificar ErrSaleAlreadyVoided
    })
}
```

---

## Orden de implementación recomendado

1. Migración SQL (`0005`, `0006`, `0007`)
2. `customers` — alias sobre Party Model, valida el patrón (usa party module de Prompt 00)
3. `suppliers` — alias sobre Party Model, valida party con rol dual
4. `products` — agrega tipo y stock tracking
5. `inventory` — movimientos de stock
6. `quotes` — presupuestos con items y estados
7. `sales` — ventas con efectos secundarios (stock, caja, audit)
8. `cashflow` — movimientos manuales + los automáticos de ventas
9. `reports` — queries de lectura sobre todo lo anterior
10. Wiring en `bootstrap.go` + tests E2E

---

## Criterios de éxito

- [ ] `go build ./...` compila sin errores
- [ ] `go test ./...` todos los tests pasan
- [ ] CRUD de clientes via Party Model (crear party + rol customer)
- [ ] CRUD de proveedores via Party Model (crear party + rol supplier)
- [ ] Party dual: crear party que sea customer Y supplier simultáneamente
- [ ] CRUD de productos
- [ ] Crear venta → descuenta stock → genera movimiento de caja → genera audit entry
- [ ] Anular venta → revierte stock → genera movimiento reverso
- [ ] Crear presupuesto → aceptar → convertir a venta (flujo completo)
- [ ] Stock bajo: `GET /v1/inventory/low-stock` muestra productos por debajo del mínimo
- [ ] Reportes: ventas por período, por producto, por cliente, margen
- [ ] Resumen de caja: income/expense/balance por rango de fechas
- [ ] Paginación con cursor funciona en todos los listados
- [ ] Búsqueda (`?search=`) funciona en customers, suppliers, products
- [ ] Tests E2E pasan (flujo: crear party/customer → crear producto → crear venta → verificar stock → verificar caja)
- [ ] Seed data cargado y funcional para dev local (incluye party dual customer+supplier)
