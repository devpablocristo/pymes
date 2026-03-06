# Prompt 04 — Pasarelas de Cobro

## Contexto

Este prompt agrega la capacidad de que **la pyme cobre digitalmente a sus clientes** desde la plataforma. No confundir con Stripe (Prompt 00) que cobra la suscripcion del SaaS. Aca la pyme le cobra a su propio cliente.

**Prerequisitos**: Prompts 00, 01 y 02 implementados. El modulo `payments` (Prompt 02) ya registra pagos manualmente. Este prompt automatiza ese registro cuando el pago llega via pasarela.

**Regla fundamental**: estos modulos viven dentro de `control-plane/backend/internal/` como los demas. La integracion con cada pasarela es un adapter que implementa una interfaz comun.

**Party Model & Services**: Mercado Pago se registra como `service` en la tabla `services` (direction='inbound' para el webhook IPN, direction='outbound' para la creación de preferencias). Las acciones automáticas del webhook (registrar pagos) usan `audit_log.actor_type='service'` referenciando el service de MP. Las `payment_preferences` referencian `party_id` del pagador cuando se conoce (post-pago).

**Realidad Argentina/LATAM**: en la calle se cobra con QR (Mercado Pago, Modo, bancos), transferencia bancaria (alias/CBU) y efectivo. Las tarjetas de credito/debito en mostrador usan terminales POS que no se integran via API. Este prompt cubre los medios digitales que SI se pueden integrar.

## Alcance obligatorio

Todo lo definido en este prompt forma parte del alcance requerido del módulo de cobros: transferencia, QR estático, integración con Mercado Pago, seguridad, idempotencia, inbox/webhooks, auditoría, testing y criterios operativos. No debe leerse como una lista de ideas opcionales.

Que un método tenga menor complejidad o que otro dependa de OAuth/webhooks no cambia su obligatoriedad. El orden de construcción solo responde a dependencias técnicas.

---

## Metodos de cobro a implementar

| # | Metodo | Automatiza pago? | Complejidad | Prioridad |
|---|--------|-----------------|-------------|-----------|
| 1 | Transferencia bancaria | No (manual) | Baja | 1 |
| 2 | QR estatico | No (manual) | Baja | 1 |
| 3 | Mercado Pago (QR dinamico + link de pago) | Si (webhook) | Media | 2 |
| 4 | Otros providers (Uala Bis, Modo) | Si (webhook) | Media | Futuro |

---

## 1. Transferencia Bancaria

### Problema

Es el metodo de pago digital mas usado entre pymes argentinas. El cliente transfiere a un alias/CBU y manda el comprobante por WhatsApp. Hoy la pyme tiene que recordar sus datos bancarios, dictarlos, y registrar el pago manualmente.

### Solucion

Guardar los datos bancarios en `tenant_settings` para que aparezcan automaticamente en PDFs, comprobantes, mensajes de WhatsApp y en la UI.

### tenant_settings — columnas nuevas

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS bank_holder text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_cbu text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_alias text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_name text NOT NULL DEFAULT '';
```

### Donde se muestran

- **PDFs** (modulo `pdfgen` de Prompt 02): al pie del presupuesto y comprobante de venta, seccion "Datos para transferencia".
- **WhatsApp**: agregar template configurable en `tenant_settings`:

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS wa_payment_template text NOT NULL DEFAULT
        'Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}';
```

- **API**: nuevo endpoint para generar link de WhatsApp con datos de pago:

```
GET /v1/whatsapp/sale/:id/payment-info — Link con datos bancarios + monto de la venta
```

- **UI**: en la pantalla de venta, boton "Enviar datos de pago" que genera el link de WhatsApp.

### Confirmacion del pago

Manual. El vendedor/cajero registra el pago como ya existe:

```
POST /v1/sales/:id/payments
Body: { "method": "transfer", "amount": 8500 }
```

Esto ya actualiza `payments`, `sales.payment_status`, `cashflow` y `accounts` (Prompt 02).

---

## 2. QR Estatico

### Problema

Muchas pymes tienen un QR impreso en el mostrador (Mercado Pago, banco, etc.). Ese QR siempre es el mismo — no tiene monto. El cliente lo escanea, pone el monto manualmente y paga.

### Solucion

Generar un QR con el alias de la pyme y mostrarlo en la plataforma para que lo impriman o lo muestren en pantalla.

### API

```
GET /v1/payment-methods/qr-static         — Imagen QR PNG con el alias del negocio
GET /v1/payment-methods/qr-static/download — Descarga QR en alta resolucion (para imprimir)
```

### Logica

```go
func GenerateStaticQR(alias string) ([]byte, error) {
    // 1. Si alias esta vacio, retornar error 422: "Configura tu alias en Ajustes"
    // 2. Generar QR con contenido = alias
    // 3. Retornar imagen PNG (512x512 para pantalla, 1024x1024 para descarga)
}
```

### Dependencia Go

```bash
go get github.com/skip2/go-qrcode
```

### Donde se muestra

- **UI**: seccion "Cobrar" → pestaña "QR" → muestra el QR + boton "Descargar para imprimir"
- **PDFs**: opcionalmente incluir QR estatico en los comprobantes (configurable por org)

### Confirmacion del pago

Manual, igual que transferencia. El vendedor no sabe automaticamente que le pagaron.

### tenant_settings — columna nueva

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS show_qr_in_pdf boolean NOT NULL DEFAULT false;
```

---

## 3. Mercado Pago — QR Dinamico + Link de Pago

### Problema

El QR estatico y la transferencia requieren confirmacion manual. Con Mercado Pago integrado, el pago se confirma automaticamente y el sistema se actualiza solo.

### Flujo general

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│ Vendedor │     │ Sistema  │     │   MP     │     │ Cliente  │
│          │     │          │     │   API    │     │          │
└────┬─────┘     └────┬─────┘     └────┬─────┘     └────┬─────┘
     │ Crear venta     │                │                │
     │────────────────►│                │                │
     │                 │ Crear pref.    │                │
     │                 │───────────────►│                │
     │                 │◄───────────────│                │
     │                 │  QR + link     │                │
     │◄────────────────│                │                │
     │  Mostrar QR     │                │                │
     │  o enviar link  │                │                │
     │─ ── ── ── ── ── ── ── ── ── ── ── ── ── ── ── ──►│
     │                 │                │   Paga con QR  │
     │                 │                │◄───────────────│
     │                 │   Webhook IPN  │                │
     │                 │◄───────────────│                │
     │                 │ Auto-registra: │                │
     │                 │ • payment      │                │
     │                 │ • cashflow     │                │
     │                 │ • sale status  │                │
     │                 │ • account      │                │
     │                 │ • timeline     │                │
     │  Notificacion   │                │                │
     │◄────────────────│                │                │
     │ "Pago recibido" │                │                │
```

### Conexion OAuth (la pyme conecta SU cuenta de MP)

Cada pyme tiene su propia cuenta de Mercado Pago. Para que tu plataforma genere QRs y reciba webhooks en su nombre, la pyme tiene que autorizar tu aplicacion via OAuth.

**Flujo OAuth**:

```
1. La pyme hace click en "Conectar Mercado Pago" en Ajustes
2. Redirige a MP: https://auth.mercadopago.com/authorization?client_id={app_id}&redirect_uri={callback}&response_type=code
3. La pyme autoriza en MP
4. MP redirige a tu callback con un ?code=xxx
5. Tu backend intercambia el code por access_token + refresh_token
6. Guardas los tokens encriptados en la DB
```

### Entidad de dominio

```go
type PaymentGatewayConnection struct {
    OrgID          uuid.UUID
    Provider       string     // "mercadopago"
    ExternalUserID string     // user_id de MP del comercio
    AccessToken    string     // encriptado en DB
    RefreshToken   string     // encriptado en DB
    TokenExpiresAt time.Time
    IsActive       bool
    ConnectedAt    time.Time
    UpdatedAt      time.Time
}

type PaymentPreference struct {
    ID              uuid.UUID
    OrgID           uuid.UUID
    Provider        string
    ExternalID      string     // preference_id de MP
    ReferenceType   string     // "sale" | "quote"
    ReferenceID     uuid.UUID
    Amount          float64
    Description     string
    PaymentURL      string     // link de pago
    QRData          string     // datos para generar QR dinamico
    Status          string     // "pending" | "approved" | "expired"
    ExternalPayerID string     // ID del pagador en MP (post-pago)
    PaidAt          *time.Time
    ExpiresAt       time.Time
    CreatedAt       time.Time
}
```

### Tabla SQL — Conexiones

```sql
CREATE TABLE IF NOT EXISTS payment_gateway_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago'
        CHECK (provider IN ('mercadopago')),
    external_user_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    refresh_token_encrypted text NOT NULL,
    token_expires_at timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    connected_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

### Tabla SQL — Preferencias de pago

```sql
CREATE TABLE IF NOT EXISTS payment_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago',
    external_id text NOT NULL DEFAULT '',
    reference_type text NOT NULL CHECK (reference_type IN ('sale', 'quote')),
    reference_id uuid NOT NULL,
    amount numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    payment_url text NOT NULL DEFAULT '',
    qr_data text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'refunded')),
    external_payer_id text NOT NULL DEFAULT '',
    paid_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_prefs_org
    ON payment_preferences(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_reference
    ON payment_preferences(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_external
    ON payment_preferences(provider, external_id)
    WHERE external_id != '';
```

### API

```
-- Conexion OAuth
GET    /v1/payment-gateway/connect           — Redirige a OAuth de MP
GET    /v1/payment-gateway/callback          — Callback de OAuth (recibe code, guarda tokens)
GET    /v1/payment-gateway/status            — Estado de la conexion (activa, provider, etc.)
DELETE /v1/payment-gateway/disconnect        — Desconectar (revoca tokens)

-- QR estatico (sin MP)
GET    /v1/payment-methods/qr-static         — QR con alias del negocio
GET    /v1/payment-methods/qr-static/download — QR alta resolucion

-- Preferencias de pago (con MP conectado)
POST   /v1/sales/:id/payment-link            — Generar link de pago + QR dinamico para una venta
POST   /v1/quotes/:id/payment-link           — Generar link de pago para un presupuesto
GET    /v1/sales/:id/payment-link            — Ver link/QR existente de una venta
GET    /v1/quotes/:id/payment-link           — Ver link/QR existente de un presupuesto

-- Webhook (publico, sin auth de usuario)
POST   /v1/webhooks/mercadopago              — IPN de Mercado Pago
```

### Crear preferencia de pago (Mercado Pago)

Cuando el vendedor hace click en "Generar link de pago" en una venta:

```go
type mpCreatePreference interface {
    CreatePreference(ctx context.Context, orgID uuid.UUID, req CreatePreferenceRequest) (*PaymentPreference, error)
}

type CreatePreferenceRequest struct {
    ReferenceType string    // "sale" | "quote"
    ReferenceID   uuid.UUID
}
```

**Logica**:

```go
func (uc *Usecases) CreatePreference(ctx context.Context, orgID uuid.UUID, req CreatePreferenceRequest) (*PaymentPreference, error) {
    // 1. Verificar que el org tiene MP conectado
    conn, err := uc.repo.GetConnection(ctx, orgID)
    if err != nil { return nil, ErrGatewayNotConnected }

    // 2. Refresh token si esta vencido
    if conn.TokenExpiresAt.Before(time.Now()) {
        conn, err = uc.refreshToken(ctx, conn)
    }

    // 3. Obtener datos de la venta/presupuesto
    var amount float64
    var description string
    switch req.ReferenceType {
    case "sale":
        sale, _ := uc.salesUC.GetByID(ctx, orgID, req.ReferenceID)
        amount = sale.Total
        description = fmt.Sprintf("Venta %s - %s", sale.Number, sale.PartyName)
    case "quote":
        quote, _ := uc.quotesUC.GetByID(ctx, orgID, req.ReferenceID)
        amount = quote.Total
        description = fmt.Sprintf("Presupuesto %s - %s", quote.Number, quote.PartyName)
    }

    // 4. Crear preferencia en MP
    mpPref, err := uc.mpClient.CreatePreference(ctx, conn.AccessToken, MPPreferenceInput{
        Items: []MPItem{{
            Title:     description,
            Quantity:  1,
            UnitPrice: amount,
            CurrencyID: "ARS",
        }},
        ExternalReference: fmt.Sprintf("%s:%s:%s", orgID, req.ReferenceType, req.ReferenceID),
        NotificationURL:   uc.webhookURL + "/v1/webhooks/mercadopago",
        AutoReturn:        "approved",
        BackURLs: MPBackURLs{
            Success: uc.frontendURL + "/payment/success",
            Failure: uc.frontendURL + "/payment/failure",
            Pending: uc.frontendURL + "/payment/pending",
        },
        ExpirationDateTo: time.Now().Add(72 * time.Hour).Format(time.RFC3339),
    })

    // 5. Guardar en DB
    pref := &PaymentPreference{
        OrgID:         orgID,
        Provider:      "mercadopago",
        ExternalID:    mpPref.ID,
        ReferenceType: req.ReferenceType,
        ReferenceID:   req.ReferenceID,
        Amount:        amount,
        Description:   description,
        PaymentURL:    mpPref.InitPoint,    // link de pago
        QRData:        mpPref.QRData,       // datos para QR dinamico
        Status:        "pending",
        ExpiresAt:     time.Now().Add(72 * time.Hour),
    }
    uc.repo.SavePreference(ctx, pref)

    // 6. Timeline entry
    uc.auditUC.Log(ctx, orgID, "payment_link_created", req.ReferenceType, req.ReferenceID.String(), nil)

    return pref, nil
}
```

### Webhook IPN (Instant Payment Notification)

Mercado Pago envia un POST a `/v1/webhooks/mercadopago` cuando un pago cambia de estado.

```go
func (h *Handler) HandleMPWebhook(c *gin.Context) {
    // 1. Verificar firma del webhook (X-Signature header)
    // MP usa HMAC-SHA256 con el webhook_secret de la aplicacion
    body, _ := io.ReadAll(c.Request.Body)
    if !verifyMPSignature(c.Request.Header, body, h.mpWebhookSecret) {
        c.Status(401)
        return
    }

    var notification MPNotification
    json.Unmarshal(body, &notification)

    // 2. Solo procesar notificaciones de pago
    if notification.Type != "payment" {
        c.Status(200)
        return
    }

    // 3. Persistir el evento crudo y responder rapido.
    // Best practice: inbox/journal primero, procesamiento despues.
    _ = h.uc.StoreWebhookEvent(c.Request.Context(), StoreWebhookEventInput{
        Provider:     "mercadopago",
        EventType:    notification.Type,
        ExternalID:   notification.Data.ID,
        RawPayload:   body,
        Signature:    c.Request.Header.Get("X-Signature"),
    })
    c.Status(200)
}
```

### Inbox pattern para webhooks de pago

Para pagos, no conviene hacer trabajo pesado en el request del webhook. El endpoint:

1. valida firma
2. persiste el evento crudo en `payment_gateway_events`
3. responde `200` rapido
4. un worker/scheduler procesa eventos pendientes con idempotencia

```sql
CREATE TABLE IF NOT EXISTS payment_gateway_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    external_event_id text NOT NULL,
    event_type text NOT NULL,
    raw_payload jsonb NOT NULL,
    signature text NOT NULL DEFAULT '',
    processed_at timestamptz,
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(provider, external_event_id)
);

CREATE INDEX IF NOT EXISTS idx_payment_gateway_events_pending
    ON payment_gateway_events(created_at)
    WHERE processed_at IS NULL;
```

```go
func (uc *Usecases) ProcessPendingWebhookEvents(ctx context.Context, limit int) error {
    events, err := uc.repo.LockPendingWebhookEvents(ctx, limit)
    if err != nil { return err }

    for _, evt := range events {
        paymentDetail, err := uc.GetMPPaymentDetail(ctx, evt.ExternalEventID)
        if err != nil {
            uc.repo.MarkWebhookEventError(ctx, evt.ID, err.Error())
            continue
        }

        parts := strings.Split(paymentDetail.ExternalReference, ":")
        orgID, _ := uuid.Parse(parts[0])
        referenceType := parts[1]
        referenceID, _ := uuid.Parse(parts[2])

        if paymentDetail.Status == "approved" {
            err = uc.ProcessApprovedPayment(ctx, ProcessPaymentInput{
                OrgID:         orgID,
                ReferenceType: referenceType,
                ReferenceID:   referenceID,
                Amount:        paymentDetail.TransactionAmount,
                Method:        "mercadopago",
                ExternalID:    paymentDetail.ID,
                PayerEmail:    paymentDetail.Payer.Email,
            })
        }
        if err != nil {
            uc.repo.MarkWebhookEventError(ctx, evt.ID, err.Error())
            continue
        }
        uc.repo.MarkWebhookEventProcessed(ctx, evt.ID)
    }
    return nil
}
```

### Procesar pago aprobado

Este es el punto clave — cuando MP confirma un pago, impacta en todo el sistema. La transacción garantiza atomicidad. Los side-effects van fuera.

```go
func (uc *Usecases) ProcessApprovedPayment(ctx context.Context, input ProcessPaymentInput) error {
    // Idempotency: verificar si ya procesamos este pago externo
    already, err := uc.repo.PreferenceAlreadyApproved(ctx, input.OrgID, input.ExternalID)
    if err != nil { return fmt.Errorf("check idempotency: %w", err) }
    if already {
        uc.logger.Info().Str("external_id", input.ExternalID).Msg("payment already processed")
        return nil
    }

    // Transacción atómica: todo lo financiero dentro
    err = uc.db.Transaction(ctx, func(tx *gorm.DB) error {
        // 1. Actualizar payment_preference → status = "approved"
        if err := uc.repo.UpdatePreferenceStatus(ctx, tx, input.OrgID, input.ExternalID, "approved"); err != nil {
            return fmt.Errorf("update preference: %w", err)
        }

        // 2. Registrar payment (reutiliza el flujo de Prompt 02)
        payment := Payment{
            OrgID:         input.OrgID,
            ReferenceType: input.ReferenceType,
            ReferenceID:   input.ReferenceID,
            Method:        "mercadopago",
            Amount:        input.Amount,
            Notes:         fmt.Sprintf("Pago MP #%s", input.ExternalID),
        }
        if err := uc.paymentsUC.RegisterPayment(ctx, tx, payment); err != nil {
            return fmt.Errorf("register payment: %w", err)
        }
        // RegisterPayment ya se encarga de (todo dentro del tx):
        //   - Crear el payment record
        //   - Actualizar sales.amount_paid y sales.payment_status
        //   - Generar cash_movement (income)
        //   - Actualizar account balance (si habia fiado)

        return nil
    })
    if err != nil { return err }

    // Side-effects best-effort (fuera de transacción, nil-safe)
    uc.notificationsUC.Send(ctx, input.OrgID, "payment_received", map[string]string{
        "amount": fmt.Sprintf("%.2f", input.Amount),
        "method": "Mercado Pago",
    })
    uc.webhookDispatcher.Dispatch(ctx, input.OrgID, "payment.gateway_received", map[string]any{
        "gateway": "mercadopago", "external_id": input.ExternalID,
        "reference_type": input.ReferenceType, "reference_id": input.ReferenceID,
    })
    uc.timelinePort.Record(ctx, domain.TimelineEntry{OrgID: input.OrgID, EventType: "payment_received"})
    uc.auditPort.Log(ctx, input.OrgID, "payment.gateway_received", input.ReferenceType, input.ReferenceID.String(), map[string]any{"gateway": "mercadopago", "amount": input.Amount})

    return nil
}
```

### Enviar link de pago por WhatsApp

Integrar con el modulo WhatsApp existente (Prompt 02):

```
GET /v1/whatsapp/sale/:id/payment-link — Genera link de WhatsApp con link de pago MP
```

Template nuevo en `tenant_settings`:

```sql
ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS wa_payment_link_template text NOT NULL DEFAULT
        'Hola {customer_name}, podes pagar {total} de tu compra {number} con este link: {payment_url}';
```

**Logica**: si la venta tiene un `payment_preference` activo, usa el `payment_url`. Si no tiene, genera uno automaticamente y luego arma el link de WhatsApp.

---

## Interfaz de abstraccion

Para soportar otros providers en el futuro sin cambiar codigo:

```go
type PaymentGateway interface {
    CreatePreference(ctx context.Context, accessToken string, input GatewayPreferenceInput) (*GatewayPreferenceOutput, error)
    GetPaymentDetail(ctx context.Context, accessToken string, paymentID string) (*GatewayPaymentDetail, error)
    RefreshToken(ctx context.Context, clientID, clientSecret, refreshToken string) (*GatewayTokens, error)
}

type GatewayPreferenceInput struct {
    Title            string
    Amount           float64
    Currency         string
    ExternalRef      string
    NotificationURL  string
    ExpiresAt        time.Time
}

type GatewayPreferenceOutput struct {
    ID         string
    PaymentURL string
    QRData     string
}

type GatewayPaymentDetail struct {
    ID                string
    Status            string    // "approved", "pending", "rejected"
    Amount            float64
    ExternalReference string
    PayerEmail        string
}
```

La implementacion de Mercado Pago:

```go
type MercadoPagoGateway struct {
    httpClient *http.Client
    baseURL    string  // "https://api.mercadopago.com"
}

func NewMercadoPagoGateway() *MercadoPagoGateway {
    return &MercadoPagoGateway{
        httpClient: &http.Client{Timeout: 10 * time.Second},
        baseURL:    "https://api.mercadopago.com",
    }
}
```

Para agregar Uala Bis u otro provider: crear `uala_gateway.go` que implemente `PaymentGateway` y agregar un case al factory.

---

## Mercado Pago Client — Endpoints utilizados

| Endpoint MP | Uso |
|-------------|-----|
| `POST /checkout/preferences` | Crear preferencia (link de pago + QR) |
| `GET /v1/payments/:id` | Consultar detalle de un pago |
| `POST /oauth/token` | Intercambiar code por tokens (OAuth) |
| `POST /oauth/token` (refresh) | Renovar access_token |

**No se usa SDK de MP.** Se hacen las llamadas HTTP directas — menos dependencias, mas control, y el SDK de Go de MP no es oficial ni estable.

---

## Seguridad

### Tokens encriptados

Los `access_token` y `refresh_token` de MP se guardan encriptados en la DB con AES-256-GCM. La clave de encriptacion viene de una variable de entorno:

```go
PAYMENT_GATEWAY_ENCRYPTION_KEY=<32 bytes hex>
```

En produccion, esta clave se guarda en AWS Secrets Manager.

**Crypto module con tests:**

```go
// crypto.go
func Encrypt(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return "", fmt.Errorf("create cipher: %w", err) }
    aesGCM, err := cipher.NewGCM(block)
    if err != nil { return "", fmt.Errorf("create GCM: %w", err) }
    nonce := make([]byte, aesGCM.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", fmt.Errorf("generate nonce: %w", err) }
    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(encoded string, key []byte) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil { return "", fmt.Errorf("decode base64: %w", err) }
    // ... decrypt AES-256-GCM
}
```

### Webhook verification

El webhook de MP se verifica con HMAC-SHA256 usando el `webhook_secret` de la aplicacion MP (no de la pyme). Esto garantiza que la notificacion viene de MP.

### Idempotency para webhooks

MP puede enviar la misma notificación múltiples veces. El webhook handler es **idempotente**:

```go
func (uc *Usecases) ProcessWebhook(ctx context.Context, notification MPNotification) error {
    // 1. Verificar si ya procesamos este payment ID (idempotency check)
    exists, err := uc.repo.PaymentPreferenceExistsByExternalPayment(ctx, notification.Data.ID)
    if err != nil { return err }
    if exists {
        uc.logger.Info().Str("payment_id", notification.Data.ID).Msg("webhook already processed, skipping")
        return nil // idempotente — retorna 200 sin reprocesar
    }

    // 2. Procesar normalmente...
}
```

### No guardar datos de tarjeta

Nunca se tocan datos de tarjeta. Todo pasa por el checkout de MP (hosted). La pyme y tu plataforma estan fuera de scope PCI.

### Domain Errors específicos

```go
var (
    ErrGatewayNotConnected  = apperror.NewPrecondition("La pasarela de pago no está conectada")
    ErrGatewayTokenExpired  = apperror.NewPrecondition("Token de pasarela expirado, reconecte")
    ErrPreferenceNotFound   = func(id string) *apperror.Error { return apperror.NewNotFound("payment_preference", id) }
    ErrPreferenceExpired    = apperror.NewBusinessRule("El link de pago ha expirado")
    ErrPlanLimitExceeded    = apperror.NewBusinessRule("Plan Starter no permite Mercado Pago. Upgrade a Growth.")
    ErrWebhookSignatureInvalid = apperror.NewError(apperror.CodeUnauthorized, "Firma de webhook inválida", 401)
)
```

---

## Modulo en el backend

### Estructura

```
internal/paymentgateway/
    usecases.go              — logica: connect, create preference, process payment
    handler.go               — endpoints HTTP + webhook handler
    repository.go            — GORM: connections, preferences
    handler/dto/dto.go       — DTOs
    usecases/domain/         — entidades
    repository/models/       — modelos GORM
    gateway/                 — implementaciones de PaymentGateway
        mercadopago.go       — cliente HTTP para MP
    crypto.go                — encriptar/desencriptar tokens
```

### Hexagonal

```go
// handler.go — define su port
type gatewayUsecases interface {
    GetConnectionStatus(ctx context.Context, orgID uuid.UUID) (*ConnectionStatus, error)
    InitOAuth(ctx context.Context, orgID uuid.UUID) (string, error)
    HandleOAuthCallback(ctx context.Context, orgID uuid.UUID, code string) error
    Disconnect(ctx context.Context, orgID uuid.UUID) error
    CreatePreference(ctx context.Context, orgID uuid.UUID, req CreatePreferenceRequest) (*PaymentPreference, error)
    GetPreference(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (*PaymentPreference, error)
    ProcessWebhook(ctx context.Context, provider string, headers http.Header, body []byte) error
}

type Handler struct {
    uc gatewayUsecases
}
```

---

## Actualizaciones al modulo payments (Prompt 02)

El campo `method` de la tabla `payments` necesita un valor nuevo:

```sql
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note', 'mercadopago'));
```

Cuando se agreguen mas providers, se extiende el CHECK: `'uala'`, `'modo'`, etc.

---

## Migraciones SQL

### `0016_payment_gateway.up.sql`

```sql
CREATE TABLE IF NOT EXISTS payment_gateway_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago'
        CHECK (provider IN ('mercadopago')),
    external_user_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    refresh_token_encrypted text NOT NULL,
    token_expires_at timestamptz NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    connected_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payment_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'mercadopago',
    external_id text NOT NULL DEFAULT '',
    reference_type text NOT NULL CHECK (reference_type IN ('sale', 'quote')),
    reference_id uuid NOT NULL,
    amount numeric(15,2) NOT NULL,
    description text NOT NULL DEFAULT '',
    payment_url text NOT NULL DEFAULT '',
    qr_data text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'refunded')),
    external_payer_id text NOT NULL DEFAULT '',
    paid_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_prefs_org
    ON payment_preferences(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_reference
    ON payment_preferences(org_id, reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_payment_prefs_external
    ON payment_preferences(provider, external_id)
    WHERE external_id != '';

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note', 'mercadopago'));

ALTER TABLE tenant_settings
    ADD COLUMN IF NOT EXISTS bank_holder text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_cbu text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_alias text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS bank_name text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS show_qr_in_pdf boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS wa_payment_link_template text NOT NULL DEFAULT
        'Podes transferir a:\nAlias: {bank_alias}\nCBU: {bank_cbu}\nTitular: {bank_holder}\nBanco: {bank_name}\nMonto: {total}';
```

**Seed de services** — registrar Mercado Pago como service:

```sql
INSERT INTO services (name, direction, kind, description) VALUES
    ('mercadopago_webhook', 'inbound', 'gateway', 'Mercado Pago IPN webhook'),
    ('mercadopago_api', 'outbound', 'gateway', 'Mercado Pago API (preferences, payments)')
ON CONFLICT (name) DO NOTHING;
```

### `0016_payment_gateway.down.sql`

```sql
ALTER TABLE tenant_settings
    DROP COLUMN IF EXISTS wa_payment_link_template,
    DROP COLUMN IF EXISTS show_qr_in_pdf,
    DROP COLUMN IF EXISTS bank_name,
    DROP COLUMN IF EXISTS bank_alias,
    DROP COLUMN IF EXISTS bank_cbu,
    DROP COLUMN IF EXISTS bank_holder;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments ADD CONSTRAINT payments_method_check
    CHECK (method IN ('cash', 'card', 'transfer', 'check', 'other', 'credit_note'));

DROP TABLE IF EXISTS payment_preferences;
DROP TABLE IF EXISTS payment_gateway_connections;
```

---

## Variables de entorno nuevas

Agregar a `.env.example` y `config.go`:

```env
# Payment Gateway (Mercado Pago)
MP_APP_ID=
MP_CLIENT_SECRET=
MP_WEBHOOK_SECRET=
MP_REDIRECT_URI=http://localhost:8100/v1/payment-gateway/callback
PAYMENT_GATEWAY_ENCRYPTION_KEY=0000000000000000000000000000000000000000000000000000000000000000
```

En `config.go`:

```go
MPAppID                      string  // MP_APP_ID
MPClientSecret               string  // MP_CLIENT_SECRET
MPWebhookSecret              string  // MP_WEBHOOK_SECRET
MPRedirectURI                string  // MP_REDIRECT_URI
PaymentGatewayEncryptionKey  string  // PAYMENT_GATEWAY_ENCRYPTION_KEY (64 hex chars = 32 bytes)
```

---

## Dependencias Go nuevas

```bash
go get github.com/skip2/go-qrcode
```

No se usa SDK de Mercado Pago — las llamadas HTTP se hacen con `net/http` estándar.

---

## Integracion con el AI Assistant (Prompt 03)

### Tools nuevas para modo internal

| Tool | Descripcion | Backend endpoint |
|------|-------------|-----------------|
| `generate_payment_link` | Generar link de pago MP para una venta | `POST /v1/sales/:id/payment-link` |
| `get_payment_status` | Ver estado de un link de pago | `GET /v1/sales/:id/payment-link` |
| `send_payment_info` | Enviar datos bancarios por WhatsApp | `GET /v1/whatsapp/sale/:id/payment-info` |

### Tools nuevas para modo external

| Tool | Descripcion | Backend endpoint |
|------|-------------|-----------------|
| `get_payment_link` | Obtener link de pago de un presupuesto | `GET /v1/public/:org_id/quote/:id/payment-link` |

Ejemplo de interaccion:

```
Cliente (WhatsApp): "Quiero pagar el presupuesto que me mandaron"
AI: [busca presupuesto del cliente → genera link de pago]
    "Aca tenes el link para pagar tu presupuesto PRE-00042 por $8.500:
     https://www.mercadopago.com.ar/checkout/v1/redirect?pref_id=xxx
     Podes pagar con QR, tarjeta o transferencia."
```

---

## Limites por plan

| Plan | Transferencia + QR estatico | Mercado Pago |
|------|----------------------------|-------------|
| Starter | Si | No |
| Growth | Si | Si (50 links/mes) |
| Enterprise | Si | Ilimitado |

---

## Actualizaciones a archivos existentes

| Archivo | Cambio |
|---------|--------|
| `wire/bootstrap.go` | Agregar paymentgateway handler, routes, webhook |
| `docker-compose.yml` | No cambia (no hay servicio nuevo) |
| `.env.example` | Agregar `MP_*`, `PAYMENT_GATEWAY_ENCRYPTION_KEY` |
| `.env` | Copiar nuevas vars |
| `config.go` | Agregar campos MP |
| `.gitignore` | No cambia |
| `README.md` | Agregar seccion Pasarelas de Cobro |

---

## Resilience — Cliente HTTP Mercado Pago

Todas las llamadas al API de MP usan retry con backoff:

```go
// gateway/mercadopago.go
func (gw *MercadoPagoGateway) CreatePreference(ctx context.Context, input CreatePrefInput) (*MPPreference, error) {
    var result MPPreference
    err := resilience.WithRetry(ctx, resilience.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   500 * time.Millisecond,
        MaxDelay:    5 * time.Second,
        RetryableCheck: func(err error) bool {
            // Retry solo en errores transitorios (5xx, timeout, connection reset)
            var httpErr *HTTPError
            if errors.As(err, &httpErr) {
                return httpErr.StatusCode >= 500 || httpErr.StatusCode == 429
            }
            return errors.Is(err, context.DeadlineExceeded) || isConnectionError(err)
        },
    }, func() error {
        resp, err := gw.httpClient.Post(ctx, "/checkout/preferences", input.ToMPBody())
        if err != nil { return err }
        return json.Unmarshal(resp, &result)
    })
    return &result, err
}
```

**Token refresh automático:**

```go
func (gw *MercadoPagoGateway) ensureValidToken(ctx context.Context, orgID uuid.UUID) (string, error) {
    conn, err := gw.repo.GetConnection(ctx, orgID)
    if err != nil { return "", ErrGatewayNotConnected }

    if conn.TokenExpiresAt.After(time.Now().Add(5 * time.Minute)) {
        token, _ := crypto.Decrypt(conn.AccessToken, gw.encryptionKey)
        return token, nil
    }

    // Token próximo a expirar → refresh
    refreshToken, _ := crypto.Decrypt(conn.RefreshToken, gw.encryptionKey)
    newTokens, err := gw.refreshOAuthToken(ctx, refreshToken)
    if err != nil { return "", ErrGatewayTokenExpired }

    // Guardar nuevos tokens encriptados
    encAccess, _ := crypto.Encrypt(newTokens.AccessToken, gw.encryptionKey)
    encRefresh, _ := crypto.Encrypt(newTokens.RefreshToken, gw.encryptionKey)
    gw.repo.UpdateTokens(ctx, orgID, encAccess, encRefresh, newTokens.ExpiresAt)

    return newTokens.AccessToken, nil
}
```

---

## Testing — Payment Gateway

### Tests unitarios

```go
func TestProcessApprovedPayment(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockRepo := mocks.NewMockpaymentGatewayRepo(ctrl)
    mockPayments := mocks.NewMockpaymentsUsecase(ctrl)
    mockNotif := mocks.NewMocknotificationsPort(ctrl)
    mockWebhook := mocks.NewMockwebhookDispatcher(ctrl)

    uc := NewUsecases(mockRepo, mockPayments, mockNotif, mockWebhook, logger)

    tests := []struct {
        name      string
        input     ProcessPaymentInput
        setup     func()
        wantErr   error
    }{
        {
            name:  "pago nuevo se procesa correctamente",
            input: validPaymentInput(),
            setup: func() {
                mockRepo.EXPECT().PreferenceAlreadyApproved(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
                mockRepo.EXPECT().UpdatePreferenceStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "approved").Return(nil)
                mockPayments.EXPECT().RegisterPayment(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
                mockNotif.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
                mockWebhook.EXPECT().Dispatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
            },
        },
        {
            name:  "pago duplicado retorna nil (idempotente)",
            input: validPaymentInput(),
            setup: func() {
                mockRepo.EXPECT().PreferenceAlreadyApproved(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
                // No se llama a nada más
            },
        },
        {
            name:  "gateway no conectado retorna error",
            input: noGatewayInput(),
            setup: func() {
                mockRepo.EXPECT().PreferenceAlreadyApproved(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
                mockRepo.EXPECT().UpdatePreferenceStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "approved").
                    Return(ErrGatewayNotConnected)
            },
            wantErr: ErrGatewayNotConnected,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            err := uc.ProcessApprovedPayment(context.Background(), tt.input)
            if tt.wantErr != nil {
                assert.ErrorIs(t, err, tt.wantErr)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Tests de crypto

```go
func TestCrypto_EncryptDecrypt(t *testing.T) {
    key := make([]byte, 32)
    rand.Read(key)

    tests := []struct {
        name      string
        plaintext string
    }{
        {"token normal", "APP_USR-1234567890abcdef"},
        {"token largo", strings.Repeat("x", 1024)},
        {"token vacío", ""},
        {"caracteres unicode", "token-cón-ñ-y-ü"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            encrypted, err := Encrypt(tt.plaintext, key)
            require.NoError(t, err)
            assert.NotEqual(t, tt.plaintext, encrypted)

            decrypted, err := Decrypt(encrypted, key)
            require.NoError(t, err)
            assert.Equal(t, tt.plaintext, decrypted)
        })
    }
}

func TestCrypto_WrongKeyFails(t *testing.T) {
    key1 := make([]byte, 32)
    key2 := make([]byte, 32)
    rand.Read(key1)
    rand.Read(key2)

    encrypted, _ := Encrypt("secret-token", key1)
    _, err := Decrypt(encrypted, key2)
    assert.Error(t, err)
}
```

### Tests de webhook signature

```go
func TestWebhookSignatureVerification(t *testing.T) {
    secret := "test-webhook-secret"
    payload := []byte(`{"action":"payment.updated","data":{"id":"12345"}}`)

    // Firma válida
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    validSig := hex.EncodeToString(mac.Sum(nil))

    tests := []struct {
        name    string
        sig     string
        wantErr bool
    }{
        {"firma válida", validSig, false},
        {"firma inválida", "abc123", true},
        {"firma vacía", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := VerifyWebhookSignature(payload, tt.sig, secret)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Tests de integración

```go
func TestPaymentGatewayRepository_Integration(t *testing.T) {
    // Usa testcontainers-go con PostgreSQL real
    // Verifica:
    // - SaveConnection + GetConnection round-trip
    // - Tokens encriptados en DB (leer raw de pg y verificar != plaintext)
    // - SavePreference + UpdateStatus
    // - PreferenceAlreadyApproved idempotency check
    // - ListPreferences con paginación cursor-based
}
```

---

## Orden de ejecucion recomendado

**Aclaración importante**: este orden es únicamente técnico. Todo lo especificado en este prompt sigue siendo obligatorio y parte del alcance final.

1. Migracion `0016_payment_gateway` (up + down)
2. `internal/paymentgateway/` — estructura de directorios
3. Entidades de dominio + modelos GORM
4. `pkg/crypto/` — módulo de encriptación AES-256-GCM + tests
5. Repository (connections + preferences)
6. `gateway/mercadopago.go` — cliente HTTP para MP con retry
7. Tabla `payment_gateway_events` + repository (inbox de webhooks)
8. Usecases: OAuth flow, create preference, process webhook (idempotente)
9. Handler: endpoints + webhook handler (verificación HMAC + persistencia rápida)
10. Scheduler/worker: `ProcessPendingWebhookEvents`
11. QR estatico: endpoint + generacion con `go-qrcode`
12. Actualizar `wire/bootstrap.go` — wiring + rutas
13. Actualizar WhatsApp handler: nuevo endpoint `/payment-info`
14. Actualizar PDF generator: incluir datos bancarios y QR
15. Actualizar `.env.example`, `.env`, `config.go`
16. Tests unitarios (crypto, webhook, usecases, gateway)
17. Tests integración (repository con testcontainers)
18. Verificar compilacion, webhook con mock, flujo OAuth

---

## Criterios de exito

### Funcional
- [ ] `go build ./...` compila sin errores
- [ ] QR estatico: `GET /v1/payment-methods/qr-static` retorna imagen PNG con alias del negocio
- [ ] Datos bancarios aparecen en PDFs generados y en mensajes de WhatsApp
- [ ] OAuth MP: `GET /v1/payment-gateway/connect` redirige a MP
- [ ] OAuth callback: recibe code, guarda tokens encriptados, status = activo
- [ ] `POST /v1/sales/:id/payment-link` genera link de pago MP con monto correcto
- [ ] Webhook MP: `POST /v1/webhooks/mercadopago` con pago approved → registra payment + cashflow + actualiza sale
- [ ] Verificacion de firma del webhook funciona (rechaza requests falsos)
- [ ] Tokens encriptados en DB (no plain text)
- [ ] `POST /v1/sales/:id/payment-link` retorna error si org no tiene MP conectado
- [ ] Plan starter: retorna error al intentar generar link MP
- [ ] `GET /v1/whatsapp/sale/:id/payment-info` genera link WhatsApp con datos bancarios
- [ ] Service registry: Mercado Pago registrado como service (inbound webhook + outbound API)
- [ ] Audit log: acciones del webhook MP registran actor_type='service'
- [ ] Webhook inbound usa inbox pattern (`payment_gateway_events`) y responde 200 sin hacer trabajo pesado
- [ ] Scheduler/worker procesa eventos pendientes y permite replay operativo

### Engineering Standards
- [ ] Domain errors tipados: `ErrGatewayNotConnected`, `ErrPreferenceNotFound`, `ErrWebhookSignatureInvalid`, etc.
- [ ] DTOs con binding tags de validación (required, uuid, oneof)
- [ ] Webhook handler idempotente (mismo payment_id → 200 sin reprocesar)
- [ ] Eventos inbound de MP persistidos con `UNIQUE(provider, external_event_id)` para deduplicación
- [ ] `ProcessApprovedPayment` usa transacción explícita; side-effects fuera del tx
- [ ] Retry con backoff en todas las llamadas HTTP a Mercado Pago (3 intentos, solo 5xx/429/timeout)
- [ ] Token refresh automático cuando `expires_at` < now + 5min
- [ ] Crypto module con tests (encrypt/decrypt roundtrip, wrong key fails)
- [ ] Webhook signature tests (firma válida, inválida, vacía)
- [ ] Tests unitarios table-driven con gomock para ProcessApprovedPayment (≥3 escenarios)
- [ ] Tests integración para repository con testcontainers-go
- [ ] Structured logging con request_id en todas las operaciones del webhook
- [ ] `go test ./internal/paymentgateway/... -cover` ≥ 70%
