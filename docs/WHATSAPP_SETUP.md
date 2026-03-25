# WhatsApp Business (Meta) + Pymes — guía corta

Pasos para conectar la API de WhatsApp con el **control plane** (`pymes-core`), webhooks y envíos. Sin secretos reales en este archivo: usá `.env` local o el gestor de secretos en prod.

---

## Qué ya está en el monorepo

| Ítem | Dónde / notas |
|------|----------------|
| Variables de entorno documentadas | `.env.example`: `WHATSAPP_WEBHOOK_VERIFY_TOKEN`, `WHATSAPP_APP_SECRET`, `WHATSAPP_GRAPH_API_BASE_URL` |
| Plantilla local | Tu `.env` puede incluir el mismo bloque (no versionar secretos) |
| Rutas HTTP del backend | `GET/POST/DELETE /v1/whatsapp/connection`, envíos bajo `/v1/whatsapp/send/*`, opt-in, plantillas, mensajes |
| Webhook público | `GET` y `POST /v1/webhooks/whatsapp` (sin el mismo auth JWT que el resto; firma + verify token) |
| Validación de firma | El backend usa `WHATSAPP_APP_SECRET` y la cabecera `X-Hub-Signature-256` en los `POST` |

---

## Qué tenés que tener en Meta (developers)

| Paso | Estado típico |
|------|----------------|
| App en [Meta for Developers](https://developers.facebook.com/apps) con producto **WhatsApp** | **Hecho** si ya creaste la app (ej. nombre de producto Pymes) |
| **App ID** y **Clave secreta de la app** (App Secret) en Configuración → Básica | **Hecho** para pruebas si ya copiaste el secret |
| Número / WABA vinculados al producto WhatsApp | Según tu cuenta Meta Business |
| **Phone Number ID** y **WhatsApp Business Account ID (WABA ID)** | Desde el panel API de WhatsApp de la app |
| **Access token** de la API (temporal de prueba o larga duración según doc Meta) | Lo vas a registrar en Pymes por organización (ver abajo) |
| Webhook: **URL de callback** + **Verify token** + campos suscritos | **Pendiente** hasta tener URL pública HTTPS (ver siguiente sección) |

---

## Qué tenés que configurar en Pymes (servidor)

### 1. Variables de entorno (`cp-backend`)

| Variable | Origen | Tu checklist |
|----------|--------|--------------|
| `WHATSAPP_APP_SECRET` | Misma **clave secreta de la app** que en Meta (Básica) | Completar en `.env` / secret de despliegue |
| `WHATSAPP_WEBHOOK_VERIFY_TOKEN` | **Lo inventás vos** (string largo); debe ser **idéntico** al “Verify token” del webhook en Meta | Definir valor y copiarlo en `.env` y en Meta |
| `WHATSAPP_GRAPH_API_BASE_URL` | Por defecto `https://graph.facebook.com/v23.0` | Dejar default salvo que Meta indique otra versión |

Después de cambiar env: **reiniciar** el contenedor o proceso de `cp-backend`.

### 2. URL pública del webhook

Meta **no** puede llamar a `localhost`.

| Situación | Qué hacer |
|-----------|-----------|
| Solo desarrollo en tu PC | Usar un **túnel HTTPS** (ngrok, Cloudflare Tunnel, etc.) hacia el puerto donde escucha el API (ej. `8100` en host con Compose). Callback: `https://<túnel>/v1/webhooks/whatsapp` |
| Staging / producción | Dominio o URL del balanceador donde expongas el mismo path |

**Pendiente** si aún no tenés túnel ni servidor público: no vas a poder dar de alta el webhook en Meta hasta entonces. Podés igual **probar conexión y envíos** con `POST /v1/whatsapp/connection` si ya tenés token e IDs.

### 3. Registrar la línea por organización

Autenticado como la org (JWT de Clerk o API key según tu setup):

```http
POST /v1/whatsapp/connection
Content-Type: application/json

{
  "phone_number_id": "...",
  "waba_id": "...",
  "access_token": "..."
}
```

Opcional: `display_phone_number`, `verified_name`.

Comprobar: `GET /v1/whatsapp/connection` (no devuelve el token).

### 4. Opt-in (envíos por API)

Los envíos servidor → WhatsApp (`/v1/whatsapp/send/*`) exigen **teléfono del party** y **opt-in** registrado:

```http
POST /v1/whatsapp/opt-ins
```

Cuerpo: `party_id`, `phone`, opcional `source`.

Sin opt-in, la API responde con regla de negocio de consentimiento faltante.

### 5. Enlaces `wa.me` (presupuesto, venta, mensaje libre)

No usan el webhook. Requieren permisos de lectura (`quotes`, `sales`, `customers`) y datos del tenant (plantillas / país por defecto vienen de settings de org en backend).

---

## Resumen: ya vs falta

| Ya (proyecto + Meta de prueba) | Falta (típico en dev) |
|--------------------------------|------------------------|
| Código y rutas del módulo WhatsApp en `pymes-core` | Completar `WHATSAPP_*` en `.env` y reiniciar `cp-backend` |
| Placeholders en `.env.example` | Mismo verify token en Meta y en `WHATSAPP_WEBHOOK_VERIFY_TOKEN` |
| App Meta + App Secret para pruebas | URL **HTTPS pública** (túnel o deploy) para el webhook |
| | Alta del webhook en Meta con esa URL |
| | `POST /v1/whatsapp/connection` por tenant |
| | `POST /v1/whatsapp/opt-ins` para contactos que reciban mensajes por API |

---

## Con todo configurado: qué podés hacer

Con **Meta bien configurado**, **variables de entorno del backend** (`WHATSAPP_APP_SECRET`, `WHATSAPP_WEBHOOK_VERIFY_TOKEN`, etc.), **webhook** apuntando a tu URL y **`POST /v1/whatsapp/connection`** hecho para la org, en Pymes podés:

### Conexión y operación básica

- Ver **estado** de la línea por org (`GET /v1/whatsapp/connection`) y **desconectar** si hace falta (`DELETE /v1/whatsapp/connection`).
- Ver **estadísticas** locales de mensajes (`GET /v1/whatsapp/connection/stats`: enviados, recibidos, entregados, leídos, fallidos según lo registrado en backend).

### Enviar desde el servidor (Graph API)

Con permiso **`whatsapp:write`**, **opt-in** del contacto y **teléfono** en el party:

- **Texto** libre al cliente.
- **Plantillas** aprobadas en Meta (nombre + idioma + parámetros).
- **Media** (imagen, documento, audio, video) por URL pública.
- **Mensajes con botones** (hasta 3).

Los envíos quedan **registrados** en base (y pueden reflejarse en timeline según el flujo).

### Historial y plantillas en Pymes

- **Listar / filtrar mensajes** almacenados (`GET /v1/whatsapp/messages`).
- **Plantillas locales** (borradores, listado, detalle, borrado; el alta local es previo a lo que apruebe Meta).

### Opt-in / cumplimiento

- **Registrar y listar** consentimientos, **opt-out** y **consultar** si un party tiene opt-in (`/v1/whatsapp/opt-ins`).

### Enlaces `wa.me` (sin depender del webhook)

- **Link para abrir WhatsApp** con texto armado para **presupuesto**, **comprobante de venta** o **mensaje libre a un cliente** (permisos de lectura `quotes`, `sales`, `customers`).

### Webhook (cuando también está bien)

- Meta puede **avisar** de **mensajes entrantes** y **cambios de estado** (entregado, leído, error). El pipeline de **respuesta automática** al inbound en código actual pasa por el **puente de IA** si está configurado; la **validación de firma** y la recepción del `POST` dependen del App Secret y la URL.

**Resumen:** operás la línea como **API de WhatsApp Business** desde la org (enviar, historial, plantillas, opt-in) y **abrís chats vía wa.me** desde documentos comerciales; con webhook + IA configurados, además podés **reaccionar a lo que entra** según lo montado en el backend.

---

## Referencias en código

- Handler y rutas: `pymes-core/backend/internal/whatsapp/handler.go`
- Firma y verify: `pymes-core/backend/internal/whatsapp/inbound.go`
- Config env: `pymes-core/backend/internal/shared/config/config.go` (`WHATSAPP_*`)

---

## Consola (frontend)

La pantalla del módulo **WhatsApp** en la consola puede no incluir formularios de conexión; las operaciones anteriores aplican vía **API** o herramientas tipo Postman hasta que exista UI dedicada.
