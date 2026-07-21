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
