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
pymes/
├── .github/workflows/
│   ├── ci.yml              # Tests, lint, build
│   └── deploy.yml          # Build → zip → Lambda update + S3 sync
├── infra/                  # Terraform
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── terraform.tfvars.example
│   └── modules/
│       ├── networking/     # VPC, subnets, security groups
│       ├── database/       # RDS + RDS Proxy
│       ├── lambda/         # Lambda functions + API Gateway
│       ├── cdn/            # S3 + CloudFront
│       ├── secrets/        # Secrets Manager
│       └── monitoring/     # CloudWatch
├── backend/
│   ├── cmd/
│   │   ├── lambda/         # Lambda entrypoint
│   │   │   └── main.go
│   │   └── local/          # Local dev server (Gin directo)
│   │       └── main.go
│   ├── internal/
│   │   ├── identity/       # Clerk JWKS verification
│   │   ├── clerkwebhook/   # Clerk webhook handler
│   │   ├── billing/        # Stripe billing
│   │   ├── notifications/  # Email notifications (SES/SMTP/Noop)
│   │   ├── admin/          # Admin console, tenant settings
│   │   ├── users/          # User management, API keys
│   │   ├── org/            # Organization CRUD
│   │   ├── audit/          # Audit log
│   │   ├── shared/
│   │   │   ├── handlers/   # Auth middleware
│   │   │   └── authz/      # Permissions, scopes
│   │   └── verticals/      # Plugin point for verticals (empty for now)
│   ├── pkg/
│   │   ├── http/errors/    # Structured HTTP errors
│   │   ├── utils/          # AES-GCM, SHA256, canonical JSON
│   │   └── types/          # Context keys, error codes
│   ├── migrations/
│   │   ├── 0001_base_schema.up.sql
│   │   ├── 0001_base_schema.down.sql
│   │   ├── 0002_billing.up.sql
│   │   ├── 0002_billing.down.sql
│   │   ├── 0003_notifications.up.sql
│   │   └── 0003_notifications.down.sql
│   ├── wire/               # DI providers
│   ├── go.mod
│   └── go.sum
├── frontend/
│   ├── src/
│   │   ├── app/App.tsx
│   │   ├── api/client.ts   # HTTP client with Clerk JWT
│   │   ├── lib/
│   │   │   ├── api.ts      # API functions
│   │   │   ├── types.ts    # TypeScript types
│   │   │   └── auth.ts     # clerkEnabled flag
│   │   ├── components/
│   │   │   ├── Shell.tsx
│   │   │   ├── ProtectedRoute.tsx
│   │   │   └── AuthTokenBridge.tsx
│   │   └── pages/
│   │       ├── LoginPage.tsx
│   │       ├── SignupPage.tsx
│   │       ├── DashboardPage.tsx
│   │       ├── SettingsPage.tsx
│   │       ├── BillingPage.tsx
│   │       ├── AdminPage.tsx
│   │       ├── NotificationPreferencesPage.tsx
│   │       └── APIKeysPage.tsx
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── index.html
├── .env.example
├── docker-compose.yml      # Dev local: postgres, mailhog
├── Makefile
└── README.md
```

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
- La DI se resuelve en `wire/` con Google Wire

**Ejemplo de DI con Wire:**

```go
// wire/<module>_providers.go
var BillingSet = wire.NewSet(
    billing.NewRepository,
    ProvideStripeClient,
    ProvideTenantSettingsPort,    // adapter: admin.Repository → billing.TenantSettingsPort
    ProvideBillingNotificationPort, // adapter: notifications.Usecases → billing.NotificationPort
    billing.NewUsecases,
    billing.NewHandler,
)
```

Los adapters entre módulos se implementan como funciones provider que castean un tipo concreto a la interface del consumidor.

---

## Lambda entrypoint

```go
// backend/cmd/lambda/main.go
package main

import (
    "context"
    "github.com/aws/aws-lambda-go/lambda"
    ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
    // wire-generated app
)

var ginLambda *ginadapter.GinLambdaV2

func init() {
    // Wire injects everything; returns *gin.Engine
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

## Módulo: Audit log

Tabla `audit_log`:

```sql
CREATE TABLE audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
```

**Hash chain**: cada entry calcula `SHA256(prev_hash + canonical_json(payload))`. Esto permite verificar integridad del log.

### Endpoints

| Método | Ruta | Descripción |
|--------|------|-------------|
| GET | /v1/audit | Listar (paginado, filtros por action, actor, resource_type, date range) |
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

-- Org members
CREATE TABLE IF NOT EXISTS org_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

-- Audit log
CREATE TABLE IF NOT EXISTS audit_log (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL,
    actor text,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id text,
    payload jsonb,
    prev_hash text,
    hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_org_created ON audit_log(org_id, created_at DESC);

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
      - "5432:5432"
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
```

No se necesita Redis (no hay rate-limiting complejo en base). Si un vertical lo necesita, se agrega después.

---

## Variables de entorno

```env
# ── Database ──
DATABASE_URL=postgres://postgres:postgres@localhost:5432/pymes?sslmode=disable

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
SMTP_HOST=localhost
SMTP_PORT=1025

# ── CORS / Frontend URLs ──
FRONTEND_URL=http://localhost:5173

# ── Frontend (Vite) ──
VITE_CLERK_PUBLISHABLE_KEY=
VITE_API_URL=http://localhost:8080
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

1. **Go**: 1.24, módulos, `zerolog` para logging, GORM para DB
2. **Gin**: mismo engine para Lambda y local, solo cambia entrypoint
3. **Hexagonal**: handler → usecases → repository, interfaces definidas por el consumidor
4. **Wire**: para DI, un Set por módulo
5. **Secrets**: nunca en código ni en .tfvars. Secrets Manager en prod, .env en dev
6. **Notificaciones**: sincrónicas antes de responder (en Lambda, goroutines fire-and-forget no son confiables). Errores se logean, nunca fallan el request. Si port es nil, no envían.
7. **Stripe client**: usar `client.API` (per-instance), NO `stripe.Key` (global)
8. **API keys**: SHA256 hash en DB, raw nunca se persiste
9. **Clerk webhooks**: verificación Svix manual (HMAC-SHA256), sin SDK extra
10. **Rate limits en webhooks**: throttling en API Gateway por ruta (60/min Clerk, 120/min Stripe). No mutex en memoria — no funciona en Lambda multi-instancia
11. **Frontend**: React 18, TypeScript, Vite, TanStack Query, Clerk SDK
12. **Tests**: unitarios para usecases (mockear EmailSender, Repository), testcontainers-go con PostgreSQL para repo tests (SQLite no soporta gen_random_uuid, jsonb, timestamptz, CHECK constraints)
13. **Migraciones**: numeradas, up/down, `IF NOT EXISTS` para idempotencia

---

## Criterios de éxito

- [ ] `go build ./...` compila sin errores
- [ ] `go test ./...` todos los tests pasan
- [ ] `npm run build` (frontend) exitoso
- [ ] Lambda entrypoint compila y expone Gin via `aws-lambda-go-api-proxy`
- [ ] Local entrypoint corre Gin en :8080
- [ ] POST /v1/webhooks/clerk verifica Svix y sincroniza usuarios
- [ ] POST /v1/webhooks/stripe verifica firma y procesa checkout/cancellation/payment
- [ ] GET/PUT /v1/notifications/preferences funciona
- [ ] GET/PUT /v1/admin/tenant-settings funciona
- [ ] GET/POST/DELETE /v1/orgs/:org_id/api-keys funciona
- [ ] GET /v1/audit retorna entries con hash chain
- [ ] Auth middleware soporta JWT y API key dual
- [ ] Frontend: login → dashboard → billing → admin → settings funcional
- [ ] docker-compose up levanta postgres + mailhog
- [ ] Estructura de archivos limpia, sin código de dominio específico

---

## Orden de ejecución recomendado

1. Crear estructura de directorios
2. `go mod init` + dependencias base
3. Migración SQL base (0001)
4. `pkg/` — utils, types, http errors
5. `internal/identity/` — JWKS verifier
6. `internal/shared/` — auth middleware, authz
7. `internal/org/` — org CRUD
8. `internal/users/` — users + API keys
9. `internal/audit/` — audit log con hash chain
10. `internal/admin/` — tenant settings + activity
11. `internal/clerkwebhook/` — Clerk webhooks
12. `internal/billing/` — Stripe billing
13. `internal/notifications/` — email (SES/SMTP/Noop) + preferences
14. `wire/` — providers y bootstrap routes
15. `cmd/lambda/main.go` + `cmd/local/main.go`
16. Frontend: pages base
17. docker-compose.yml + .env.example + Makefile
18. Tests unitarios
19. Verificar compilación y tests
