# Fiscal Adapter

Backend privado TypeScript de Pymes v3. Encapsula el mock contractual y la
integración real WSAA/WSFEv1 mediante el paquete publicado
`@devpablocristo/arca-facturacion`. Ningún navegador accede a este servicio.

## Estado actual

El proceso falla al iniciar si no se selecciona explícitamente `mock` o `arca`.
El entorno local usa un emulador del límite KMS con una clave de 32 bytes:

```sh
PYMES_ENVIRONMENT=development \
FISCAL_ADAPTER_MODE=mock \
FISCAL_ALLOW_INSECURE_LOCAL=true \
FISCAL_DATABASE_URL=postgres://fiscal:fiscal@127.0.0.1:55435/pymes_fiscal \
FISCAL_LOCAL_KMS_KEY_B64=BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc= \
npm start
```

`FISCAL_MOCK_SCENARIO` admite `authorized`, `rejected`,
`timeout_before_processing` y `response_lost_after_processing`.

El modo autenticado exige `PYMES_INTERNAL_ISSUER` y
`PYMES_INTERNAL_JWKS_JSON`. El JWKS usa claves públicas `OKP`/`Ed25519`,
`alg=EdDSA`, un `kid` único y puede contener la clave anterior y la siguiente
durante una rotación. `FISCAL_ALLOW_INSECURE_LOCAL` y la compatibilidad con
`PYMES_INTERNAL_PUBLIC_KEY_B64` requieren opt-in y
`PYMES_ENVIRONMENT=development|test`; ambas fallan en producción.

El modo `arca` exige una clave simétrica regional de Cloud KMS mediante
`FISCAL_KMS_KEY_NAME`, patrones explícitos de issuer para homologación y
producción y una identidad interna válida. Cada organización genera su propia
clave RSA dentro de Fiscal, recibe sólo el CSR, carga su certificado y configura
sus puntos de venta. Certificados, claves privadas, tickets WSAA y artefactos de
respuesta se guardan cifrados con envelope encryption y AAD tenant-aware.
Pymes no tiene CUIT ni certificado fiscal global.

## Arquitectura

- `fiscal/usecases/domain`: tipos, estados y errores fiscales estables.
- `fiscal/usecases.ts`: casos de uso y puertos consumer-owned.
- `fiscal/handler.ts` + `handler/models|helpers`: API HTTP privada.
- `fiscal/repository.ts` + `repository/models|helpers`: PostgreSQL, RLS,
  idempotencia y métricas.
- `fiscal/arca.ts` + `arca/models|helpers`: numeración explícita, WSAA durable,
  WSFE, reconciliación exacta y artefactos cifrados.
- `fiscal/mock_authority.ts` + `mock_authority/models|helpers`: fake
  contractual seleccionado por configuración.
- `credentials`: CSR, validación X.509, vault tenant-aware y envelope encryption.
- `identity/internal_jwt.ts` + `models|helpers`: JWT Ed25519 de workloads.
- `wire.ts`: única composición de dependencias.

Todos los adapters tienen archivo raíz de coordinación y directorios propios
`models` y `helpers`; no existen capas horizontales `ports`, `domain`,
`companion`, `handler` o `repository`.

## Verificación

```sh
npm ci
npm run ci
FISCAL_DATABASE_TEST_URL=postgres://... npm run test:postgres
```

Desde `v3/`, `make fiscal-e2e` valida además el cliente Go contra este backend.
