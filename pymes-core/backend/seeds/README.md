# Seeds (datos de demo) — control plane

**Las migraciones solo definen esquema.** Todo INSERT de desarrollo vive aquí.

## Archivos (orden)

| Archivo | Contenido |
|---------|-----------|
| `01_local_org.sql` | Org local fija, usuario, API key `psk_local_admin` (modo sin `PYMES_SEED_DEMO_ORG_EXTERNAL_ID`) |
| `01_clerk_prereqs.sql` | `tenant_settings` + API key demo sobre org ya existente (Clerk) |
| `02_core_business.sql` | Clientes, proveedores, productos, cotización, ventas, stock, caja |
| `03_rbac.sql` | Roles, permisos, `user_roles`, lista de precios default |
| `04_transversal_modules_demo.sql` | Citas, recurrentes, compras, procurement, webhooks, cuentas |
| `modules/scheduling/go/seeds/0001_demo.sql` | Demo reusable de scheduling/queue, invocado por `make seed` |

## Cómo aplicar

- **Cargar demo con Docker:** desde la raíz del monorepo:

```bash
make seed
```

- **Limpiar demo con Docker:**

```bash
make seed-clear
```

Variables:

- **`PYMES_SEED_DEMO_ORG_EXTERNAL_ID`** (opcional): `org_…` de Clerk. Resuelve `orgs.id` por `external_id`, sustituye el UUID fijo de demo en los SQL y en modo Clerk omite `01_local_org.sql` para no pisar `external_id`.
