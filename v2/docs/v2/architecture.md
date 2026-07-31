# Arquitectura objetivo de Pymes v2

V2 será un monolito modular: una API Go, una aplicación web y una base
PostgreSQL independiente. Los contextos iniciales serán identidad,
organizaciones, parties, catálogo, inventario, comercial, contabilidad y fiscal.

Las capacidades técnicas reutilizables se consumen desde releases de
`platform`. Los adapters de proveedor y las reglas comerciales permanecen en
Pymes. Todos los módulos tenant resuelven la organización desde la identidad
verificada, escriben dentro de una transacción y se protegen con RLS.

La API de producto se publica bajo `/api/v1`; esto es independiente del nombre
interno “v2”. OpenAPI genera los tipos de servidor y el cliente TypeScript.

## Runtime técnico

El runtime se divide en tres unidades desplegables y verificables:

- `v2/backend` es un módulo Go independiente. Usa `net/http`, logging JSON y
  request IDs de `platform/observability/go`, y readiness real contra
  PostgreSQL mediante `platform/databases/postgres/go`.
- `v2/db` es otro módulo Go. Es el único dueño de las migraciones del producto y
  comienza con el schema vacío `app`.
- `v2/web` es una aplicación Vite, React y TypeScript con providers de tema,
  idioma y contención de errores. Todavía no contiene rutas ni módulos de
  negocio.

No existe `go.work` compartido ni referencias locales a `platform`. Cada
consumidor declara releases publicadas y puede validarse aisladamente con
`GOWORK=off`.

Los endpoints `/healthz` y `/readyz` no forman parte de la API de negocio. La
primera API funcional seguirá naciendo bajo `/api/v1` a partir de OpenAPI.
