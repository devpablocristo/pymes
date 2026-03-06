# Prompt 03 — Asistente IA para Pymes

## Contexto

Este prompt agrega un **asistente conversacional con IA** al control-plane. Es un servicio Python/FastAPI separado del backend Go, pero dentro del mismo monorepo en `control-plane/ai/`. Se comunica con el backend Go via HTTP interno para ejecutar acciones y consultar datos.

**Prerequisitos**: Prompts 00, 01 y 02 implementados y funcionales.

**Regla fundamental**: el servicio AI NO contiene logica de negocio. Toda lectura y escritura de datos pasa por los endpoints del backend Go, que ya manejan auth, org_id, RBAC, audit y validaciones. El servicio AI solo orquesta la interaccion entre el usuario y el LLM.

**Party Model**: el asistente IA se registra como un `party` con `party_type = 'automated_agent'` y extension `party_agents(agent_kind='ai')`. Esto permite que las acciones ejecutadas por el asistente sean trazables en el `audit_log` con `actor_type='party'`. Las tools que interactúan con clientes, proveedores y turnos operan sobre `parties` con sus respectivos roles.

**Estándares de Ingeniería (Python)**: el servicio AI aplica los equivalentes Python de los patrones del backend Go:
- **Structured Errors**: excepciones tipadas que mapean a HTTP status codes.
- **Validation**: pydantic models para request/response validation.
- **Resilience**: retry con backoff para llamadas al backend Go y al LLM.
- **Structured Logging**: structlog/loguru con request_id, org_id en cada log.
- **Metrics + Tracing**: OpenTelemetry para FastAPI, httpx y llamadas al LLM/backend.
- **Testing**: pytest con mocks, fixtures, y tests parametrizados (equivalente a table-driven).
- **Circuit Breaker**: para llamadas al LLM que podrían fallar o estar lentas.

## Alcance obligatorio

Todo lo definido en este prompt para el servicio AI es parte del alcance requerido: arquitectura, seguridad, herramientas, flujos internos/externos, observabilidad, resiliencia y testing. No debe interpretarse como un módulo experimental ni como una fase opcional.

El hecho de que algunas partes dependan de endpoints del backend Go o de integraciones externas no reduce su importancia. La implementación puede secuenciarse por dependencias, pero el alcance final sigue siendo **todo** este prompt.

---

## Vision del producto

Cada pyme que se registra obtiene un asistente que:

1. **Guia el onboarding** — configura el negocio en 5 minutos con una conversacion natural en vez de formularios
2. **Responde consultas del negocio** — "¿cuanto vendi esta semana?", "¿quien me debe plata?"
3. **Ejecuta acciones** — "agendame un turno para Gomez manana a las 15"
4. **Aclara dudas de la plataforma** — "¿como cargo una devolucion?"
5. **Atiende clientes de la pyme** — "¿tienen turno libre manana?" via WhatsApp o widget web

---

## Stack tecnologico

| Capa | Tecnologia |
|------|-----------|
| **Lenguaje** | Python 3.12 |
| **Framework** | FastAPI |
| **ASGI server** | uvicorn |
| **LLM principal** | Gemini 2.0 Flash (Google AI) |
| **LLM abstraccion** | Interfaz propia para cambiar provider sin tocar codigo |
| **HTTP client** | httpx (async) |
| **ORM** | SQLAlchemy 2.0 (async) + asyncpg |
| **Streaming** | SSE via sse-starlette |
| **Validacion** | pydantic 2 + pydantic-settings |
| **Testing** | pytest + pytest-asyncio + httpx (TestClient) |

---

## Estructura del proyecto

```
control-plane/ai/
├── Dockerfile.dev
├── Dockerfile                   # Prod (Lambda container image o ECS)
├── pyproject.toml               # uv/pip deps
├── requirements.txt             # lock file
├── src/
│   ├── main.py                  # FastAPI app, lifespan, CORS, healthz
│   ├── config.py                # Settings (pydantic-settings, lee env vars)
│   ├── api/
│   │   ├── router.py            # /v1/chat endpoints
│   │   ├── public_router.py     # /v1/public/chat endpoints (modo external)
│   │   └── deps.py              # dependency injection (get_db, get_orchestrator, etc.)
│   ├── core/
│   │   ├── orchestrator.py      # loop ReAct: user msg → LLM → tool calls → response
│   │   ├── onboarding.py        # state machine de onboarding
│   │   ├── system_prompt.py     # builder del system prompt (fijo + dossier)
│   │   └── dossier.py           # CRUD y update logico del dossier
│   ├── llm/
│   │   ├── base.py              # Protocol LLMProvider
│   │   ├── gemini.py            # Google Gemini implementation
│   │   └── factory.py           # select_model(conversation_type, plan)
│   ├── tools/
│   │   ├── registry.py          # registro de tools + JSON Schemas
│   │   ├── base.py              # BaseTool con org_id automatico
│   │   ├── sales.py             # get_sales_summary, get_recent_sales
│   │   ├── customers.py         # search_customers, get_top_customers
│   │   ├── inventory.py         # get_low_stock, get_stock_level
│   │   ├── cashflow.py          # get_cashflow_summary, get_balance
│   │   ├── accounts.py          # get_account_balances, get_debtors
│   │   ├── appointments.py      # get_appointments, check_availability, book_appointment
│   │   ├── quotes.py            # create_quote, get_quotes
│   │   ├── products.py          # search_products, get_product
│   │   ├── suppliers.py         # search_suppliers
│   │   ├── purchases.py         # get_purchases_summary
│   │   ├── recurring.py         # get_recurring_expenses
│   │   ├── currency.py          # get_exchange_rates
│   │   ├── reports.py           # get_report (generico)
│   │   ├── settings.py          # update_tenant_settings (onboarding)
│   │   └── help.py              # search_help_docs (FAQ/documentacion)
│   ├── backend_client/
│   │   ├── client.py            # httpx async client → Go backend
│   │   └── auth.py              # forward de JWT/API key al backend
│   ├── db/
│   │   ├── engine.py            # async SQLAlchemy engine + session
│   │   ├── models.py            # ai_dossiers, ai_conversations, ai_usage_daily
│   │   └── repository.py        # CRUD para las tablas de AI
│   └── middleware/
│       ├── auth.py              # valida JWT o API key (misma logica que Go)
│       └── rate_limit.py        # throttling por org/plan
└── tests/
    ├── conftest.py
    ├── test_orchestrator.py
    ├── test_tools.py
    ├── test_onboarding.py
    └── test_api.py
```

---

## Arquitectura

### Flujo de un request

```
Frontend/WhatsApp
       │
       ▼
  AI Service (FastAPI :8000)
       │
       ├─── 1. Valida auth (JWT/API key)
       ├─── 2. Carga dossier del org
       ├─── 3. Arma system prompt + historial
       ├─── 4. Envia a LLM (Gemini Flash)
       │         │
       │         ├─── LLM responde texto → stream al cliente
       │         │
       │         └─── LLM pide tool call
       │                   │
       │                   ▼
       │            AI ejecuta tool
       │            (HTTP al Backend Go)
       │                   │
       │                   ▼
       │            Resultado → se envia al LLM
       │            (loop hasta que LLM responda texto)
       │
       ├─── 5. Guarda conversacion
       ├─── 6. Actualiza dossier si corresponde
       └─── 7. Actualiza usage
```

### Comunicacion interna (Docker Compose)

```
┌──────────┐         ┌──────────┐         ┌──────────┐
│ Frontend │ ──SSE──►│ AI svc   │ ──HTTP─►│ Backend  │
│  :5180   │         │  :8200   │         │  :8100   │
└──────────┘         └──────────┘         └──────────┘
                          │                     │
                          ▼                     ▼
                     ┌──────────┐          ┌──────────┐
                     │ Gemini   │          │ Postgres │
                     │  API     │          │  :5434   │
                     └──────────┘          └──────────┘
```

Dentro de Docker, el AI service llama al backend Go en `http://backend:8080` (nombre del servicio compose). El frontend llama al AI service en `http://localhost:8200`.

---

## LLM — Capa de abstraccion

### Protocol base

```python
from typing import AsyncIterator, Protocol
from dataclasses import dataclass

@dataclass
class Message:
    role: str           # "system", "user", "assistant", "tool"
    content: str
    tool_call_id: str | None = None
    tool_calls: list | None = None

@dataclass
class ToolDeclaration:
    name: str
    description: str
    parameters: dict    # JSON Schema

@dataclass
class ChatChunk:
    type: str           # "text", "tool_call", "done", "error"
    text: str | None = None
    tool_call: dict | None = None

class LLMProvider(Protocol):
    async def chat(
        self,
        messages: list[Message],
        tools: list[ToolDeclaration] | None = None,
        temperature: float = 0.3,
        max_tokens: int = 2048,
    ) -> AsyncIterator[ChatChunk]: ...
```

### Implementacion Gemini

```python
from google import genai
from google.genai import types

class GeminiProvider:
    def __init__(self, api_key: str, model: str = "gemini-2.0-flash"):
        self.client = genai.Client(api_key=api_key)
        self.model = model

    async def chat(self, messages, tools=None, temperature=0.3, max_tokens=2048):
        config = types.GenerateContentConfig(
            temperature=temperature,
            max_output_tokens=max_tokens,
        )
        if tools:
            config.tools = [self._to_gemini_tools(tools)]

        response = self.client.models.generate_content_stream(
            model=self.model,
            contents=self._to_gemini_messages(messages),
            config=config,
        )

        for chunk in response:
            part = chunk.candidates[0].content.parts[0]
            if part.function_call:
                yield ChatChunk(
                    type="tool_call",
                    tool_call={
                        "name": part.function_call.name,
                        "arguments": dict(part.function_call.args),
                    },
                )
            elif part.text:
                yield ChatChunk(type="text", text=part.text)

        yield ChatChunk(type="done")
```

### Factory

```python
def create_provider(config: Settings) -> LLMProvider:
    match config.llm_provider:
        case "gemini":
            return GeminiProvider(api_key=config.gemini_api_key, model=config.gemini_model)
        case _:
            raise ValueError(f"LLM provider desconocido: {config.llm_provider}")
```

Para agregar OpenAI o Anthropic en el futuro, se crea `openai.py` o `anthropic.py` que implemente el mismo Protocol y se agrega un case al factory. Cero cambios en el resto del codigo.

---

## Agente ReAct — Orquestador

El orquestador implementa el loop ReAct (Reasoning + Acting) sin frameworks externos:

```python
MAX_TOOL_CALLS = 10

async def orchestrate(
    llm: LLMProvider,
    messages: list[Message],
    tools: list[ToolDeclaration],
    tool_handlers: dict[str, Callable],
    org_id: UUID,
) -> AsyncIterator[ChatChunk]:
    """
    Loop ReAct:
    1. Envia mensajes al LLM
    2. Si el LLM responde texto → stream al cliente
    3. Si el LLM pide tool call → ejecuta, agrega resultado, vuelve a 1
    4. Maximo MAX_TOOL_CALLS iteraciones para evitar loops infinitos
    """
    tool_calls_count = 0

    while tool_calls_count < MAX_TOOL_CALLS:
        pending_tool_calls = []
        text_buffer = []

        async for chunk in llm.chat(messages, tools=tools):
            if chunk.type == "text":
                text_buffer.append(chunk.text)
                yield chunk
            elif chunk.type == "tool_call":
                pending_tool_calls.append(chunk.tool_call)

        if not pending_tool_calls:
            break

        assistant_msg = Message(
            role="assistant",
            content="".join(text_buffer) if text_buffer else "",
            tool_calls=pending_tool_calls,
        )
        messages.append(assistant_msg)

        for tc in pending_tool_calls:
            tool_calls_count += 1
            handler = tool_handlers.get(tc["name"])
            if not handler:
                result = {"error": f"Tool {tc['name']} no encontrada"}
            else:
                try:
                    result = await handler(org_id=org_id, **tc["arguments"])
                except Exception as e:
                    result = {"error": str(e)}

            messages.append(Message(
                role="tool",
                content=json.dumps(result, ensure_ascii=False, default=str),
                tool_call_id=tc["name"],
            ))

    yield ChatChunk(type="done")
```

### Limites de seguridad y resiliencia

- **MAX_TOOL_CALLS = 10**: evita loops infinitos. Si el LLM llama mas de 10 tools en un turno, se corta y responde con lo que tiene.
- **Timeout por tool call**: 10 segundos. Si el backend no responde, se retorna error al LLM.
- **Timeout total**: 60 segundos por conversacion. Si se excede, se responde con "Lo siento, la consulta esta tardando demasiado".

### Error handling estructurado

```python
# src/core/errors.py
from fastapi import HTTPException

class AppError(Exception):
    def __init__(self, code: str, message: str, status_code: int = 400, details: dict | None = None):
        self.code = code
        self.message = message
        self.status_code = status_code
        self.details = details or {}

class QuotaExceededError(AppError):
    def __init__(self, message: str):
        super().__init__("QUOTA_EXCEEDED", message, 429)

class LLMError(AppError):
    def __init__(self, provider: str, message: str):
        super().__init__("LLM_ERROR", f"{provider}: {message}", 502)

class BackendError(AppError):
    def __init__(self, status_code: int, message: str):
        super().__init__("BACKEND_ERROR", message, 502)

class ConversationNotFoundError(AppError):
    def __init__(self, conversation_id: str):
        super().__init__("NOT_FOUND", f"Conversation {conversation_id} not found", 404)
```

**Error handler global de FastAPI:**

```python
# src/main.py
@app.exception_handler(AppError)
async def app_error_handler(request: Request, exc: AppError):
    return JSONResponse(
        status_code=exc.status_code,
        content={
            "error": {
                "code": exc.code,
                "message": exc.message,
                "details": exc.details,
                "request_id": request.state.request_id,
            }
        },
    )
```

### Resilience para Backend Client

```python
# src/backend_client/client.py
import tenacity

class BackendClient:
    @tenacity.retry(
        stop=tenacity.stop_after_attempt(3),
        wait=tenacity.wait_exponential(multiplier=0.5, min=0.5, max=5),
        retry=tenacity.retry_if_exception_type(httpx.TransportError),
        before_sleep=tenacity.before_sleep_log(logger, logging.WARNING),
    )
    async def request(self, method: str, path: str, org_id: UUID, **kwargs) -> dict:
        response = await self.client.request(method, path, headers=headers, **kwargs)
        if response.status_code >= 500:
            raise BackendError(response.status_code, f"Backend returned {response.status_code}")
        response.raise_for_status()
        return response.json()
```

### Circuit breaker para LLM

```python
# Si el LLM falla 5 veces consecutivas, abrir circuito por 30 segundos
from src.core.circuit_breaker import CircuitBreaker

llm_breaker = CircuitBreaker(
    failure_threshold=5,
    recovery_timeout=30,
    expected_exception=LLMError,
)

async def chat_with_breaker(llm, messages, tools, **kwargs):
    if llm_breaker.is_open():
        raise LLMError("gemini", "Service temporarily unavailable (circuit open)")
    try:
        async for chunk in llm.chat(messages, tools=tools, **kwargs):
            yield chunk
        llm_breaker.record_success()
    except Exception as e:
        llm_breaker.record_failure()
        raise LLMError("gemini", str(e))
```

### Structured Logging

```python
import structlog

logger = structlog.get_logger()

# Middleware que inyecta request_id
@app.middleware("http")
async def request_id_middleware(request: Request, call_next):
    request_id = request.headers.get("X-Request-ID", f"req_{uuid4().hex[:8]}")
    request.state.request_id = request_id
    structlog.contextvars.bind_contextvars(request_id=request_id)

    response = await call_next(request)
    response.headers["X-Request-ID"] = request_id
    return response
```

---

## System Prompt + Dossier

### System prompt (fijo, igual para todos)

```
Sos el asistente de gestion de {platform_name}. Ayudas a duenos de pymes
argentinas y latinoamericanas a gestionar su negocio desde una conversacion.

Tus capacidades:
- Guiar al usuario en la configuracion inicial de su negocio
- Responder preguntas sobre como usar la plataforma
- Consultar datos del negocio (ventas, clientes, stock, caja, turnos, etc.)
- Crear presupuestos, registrar ventas, agendar turnos
- Aclarar dudas de gestion basica

Reglas:
- Siempre responde en espanol
- Usa lenguaje simple y directo, como si hablaras con el dueno del negocio
- Si no sabes algo, decilo — nunca inventes datos ni numeros
- Siempre confirma antes de ejecutar una accion que modifique datos
- Cuando muestres montos, usa formato argentino: $1.234,56
- Si el usuario pide algo que no podes hacer, sugeri como resolverlo
- No muestres JSON, UUIDs ni detalles tecnicos al usuario
- Responde conciso: 2-3 oraciones para consultas simples, mas si es necesario
- Usa emojis con moderacion (1-2 por mensaje como maximo)
```

### Dossier (dinamico, unico por org)

El dossier es un documento JSON que el asistente construye y enriquece con cada interaccion. Se inyecta despues del system prompt como contexto adicional.

```json
{
    "business": {
        "name": "",
        "type": "",
        "profile": "",
        "description": "",
        "currency": "ARS",
        "secondary_currency": null,
        "tax_rate": 21.0
    },
    "onboarding": {
        "status": "pending",
        "current_step": "welcome",
        "steps_completed": [],
        "steps_skipped": []
    },
    "modules_active": [],
    "modules_inactive": [],
    "preferences": {},
    "team": [],
    "learned_context": [],
    "kpis_baseline": {}
}
```

### Ensamble del contexto por request

```
┌─────────────────────────────────────────┐
│ 1. System prompt (fijo)                 │  ~400 tokens
│ 2. Dossier del org (JSON resumido)      │  ~600 tokens
│ 3. Tool declarations (filtradas)        │  ~1200 tokens
│ 4. Historial (ultimos 10 mensajes)      │  ~2000 tokens
│ 5. Mensaje del usuario                  │  variable
│                                         │
│ TOTAL: ~4500-6000 tokens/request        │
│ Costo Gemini Flash: ~$0.0006/query      │
└─────────────────────────────────────────┘
```

---

## Dos modos: Internal y External

El servicio es uno solo. Lo que cambia entre modos es: el system prompt, los tools disponibles, y el nivel de acceso.

### Modo Internal (empleados de la pyme)

Usuarios logueados en la plataforma, con JWT o API key. Tienen acceso a datos internos del negocio segun su rol RBAC.

**System prompt adicional**:
```
El usuario es {user_name}, con rol "{role}" en {business_name}.
Tiene acceso a: {modules_active}.
```

**Tools disponibles** (filtradas por `modules_active` del dossier + RBAC del usuario):

| Tool | Descripcion | Backend endpoint | Tipo |
|------|-------------|-----------------|------|
| `get_sales_summary` | Ventas por periodo con totales | `GET /v1/reports/sales-summary` | read |
| `get_recent_sales` | Ultimas N ventas | `GET /v1/sales?limit=N` | read |
| `get_top_customers` | Top N clientes (parties con rol customer) por facturacion | `GET /v1/reports/sales-by-party` | read |
| `search_customers` | Buscar party con rol customer por nombre/email | `GET /v1/customers?q=X` | read |
| `search_products` | Buscar producto por nombre/SKU | `GET /v1/products?q=X` | read |
| `get_low_stock` | Productos con stock bajo | `GET /v1/reports/low-stock` | read |
| `get_stock_level` | Stock de un producto | `GET /v1/inventory/:product_id` | read |
| `get_cashflow_summary` | Balance de caja por periodo | `GET /v1/reports/cashflow-summary` | read |
| `get_account_balances` | Cuentas corrientes con saldo | `GET /v1/accounts/summary` | read |
| `get_debtors` | Parties (clientes) que deben plata | `GET /v1/accounts/receivable` | read |
| `get_appointments` | Turnos por fecha/estado | `GET /v1/appointments` | read |
| `check_availability` | Slots libres para un dia | `GET /v1/appointments/availability` | read |
| `get_quotes` | Presupuestos por estado | `GET /v1/quotes` | read |
| `get_purchases` | Compras por estado/proveedor | `GET /v1/purchases` | read |
| `get_recurring_expenses` | Gastos recurrentes activos | `GET /v1/recurring-expenses` | read |
| `get_exchange_rates` | Cotizaciones del dia | `GET /v1/exchange-rates/today` | read |
| `create_quote` | Crear presupuesto | `POST /v1/quotes` | write |
| `create_sale` | Registrar venta | `POST /v1/sales` | write |
| `book_appointment` | Agendar turno | `POST /v1/appointments` | write |
| `create_cash_movement` | Registrar movimiento de caja | `POST /v1/cashflow` | write |
| `search_parties` | Buscar party por nombre/email (cualquier rol) | `GET /v1/parties?q=X` | read |
| `search_help` | Buscar en documentacion de la plataforma | (local, sin backend) | read |

**Filtrado por RBAC**: si el usuario tiene rol `cajero`, no ve tools de reportes ni inventario. El AI service consulta el rol del usuario y filtra los tools antes de enviarlos al LLM.

```python
ROLE_TOOL_ACCESS = {
    "admin": "*",
    "vendedor": [
        "search_customers", "search_products", "get_appointments",
        "check_availability", "book_appointment", "get_quotes",
        "create_quote", "create_sale", "get_account_balances",
        "get_low_stock", "search_help",
    ],
    "cajero": [
        "get_recent_sales", "create_sale", "get_cashflow_summary",
        "create_cash_movement", "search_customers", "get_account_balances",
        "get_appointments", "search_help",
    ],
    "contador": [
        "get_sales_summary", "get_cashflow_summary", "get_account_balances",
        "get_purchases", "get_recurring_expenses", "get_debtors",
        "get_exchange_rates", "search_help",
    ],
    "almacenero": [
        "search_products", "get_low_stock", "get_stock_level",
        "get_purchases", "search_help",
    ],
}
```

### Modo External (clientes de la pyme)

Usuarios NO logueados en la plataforma. Interactuan via WhatsApp, widget web o link publico. Se identifican por telefono o email.

**System prompt adicional**:
```
Sos el asistente de {business_name}. Estas hablando con un cliente del negocio.
NUNCA reveles informacion interna: costos, margenes, otros clientes, caja,
proveedores, empleados ni datos financieros del negocio.
Solo podes ayudar con: turnos, precios de venta, horarios e informacion publica.
```

**Tools disponibles** (solo 5, datos publicos):

| Tool | Descripcion | Backend endpoint | Tipo |
|------|-------------|-----------------|------|
| `check_availability` | Slots libres para un dia | `GET /v1/public/{org_id}/availability` | read |
| `book_appointment` | Reservar turno (nombre + tel) | `POST /v1/public/{org_id}/book` | write |
| `get_public_services` | Servicios/productos con precio venta | `GET /v1/public/{org_id}/services` | read |
| `get_business_info` | Direccion, horarios, telefono | `GET /v1/public/{org_id}/info` | read |
| `get_my_appointments` | Turnos del cliente (por telefono) | `GET /v1/public/{org_id}/my-appointments` | read |

**Estos endpoints son nuevos en el backend Go** (ver seccion "Endpoints publicos").

---

## Onboarding — State Machine

Cuando una org es nueva (`onboarding.status == "pending"`), el asistente activa el flujo de onboarding. No es un wizard rigido — es una conversacion guiada que el usuario puede interrumpir, saltear pasos, o volver atras.

### Estados

```
welcome → business_type → business_info → currency_setup →
tax_setup → modules_setup → first_record → feature_tips → completed
```

### Transiciones

| Estado | Que pregunta | Que hace con la respuesta | Siguiente |
|--------|-------------|--------------------------|-----------|
| `welcome` | "¿A que se dedica tu negocio?" | Clasifica tipo de negocio | `business_type` |
| `business_type` | (procesamiento interno) | Asigna perfil, sugiere modulos | `business_info` |
| `business_info` | "¿Como se llama tu negocio? ¿CUIT?" | `PATCH /v1/admin/tenant-settings` con business_name, tax_id, address, phone | `currency_setup` |
| `currency_setup` | "¿Manejas dolares ademas de pesos?" | Si: activa `secondary_currency = 'USD'` + `auto_fetch_rates` | `tax_setup` |
| `tax_setup` | "¿Sos responsable inscripto o monotributista?" | Configura `tax_rate` (21% o 0%) | `modules_setup` |
| `modules_setup` | "Te active estos modulos: {lista}. ¿Queres cambiar algo?" | Actualiza `modules_active` en dossier | `first_record` |
| `first_record` | "¿Queres cargar tu primer {producto/cliente/servicio}?" | Ejecuta `POST /v1/{entity}` via tool | `feature_tips` |
| `feature_tips` | Muestra 3 tips segun el perfil | Solo informativo | `completed` |
| `completed` | "¡Listo! Ya podes empezar." | `onboarding.status = 'completed'`, `onboarding.completed_at = now()` | (fin) |

### Perfiles de negocio

El LLM clasifica la descripcion del usuario en un perfil que determina modulos recomendados y configuracion por defecto.

| Perfil | Ejemplos | Modulos sugeridos | Config especial |
|--------|----------|-------------------|-----------------|
| `comercio_minorista` | Kiosco, almacen, tienda de ropa | customers, products, inventory, sales, cashflow, suppliers | `track_stock=true` |
| `servicio_profesional` | Peluqueria, consultorio, estudio juridico | customers, appointments, sales, cashflow | `appointments_enabled=true`, `track_stock=false` |
| `gastronomia` | Restaurante, bar, delivery | products, sales, cashflow, suppliers, recurring | `track_stock=false`, sin presupuestos |
| `distribuidora` | Mayorista, distribuidor | customers, suppliers, products, inventory, purchases, sales, accounts | Listas de precio activas, ctas corrientes |
| `freelancer` | Disenador, programador, consultor | customers, quotes, sales, cashflow | Minimal, 1 usuario |
| `otro` | Cualquier otro | customers, sales, cashflow | Config basica, usuario elige |

```python
BUSINESS_PROFILES = {
    "comercio_minorista": {
        "modules": ["customers", "products", "inventory", "sales", "cashflow", "suppliers"],
        "settings": {"track_stock": True, "appointments_enabled": False},
    },
    "servicio_profesional": {
        "modules": ["customers", "appointments", "sales", "cashflow"],
        "settings": {"track_stock": False, "appointments_enabled": True},
    },
    "gastronomia": {
        "modules": ["products", "sales", "cashflow", "suppliers", "recurring"],
        "settings": {"track_stock": False, "appointments_enabled": False},
    },
    "distribuidora": {
        "modules": ["customers", "suppliers", "products", "inventory", "purchases", "sales", "accounts", "cashflow"],
        "settings": {"track_stock": True, "appointments_enabled": False},
    },
    "freelancer": {
        "modules": ["customers", "quotes", "sales", "cashflow"],
        "settings": {"track_stock": False, "appointments_enabled": False},
    },
    "otro": {
        "modules": ["customers", "sales", "cashflow"],
        "settings": {},
    },
}
```

### Instrucciones de onboarding para el LLM

Cuando `onboarding.status != 'completed'`, se inyecta esto al system prompt:

```
MODO ONBOARDING ACTIVO. El negocio esta configurando su cuenta por primera vez.
Paso actual: {current_step}.
Pasos completados: {steps_completed}.

Tu objetivo es guiar al usuario paso a paso. Se amigable, paciente y concreto.
Si el usuario quiere saltear un paso, registralo como "skipped" y avanza.
Si el usuario pregunta algo no relacionado al onboarding, respondele normalmente
y luego retoma el paso donde estaba.
```

---

## Dossier — Modelo de datos y actualizacion

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS ai_dossiers (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    dossier jsonb NOT NULL DEFAULT '{}'::jsonb,
    version int NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

### Cuando se actualiza el dossier

| Evento | Campos actualizados |
|--------|--------------------|
| Onboarding step completado | `onboarding.*`, `modules_active`, `preferences` |
| El usuario configura algo en la UI | Sync `business.*` desde `tenant_settings` (via cron o lazy load) |
| Consulta recurrente detectada | `learned_context` (append, max 20 entries) |
| Primera semana de uso | `kpis_baseline` (calculado de datos reales via scheduler) |
| Cada 7 dias | `kpis_baseline` recalculado automaticamente |
| Nuevo miembro del equipo | `team` (sync desde `org_members`) |

### Update atomico

```python
async def update_dossier(db: AsyncSession, org_id: UUID, patch: dict):
    """Merge parcial del dossier — solo actualiza los campos del patch."""
    stmt = (
        update(AIDossier)
        .where(AIDossier.org_id == org_id)
        .values(
            dossier=func.jsonb_deep_merge(AIDossier.dossier, cast(patch, JSONB)),
            version=AIDossier.version + 1,
            updated_at=func.now(),
        )
    )
    await db.execute(stmt)
```

Se usa `jsonb_deep_merge` (funcion SQL custom o `||` con `jsonb_set` anidado) para evitar pisar todo el dossier en cada update.

---

## Conversaciones — Modelo de datos

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id),
    mode text NOT NULL DEFAULT 'internal'
        CHECK (mode IN ('internal', 'external')),
    external_contact text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    tool_calls_count int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_org
    ON ai_conversations(org_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user
    ON ai_conversations(org_id, user_id, updated_at DESC)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ai_conversations_external
    ON ai_conversations(org_id, external_contact, updated_at DESC)
    WHERE mode = 'external' AND external_contact != '';
```

### Formato de mensajes en `messages` (jsonb)

```json
[
    {"role": "user", "content": "¿Cuanto vendi hoy?", "ts": "2026-03-05T14:30:00Z"},
    {"role": "assistant", "content": "Hoy vendiste $45.200 en 6 ventas.", "ts": "2026-03-05T14:30:02Z", "tool_calls": ["get_sales_summary"]},
    {"role": "user", "content": "¿Y esta semana?", "ts": "2026-03-05T14:30:15Z"},
    {"role": "assistant", "content": "Esta semana llevas $198.500 en 23 ventas.", "ts": "2026-03-05T14:30:17Z", "tool_calls": ["get_sales_summary"]}
]
```

### Ventana de contexto

- Se cargan los **ultimos 10 mensajes** de la conversacion como historial
- Si la conversacion tiene mas de 10 mensajes, los anteriores no se incluyen
- El titulo de la conversacion se genera automaticamente con el primer mensaje del usuario (el LLM resume en 5-6 palabras)

---

## Usage tracking — Modelo de datos

### Tabla SQL

```sql
CREATE TABLE IF NOT EXISTS ai_usage_daily (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    usage_date date NOT NULL,
    queries int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, usage_date)
);
```

### Incremento por query

```python
async def track_usage(db: AsyncSession, org_id: UUID, tokens_in: int, tokens_out: int):
    stmt = insert(AIUsageDaily).values(
        org_id=org_id,
        usage_date=date.today(),
        queries=1,
        tokens_input=tokens_in,
        tokens_output=tokens_out,
    ).on_conflict_do_update(
        index_elements=["org_id", "usage_date"],
        set_={
            "queries": AIUsageDaily.queries + 1,
            "tokens_input": AIUsageDaily.tokens_input + tokens_in,
            "tokens_output": AIUsageDaily.tokens_output + tokens_out,
        },
    )
    await db.execute(stmt)
```

---

## Limites por plan

| Plan | AI Interno (empleados) | AI Externo (clientes) | Tokens/mes |
|------|------------------------|-----------------------|------------|
| Starter | 50 consultas/mes | No disponible | ~100K |
| Growth | 500 consultas/mes | 200 conversaciones/mes | ~1M |
| Enterprise | Ilimitado | Ilimitado | Ilimitado |

La verificacion se hace antes de llamar al LLM:

```python
async def check_quota(db: AsyncSession, org_id: UUID, plan: str, mode: str) -> bool:
    usage = await get_monthly_usage(db, org_id)
    limits = PLAN_LIMITS[plan]

    if mode == "external" and plan == "starter":
        raise QuotaExceededError("AI para clientes no esta disponible en plan Starter")

    if limits["queries"] != -1 and usage.queries >= limits["queries"]:
        raise QuotaExceededError(f"Alcanzaste el limite de {limits['queries']} consultas/mes")

PLAN_LIMITS = {
    "starter":    {"queries": 50,  "external": False},
    "growth":     {"queries": 500, "external": True, "external_limit": 200},
    "enterprise": {"queries": -1,  "external": True, "external_limit": -1},
}
```

---

## API Endpoints

### Modo Internal (requiere auth)

```
POST   /v1/chat                         — Enviar mensaje (SSE streaming)
GET    /v1/chat/conversations            — Listar conversaciones del usuario
GET    /v1/chat/conversations/:id        — Ver conversacion
DELETE /v1/chat/conversations/:id        — Borrar conversacion
GET    /v1/chat/usage                    — Ver uso del mes actual
```

### Modo External (sin auth de plataforma, identificado por org_slug + phone)

```
POST   /v1/public/:org_slug/chat        — Enviar mensaje (SSE streaming)
POST   /v1/public/:org_slug/chat/identify — Identificarse (nombre + telefono)
```

### Request — Enviar mensaje

```json
{
    "conversation_id": "uuid | null",
    "message": "¿Cuanto vendi hoy?"
}
```

Si `conversation_id` es null, crea una nueva conversacion.

### Response — SSE Stream

```
event: text
data: {"content": "Hoy vendiste "}

event: text
data: {"content": "$45.200 en 6 ventas."}

event: tool_call
data: {"tool": "get_sales_summary", "status": "executing"}

event: tool_result
data: {"tool": "get_sales_summary", "status": "done"}

event: done
data: {"conversation_id": "uuid", "tokens_used": 847}
```

### HealthCheck

```
GET    /healthz                          — {"status": "ok"}
```

---

## Endpoints publicos en Backend Go (nuevos)

El modo external necesita endpoints en el backend Go que NO requieran auth de usuario. Se autentican con un token interno entre servicios (AI → Backend).

```
GET    /v1/public/:org_id/availability   — Slots libres para una fecha
POST   /v1/public/:org_id/book           — Reservar turno (nombre + telefono requeridos)
GET    /v1/public/:org_id/services       — Productos/servicios activos con precio (sin cost_price)
GET    /v1/public/:org_id/info           — Nombre, direccion, telefono, horarios, logo
GET    /v1/public/:org_id/my-appointments?phone=X — Turnos de un cliente por telefono
```

**Seguridad**:
- Estos endpoints NUNCA exponen: `cost_price`, `margin`, otros clientes, caja, proveedores, cuentas corrientes, ni datos financieros internos.
- Se autentican con header `X-Internal-Service-Token` que comparten AI y Backend via env var.
- Rate limit: 30 requests/minuto por IP para evitar scraping.

### En el backend Go

Estos endpoints se registran en `wire/bootstrap.go` en un grupo publico separado:

```go
public := router.Group("/v1/public/:org_id")
public.Use(handlers.NewInternalServiceAuth(cfg.InternalServiceToken))
public.Use(handlers.NewPublicRateLimit(30))
appointmentsHandler.RegisterPublicRoutes(public)
productsHandler.RegisterPublicRoutes(public)
orgHandler.RegisterPublicRoutes(public)
```

---

## Canales del modo External

### WhatsApp (via Meta Cloud API)

Cuando la pyme conecta su WhatsApp Business:

```
Cliente envia WhatsApp al negocio
        │
        ▼
Meta Cloud API webhook
        │
        ▼
POST /v1/webhooks/whatsapp (Backend Go)
        │
        ├── Identifica org por phone_number_id
        ├── Forward al AI service: POST /v1/internal/whatsapp/message
        │
        ▼
AI Service procesa en modo external
        │
        ▼
Respuesta via Meta Cloud API → send message
```

**Tabla nueva** (para mapear numeros de WhatsApp a orgs):

```sql
CREATE TABLE IF NOT EXISTS whatsapp_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    waba_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_connections_phone
    ON whatsapp_connections(phone_number_id) WHERE is_active = true;
```

**Nota**: la integracion WhatsApp Business API es compleja (requiere verificacion de negocio con Meta, templates aprobados, etc.). Para la v1, se puede arrancar con el link `wa.me` existente (Prompt 02) y agregar la API como mejora futura. La arquitectura esta preparada.

### Widget Web

Un script JS que la pyme embebe en su sitio web:

```html
<script src="https://app.pymes.com/widget.js" data-org="uuid-or-slug"></script>
```

El widget abre un chat bubble que se conecta via SSE a `POST /v1/public/:org_slug/chat`.

### Link Directo

```
https://app.pymes.com/chat/{org_slug}
```

Una pagina publica con el chat del asistente. La pyme comparte este link en Instagram, Google Maps, tarjeta de visita, etc.

---

## Backend Client — Comunicacion AI → Go

```python
class BackendClient:
    def __init__(self, base_url: str, internal_token: str):
        self.client = httpx.AsyncClient(
            base_url=base_url,
            timeout=10.0,
            headers={"X-Internal-Service-Token": internal_token},
        )

    async def request(
        self,
        method: str,
        path: str,
        org_id: UUID,
        user_token: str | None = None,
        **kwargs,
    ) -> dict:
        headers = {"X-Org-ID": str(org_id)}
        if user_token:
            headers["Authorization"] = f"Bearer {user_token}"

        response = await self.client.request(method, path, headers=headers, **kwargs)
        response.raise_for_status()
        return response.json()

    async def get_sales_summary(self, org_id: UUID, token: str, period: str) -> dict:
        return await self.request("GET", "/v1/reports/sales-summary", org_id, token, params={"period": period})

    async def search_customers(self, org_id: UUID, token: str, query: str) -> dict:
        return await self.request("GET", "/v1/customers", org_id, token, params={"q": query})

    async def search_parties(self, org_id: UUID, token: str, query: str, role: str | None = None) -> dict:
        params = {"q": query}
        if role:
            params["role"] = role
        return await self.request("GET", "/v1/parties", org_id, token, params=params)

    async def create_appointment(self, org_id: UUID, token: str, data: dict) -> dict:
        return await self.request("POST", "/v1/appointments", org_id, token, json=data)
```

Las tools del AI llaman a metodos del `BackendClient`. El token JWT del usuario se forwarded al backend para que se apliquen los permisos RBAC correctos.

---

## Configuracion

### Variables de entorno

```env
# AI Service
AI_PORT=8000
AI_LOG_LEVEL=info

# Database (misma DB que el backend)
DATABASE_URL=postgres://postgres:postgres@postgres:5432/pymes?sslmode=disable

# Backend Go (URL interna en Docker)
BACKEND_URL=http://backend:8080
INTERNAL_SERVICE_TOKEN=local-internal-token

# LLM
LLM_PROVIDER=gemini
GEMINI_API_KEY=
GEMINI_MODEL=gemini-2.0-flash

# Auth (para validar JWTs — misma config que backend)
JWKS_URL=https://<clerk-domain>/.well-known/jwks.json
JWT_ISSUER=https://<clerk-domain>
AUTH_ALLOW_API_KEY=true
```

### Config dataclass

```python
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    ai_port: int = 8000
    ai_log_level: str = "info"

    database_url: str = "postgres://postgres:postgres@localhost:5434/pymes?sslmode=disable"

    backend_url: str = "http://backend:8080"
    internal_service_token: str = "local-internal-token"

    llm_provider: str = "gemini"
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.0-flash"

    jwks_url: str = ""
    jwt_issuer: str = ""
    auth_allow_api_key: bool = True

    class Config:
        env_file = ".env"
```

---

## Docker

### Dockerfile.dev

```dockerfile
FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY src/ ./src/

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000", "--reload"]
```

### Agregar al docker-compose.yml

```yaml
  ai:
    build:
      context: ./control-plane/ai
      dockerfile: Dockerfile.dev
    ports:
      - "8200:8000"
    volumes:
      - ./control-plane/ai/src:/app/src
    env_file:
      - .env
    environment:
      DATABASE_URL: postgres://postgres:postgres@postgres:5432/pymes?sslmode=disable
      BACKEND_URL: http://backend:8080
    depends_on:
      - backend
```

---

## Dependencias Python

```
# requirements.txt
fastapi>=0.115.0
uvicorn[standard]>=0.32.0
httpx>=0.27.0
sqlalchemy[asyncio]>=2.0.0
asyncpg>=0.30.0
pydantic>=2.0.0
pydantic-settings>=2.0.0
sse-starlette>=2.0.0
google-genai>=1.0.0
python-jose[cryptography]>=3.3.0
```

---

## Migraciones SQL

### `0014_ai_tables.up.sql`

```sql
CREATE TABLE IF NOT EXISTS ai_dossiers (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    dossier jsonb NOT NULL DEFAULT '{}'::jsonb,
    version int NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id),
    mode text NOT NULL DEFAULT 'internal'
        CHECK (mode IN ('internal', 'external')),
    external_contact text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    tool_calls_count int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_org
    ON ai_conversations(org_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_user
    ON ai_conversations(org_id, user_id, updated_at DESC)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_ai_conversations_external
    ON ai_conversations(org_id, external_contact, updated_at DESC)
    WHERE mode = 'external' AND external_contact != '';

CREATE TABLE IF NOT EXISTS ai_usage_daily (
    org_id uuid NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    usage_date date NOT NULL,
    queries int NOT NULL DEFAULT 0,
    tokens_input int NOT NULL DEFAULT 0,
    tokens_output int NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, usage_date)
);
```

**Party del AI Assistant** — al ejecutar el seed o al inicializar, crear un party representando al asistente IA:

```sql
-- Crear party del asistente IA (uno por org, se crea al activar AI)
-- En el seed de desarrollo:
INSERT INTO parties (org_id, party_type, display_name, metadata) VALUES
    (:org_id, 'automated_agent', 'Asistente IA', '{"system": true}');

INSERT INTO party_agents (party_id, agent_kind, provider, config, is_active) VALUES
    (:party_id, 'ai', 'gemini', '{"model": "gemini-2.0-flash"}', true);
```

El `party_id` del asistente se usa para:
- `audit_log.actor_id` cuando el AI ejecuta acciones (crear venta, agendar turno)
- `ai_conversations.agent_party_id` para saber qué agente atendió la conversación
- Trazabilidad completa de acciones automáticas vs manuales

### `0014_ai_tables.down.sql`

```sql
DROP TABLE IF EXISTS ai_usage_daily;
DROP TABLE IF EXISTS ai_conversations;
DROP TABLE IF EXISTS ai_dossiers;
-- Los party agents se eliminan via CASCADE al borrar parties
```

### `0015_whatsapp_connections.up.sql` (futuro, cuando se implemente WhatsApp Business API)

```sql
CREATE TABLE IF NOT EXISTS whatsapp_connections (
    org_id uuid PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    phone_number_id text NOT NULL,
    waba_id text NOT NULL,
    access_token_encrypted text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_connections_phone
    ON whatsapp_connections(phone_number_id) WHERE is_active = true;
```

### `0015_whatsapp_connections.down.sql`

```sql
DROP TABLE IF EXISTS whatsapp_connections;
```

---

## Actualizaciones al Backend Go

### Nuevas variables de entorno

Agregar a `config.go`:

```go
InternalServiceToken string  // INTERNAL_SERVICE_TOKEN — token compartido con AI service
```

Agregar a `.env.example`:

```env
# AI Service (comunicacion interna)
INTERNAL_SERVICE_TOKEN=local-internal-token

# Gemini
GEMINI_API_KEY=
```

### Middleware de servicio interno

```go
func NewInternalServiceAuth(token string) gin.HandlerFunc {
    return func(c *gin.Context) {
        if token == "" {
            c.Next()
            return
        }
        provided := c.GetHeader("X-Internal-Service-Token")
        if provided != token {
            c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
            return
        }
        c.Next()
    }
}
```

### Endpoints publicos nuevos

Los handlers existentes (`appointments`, `products`, `org`) agregan un metodo `RegisterPublicRoutes(group)` que expone endpoints read-only y acotados, sin datos internos sensibles.

---

## Testing

### Estrategia

| Tipo | Que testea | Como |
|------|-----------|------|
| **Unit: orchestrator** | Loop ReAct, max tool calls, timeout | LLM mock con respuestas predefinidas |
| **Unit: tools** | Cada tool llama al endpoint correcto | Mock de BackendClient |
| **Unit: onboarding** | Transiciones de estado, skip steps | Dossier mock |
| **Unit: errors** | Errores tipados, circuit breaker, retry | Mocks que fallan |
| **Unit: quota** | Límites por plan, rechazo quota | Mock de usage tracker |
| **Integration: API** | Endpoints de chat con SSE | TestClient FastAPI + mock LLM + DB test |
| **E2E** | Flujo completo con Docker | curl contra AI service (Gemini mockeado) |

### Ejemplo: test parametrizado del orquestador (equivalente a table-driven)

```python
@pytest.mark.parametrize("scenario,mock_responses,expected_tools,expected_text", [
    (
        "consulta de ventas",
        [[ChatChunk(type="tool_call", tool_call={"name": "get_sales_summary", "arguments": {"period": "today"}})],
         [ChatChunk(type="text", text="Hoy vendiste $45.200")]],
        ["get_sales_summary"],
        "45.200",
    ),
    (
        "respuesta directa sin tools",
        [[ChatChunk(type="text", text="Hola, ¿en qué puedo ayudarte?")]],
        [],
        "ayudarte",
    ),
    (
        "max tool calls — se corta en 10",
        [[ChatChunk(type="tool_call", tool_call={"name": "search_customers", "arguments": {"q": "x"}})] for _ in range(12)],
        ["search_customers"] * 10,
        None,
    ),
])
async def test_orchestrate(scenario, mock_responses, expected_tools, expected_text):
    mock_llm = MockLLM(responses=mock_responses)
    mock_handlers = {name: AsyncMock(return_value={"items": []}) for name in set(expected_tools)}

    chunks = []
    async for chunk in orchestrate(mock_llm, messages, tools, mock_handlers, org_id):
        chunks.append(chunk)

    tool_calls = [c.tool_call["name"] for c in chunks if c.type == "tool_call"]
    assert len(tool_calls) <= 10
    if expected_text:
        assert any(c.text and expected_text in c.text for c in chunks if c.text)
```

### Ejemplo: test de resiliencia

```python
async def test_backend_client_retries_on_transport_error():
    """Verifica retry automático en errores de transporte."""
    client = BackendClient(base_url="http://backend:8080", internal_token="test")
    with respx.mock:
        route = respx.get("http://backend:8080/v1/sales").mock(
            side_effect=[httpx.ConnectError("refused"), httpx.Response(200, json={"data": []})]
        )
        result = await client.request("GET", "/v1/sales", org_id=uuid4())
        assert route.call_count == 2

async def test_quota_exceeded_returns_429():
    """Verifica que exceder quota retorna 429 con QUOTA_EXCEEDED."""
    # Simular 51 queries para plan starter → último request falla
```

### Dependencias de testing adicionales

```
# requirements-dev.txt (separado de producción)
pytest>=8.0.0
pytest-asyncio>=0.24.0
respx>=0.21.0
```

---

## Actualizaciones a archivos existentes

| Archivo | Cambio |
|---------|--------|
| `docker-compose.yml` | Agregar servicio `ai` |
| `.env.example` | Agregar `INTERNAL_SERVICE_TOKEN`, `GEMINI_API_KEY`, `AI_*` vars |
| `.env` | Copiar nuevas vars de `.env.example` |
| `config.go` | Agregar `InternalServiceToken` |
| `wire/bootstrap.go` | Agregar grupo `/v1/public/:org_id` con middleware interno + rate limit |
| `go.work` | No cambia (ai/ es Python, no Go) |
| `.gitignore` | Agregar `__pycache__/`, `*.pyc`, `.venv/` |
| `README.md` | Agregar seccion AI service con endpoints y arquitectura |
| `Makefile` | Agregar targets `ai-dev`, `ai-test`, `ai-lint` |
| `requirements.txt` | Agregar dependencias OpenTelemetry (`opentelemetry-sdk`, instrumentors FastAPI/httpx`) |

---

## Orden de ejecucion recomendado

**Aclaración importante**: este orden es solo una secuencia técnica para construir sin retrabajo. No convierte ningún bloque en opcional ni en "fase 2".

1. Crear `control-plane/ai/` con estructura de directorios
2. `requirements.txt` + `Dockerfile.dev`
3. `src/config.py` + `src/main.py` (FastAPI app con healthz)
4. Agregar servicio `ai` al `docker-compose.yml`
5. Migracion SQL `0014_ai_tables` (up + down)
6. `src/db/` — engine, models, repository
7. `src/llm/base.py` + `src/llm/gemini.py` + `src/llm/factory.py`
8. `src/core/orchestrator.py` — loop ReAct
9. `src/tools/registry.py` + `src/tools/base.py`
10. `src/backend_client/client.py`
11. Implementar tools de lectura: `sales.py`, `customers.py`, `inventory.py`, `cashflow.py`, etc.
12. `src/core/system_prompt.py` + `src/core/dossier.py`
13. `src/api/router.py` — endpoints de chat con SSE
14. `src/middleware/auth.py` — validacion JWT/API key
15. `src/middleware/rate_limit.py`
16. `src/observability/otel.py` — tracer provider, OTLP exporter, métricas HTTP/LLM/backend
17. `src/core/onboarding.py` — state machine
18. Implementar tools de escritura: `quotes.py` (create), `appointments.py` (book), `settings.py` (onboarding)
19. `src/api/public_router.py` — endpoints modo external
20. Backend Go: endpoints publicos + middleware de servicio interno
21. Actualizar `.env.example`, `.env`, `config.go`, `docker-compose.yml`, `.gitignore`, `README.md`, `Makefile`
22. Tests
23. Verificar todo: `docker compose up -d`, probar chat interno y externo

---

## Criterios de exito

- [ ] `docker compose up -d` levanta el servicio AI en :8200
- [ ] `GET /healthz` retorna `{"status": "ok"}`
- [ ] `POST /v1/chat` con JWT valido: stream SSE con respuesta del LLM
- [ ] Tool call funciona: "¿cuanto vendi hoy?" ejecuta `get_sales_summary` y responde con datos reales
- [ ] Onboarding: org nueva recibe flujo guiado, dossier se construye
- [ ] Dossier persiste entre conversaciones
- [ ] Rate limiting: plan starter rechaza consulta #51 del mes
- [ ] Modo external: `POST /v1/public/:slug/chat` responde sin auth de plataforma
- [ ] Modo external NO revela datos internos (costos, margenes, otros clientes)
- [ ] Endpoints publicos del backend Go funcionan con token interno
- [ ] Conversaciones se guardan y se pueden listar/ver/borrar
- [ ] Usage tracking: `ai_usage_daily` se incrementa por query
- [ ] `pytest` pasa todos los tests
- [ ] Party Model: AI assistant tiene party(automated_agent) por org
- [ ] Party Model: acciones del AI registran actor_type='party' en audit_log
- [ ] Party Model: tools de customers/suppliers operan sobre parties con roles
- [ ] `.env.example` contiene todas las variables nuevas

### Engineering Standards
- [ ] Errores tipados: `AppError`, `QuotaExceededError`, `LLMError`, `BackendError` con códigos
- [ ] Error responses siguen formato `{"error": {"code", "message", "details", "request_id"}}`
- [ ] Request ID propagado en todo request (`X-Request-ID` header)
- [ ] Structured logging con `structlog`: request_id, org_id, user_id en cada log
- [ ] Backend client con retry automático (3 intentos, backoff exponencial) via tenacity
- [ ] Circuit breaker para LLM: abre después de 5 fallos, recupera en 30s
- [ ] FastAPI instrumentado con OpenTelemetry (HTTP server + `httpx` client)
- [ ] Métricas mínimas: latencia de chat, errores por tool, tokens consumidos, retries, circuit-breaker open events
- [ ] Tests parametrizados para orchestrator, quota, y resiliencia
- [ ] Validación pydantic en todos los request models
- [ ] Health check en `/healthz`
