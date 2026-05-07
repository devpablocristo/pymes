# Seeds

Este flujo deja datos demo coherentes para las pantallas operativas del panel.

## Comandos

- `make seed-clear`: limpia datos demo/operativos y preserva bootstrap de tenant, org, usuarios, miembros, settings y API keys.
- `make seed-clear-verify`: valida que las pantallas operativas hayan quedado vacias sin borrar bootstrap.
- `make seed`: carga datos base, demo core, agenda, verticales y governance demo en el tenant seleccionado.
- `make seed-verify`: valida conteos por DB y por API contra el contrato central.
- `make seed-reset`: ejecuta clear, seed y verify en ese orden.

## Contrato

El contrato vive en `scripts/seeds/seed_contract.sh`. Cada modulo visible debe tener al menos 10 registros, salvo tablas tecnicas o configuracion no listable. La verificacion por API usa los mismos endpoints principales que consume la UI.

Los pagos se validan por DB porque la pantalla/API visible lista pagos asociados a ventas, no un listado global independiente.

## Agregar un modulo

1. Crear seeds idempotentes con IDs deterministiscos (`uuid_generate_v5`) y `ON CONFLICT` sobre constraints reales.
2. Hacer que `seed-clear` limpie sus tablas operativas sin borrar bootstrap.
3. Agregar un chequeo DB y, si hay pantalla visible, un chequeo API en `seed_contract.sh`.
4. Correr `make seed-reset` y luego `make seed && make seed-verify` para probar idempotencia.

## Variables utiles

- `DEFAULT_SEED_TENANT_SLUG`: slug local por defecto dentro de `scripts/seeds/lib.sh`. Default: `medlab`.
- `PYMES_SEED_DEMO_TENANT_SLUG`: equivalente para llamar scripts directamente.
- `PYMES_SEED_DEMO_TENANT_EXTERNAL_ID`: selector legacy/CI/GCP por identificador externo cuando no se usa slug.
- `SEED_VERIFY_API_KEY`: API key para checks HTTP (default: `VITE_API_KEY` o `psk_local_admin`).
- `SEED_VERIFY_CORE_URL`, `SEED_VERIFY_WORKSHOPS_URL`, `SEED_VERIFY_PROFESSIONALS_URL`, `SEED_VERIFY_RESTAURANTS_URL`: URLs de backends.
- `SEED_VERIFY_SKIP_API=1`: ejecuta solo checks DB.
