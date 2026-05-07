# Pymes AI Migration Inventory

Estado: Sprint 0, inventario operativo para migrar `pymes/ai` hacia Companion.

## Runtime que se elimina al final

- `src/agents/*`: runtime IA Python, routing, service layer, tool access y review/governance gate.
- `src/internal_chat/*`: chat interno con fact packs y Gemini/Vertex.
- `src/routing/*`: decision/routing pipeline Python.
- `src/insights/*`: servicio IA de insights.
- `src/domains/*`: perfiles verticales Python.

Destino: Companion. Pymes no conserva runtime LLM/agente.

## APIs actuales

- `src/api/internal_router.py`: chat interno operador.
- `src/api/public_router.py`: chat público general.
- `src/api/public_sales_router.py`: chat público comercial.
- `src/api/governance_callback.py`: callback de governance.
- `src/api/notifications_router.py`: notificaciones relacionadas con IA.
- `src/api/router.py`: composición de routers.

Destino:
- interno operador: Companion + perfil por vertical.
- público: gateway o BFF a decidir en Sprint 4.
- callbacks governance: Nexus/Companion, no runtime Python propio.

## Tools Python actuales

Read-only primero:
- `accounts`
- `cashflow`
- `currency`
- `customers`
- `help`
- `inventory`
- `products`
- `purchases`
- `recurring`
- `reports`
- `services`
- `settings`
- `suppliers`

Writes simples:
- `quotes`
- `sales`

Writes governed:
- `payments`
- `procurement_requests`
- `scheduling`

No migrar como tool Pymes:
- `review_policy`: debe ser proxy/CRUD directo a Nexus Governance.

## Capability gaps a cerrar en Pymes backend

- `pymes.customers.search`
- `pymes.services.search`
- `pymes.inventory.search`
- `pymes.scheduling.book`
- `pymes.cashflow.summary`
- `pymes.accounts.summary`
- `pymes.quotes.create`
- `pymes.sales.create`
- `pymes.payments.link`
- `pymes.procurement_requests.create`

Cada capability debe reautorizar tenant, actor, rol y permisos en Pymes. Companion nunca decide permisos finales.
