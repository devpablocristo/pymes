# Prompt 00 — Base Transversal SaaS para Pymes LATAM

## Visión del producto

SaaS multi-vertical para Pymes y profesionales independientes de Latinoamérica. Público objetivo: negocios que hoy operan con papel, lápiz, WhatsApp y Excel. Precio: **~USD 50/mes** por suscripción.

Verticales futuros (NO implementar ahora, solo diseñar la base para soportarlos):
- Salud (turnos, historias clínicas, obras sociales)
- Educación (alumnos, asistencias, notas, cuotas)
- Talleres mecánicos (órdenes de trabajo, presupuestos, repuestos)
- Kioscos/comercios (ventas, stock, proveedores)
- Profesionales independientes (clientes, proyectos, facturas)

**Este prompt implementa SOLO la base transversal** — la capa común que comparten todos los verticales. Ninguna lógica de negocio específica de un vertical.

## Alcance obligatorio

Todo lo definido en este prompt forma parte del alcance requerido del proyecto. Nada de lo documentado acá debe interpretarse como opcional, postergable por defecto, o "nice to have" salvo que el prompt lo diga explícitamente.

Si un bloque aparece antes o después en el documento, eso **no cambia su importancia**. La única razón para secuenciar tareas es respetar dependencias técnicas y reducir retrabajo.

---

## Stack tecnológico

| Capa | Tecnología |
|------|-----------|
| **Backend** | Go 1.24, Gin framework |
| **Frontend** | React 18 + TypeScript + Vite |
| **Runtime** | AWS Lambda (Go) via `aws-lambda-go-api-proxy/gin` |
| **API routing** | API Gateway HTTP API |
| **Database** | RDS PostgreSQL 16 (1 instancia) via RDS Proxy |
| **Auth/Identity** | Clerk |
| **Billing** | Stripe |
| **Email** | AWS SES (prod), SMTP/MailHog (dev) |
| **Storage** | S3 (documentos, reportes) |
| **Frontend hosting** | S3 + CloudFront |
| **IaC** | Terraform |
| **CI/CD** | GitHub Actions |

---

## Estructura del proyecto

```
pymes/                          # Monorepo raíz
├── .github/workflows/
│   ├── ci.yml                  # Tests, lint, build
│   └── deploy.yml              # Build → zip → Lambda update + S3 sync
├── go.work                     # Go workspace
├── pkgs/go-pkg/                # Librería Go compartida entre servicios
├── docker-compose.yml          # Dev: postgres, mailhog, backend (Air), frontend (Vite)
├── Makefile
├── .env.example
├── control-plane/              # Servicio base transversal (auth, billing, tenants, admin)
│   ├── infra/                  # Terraform
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   ├── terraform.tfvars.example
│   │   └── modules/
│   │       ├── networking/     # VPC, subnets, security groups
│   │       ├── database/       # RDS + RDS Proxy
│   │       ├── lambda/         # Lambda functions + API Gateway
│   │       ├── cdn/            # S3 + CloudFront
│   │       ├── secrets/        # Secrets Manager
│   │       └── monitoring/     # CloudWatch
│   ├── backend/
│   │   ├── cmd/
│   │   │   ├── lambda/         # Lambda entrypoint
│   │   │   │   └── main.go
│   │   │   └── local/          # Local dev server (Gin directo)
│   │   │       └── main.go
│   │   ├── internal/
│   │   │   ├── identity/       # Clerk JWKS verification
│   │   │   ├── clerkwebhook/   # Clerk webhook handler
│   │   │   ├── billing/        # Stripe billing
│   │   │   ├── notifications/  # Email notifications (SES/SMTP/Noop)
│   │   │   ├── admin/          # Admin console, tenant settings
│   │   │   ├── users/          # User management, API keys
│   │   │   ├── org/            # Organization CRUD
│   │   │   ├── party/          # Party Model: CRUD de parties, roles, relaciones
│   │   │   ├── audit/          # Audit log
│   │   │   ├── shared/
│   │   │   │   ├── handlers/   # Auth middleware, CORS middleware
│   │   │   │   ├── authz/      # Permissions, scopes
│   │   │   │   ├── config/     # Config from env vars
│   │   │   │   ├── store/      # GORM DB connection
│   │   │   │   └── app/        # App struct (Router)
│   │   │   └── verticals/      # Plugin point for verticals (empty for now)
│   │   ├── pkg/
│   │   │   ├── apperror/       # Domain errors (Code, Message, HTTPStatus) — E1
│   │   │   ├── pagination/     # Cursor pagination (Params, Result[T]) — E12
│   │   │   ├── resilience/     # Retry con backoff exponencial — E8
│   │   │   ├── validation/     # Custom validators + error translator — E3
│   │   │   ├── utils/          # SHA256, canonical JSON, API key generation
│   │   │   └── types/          # Context keys
│   │   ├── migrations/
│   │   │   ├── runner.go       # golang-migrate runner (embed.FS)
│   │   │   ├── 0001_base_schema.up.sql / .down.sql
│   │   │   ├── 0002_billing.up.sql / .down.sql
│   │   │   ├── 0003_notifications.up.sql / .down.sql
│   │   │   └── 0004_local_seed.up.sql / .down.sql
│   │   ├── wire/               # DI manual (bootstrap.go)
│   │   ├── go.mod
│   │   └── go.sum
│   └── frontend/
│       ├── src/
│       │   ├── app/App.tsx
│       │   ├── api/client.ts   # HTTP client with Clerk JWT
│       │   ├── lib/
│       │   │   ├── api.ts      # API functions
│       │   │   ├── types.ts    # TypeScript types
│       │   │   └── auth.ts     # clerkEnabled flag
│       │   ├── components/
│       │   │   ├── Shell.tsx
│       │   │   ├── ProtectedRoute.tsx
│       │   │   └── AuthTokenBridge.tsx
│       │   └── pages/
│       │       ├── LoginPage.tsx
│       │       ├── SignupPage.tsx
│       │       ├── DashboardPage.tsx
│       │       ├── SettingsPage.tsx
│       │       ├── BillingPage.tsx
│       │       ├── AdminPage.tsx
│       │       ├── NotificationPreferencesPage.tsx
│       │       └── APIKeysPage.tsx
│       ├── package.json
│       ├── vite.config.ts
│       ├── tsconfig.json
│       └── index.html
├── prompts/                    # Prompts de diseño del proyecto
└── README.md
```

---

## Party Model — Modelo unificado de actores

### Filosofía

Basado en el **Party Model** (Apache OFBiz, Oracle, Len Silverston), el sistema separa **identidad** de **función**. No existen tablas separadas `customers` ni `suppliers` — existe una sola tabla `parties` que representa a cualquier actor de negocio, y las funciones (cliente, proveedor, empleado) se asignan como **roles**.

### Tres capas

| Capa | Tabla | Pregunta que responde |
|------|-------|----------------------|
| **Identidad** | `parties` | ¿Quién es? (persona, empresa, agente IA) |
| **Capacidad** | `party_roles` | ¿Qué puede hacer? (cliente, proveedor, empleado) |
| **Transacción** | `sales`, `purchases`, etc. | ¿Qué está haciendo ahora? |

### Beneficios

- Una empresa que es **cliente y proveedor al mismo tiempo** tiene UN solo registro con 2 roles — sin duplicación.
- El historial completo de un actor (ventas, compras, turnos, deuda) está centralizado.
- Los verticales extienden con roles propios (`paciente`, `alumno`, `conductor`) sin tocar el modelo base.
- El `audit_log` referencia al actor de forma uniforme (`party_id`).

### Tipos de party

| `party_type` | Tabla de extensión | Ejemplos |
|--------------|-------------------|----------|
| `person` | `party_persons` | Cliente Juan, Empleada María, Contacto de proveedor |
| `organization` | `party_organizations` | Empresa proveedora, cliente corporativo |
| `automated_agent` | `party_agents` | Asistente IA, bot de WhatsApp, servicio MP |

### Roles

Los roles son la **capa de capacidad**. Un party puede tener múltiples roles simultáneamente.

| Rol | Significado |
|-----|------------|
| `customer` | La pyme le vende / le presta servicios |
| `supplier` | La pyme le compra mercadería o servicios |
| `employee` | Trabaja en la pyme |
| `contact` | Persona de contacto de una organización |
| `sales_agent` | Representante de ventas |
| `professional` | Profesional que atiende turnos (médico, mecánico, etc.) |

Los verticales agregan roles propios: `patient`, `student`, `driver`, etc.

### Relaciones entre parties

`party_relationships` modela vínculos: "María es contacto de Proveedor SA", "Juan es empleado de Mi Pyme". Esto permite:

- Ver todos los contactos de un proveedor
- Saber qué empleado atendió a qué cliente (futuro CRM)
- Modelar jerarquías organizacionales

### Services — Registro de infraestructura

Los servicios determinísticos (webhooks, schedulers, pasarelas de pago, notificaciones) NO son parties. Se registran en la tabla `services` como componentes de infraestructura auditables:

| `direction` | Ejemplos |
|-------------|----------|
| `inbound` | Webhook de Clerk, webhook de Stripe, IPN de Mercado Pago |
| `outbound` | Notificaciones email, webhooks salientes, WhatsApp |
| `internal` | Scheduler, rate limiter, PDF generator |

La diferencia fundamental:
- **Party (automated_agent)**: tiene inteligencia, toma decisiones (IA, bot conversacional)
- **Service**: reacciona determinísticamente a eventos (webhook, cron, notification sender)

---

## Arquitectura hexagonal (patrón por módulo)

Cada módulo sigue esta estructura:

```
internal/<module>/
├── handler.go              # HTTP handler, recibe *Usecases
├── handler/dto/dto.go      # Request/Response DTOs
├── usecases.go             # Business logic, define ports (interfaces)
├── usecases/domain/
│   └── entities.go         # Domain types
├── repository.go           # DB implementation (GORM)
└── repository/models/
    └── models.go           # GORM models
```

**Reglas:**
- `handler.go` SOLO depende de `*Usecases` (no de repository, no de DB)
- `usecases.go` define sus ports (interfaces) en el mismo archivo: `RepositoryPort`, `NotificationPort`, etc.
- `repository.go` implementa `RepositoryPort` usando GORM
- La DI se resuelve en `wire/bootstrap.go` con constructores explícitos (DI manual, sin code generation)

**Ejemplo de DI manual en bootstrap:**

```go
// wire/bootstrap.go
billingRepo := billing.NewRepository(db)
stripeClient := billing.NewStripeClient(cfg.StripeSecretKey)
billingUC := billing.NewUsecases(billingRepo, stripeClient, notificationsUC, cfg.FrontendURL, priceIDs, cfg.StripeWebhookSecret, logger)
billingHandler := billing.NewHandler(billingUC)
```

Los constructores reciben dependencias concretas; los handlers definen sus propios port interfaces para los usecases que consumen.

---

## Estándares de Ingeniería

Estos patrones son **obligatorios** en todo el codebase. Cada módulo los aplica sin excepción.

---

### E1. Domain Errors — Errores tipados con código y HTTP status

No usar `errors.New("algo falló")` genérico. Cada módulo define errores de dominio tipados que el middleware convierte automáticamente a respuestas HTTP consistentes.

```go
// pkg/apperror/apperror.go
type Code string

const (
    CodeNotFound       Code = "NOT_FOUND"
    CodeValidation     Code = "VALIDATION_ERROR"
    CodeConflict       Code = "CONFLICT"
    CodeForbidden      Code = "FORBIDDEN"
    CodeUnauthorized   Code = "UNAUTHORIZED"
    CodeBusinessRule   Code = "BUSINESS_RULE_VIOLATION"
    CodeGatewayError   Code = "GATEWAY_ERROR"
    CodeInternal       Code = "INTERNAL_ERROR"
    CodeQuotaExceeded  Code = "QUOTA_EXCEEDED"
    CodePrecondition   Code = "PRECONDITION_FAILED"
)

type Error struct {
    Code       Code              `json:"code"`
    Message    string            `json:"message"`
    Details    map[string]string `json:"details,omitempty"`
    HTTPStatus int               `json:"-"`
    Err        error             `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Err }

func NewNotFound(resource, id string) *Error {
    return &Error{Code: CodeNotFound, Message: fmt.Sprintf("%s %s not found", resource, id), HTTPStatus: 404}
}
func NewValidation(msg string, details map[string]string) *Error {
    return &Error{Code: CodeValidation, Message: msg, Details: details, HTTPStatus: 422}
}
func NewConflict(msg string) *Error {
    return &Error{Code: CodeConflict, Message: msg, HTTPStatus: 409}
}
func NewBusinessRule(msg string) *Error {
    return &Error{Code: CodeBusinessRule, Message: msg, HTTPStatus: 422}
}
func NewForbidden(msg string) *Error {
    return &Error{Code: CodeForbidden, Message: msg, HTTPStatus: 403}
}
func NewGatewayError(provider string, err error) *Error {
    return &Error{Code: CodeGatewayError, Message: fmt.Sprintf("%s error: %s", provider, err), HTTPStatus: 502, Err: err}
}
```

**Regla**: los usecases retornan `*apperror.Error` para errores de negocio. Para errores inesperados (DB down, nil pointer), se envuelven con `fmt.Errorf("context: %w", err)` y el middleware los captura como 500.

**Regla para repositorios**: el repository convierte errores de GORM a domain errors:

```go
func (r *Repository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*domain.Party, error) {
    var model models.Party
    err := r.db.WithContext(ctx).Where("id = ? AND org_id = ? AND deleted_at IS NULL", id, orgID).First(&model).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, apperror.NewNotFound("party", id.String())
    }
    if err != nil {
        return nil, fmt.Errorf("get party %s: %w", id, err)
    }
    return model.ToDomain(), nil
}
```

---

### E2. API Error Response — Formato estándar RFC 7807 inspirado

Toda respuesta de error sigue el mismo formato. El frontend nunca interpreta strings ad-hoc.

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Input inválido",
        "details": {
            "display_name": "obligatorio, mínimo 2 caracteres",
            "email": "formato inválido"
        },
        "request_id": "req_a1b2c3d4"
    }
}
```

Respuestas exitosas usan envelope consistente:

```json
{
    "data": { ... },
    "meta": {
        "total": 150,
        "has_more": true,
        "next_cursor": "uuid"
    }
}
```

Para respuestas de un solo item: `{"data": {...}}`. Para listas: `{"data": [...], "meta": {...}}`.

**Error middleware** (único lugar que serializa errores):

```go
// internal/shared/handlers/error_middleware.go
func ErrorHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        if len(c.Errors) == 0 { return }

        err := c.Errors.Last().Err
        requestID := c.GetString("request_id")

        var appErr *apperror.Error
        if errors.As(err, &appErr) {
            c.JSON(appErr.HTTPStatus, gin.H{
                "error": gin.H{
                    "code":       appErr.Code,
                    "message":    appErr.Message,
                    "details":    appErr.Details,
                    "request_id": requestID,
                },
            })
            return
        }

        // Error inesperado — log completo, respuesta genérica
        logger.Error().Err(err).Str("request_id", requestID).Msg("unhandled error")
        c.JSON(500, gin.H{
            "error": gin.H{
                "code":       apperror.CodeInternal,
                "message":    "Error interno del servidor",
                "request_id": requestID,
            },
        })
    }
}
```

**Los handlers usan `c.Error(err)` en vez de `c.JSON` para errores:**

```go
func (h *Handler) GetByID(c *gin.Context) {
    id, err := uuid.Parse(c.Param("id"))
    if err != nil {
        _ = c.Error(apperror.NewValidation("ID inválido", nil))
        return
    }
    result, err := h.uc.GetByID(c.Request.Context(), orgID, id)
    if err != nil {
        _ = c.Error(err)
        return
    }
    c.JSON(200, gin.H{"data": dto.FromDomain(result)})
}
```

---

### E3. Input Validation — Binding + Custom Validators

Validación en 2 capas: (1) binding de Gin con struct tags para formato, (2) validación de negocio en el usecase.

```go
// handler/dto/dto.go
type CreatePartyRequest struct {
    PartyType   string  `json:"party_type" binding:"required,oneof=person organization automated_agent"`
    DisplayName string  `json:"display_name" binding:"required,min=2,max=200"`
    Email       *string `json:"email" binding:"omitempty,email"`
    Phone       *string `json:"phone" binding:"omitempty,min=6,max=30"`
    TaxID       *string `json:"tax_id" binding:"omitempty,min=5,max=30"`
    Address     *AddressDTO `json:"address" binding:"omitempty"`
    Tags        []string `json:"tags" binding:"omitempty,max=20,dive,min=1,max=50"`
}
```

**Custom validators** (registrar una vez al inicializar Gin):

```go
// pkg/validation/validators.go
func RegisterCustomValidators(v *validator.Validate) {
    v.RegisterValidation("currency_iso", validateCurrencyISO)
    v.RegisterValidation("positive_amount", validatePositiveAmount)
}

func validateCurrencyISO(fl validator.FieldLevel) bool {
    valid := map[string]bool{"ARS": true, "USD": true, "BRL": true, "CLP": true, "MXN": true, "COP": true, "PEN": true, "UYU": true}
    return valid[fl.Field().String()]
}
```

**Traducción de errores de validación a formato legible:**

```go
func TranslateValidationErrors(err error) map[string]string {
    var ve validator.ValidationErrors
    if !errors.As(err, &ve) { return nil }
    details := make(map[string]string, len(ve))
    for _, fe := range ve {
        field := toSnakeCase(fe.Field())
        switch fe.Tag() {
        case "required": details[field] = "campo obligatorio"
        case "email":    details[field] = "formato de email inválido"
        case "min":      details[field] = fmt.Sprintf("mínimo %s caracteres", fe.Param())
        case "max":      details[field] = fmt.Sprintf("máximo %s caracteres", fe.Param())
        case "oneof":    details[field] = fmt.Sprintf("debe ser uno de: %s", fe.Param())
        default:         details[field] = fmt.Sprintf("validación '%s' fallida", fe.Tag())
        }
    }
    return details
}
```

En los handlers, un helper estándar:

```go
func bindAndValidate[T any](c *gin.Context) (*T, error) {
    var req T
    if err := c.ShouldBindJSON(&req); err != nil {
        details := validation.TranslateValidationErrors(err)
        return nil, apperror.NewValidation("Input inválido", details)
    }
    return &req, nil
}
```

---

### E4. Transaction Management — Patrón Callback

Operaciones que modifican múltiples tablas usan transacciones explícitas. Patrón estándar: callback que recibe `*gorm.DB` transaccional.

```go
// internal/shared/store/store.go
type DB struct {
    conn *gorm.DB
}

func (d *DB) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
    return d.conn.WithContext(ctx).Transaction(fn)
}
```

**Cada usecase que orquesta múltiples cambios usa Transaction:**

```go
func (uc *Usecases) CreateSale(ctx context.Context, orgID uuid.UUID, req CreateSaleRequest) (*Sale, error) {
    var sale *Sale
    err := uc.db.Transaction(ctx, func(tx *gorm.DB) error {
        // 1. Generar número secuencial (SELECT ... FOR UPDATE)
        number, err := uc.repo.NextNumber(ctx, tx, orgID, "sale")
        if err != nil { return err }

        // 2. Crear venta
        sale, err = uc.repo.Create(ctx, tx, orgID, number, req)
        if err != nil { return err }

        // 3. Descontar stock (si aplica)
        for _, item := range sale.Items {
            if item.ProductID != nil && item.TrackStock {
                if err := uc.inventoryPort.DeductStock(ctx, tx, orgID, *item.ProductID, item.Quantity, sale.ID); err != nil {
                    return err
                }
            }
        }

        // 4. Registrar pago + movimiento de caja
        if req.PaymentMethod != "credit" {
            if err := uc.paymentsPort.RegisterAutoPayment(ctx, tx, orgID, sale.ID, sale.Total, req.PaymentMethod); err != nil {
                return err
            }
        }

        return nil
    })
    if err != nil { return nil, err }

    // Efectos fuera de la transacción (best-effort, no fallan la operación)
    uc.auditPort.Log(ctx, orgID, "sale.created", "sale", sale.ID.String(), nil)
    uc.timelinePort.Record(ctx, TimelineEntry{...})
    uc.webhookPort.Dispatch(ctx, orgID, "sale.created", sale)

    return sale, nil
}
```

**Reglas de transacciones:**
- Todo lo que DEBE ser atómico va dentro del `Transaction` callback.
- Side-effects no-críticos (audit, timeline, webhooks) van FUERA de la transacción. Son nil-safe y best-effort.
- Los números secuenciales (`next_sale_number`) se obtienen con `SELECT ... FOR UPDATE` dentro de la transacción para evitar race conditions.
- Isolation level: `READ COMMITTED` (default de PostgreSQL). No usar `SERIALIZABLE` salvo justificación explícita.
- Timeout de transacción: 10 segundos máximo. El context lo propaga.

---

### E5. Middleware Pipeline — Orden obligatorio

Los middlewares se aplican en este orden estricto:

```go
// wire/bootstrap.go
router := gin.New() // no usar gin.Default() — configurar explícitamente

// 1. Recovery (panic → 500, no crash)
router.Use(gin.Recovery())

// 2. Request ID (genera UUID, lo inyecta en context y header)
router.Use(handlers.RequestID())

// 3. Structured Logger (log de cada request con request_id, status, latency)
router.Use(handlers.StructuredLogger(logger))

// 4. Security Headers
router.Use(handlers.SecurityHeaders())

// 5. CORS
router.Use(handlers.CORS(cfg.FrontendURL))

// 6. Error Handler (convierte errors a JSON estándar)
router.Use(handlers.ErrorHandler())

// 7. Timeout (context con deadline para cada request)
router.Use(handlers.Timeout(30 * time.Second))

// -- Rutas públicas (webhooks) van AQUÍ, antes de auth --
v1 := router.Group("/v1")
v1.POST("/webhooks/clerk", clerkHandler.HandleWebhook)
v1.POST("/webhooks/stripe", billingHandler.HandleWebhook)

// 8. Auth middleware
authGroup := v1.Group("", handlers.Auth(identityUC, usersUC, cfg))

// 9. RBAC middleware (por ruta, dentro de cada handler)
```

**Request ID middleware:**

```go
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Request-ID")
        if id == "" {
            id = "req_" + uuid.New().String()[:8]
        }
        c.Set("request_id", id)
        c.Header("X-Request-ID", id)
        c.Next()
    }
}
```

**Security Headers:**

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Content-Security-Policy", "default-src 'self'")
        c.Next()
    }
}
```

**Timeout middleware:**

```go
func Timeout(duration time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), duration)
        defer cancel()
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

---

### E6. Structured Logging — Zerolog con context

Cada log incluye `request_id`, `org_id`, `actor`, `method`, `path`. No logear datos sensibles (tokens, passwords, PII).

```go
// internal/shared/handlers/logger_middleware.go
func StructuredLogger(logger zerolog.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        logger.Info().
            Str("request_id", c.GetString("request_id")).
            Str("method", c.Request.Method).
            Str("path", c.Request.URL.Path).
            Int("status", c.Writer.Status()).
            Dur("latency", time.Since(start)).
            Str("org_id", c.GetString("org_id")).
            Str("actor", c.GetString("actor")).
            Str("ip", c.ClientIP()).
            Msg("request")
    }
}
```

**Logger en usecases**: cada usecase recibe un `zerolog.Logger` y agrega contexto:

```go
func (uc *Usecases) CreateSale(ctx context.Context, orgID uuid.UUID, req CreateSaleRequest) (*Sale, error) {
    log := uc.logger.With().Str("org_id", orgID.String()).Str("op", "create_sale").Logger()
    log.Info().Msg("creating sale")
    // ...
    log.Info().Str("sale_id", sale.ID.String()).Msg("sale created")
}
```

**Reglas de logging:**
- `Info`: operaciones normales (request completado, entidad creada).
- `Warn`: situaciones recuperables (rate limit cerca, retry de webhook, token por vencer).
- `Error`: errores inesperados que requieren atención (DB down, external API 500).
- NUNCA logear: tokens, passwords, API keys, datos de tarjeta, PII sin necesidad.

---

### E7. Health Checks — Readiness y Liveness

```go
// Registrar ANTES de todo middleware
router.GET("/healthz", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok"})
})
router.GET("/readyz", func(c *gin.Context) {
    if err := db.Exec("SELECT 1").Error; err != nil {
        c.JSON(503, gin.H{"status": "not ready", "db": "down"})
        return
    }
    c.JSON(200, gin.H{"status": "ready", "db": "ok"})
})
```

---

### E8. Resilience — Retry + Circuit Breaker para APIs externas

Las llamadas a Stripe, Clerk, Mercado Pago, APIs de cotizaciones y cualquier servicio externo usan retry con backoff exponencial.

```go
// pkg/resilience/retry.go
type RetryConfig struct {
    MaxAttempts int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
    Jitter      bool
}

func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
    var lastErr error
    var zero T
    delay := cfg.BaseDelay

    for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
        result, err := fn()
        if err == nil { return result, nil }
        lastErr = err

        if !isRetryable(err) { return zero, err }

        if attempt < cfg.MaxAttempts-1 {
            jitter := time.Duration(0)
            if cfg.Jitter {
                jitter = time.Duration(rand.Int63n(int64(delay) / 2))
            }
            select {
            case <-ctx.Done(): return zero, ctx.Err()
            case <-time.After(delay + jitter):
            }
            delay = min(delay*2, cfg.MaxDelay)
        }
    }
    return zero, fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryable(err error) bool {
    // 5xx, timeout, connection refused → retry
    // 4xx → no retry (salvo 429 Too Many Requests)
    var appErr *apperror.Error
    if errors.As(err, &appErr) && appErr.HTTPStatus >= 400 && appErr.HTTPStatus < 500 && appErr.HTTPStatus != 429 {
        return false
    }
    return true
}
```

**Config recomendada por provider:**
- Stripe: `{MaxAttempts: 3, BaseDelay: 500ms, MaxDelay: 5s, Jitter: true}`
- Mercado Pago: `{MaxAttempts: 3, BaseDelay: 1s, MaxDelay: 10s, Jitter: true}`
- API de cotizaciones: `{MaxAttempts: 2, BaseDelay: 2s, MaxDelay: 8s, Jitter: false}`

---

### E9. Security — Input Sanitization y Rate Limiting

**Rate Limiting por endpoint** (basado en `golang.org/x/time/rate`):

```go
// internal/shared/handlers/rate_limit.go
type RateLimiter struct {
    limiters sync.Map // key: orgID+endpoint → *rate.Limiter
    rps      float64
    burst    int
}

func NewRateLimiter(rps float64, burst int) *RateLimiter {
    return &RateLimiter{rps: rps, burst: burst}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        key := c.GetString("org_id") + ":" + c.FullPath()
        limiter, _ := rl.limiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(rl.rps), rl.burst))
        if !limiter.(*rate.Limiter).Allow() {
            c.Header("Retry-After", "1")
            _ = c.Error(apperror.NewError(apperror.CodeQuotaExceeded, "Rate limit exceeded", 429))
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**En Lambda** el rate limiting per-instance es limitado (cada instancia tiene su propio rate limiter). Para rate limiting distribuido, usar API Gateway throttling (configurado en Terraform). El rate limiter in-process protege contra bursts dentro de una misma instancia.

**Headers de Rate Limit en respuestas:**

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1709654400
```

**Input Sanitization:**
- Todos los inputs de texto se trimean (`strings.TrimSpace`) antes de procesar.
- SQL injection: prevenido por GORM (parameterized queries). NUNCA concatenar strings en queries.
- XSS: los datos se almacenan raw; el frontend escapa al renderizar (React lo hace por defecto).
- SSRF: validar URLs de webhooks salientes — no permitir IPs privadas (10.x, 172.16.x, 192.168.x, 127.x, ::1).

---

### E10. Testing Strategy — Pirámide de tests

```
         ┌─────────┐
         │  E2E    │  (scripts/e2e-test.sh, curl contra Docker Compose)
         ├─────────┤
         │ Integr. │  (testcontainers-go, PostgreSQL real, GORM)
     ┌───┴─────────┴───┐
     │   Unit tests    │  (table-driven, interfaces mockeadas, sin DB)
     └─────────────────┘
```

**Unit tests (usecases)** — table-driven con interfaces mockeadas:

```go
func TestCreateSale(t *testing.T) {
    tests := []struct {
        name    string
        req     CreateSaleRequest
        setup   func(mocks)
        wantErr *apperror.Error
    }{
        {
            name: "venta exitosa con stock",
            req:  CreateSaleRequest{Items: []SaleItemReq{{ProductID: &prodID, Qty: 2, UnitPrice: 100}}},
            setup: func(m mocks) {
                m.repo.EXPECT().NextNumber(gomock.Any(), gomock.Any(), orgID, "sale").Return("VTA-00001", nil)
                m.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&Sale{ID: saleID}, nil)
                m.inventory.EXPECT().DeductStock(gomock.Any(), gomock.Any(), orgID, prodID, float64(2), saleID).Return(nil)
                m.payments.EXPECT().RegisterAutoPayment(gomock.Any(), gomock.Any(), orgID, saleID, float64(200), "cash").Return(nil)
            },
            wantErr: nil,
        },
        {
            name: "falla por stock insuficiente",
            req:  CreateSaleRequest{Items: []SaleItemReq{{ProductID: &prodID, Qty: 100}}},
            setup: func(m mocks) {
                m.repo.EXPECT().NextNumber(gomock.Any(), gomock.Any(), orgID, "sale").Return("VTA-00001", nil)
                m.repo.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&Sale{ID: saleID}, nil)
                m.inventory.EXPECT().DeductStock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
                    Return(apperror.NewBusinessRule("stock insuficiente"))
            },
            wantErr: &apperror.Error{Code: apperror.CodeBusinessRule},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            ctrl := gomock.NewController(t)
            m := newMocks(ctrl)
            tt.setup(m)
            uc := NewUsecases(m.repo, m.inventory, m.payments, m.audit, logger)
            _, err := uc.CreateSale(context.Background(), orgID, tt.req)
            if tt.wantErr == nil {
                assert.NoError(t, err)
            } else {
                var appErr *apperror.Error
                assert.True(t, errors.As(err, &appErr))
                assert.Equal(t, tt.wantErr.Code, appErr.Code)
            }
        })
    }
}
```

**Integration tests (repository)** — PostgreSQL real con testcontainers:

```go
func TestRepository(t *testing.T) {
    if testing.Short() { t.Skip("skipping integration test") }

    ctx := context.Background()
    container, db := testutil.SetupPostgres(ctx, t) // testcontainers + migraciones
    defer container.Terminate(ctx)

    repo := NewRepository(db)

    t.Run("create and get party", func(t *testing.T) {
        party := &domain.Party{OrgID: testOrgID, PartyType: "person", DisplayName: "Test"}
        created, err := repo.Create(ctx, db, party)
        require.NoError(t, err)
        assert.NotEmpty(t, created.ID)

        fetched, err := repo.GetByID(ctx, testOrgID, created.ID)
        require.NoError(t, err)
        assert.Equal(t, "Test", fetched.DisplayName)
    })
}
```

**Reglas de testing:**
- Tests unitarios usan `gomock` para generar mocks de las interfaces.
- Tests de integración usan `testcontainers-go` con PostgreSQL real (NO SQLite — no soporta `jsonb`, `gen_random_uuid()`, `timestamptz`, `CHECK` constraints).
- `go test ./... -short` ejecuta solo unit tests. `go test ./...` ejecuta todo.
- Cada módulo tiene al mínimo: tests unitarios del usecase principal + tests de integración del repository.
- Coverage mínimo: 70% para usecases, 60% para handlers.
- E2E: `scripts/e2e-test.sh` usa `curl` contra el Docker Compose levantado. Verifica flujos completos.

**Dependencias de testing:**

```bash
go install go.uber.org/mock/mockgen@latest
go get github.com/stretchr/testify
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

---

### E11. Configuration — Validación al startup

Las variables de entorno se validan al iniciar la app. Si falta una requerida, la app falla con un mensaje claro en vez de fallar silenciosamente en runtime.

```go
// internal/shared/config/config.go
type Config struct {
    DatabaseURL   string `env:"DATABASE_URL" required:"true"`
    Port          string `env:"PORT" default:"8080"`
    FrontendURL   string `env:"FRONTEND_URL" required:"true"`
    JWKSUrl       string `env:"JWKS_URL"`
    AuthEnableJWT bool   `env:"AUTH_ENABLE_JWT" default:"true"`
    // ... todos los campos con tags
}

func Load() (*Config, error) {
    cfg := &Config{}
    // Parsear env vars
    if err := parseEnv(cfg); err != nil {
        return nil, fmt.Errorf("config: %w", err)
    }
    // Validar reglas de negocio
    if cfg.AuthEnableJWT && cfg.JWKSUrl == "" {
        return nil, fmt.Errorf("config: JWKS_URL required when AUTH_ENABLE_JWT=true")
    }
    return cfg, nil
}
```

**Database connection pool:**

```go
func NewDB(databaseURL string) (*gorm.DB, error) {
    db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
        Logger:                 gormLogger,
        SkipDefaultTransaction: true, // +30% performance, transacciones explícitas donde se necesiten
    })
    if err != nil { return nil, err }

    sqlDB, _ := db.DB()
    sqlDB.SetMaxOpenConns(25)          // Lambda: pocas conexiones, RDS Proxy las multiplexa
    sqlDB.SetMaxIdleConns(5)
    sqlDB.SetConnMaxLifetime(5 * time.Minute)
    sqlDB.SetConnMaxIdleTime(1 * time.Minute)

    return db, nil
}
```

---

### E12. API Design Standards — Idempotency y Paginación

**Idempotency para POST** (operaciones financieras):

Los endpoints que crean ventas, pagos o movimientos de caja aceptan header `Idempotency-Key`. Si se reenvía el mismo request con la misma key, retorna el resultado original sin duplicar.

```go
// Middleware o lógica en el handler
func Idempotent(c *gin.Context) {
    key := c.GetHeader("Idempotency-Key")
    if key == "" { c.Next(); return }

    // Buscar resultado previo en cache/DB
    cached, found := idempotencyStore.Get(key)
    if found {
        c.JSON(cached.StatusCode, cached.Body)
        c.Abort()
        return
    }
    c.Next()
    // Guardar resultado
    idempotencyStore.Set(key, response, 24*time.Hour)
}
```

**Cursor Pagination estándar:**

```go
// pkg/pagination/pagination.go
type Params struct {
    Limit  int       `form:"limit" binding:"omitempty,min=1,max=100"`
    After  *uuid.UUID `form:"after"`
    Search string    `form:"search" binding:"omitempty,max=200"`
    Sort   string    `form:"sort" binding:"omitempty,oneof=created_at updated_at display_name name"`
    Order  string    `form:"order" binding:"omitempty,oneof=asc desc"`
}

func (p *Params) SetDefaults() {
    if p.Limit == 0 { p.Limit = 20 }
    if p.Sort == "" { p.Sort = "created_at" }
    if p.Order == "" { p.Order = "desc" }
}

type Result[T any] struct {
    Items      []T       `json:"items"`
    Total      int64     `json:"total"`
    HasMore    bool      `json:"has_more"`
    NextCursor *string   `json:"next_cursor,omitempty"`
}
```

**Aplicación en queries GORM:**

```go
func (r *Repository) List(ctx context.Context, orgID uuid.UUID, p pagination.Params) (*pagination.Result[domain.Party], error) {
    p.SetDefaults()
    query := r.db.WithContext(ctx).Where("org_id = ? AND deleted_at IS NULL", orgID)

    if p.Search != "" {
        search := "%" + p.Search + "%"
        query = query.Where("display_name ILIKE ? OR email ILIKE ? OR phone ILIKE ? OR tax_id ILIKE ?", search, search, search, search)
    }
    if p.After != nil {
        query = query.Where("id > ?", *p.After) // o comparar por sort field
    }

    var total int64
    query.Model(&models.Party{}).Count(&total)

    var items []models.Party
    query.Order(fmt.Sprintf("%s %s", p.Sort, p.Order)).Limit(p.Limit + 1).Find(&items)

    hasMore := len(items) > p.Limit
    if hasMore { items = items[:p.Limit] }

    result := &pagination.Result[domain.Party]{
        Items:   toDomainSlice(items),
        Total:   total,
        HasMore: hasMore,
    }
    if hasMore && len(items) > 0 {
        cursor := items[len(items)-1].ID.String()
        result.NextCursor = &cursor
    }
    return result, nil
}
```

---

### E13. Graceful Shutdown

```go
// cmd/local/main.go
func main() {
    app := wire.InitializeApp()

    srv := &http.Server{
        Addr:         ":" + app.Config.Port,
        Handler:      app.Router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal().Err(err).Msg("server error")
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Fatal().Err(err).Msg("forced shutdown")
    }
    log.Info().Msg("server stopped gracefully")
}
```

---

### E14. Functional Options — Constructores complejos

Para constructores con muchas dependencias opcionales (como los usecases que reciben timeline, webhooks, currency, etc.), usar Functional Options en vez de setter injection.

```go
type Option func(*Usecases)

func WithTimeline(t TimelinePort) Option {
    return func(uc *Usecases) { uc.timeline = t }
}
func WithWebhooks(w WebhookPort) Option {
    return func(uc *Usecases) { uc.webhooks = w }
}
func WithCurrency(c CurrencyPort) Option {
    return func(uc *Usecases) { uc.currency = c }
}

func NewUsecases(repo *Repository, db *store.DB, logger zerolog.Logger, opts ...Option) *Usecases {
    uc := &Usecases{repo: repo, db: db, logger: logger}
    for _, opt := range opts {
        opt(uc)
    }
    return uc
}
```

**Regla**: dependencias obligatorias van como parámetros directos del constructor. Dependencias opcionales (ports nil-safe) van como `Option`. Esto reemplaza el patrón de setter injection (`SetTimeline`, `SetWebhooks`, etc.) con algo más idiomático y seguro (inmutable post-construcción).

---

### E15. Observability moderna — métricas + trazas distribuidas

Structured logging solo no alcanza. El sistema debe exponer las 3 señales de observabilidad: logs, métricas y trazas.

```go
// wire/bootstrap.go
router.Use(otelgin.Middleware("control-plane-api"))

// internal/shared/observability/metrics.go
var (
    httpRequests = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
        []string{"route", "method", "status"},
    )
    httpLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP latency"},
        []string{"route", "method"},
    )
    externalCalls = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "external_api_calls_total", Help: "External API calls"},
        []string{"provider", "operation", "status"},
    )
)
```

**Reglas:**
- HTTP entrante instrumentado con OpenTelemetry (`otelgin` en Go, instrumentación ASGI en Python).
- OTLP exporter configurable por ambiente. Si no está configurado, la app sigue funcionando sin exporter.
- Resource attributes obligatorios: `service.name`, `service.version`, `deployment.environment`.
- Métricas mínimas: request count, latency, errores 5xx, retries, fallos de webhooks, duración de jobs del scheduler.
- Trazar llamadas salientes a Stripe, Clerk, Mercado Pago y backend interno con propagación de contexto.

---

## Lambda entrypoint

```go
// backend/cmd/lambda/main.go
package main

import (
    "context"
    "github.com/aws/aws-lambda-go/lambda"
    ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
    "github.com/devpablocristo/pymes/control-plane/backend/wire"
)

var ginLambda *ginadapter.GinLambdaV2

func init() {
    app := wire.InitializeApp()
    ginLambda = ginadapter.NewGinLambdaV2(app.Router)
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    return ginLambda.ProxyWithContext(ctx, req)
}

func main() {
    lambda.Start(handler)
}
```

```go
// backend/cmd/local/main.go (dev)
package main

func main() {
    app := wire.InitializeApp()
    app.Router.Run(":8080")
}
```

**Mismo código Gin** para ambos entrypoints. Solo cambia cómo arranca.

---

## Módulo: Identity (Clerk JWKS)

### `internal/identity/usecases.go`

```go
type Principal struct {
    OrgID  string
    Actor  string
    Role   string
    Scopes []string
}

type JWKSVerifier interface {
    VerifyToken(ctx context.Context, tokenString string) (*jwt.Token, error)
}

type Usecases struct {
    verifier JWKSVerifier
    issuer   string
}

func (u *Usecases) ResolvePrincipal(ctx context.Context, token string) (Principal, error)
```

**Lógica de ResolvePrincipal:**
1. `verifier.VerifyToken(token)` — verifica firma con JWKS remoto (Clerk publica su JWKS en `https://<clerk-domain>/.well-known/jwks.json`)
2. Extrae claims: `sub` (actor), `org_id`, `org_role`, `org_permissions` o `scopes`
3. Clerk puede enviar scopes como string CSV o array — soportar ambos
4. Retorna `Principal`

### `internal/identity/executor/jwks/verifier.go`

Usa `github.com/MicahParks/keyfunc/v3` para cachear JWKS:

```go
type Verifier struct {
    jwks *keyfunc.JWKS
}

func NewVerifier(jwksURL string) (*Verifier, error)
func (v *Verifier) VerifyToken(ctx context.Context, token string) (*jwt.Token, error)
```

---

## Módulo: Auth middleware

### `internal/shared/handlers/cors_middleware.go`

**CORS middleware** (necesario: frontend en S3+CloudFront es un origen distinto a API Gateway).

Configurar Gin CORS middleware con:
- `AllowOrigins`: `FRONTEND_URL` (de env var). En dev: `http://localhost:5173`.
- `AllowMethods`: GET, POST, PUT, DELETE, OPTIONS
- `AllowHeaders`: Authorization, Content-Type, X-API-KEY, X-Actor, X-Role, X-Scopes
- `AllowCredentials`: true

### `internal/shared/handlers/auth_middleware.go`

**Dual auth: JWT (Clerk) + API key.**

Flujo:
1. Si `Authorization: Bearer <token>` presente y JWT habilitado → `identity.ResolvePrincipal(token)` → inyecta org_id, actor, role, scopes en context
2. Si no JWT, y header `X-API-KEY` presente → SHA256 del key → buscar en DB (`org_api_keys`) → inyecta org_id, actor, scopes
3. Si API key, headers opcionales: `X-Actor`, `X-Role`, `X-Scopes` (CSV) — se intersectan con scopes de la key
4. Si ninguno → 401

**Context keys:**
```go
const (
    CtxKeyOrgID      = "org_id"
    CtxKeyActor      = "actor"
    CtxKeyRole       = "role"
    CtxKeyScopes     = "scopes"
    CtxKeyAuthMethod = "auth_method" // "jwt" | "api_key"
)
```

---

## Módulo: Clerk webhooks

### `internal/clerkwebhook/handler.go`

**Registro:** `POST /v1/webhooks/clerk` (sin auth middleware — Clerk no envía JWT).

**Verificación Svix manual (sin SDK):**
1. Headers: `svix-id`, `svix-timestamp`, `svix-signature`
2. Verificar timestamp: `|now - timestamp| <= 5 min`
3. Mensaje: `{svix-id}.{svix-timestamp}.{body}`
4. HMAC-SHA256 con secret (base64, prefijo `whsec_` removido)
5. Comparar con `hmac.Equal` contra firma del header (puede haber varias separadas por espacio)

**Rate limit:** configurar throttling en API Gateway por ruta (60 req/min para `/v1/webhooks/clerk`). No usar mutex en memoria — Lambda escala a N instancias concurrentes y cada una tiene su propia memoria.

**Eventos manejados:**

| Evento | Acción |
|--------|--------|
| `user.created` | Upsert user en DB + enviar welcome email (sincrónico, antes de responder) |
| `user.updated` | Upsert user (email, name, avatar) |
| `user.deleted` | Soft delete |
| `organization.created` | Crear org en DB |
| `organizationMembership.created` | Crear membership |
| `organizationMembership.deleted` | Borrar membership |

**Handler estructura:**

```go
type NotificationPort interface {
    NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}

type Handler struct {
    usersUC       *users.Usecases
    notifications NotificationPort // nil-safe: si nil, no notifica
    webhookSecret string
    frontendURL   string
    logger        zerolog.Logger
}
```

**Despacho de welcome:** sincrónico antes de responder. En Lambda, las goroutines fire-and-forget pueden no completarse porque el runtime congela la instancia al retornar. Errores de notificación se logean pero no fallan el webhook (se traga el error).

---

## Módulo: Billing (Stripe)

### Entidades de dominio

```go
type PlanCode string
const (
    PlanStarter    PlanCode = "starter"
    PlanGrowth     PlanCode = "growth"
    PlanEnterprise PlanCode = "enterprise"
)

type BillingStatus string
const (
    BillingTrialing BillingStatus = "trialing"
    BillingActive   BillingStatus = "active"
    BillingPastDue  BillingStatus = "past_due"
    BillingCanceled BillingStatus = "canceled"
)

type HardLimits struct {
    // Definir según tu dominio. Ejemplo:
    UsersMax    int `json:"users_max"`
    StorageMB   int `json:"storage_mb"`
    APICallsRPM int `json:"api_calls_rpm"`
}
```

### StripeClient (thread-safe)

```go
type StripeClientPort interface {
    CreateCustomer(params *stripe.CustomerParams) (*stripe.Customer, error)
    CreateCheckoutSession(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error)
    CreatePortalSession(params *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error)
    GetSubscription(subscriptionID string) (*stripe.Subscription, error)
    ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error)
}

type StripeClient struct {
    api *client.API  // stripe-go/v81 client.API — per-instance, thread-safe
}

func NewStripeClient(secretKey string) *StripeClient {
    sc := &StripeClient{}
    if secretKey != "" {
        sc.api = &client.API{}
        sc.api.Init(secretKey, nil)
    }
    return sc
}
```

**IMPORTANTE**: usar `client.API` (por instancia), NO `stripe.Key` (global, thread-unsafe).

### Usecases

```go
type Usecases struct {
    repo            *Repository
    stripe          StripeClientPort
    tenantSettings  TenantSettingsPort
    notifications   NotificationPort // nil-safe
    frontendURL     string
    priceIDs        map[PlanCode]string
    webhookSecret   string
    logger          zerolog.Logger
}
```

**Métodos:**
- `GetBillingStatus(ctx, orgID)` → plan, status, limits, usage, period end
- `CreateCheckoutSession(ctx, orgID, planCode, successURL, cancelURL, actor)` → checkout URL
- `CreatePortalSession(ctx, orgID, returnURL, actor)` → portal URL
- `GetUsageSummary(ctx, orgID)` → counters del período actual
- `HandleWebhookEvent(ctx, stripe.Event)` → procesa webhooks

**Flujo CreateCheckoutSession:**
1. Validar que Stripe está configurado
2. Normalizar planCode
3. Mapear plan → priceID (de config)
4. Asegurar que existen tenant_settings para la org (crear si no)
5. Asegurar que existe Stripe customer (crear si no, con email del actor)
6. Crear sesión con metadata `{org_id, plan_code}`
7. Retornar session.URL

**Webhooks (POST /v1/webhooks/stripe, sin auth):**

| Evento | Acción |
|--------|--------|
| `checkout.session.completed` | Extraer org_id de metadata → aplicar plan + subscription → notificar `plan_upgraded` |
| `customer.subscription.updated` | Resolver org por subscription_id o customer_id → actualizar plan |
| `customer.subscription.deleted` | Volver a plan starter → limpiar subscription → notificar `subscription_canceled` |
| `invoice.payment_succeeded` | billing_status = active |
| `invoice.payment_failed` | billing_status = past_due → notificar `payment_failed` |

**Rate limit en webhook:** configurar throttling en API Gateway por ruta (120 req/min para `/v1/webhooks/stripe`). No usar mutex en memoria.

**Verificación:** `stripe.ConstructWebhookEvent(payload, sigHeader, webhookSecret)`.

**Notificaciones:** sincrónicas antes de responder (SES tarda ~50-100ms, aceptable). Errores se logean pero no fallan el webhook. Si `notifications` es nil, no se envían. En Lambda NO usar goroutines fire-and-forget — el runtime congela la instancia al retornar.

### Migración billing

```sql
ALTER TABLE tenant_settings
  ADD COLUMN IF NOT EXISTS stripe_customer_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS stripe_subscription_id text UNIQUE,
  ADD COLUMN IF NOT EXISTS billing_status text NOT NULL DEFAULT 'trialing'
    CHECK (billing_status IN ('trialing','active','past_due','canceled','unpaid'));

CREATE INDEX IF NOT EXISTS idx_tenant_settings_stripe_customer
  ON tenant_settings(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
```

---

## Módulo: Notifications (SES/SMTP/Noop)

### EmailSender interface

```go
type EmailSender interface {
    Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}
```

3 implementaciones:
- `NoopSender` — solo logea (to, subject), retorna nil. Default cuando no hay config.
- `SMTPSender` — usa `net/smtp` con multipart/alternative (text + html). Para dev con MailHog.
- `SESSender` — usa AWS SES SDK v2. Para producción.

**Selección por env var `NOTIFICATION_BACKEND`:**
- `""` o `"noop"` → NoopSender
- `"smtp"` → SMTPSender
- `"ses"` → SESSender

### NotificationPort

```go
type NotificationPort interface {
    Notify(ctx context.Context, orgID uuid.UUID, notifType string, data map[string]string) error
    NotifyUser(ctx context.Context, userExternalID string, notifType string, data map[string]string) error
}
```

Los módulos que envían notificaciones (billing, clerkwebhook, etc.) reciben `NotificationPort`. Si es nil, no envían. Las notificaciones se ejecutan sincrónicamente — en Lambda las goroutines fire-and-forget no son confiables. Errores de notificación se logean pero no fallan el request principal.

### Deduplicación

Dedup key: `{notifType}|{userID}|{referenceID}|{hourBucket}`

Antes de enviar: `HasLogByDedupKey(key)`. Si existe, se omite. Después de enviar: `CreateLog(entry con DedupKey)`.

### Templates

Embeber con `//go:embed templates/*.html`. Un template HTML base con variables: `Title`, `Message`, `ActionURL`, `ActionLabel`, `OrgName`, `PreferencesURL`. El contenido (subject, message, action label) varía por tipo de notificación.

### Tipos de notificación (base transversal)

| Tipo | Trigger | Destinatario |
|------|---------|-------------|
| `welcome` | Clerk `user.created` | El usuario nuevo |
| `plan_upgraded` | Stripe `checkout.session.completed` | Admins de la org |
| `payment_failed` | Stripe `invoice.payment_failed` | Admins de la org |
| `subscription_canceled` | Stripe `subscription.deleted` | Admins de la org |

Los verticales podrán agregar sus propios tipos (ej. `appointment_reminder` para salud).

### Preferencias

Tabla `notification_preferences`: `(user_id, notification_type, channel, enabled)`. Unique por `(user_id, type, channel)`. Default: todo habilitado.

Tabla `notification_log`: registro de cada envío con dedup_key único.

---

## Módulo: Admin

### Tenant settings

```go
type TenantSettings struct {
    OrgID      uuid.UUID
    PlanCode   string
    HardLimits map[string]any // JSON
    UpdatedBy  *string
    UpdatedAt  time.Time
}
```

Hard limits por defecto según plan:

| Plan | users_max | storage_mb | api_calls_rpm |
|------|-----------|------------|---------------|
| starter | 5 | 500 | 100 |
| growth | 25 | 5000 | 500 |
| enterprise | unlimited | 50000 | 2000 |

### Activity log

Cada operación admin se logea:

```go
type AdminActivityEvent struct {
    ID           uuid.UUID
    OrgID        uuid.UUID
    Actor        *string
    Action       string
    ResourceType string
    Payload      map[string]any
    CreatedAt    time.Time
}
```

### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | /v1/admin/bootstrap | Overview: permisos, settings, auth context |
| GET | /v1/admin/tenant-settings | Leer settings actuales |
| PUT | /v1/admin/tenant-settings | Actualizar plan/limits |
| GET | /v1/admin/activity | Últimos 200 eventos |

Permisos: role `admin` o scope `admin:console:read`/`admin:console:write`.

---

## Módulo: Users & API keys

### API key generation

```go
func generateAPIKey() string {
    b := make([]byte, 32)
    crypto.Read(b)
    return "psk_" + hex.EncodeToString(b)  // prefijo "psk_" (pymes saas key)
}
```

- **Almacenamiento**: SOLO el SHA256 hash del key se guarda en DB. El raw se muestra una sola vez al crear.
- **Rotación**: genera nuevo key, actualiza hash en DB (mismo ID), retorna nuevo raw.
- **Scopes**: array de strings asociados a cada key. Se intersectan con los del request.

### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | /v1/users/me | Perfil del usuario autenticado |
| GET | /v1/orgs/:org_id/members | Listar miembros de la org |
| GET | /v1/orgs/:org_id/api-keys | Listar API keys (solo hash prefix) |
| POST | /v1/orgs/:org_id/api-keys | Crear API key (retorna raw una vez) |
| DELETE | /v1/orgs/:org_id/api-keys/:id | Revocar key |
| POST | /v1/orgs/:org_id/api-keys/:id/rotate | Rotar key (nuevo raw) |

Protección cross-org: verificar que `org_id` del path == `org_id` del context (JWT/API key).

---

## Módulo: Party (Party Model base)

### Entidades de dominio

```go
type Party struct {
    ID          uuid.UUID
    OrgID       uuid.UUID
    PartyType   string         // "person", "organization", "automated_agent"
    DisplayName string
    Email       string
    Phone       string
    Address     Address
    TaxID       string
    Notes       string
    Tags        []string
    Metadata    map[string]any
    CreatedAt   time.Time
    UpdatedAt   time.Time
    DeletedAt   *time.Time
    // Extension (uno de estos, según party_type)
    Person       *PartyPerson
    Organization *PartyOrganization
    Agent        *PartyAgent
    // Roles activos
    Roles        []PartyRole
}

type PartyPerson struct {
    FirstName string
    LastName  string
}

type PartyOrganization struct {
    LegalName    string
    TradeName    string
    TaxCondition string
}

type PartyAgent struct {
    AgentKind string  // "ai", "service", "integration", "bot"
    Provider  string
    Config    map[string]any
    IsActive  bool
}

type PartyRole struct {
    ID       uuid.UUID
    PartyID  uuid.UUID
    OrgID    uuid.UUID
    Role     string
    IsActive bool
    Metadata map[string]any
    CreatedAt time.Time
}

type PartyRelationship struct {
    ID               uuid.UUID
    OrgID            uuid.UUID
    FromPartyID      uuid.UUID
    ToPartyID        uuid.UUID
    RelationshipType string
    Metadata         map[string]any
    FromDate         time.Time
    ThruDate         *time.Time
}
```

### API base (CRUD de parties)

```
GET    /v1/parties                    — Listar (paginado, filtro por party_type, role, search)
POST   /v1/parties                    — Crear party (con roles y extensión)
GET    /v1/parties/:id                — Detalle con roles, extensión y relaciones
PUT    /v1/parties/:id                — Actualizar
DELETE /v1/parties/:id                — Soft delete

POST   /v1/parties/:id/roles         — Agregar rol
DELETE /v1/parties/:id/roles/:role    — Remover rol

GET    /v1/parties/:id/relationships  — Relaciones del party
POST   /v1/parties/:id/relationships  — Crear relación
```

**IMPORTANTE**: los módulos de negocio (Prompt 01+) exponen **aliases de conveniencia** para los roles más comunes. Ejemplo: `GET /v1/customers` es equivalente a `GET /v1/parties?role=customer`, y devuelve los mismos datos con el DTO adaptado al contexto de "cliente". Esto simplifica la API para el frontend sin romper el modelo unificado.

### Reglas de negocio

- `display_name` es obligatorio, mínimo 2 caracteres.
- `tax_id` es único por org (si se provee).
- Un party puede tener múltiples roles simultáneamente.
- Soft delete: `DELETE` setea `deleted_at`. Los queries filtran `WHERE deleted_at IS NULL`.
- Al crear un party con `party_type = 'person'`, se crea automáticamente el registro en `party_persons`.
- Idem para `organization` → `party_organizations` y `automated_agent` → `party_agents`.
- Búsqueda (`?search=`) busca en `display_name`, `email`, `phone`, `tax_id` con `ILIKE`.
- Al crear un `org_member`, se crea automáticamente un party (person) vinculado si no existe.

### Service Registry

```go
type Service struct {
    ID          uuid.UUID
    Name        string
    Direction   string  // "inbound", "outbound", "internal"
    Kind        string  // "webhook", "scheduler", "notification", "gateway"
    Description string
    Config      map[string]any
    IsActive    bool
}
```

Los services se registran en el seed o al inicializar la app. Ejemplo de seed:

```sql
INSERT INTO services (name, direction, kind, description) VALUES
    ('clerk_webhook', 'inbound', 'webhook', 'Clerk user/org sync'),
    ('stripe_webhook', 'inbound', 'webhook', 'Stripe billing events'),
    ('scheduler', 'internal', 'scheduler', 'Periodic task runner'),
    ('email_notifications', 'outbound', 'notification', 'SES/SMTP email sender'),
    ('outgoing_webhooks', 'outbound', 'webhook', 'Webhook dispatcher to external URLs');
```

---

## Módulo: Audit log

El audit log usa **actor estructurado** en vez de un string libre. Esto permite trazar acciones a parties, services o usuarios del sistema.

```go
type AuditEntry struct {
    ID           uuid.UUID
    OrgID        uuid.UUID
    ActorType    string         // "user", "party", "service", "system"
    ActorID      *uuid.UUID     // referencia al user, party o service
    ActorLabel   string         // nombre legible (denormalizado)
    Action       string
    ResourceType string
    ResourceID   string
    Payload      map[string]any
    PrevHash     string
    Hash         string
    CreatedAt    time.Time
}
```

**Actor types:**
- `user`: un usuario autenticado (JWT/API key) — `actor_id` apunta a `users.id`
- `party`: un party que ejecutó la acción (ej: AI agent) — `actor_id` apunta a `parties.id`
- `service`: un servicio de infraestructura (ej: scheduler, webhook) — `actor_id` apunta a `services.id`
- `system`: operación automática del sistema sin actor específico

**Hash chain**: cada entry calcula `SHA256(prev_hash + canonical_json(payload))`. Esto permite verificar integridad del log.

### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | /v1/audit | Listar (paginado, filtros por action, actor_type, actor_id, resource_type, date range) |
| GET | /v1/audit/export | Exportar CSV o JSONL |

---

## Módulo: Org (onboarding)

### `POST /v1/orgs` (público, post-signup)

Crea una organización nueva con tenant_settings default (plan starter).

---

## Migración base (0001)

```sql
-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Organizations
CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text UNIQUE,
    name text NOT NULL,
    slug text UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text UNIQUE NOT NULL,
    email text UNIQUE NOT NULL,
    name text NOT NULL DEFAULT '',
    avatar_url text NOT NULL DEFAULT '',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Org members (party_id vincula al usuario con su representación en el Party Model)
CREATE TABLE IF NOT EXISTS org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    party_id uuid,
    role text NOT NULL DEFAULT 'member',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_org_members_org ON org_members(org_id);
CREATE INDEX IF NOT EXISTS idx_org_members_user ON org_members(user_id);

-- Tenant settings
CREATE TABLE IF NOT EXISTS tenant_settings (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    plan_code text NOT NULL DEFAULT 'starter',
    hard_limits jsonb NOT NULL DEFAULT '{}',
    updated_by text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- API keys
CREATE TABLE IF NOT EXISTS org_api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name text NOT NULL DEFAULT '',
    key_hash text UNIQUE NOT NULL,
    key_prefix text NOT NULL DEFAULT '',
    created_by text,
    rotated_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_org_api_keys_org ON org_api_keys(org_id);
CREATE INDEX IF NOT EXISTS idx_org_api_keys_hash ON org_api_keys(key_hash);

-- API key scopes
CREATE TABLE IF NOT EXISTS org_api_key_scopes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id uuid NOT NULL REFERENCES org_api_keys(id) ON DELETE CASCADE,
    scope text NOT NULL,
    UNIQUE(key_id, scope)
);

-- ===== PARTY MODEL =====

-- Parties: entidad base unificada para todos los actores del negocio
CREATE TABLE IF NOT EXISTS parties (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    party_type text NOT NULL CHECK (party_type IN ('person', 'organization', 'automated_agent')),
    display_name text NOT NULL,
    email text,
    phone text,
    address jsonb NOT NULL DEFAULT '{}',
    tax_id text,
    notes text NOT NULL DEFAULT '',
    tags text[] NOT NULL DEFAULT '{}',
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_parties_org ON parties(org_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_type ON parties(org_id, party_type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_name ON parties(org_id, display_name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_email ON parties(org_id, email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parties_org_tax ON parties(org_id, tax_id) WHERE deleted_at IS NULL AND tax_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_parties_tags ON parties USING GIN(tags) WHERE deleted_at IS NULL;

-- Extension: persona natural
CREATE TABLE IF NOT EXISTS party_persons (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    first_name text NOT NULL DEFAULT '',
    last_name text NOT NULL DEFAULT ''
);

-- Extension: organización / empresa
CREATE TABLE IF NOT EXISTS party_organizations (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    legal_name text NOT NULL DEFAULT '',
    trade_name text NOT NULL DEFAULT '',
    tax_condition text NOT NULL DEFAULT ''
);

-- Extension: agente automatizado (IA, bot, integración)
CREATE TABLE IF NOT EXISTS party_agents (
    party_id uuid PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
    agent_kind text NOT NULL CHECK (agent_kind IN ('ai', 'service', 'integration', 'bot')),
    provider text NOT NULL DEFAULT '',
    config jsonb NOT NULL DEFAULT '{}',
    is_active boolean NOT NULL DEFAULT true
);

-- Roles: capa de capacidad — qué función cumple este party
CREATE TABLE IF NOT EXISTS party_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    role text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(party_id, org_id, role)
);

CREATE INDEX IF NOT EXISTS idx_party_roles_org_role ON party_roles(org_id, role) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_party_roles_party ON party_roles(party_id);

-- Relaciones entre parties
CREATE TABLE IF NOT EXISTS party_relationships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    from_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    to_party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
    relationship_type text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
    from_date timestamptz NOT NULL DEFAULT now(),
    thru_date timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_party_rels_org ON party_relationships(org_id);
CREATE INDEX IF NOT EXISTS idx_party_rels_from ON party_relationships(from_party_id);
CREATE INDEX IF NOT EXISTS idx_party_rels_to ON party_relationships(to_party_id);

-- Services: registro de componentes de infraestructura (no son parties)
CREATE TABLE IF NOT EXISTS services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    direction text NOT NULL CHECK (direction IN ('inbound', 'outbound', 'internal')),
    kind text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    config jsonb NOT NULL DEFAULT '{}',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- ===== FIN PARTY MODEL =====

-- Audit log (con actor estructurado)
CREATE TABLE IF NOT EXISTS audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    actor_type text NOT NULL DEFAULT 'user' CHECK (actor_type IN ('user', 'party', 'service', 'system')),
    actor_id uuid,
    actor_label text NOT NULL DEFAULT '',
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_org_created ON audit_log(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(org_id, actor_type, actor_id);

-- Usage counters (for billing metering)
CREATE TABLE IF NOT EXISTS org_usage_counters (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    counter_name text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    period text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(org_id, counter_name, period)
);

-- Admin activity
CREATE TABLE IF NOT EXISTS admin_activity_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL DEFAULT '',
    resource_id text,
    payload jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_admin_activity_org ON admin_activity_events(org_id, created_at DESC);
```

---

## Frontend

### Clerk integration

```tsx
// frontend/src/lib/auth.ts
export const clerkEnabled = !!import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;
```

```tsx
// frontend/src/main.tsx — wrap App with ClerkProvider conditionally
```

```tsx
// frontend/src/components/AuthTokenBridge.tsx
// useEffect that registers Clerk's getToken() with the global HTTP client
// so all API requests carry the JWT automatically
```

```tsx
// frontend/src/api/client.ts
// Request function that:
// 1. Checks for Clerk JWT token (Bearer)
// 2. Falls back to API key header (X-API-KEY)
// 3. Adds org_id if available
```

### Pages (base transversal)

| Page | Ruta | Descripción |
|------|------|-------------|
| LoginPage | /login | Clerk `<SignIn>` |
| SignupPage | /signup | Clerk `<SignUp>` |
| DashboardPage | / | Overview (placeholder para vertical) |
| BillingPage | /billing | Plan actual, usage, upgrade/manage |
| AdminPage | /admin | Tenant settings, activity log |
| SettingsPage | /settings | Clerk `<UserProfile>` |
| APIKeysPage | /settings/keys | CRUD de API keys |
| NotificationPreferencesPage | /settings/notifications | Toggles por tipo |

### Shell (navegación)

```tsx
const navItems = [
    { to: '/', label: 'Dashboard' },
    { to: '/admin', label: 'Admin' },
    { to: '/billing', label: 'Billing' },
    { to: '/settings/keys', label: 'API Keys' },
    { to: '/settings/notifications', label: 'Notifications' },
    { to: '/settings', label: 'Profile' },
];
```

Clerk `<UserButton>` para avatar/logout en el header.

---

## Docker Compose (dev local)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: pymes
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5434:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 5

  mailhog:
    image: mailhog/mailhog:v1.0.1
    ports:
      - "1025:1025"
      - "8025:8025"

  backend:
    build:
      context: ./control-plane/backend
      dockerfile: Dockerfile.dev
    ports:
      - "8100:8080"
    volumes:
      - ./control-plane/backend:/app
      - backend_go_mod:/go/pkg/mod
      - backend_go_build:/root/.cache/go-build
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/pymes?sslmode=disable
      SMTP_HOST: mailhog
      FRONTEND_URL: http://localhost:5180
    depends_on:
      postgres:
        condition: service_healthy

  frontend:
    build:
      context: ./control-plane/frontend
      dockerfile: Dockerfile.dev
    ports:
      - "5180:5173"
    volumes:
      - ./control-plane/frontend/src:/app/src
      - ./control-plane/frontend/public:/app/public
      - ./control-plane/frontend/index.html:/app/index.html
      - ./control-plane/frontend/vite.config.ts:/app/vite.config.ts
      - ./control-plane/frontend/tsconfig.json:/app/tsconfig.json
    env_file:
      - .env
    environment:
      VITE_API_URL: http://localhost:8100
    depends_on:
      - backend

volumes:
  pgdata:
  backend_go_mod:
  backend_go_build:
```

Hot reload: backend usa Air (recompila Go en cambios), frontend usa Vite HMR. No se necesita Redis (no hay rate-limiting complejo en base). Si un vertical lo necesita, se agrega después.

---

## Variables de entorno

```env
# ── Database ──
# Dentro de Docker (compose override): postgres://postgres:postgres@postgres:5432/pymes?sslmode=disable
# Fuera de Docker (go run local):      postgres://postgres:postgres@localhost:5434/pymes?sslmode=disable
DATABASE_URL=postgres://postgres:postgres@postgres:5432/pymes?sslmode=disable

# ── Server ──
PORT=8080

# ── Auth ──
JWKS_URL=https://<clerk-domain>/.well-known/jwks.json
JWT_ISSUER=https://<clerk-domain>
AUTH_ENABLE_JWT=true
AUTH_ALLOW_API_KEY=true

# ── Clerk ──
CLERK_SECRET_KEY=
CLERK_WEBHOOK_SECRET=

# ── Stripe ──
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_PRICE_STARTER=price_xxx
STRIPE_PRICE_GROWTH=price_yyy
STRIPE_PRICE_ENTERPRISE=price_zzz

# ── Notifications ──
NOTIFICATION_BACKEND=noop
AWS_REGION=us-east-1
AWS_SES_FROM_EMAIL=noreply@example.com
SMTP_HOST=mailhog
SMTP_PORT=1025

# ── CORS / Frontend URLs (puertos Docker: backend=8100, frontend=5180) ──
FRONTEND_URL=http://localhost:5180

# ── Frontend (Vite) ──
VITE_CLERK_PUBLISHABLE_KEY=
VITE_API_URL=http://localhost:8100
VITE_API_KEY=psk_local_admin
VITE_API_ACTOR=local-admin
VITE_API_ROLE=admin
VITE_API_SCOPES=admin:console:read,admin:console:write
```

---

## Diseño para verticales (futuro, NO implementar ahora)

La base deja preparado el punto de extensión en `internal/verticals/`. Cada vertical será un módulo que:

1. Define sus propias entidades de dominio, handlers, usecases, repos
2. Define sus propias migraciones SQL (ej. `0010_vertical_salud.up.sql`)
3. Registra sus rutas en un grupo `/v1/<vertical>/` (ej. `/v1/salud/turnos`)
4. Se habilita por feature flag o config (`VERTICALS_ENABLED=salud,talleres`)
5. Puede agregar tipos de notificación propios
6. Puede agregar counters de usage propios

El frontend sigue el mismo patrón: cada vertical agrega sus pages y rutas.

---

## Reglas de implementación

### Arquitectura y código
1. **Go**: 1.24, módulos, `zerolog` para logging, GORM para DB.
2. **Gin**: mismo engine para Lambda y local, solo cambia entrypoint. Usar `gin.New()` con middlewares explícitos (no `gin.Default()`).
3. **Hexagonal**: handler → usecases → repository. Interfaces definidas por el consumidor (accept interfaces, return structs).
4. **DI**: manual en `wire/bootstrap.go`. Dependencias obligatorias como parámetros del constructor, opcionales como `Option` (Functional Options pattern — ver E14).
5. **Secrets**: nunca en código ni en .tfvars. Secrets Manager en prod, .env en dev.

### Error handling
6. **Domain Errors**: todo error de negocio es `*apperror.Error` con `Code`, `Message`, `Details`, `HTTPStatus` (ver E1). Los repositorios convierten `gorm.ErrRecordNotFound` a `apperror.NewNotFound`.
7. **Error Middleware**: un solo middleware (`ErrorHandler`) serializa todos los errores al formato estándar (ver E2). Los handlers usan `c.Error(err)` + `return`, nunca `c.JSON(4xx, ...)` para errores.
8. **Validation**: binding tags de Gin para formato + validaciones de negocio en usecases (ver E3). Custom validators registrados al inicializar Gin.

### Transacciones y datos
9. **Transacciones**: toda operación multi-tabla usa `db.Transaction(ctx, fn)` (ver E4). Side-effects (audit, timeline, webhooks) van fuera de la transacción.
10. **Números secuenciales**: `SELECT ... FOR UPDATE` dentro de transacción para evitar race conditions.
11. **Migraciones**: numeradas, up/down, `IF NOT EXISTS` para idempotencia.
12. **DB Pool**: `MaxOpenConns=25`, `MaxIdleConns=5`, `ConnMaxLifetime=5min` (Lambda + RDS Proxy).

### Seguridad
13. **API keys**: SHA256 hash en DB, raw nunca se persiste.
14. **Clerk webhooks**: verificación Svix manual (HMAC-SHA256), sin SDK extra.
15. **Rate limits**: API Gateway throttling por ruta para webhooks (60/min Clerk, 120/min Stripe). Rate limiter in-process para API general (ver E9).
16. **Security Headers**: obligatorios en toda respuesta (ver E5).
17. **Input sanitization**: trim de strings, validación de URLs (sin IPs privadas para webhooks), parameterized queries (GORM).
18. **Stripe client**: usar `client.API` (per-instance), NO `stripe.Key` (global).

### Observability
19. **Middleware Pipeline**: orden estricto: Recovery → RequestID → Logger → Security → CORS → ErrorHandler → Timeout → Auth → RBAC (ver E5).
20. **Structured Logging**: zerolog con `request_id`, `org_id`, `actor` en cada request (ver E6). Nunca logear tokens, passwords ni PII.
21. **Health Checks**: `/healthz` (liveness) + `/readyz` (readiness con DB check) (ver E7).
22. **Graceful Shutdown**: en local server, capturar SIGINT/SIGTERM y drenar requests (ver E13).
23. **Tracing + Metrics**: OpenTelemetry + métricas Prometheus/OTLP para HTTP, DB, llamadas externas y scheduler (ver E15).

### Resiliencia
24. **Retry + Backoff**: llamadas a APIs externas (Stripe, Clerk, MP) usan retry con backoff exponencial + jitter (ver E8).
25. **Notificaciones**: sincrónicas antes de responder (en Lambda, goroutines fire-and-forget no son confiables). Errores se logean, nunca fallan el request. Si port es nil, no envían.
26. **Timeout por context**: 30s para requests HTTP, 10s para transacciones DB, 5s para llamadas externas.

### API Design
27. **Response envelope**: éxito → `{"data": ...}` o `{"data": [...], "meta": {...}}`. Error → `{"error": {"code", "message", "details", "request_id"}}` (ver E2).
28. **Cursor Pagination**: todo listado usa `?limit=&after=&search=&sort=&order=` con respuesta estándar (ver E12).
29. **Idempotency**: endpoints que crean transacciones financieras (ventas, pagos, movimientos) aceptan `Idempotency-Key` header (ver E12).
30. **Config validation**: toda variable de entorno se valida al startup. Si falta una requerida → fail fast con mensaje claro (ver E11).

### Testing
31. **Unit tests**: table-driven con `gomock` para interfaces. Todo usecase con lógica no trivial tiene tests. Min 70% coverage en usecases.
32. **Integration tests**: `testcontainers-go` con PostgreSQL real para repositorios. NO SQLite (no soporta `gen_random_uuid`, `jsonb`, `timestamptz`, `CHECK`).
33. **E2E tests**: `scripts/e2e-test.sh` con `curl` contra Docker Compose.
34. **Frontend**: React 18, TypeScript, Vite, TanStack Query, Clerk SDK.

---

## Criterios de éxito

### Build y tests
- [ ] `go build ./...` compila sin errores
- [ ] `go test ./...` todos los tests pasan (unit + integration)
- [ ] `go test -short ./...` pasa solo unit tests en <10s
- [ ] `npm run build` (frontend) exitoso
- [ ] `go vet ./...` sin warnings
- [ ] Coverage de usecases ≥ 70%

### Infraestructura
- [ ] Lambda entrypoint compila y expone Gin via `aws-lambda-go-api-proxy`
- [ ] Local entrypoint corre Gin en :8080 con graceful shutdown
- [ ] `docker-compose up` levanta postgres + mailhog + backend (Air) + frontend (Vite HMR)
- [ ] `GET /healthz` retorna 200, `GET /readyz` verifica DB

### Engineering Standards
- [ ] Error responses siguen formato estándar `{"error": {"code", "message", "details", "request_id"}}`
- [ ] Validation errors incluyen `details` por campo con mensajes legibles
- [ ] Request ID propagado en toda respuesta (`X-Request-ID` header)
- [ ] Security headers presentes en toda respuesta (X-Content-Type-Options, X-Frame-Options, etc.)
- [ ] Structured logs con `request_id`, `org_id`, `actor`, `latency`
- [ ] OpenTelemetry activo para HTTP y llamadas externas con `service.name`, `service.version`, `deployment.environment`
- [ ] Métricas mínimas expuestas/exportadas: requests, latencia, errores 5xx, retries externos, deliveries fallidos
- [ ] Config se valida al startup — variables requeridas faltantes causan fail fast
- [ ] DB connection pool configurado (max_open=25, max_idle=5, lifetime=5min)

### Funcionalidad
- [ ] Party Model: CRUD de parties funcional, con extensiones por tipo y roles
- [ ] Party Model: `GET /v1/parties?role=customer` filtra correctamente
- [ ] Party Model: un party puede tener múltiples roles simultáneamente
- [ ] Service registry: services se registran en seed y son consultables
- [ ] Audit log: entries con actor estructurado (actor_type + actor_id + actor_label)
- [ ] POST /v1/webhooks/clerk verifica Svix y sincroniza usuarios + crea party vinculado
- [ ] POST /v1/webhooks/stripe verifica firma y procesa checkout/cancellation/payment
- [ ] GET/PUT /v1/notifications/preferences funciona
- [ ] GET/PUT /v1/admin/tenant-settings funciona
- [ ] GET/POST/DELETE /v1/orgs/:org_id/api-keys funciona
- [ ] GET /v1/audit retorna entries con hash chain
- [ ] Auth middleware soporta JWT y API key dual
- [ ] Cursor pagination funciona en todos los listados
- [ ] Frontend: login → dashboard → billing → admin → settings funcional

---

## Orden de ejecución recomendado

**Aclaración importante**: este orden existe solo para respetar dependencias técnicas. **No reduce alcance**. Todo lo listado en este prompt sigue siendo obligatorio.

1. Crear estructura de directorios
2. `go mod init` + dependencias base (incluyendo `testify`, `gomock`, `testcontainers-go`)
3. **`pkg/apperror/`** — domain errors con Code, Message, HTTPStatus (E1)
4. **`pkg/pagination/`** — Params, Result genérico (E12)
5. **`pkg/resilience/`** — retry con backoff exponencial (E8)
6. **`pkg/validation/`** — custom validators y traductor de errores (E3)
7. `pkg/utils/` — SHA256, canonical JSON, API key generation
8. Migración SQL base (0001) — incluye Party Model, services, audit_log
9. **`internal/shared/config/`** — Config con validación al startup (E11)
10. **`internal/shared/store/`** — DB connection con pool config + Transaction helper (E4)
11. **`internal/shared/handlers/`** — Pipeline de middlewares completo (E5): RequestID, StructuredLogger, SecurityHeaders, CORS, ErrorHandler, Timeout, Auth
12. **`internal/shared/observability/`** — OpenTelemetry + métricas HTTP/externas (E15)
13. `internal/identity/` — JWKS verifier
14. `internal/org/` — org CRUD
15. `internal/party/` — Party CRUD, roles, relaciones, service registry
16. `internal/users/` — users + API keys
17. `internal/audit/` — audit log con hash chain y actor estructurado
18. `internal/admin/` — tenant settings + activity
19. `internal/clerkwebhook/` — Clerk webhooks (crea party al crear org_member)
20. `internal/billing/` — Stripe billing (con retry para API calls)
21. `internal/notifications/` — email (SES/SMTP/Noop) + preferences
22. `wire/bootstrap.go` — DI con Functional Options, middleware pipeline, rutas
23. `cmd/lambda/main.go` + `cmd/local/main.go` (con graceful shutdown)
24. **Unit tests** — usecases de party, billing, audit (table-driven + gomock)
25. **Integration tests** — repositories con testcontainers-go
26. Frontend: pages base
27. docker-compose.yml + .env.example + Makefile
28. **E2E tests** — scripts/e2e-test.sh
29. Verificar compilación, tests y coverage
