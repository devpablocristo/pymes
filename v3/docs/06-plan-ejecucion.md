# Plan de ejecución de Pymes v3

Fecha de revisión: 2026-08-08.

## Objetivo

Cerrar Pymes v3 como SaaS modular: producto público y orquestador en Pymes,
contabilidad privada en Open Accounting, entrega WhatsApp en PerGo, Calendar y
Meet como proyección de Google, y emisión ARCA por organización desde Fiscal.

Axis no forma parte del sistema. Sólo se observó en modo lectura para documentar
el patrón estructural Go, que ahora es autónomo en
[`go-architecture.md`](go-architecture.md).

## Topología

- Un módulo Go Pymes con binarios API, worker y provisioner.
- Una Web React que sólo llama al BFF por el mismo origen.
- Fiscal Adapter TypeScript privado, seleccionable entre mock y ARCA.
- Open Accounting headless privado y con base propia.
- PostgreSQL como persistencia durable; relay outbox por HTTP interno.
- Platform aporta algoritmos/componentes publicados, nunca datos o tenancy.

Cada contexto Go es vertical. `handler`, `repository`, `worker` y cada proveedor
externo son adapters con archivo raíz, `models/` y `helpers/`; los casos de uso
declaran sus puertos, el dominio ignora infraestructura y sólo `wire` construye
dependencias.

## Hitos

### H0 — Baseline

- Fusionar y fijar Open Accounting.
- Mantener Pymes v1/v2 en sólo lectura.
- Establecer contratos, migraciones, Compose y CI.

**Estado:** cerrado localmente; publicación pendiente. Open Accounting está
fijado localmente en `7a914c4617c5252b3ba97d00d3895fbcbf381ff7` y tiene
unitarias/race/vet/lint, build, integración PostgreSQL y Docker verdes. Pymes
pasó `make accounting-test`, `make accounting-e2e` y `make ci` contra ese
commit. Falta publicar e integrar ambos cambios y obtener CI remoto verde.

### H1 — Arquitectura Go

- Migrar todos los contextos al patrón vertical.
- Eliminar capas horizontales y `internal/contracts`.
- Prohibir cualquier dependencia o referencia técnica a Axis.
- Validar estructura, imports y composición por AST/`go list`.

**Estado:** cerrado.

### H2 — Platform Scheduling

- Publicar Scheduling Go, Calendar Board y Scheduling React 0.2.
- Consumir sólo versiones publicadas, detrás del adapter de Agenda.
- Mantener persistencia, tenancy y dominio de producto en Pymes.

**Estado:** cerrado.

### H3 — Agenda

- Sucursales, servicios, profesionales y recursos múltiples.
- Disponibilidad, excepciones, buffers, holds y recurrencia.
- Turnos individuales/grupales, waitlist, cola y acciones opacas.
- RLS, referencias tenant-aware, exclusiones y capacidad transaccional.

**Estado:** cerrado en código; piloto en H8.

### H4 — Web

- React 19, Clerk, vistas de calendario y operación completa.
- Edición optimista de cliente/contacto, participantes, nota y subestado,
  separada de reprogramación, asignación de recursos y transiciones.
- Booking público y acciones por token.
- Rollback visual ante conflictos y browser E2E.

**Estado:** cerrado en código; despliegue en H8.

### H5 — Notifications/PerGo

- Pymes decide intención, template y momento.
- PerGo decide transporte y entrega.
- Outbox, idempotencia, webhook firmado e inbox.

**Estado:** cerrado en código; credenciales y piloto en H8.

### H6 — Google Calendar/Meet

- OAuth tenant en el BFF, tokens cifrados y calendario “Pymes”.
- Evento/Meet determinístico, FreeBusy opcional, ETag y reconciliación.
- Fallos de Google nunca bloquean un turno.

**Estado:** cerrado en código; OAuth real y piloto en H8.

### H7 — ARCA real multi-tenant

- CSR y clave privada generados por Fiscal para cada organización.
- Certificado validado y vault cifrado con KMS.
- WSAA, WSFE, autorización, consulta exacta e incertidumbre.
- Número reservado por Pymes; Fiscal nunca autonumera.

**Estado:** cerrado en código integrado. La extensión `2.6.0` está fusionada,
etiquetada, publicada en npm y fijada en el lockfile de Pymes; las suites del
SDK y Fiscal Adapter son verdes contra ese artefacto. Credenciales y
homologación permanecen en H8. Padrón, FCE, WSFEX y CAEA quedan fuera.

### H8 — Release, despliegue y pilotos

1. Reconciliar la protección de `main` y los environments `stg`/`prd` antes de
   crear WIF.
2. Auditar KMS e identidades SQL/runtime ya provisionados, cargar los valores
   reales pendientes de Clerk webhook, PerGo y Google, y preparar la red por
   entorno.
3. Preparar WIF separado para build, STG y PRD sin claves JSON persistentes.
4. Construir imágenes con SBOM/provenance, resolverlas a digest y conservar un
   manifiesto durable que vincule Pymes y Open Accounting.
5. Crear los recursos iniciales inertes de STG desde ese manifiesto con el Owner
   preexistente revisado, sin agregar autoridad temporal; auditar sus once
   mutaciones y finalizar únicamente los permisos STG por recurso.
6. Ejecutar migraciones y crear revisiones candidatas con tráfico cero; verificar
   digest, IAM, red, secretos, probes y release marker antes y después de
   promover.
7. Ejecutar un canary STG con el WIF nuevo, retirar el acceso WIF legado, ejecutar
   un segundo canary posterior y cerrar la transición sólo con auditoría limpia.
8. Restaurar las tres bases a destinos aislados y reconciliar sin duplicados.
9. Pilotear Agenda, PerGo, Google/Meet y ARCA homologación en STG.
10. Preparar PRD después del cierre STG y repetir todos los controles con el
   mismo source SHA, pin OA y receta reproducible.

La configuración de build actual incorpora metadata del ambiente y una
publishable key Clerk distinta en Web. Por eso STG y PRD producen digests
distintos aunque sus fuentes, dependencias y receta deban ser idénticas; el
criterio verificable es igualdad de materiales, no identidad de digest.

**Estado:** en ejecución.

## Gates

`make ci` incluye contratos, arquitectura, vet, seguridad, tests, Fiscal real
contra fakes, Web, builds, migraciones y E2E de Agenda, Notifications,
Calendars, Comercio, Fiscal, Accounting, recuperación y backup/restore.

Los tests determinísticos usan fakes. Las pruebas con Google real y ARCA
homologación son los workflows manuales protegidos
`v3-google-live.yml` y `v3-arca-homologation.yml`; no forman parte del CI de
cada commit. Ambos están fijados a STG, aceptan únicamente `main` con CI verde
para el SHA exacto, auditan los controles GitHub antes de leer secrets y
rechazan reruns. `make workflow-policy-check` valida esa política y
`make protected-live-validation-test` ejerce los transportes con fakes sin red.
La existencia del job no acredita el piloto: sólo un run protegido verde con
una organización controlada constituye evidencia operativa.

## Regla de finalización

El plan llega al 100% sólo cuando
[`08-auditoria-cierre.md`](08-auditoria-cierre.md) no conserva ninguna puerta
abierta. Código implementado, cloud provisionado y piloto son evidencias
distintas; ninguna sustituye a las otras.
