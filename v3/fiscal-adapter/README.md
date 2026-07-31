# Fiscal Adapter

Backend privado TypeScript de Pymes v3. TypeScript se conserva para que, en la
etapa posterior, el companion real pueda envolver `arca-facturacion` sin portar
el SDK a Go. Ningún navegador accede a este servicio.

## Estado actual

Sólo existe el modo `mock`; no hay dependencia, import ni conexión ARCA. El
proceso falla al iniciar si no se seleccionan explícitamente:

```sh
FISCAL_ADAPTER_MODE=mock FISCAL_ALLOW_INSECURE_LOCAL=true npm start
```

`FISCAL_MOCK_SCENARIO` admite `authorized`, `rejected`,
`timeout_before_processing` y `response_lost_after_processing`.

## Arquitectura

- `fiscal/domain`: tipos y errores fiscales estables.
- `fiscal/ports`: `FiscalAuthority`, `FiscalLedger` y observabilidad.
- `fiscal/usecases`: validación, idempotencia, autorización y consulta exacta.
- `fiscal/handler`: API HTTP privada.
- `fiscal/repository`: ledger PostgreSQL durable, idempotencia y métricas.
- `fiscal/companion`: autoridad mock; el ledger en memoria sólo se usa en tests.
- `identity/access`: JWT Ed25519; bypass únicamente local y explícito.
- `wire.ts`: única composición de dependencias.

La futura implementación ARCA debe agregar un companion de `FiscalAuthority`.
No debe modificar handlers, casos de uso, numeración ni modelos del producto.

## Verificación

```sh
npm ci
npm run ci
```

Desde `v3/`, `make fiscal-e2e` valida además el cliente Go contra este backend.
