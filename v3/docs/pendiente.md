# Pendientes operativos de Pymes v3

Fecha de registro: 2026-08-08.

Este documento conserva el trabajo operativo que el propietario decidió
postergar. No autoriza su ejecución. El cierre remoto del baseline de código
(Open Accounting fusionado, pin canónico, CI y merge de Pymes) se completa por
separado y no forma parte de esta pausa.

## Límites obligatorios al retomarlo

- operar exclusivamente sobre `pymes-dev-352318` y `us-central1`;
- modificar únicamente recursos identificados como Pymes v3;
- no quitar, ampliar ni reemplazar permisos usados por Axis, Pymes v1/v2 u
  otros workloads del proyecto compartido;
- no usar Axis como dependencia, checkout, servicio, base ni runtime;
- mantener STG y PRD separados por identidades, secretos, claves KMS, bases
  lógicas, servicios, jobs, backups y alertas;
- aplicar primero en STG y avanzar a PRD sólo con evidencia verificable.

## 1. Aislamiento IAM del proyecto compartido

La auditoría de sólo lectura detectó autoridad heredada a nivel proyecto que
alcanza recursos Pymes v3:

- `axis-github-actions-stg`, `github-actions` y
  `pymes-github-actions-stg` conservan `roles/run.admin`;
- la identidad legacy de Pymes conserva autoridad amplia de Secret Manager y
  acceso efectivo a secretos v3.

No se deben retirar esos grants a ciegas porque pueden sostener Axis o Pymes
v2. Antes de desplegar hay que implementar un límite aplicable sólo a recursos
Pymes v3 y demostrar con Policy Analyzer `fullyExplored` que las identidades
ajenas ya no pueden administrar servicios/jobs v3 ni leer sus secretos, sin
cambiar el acceso a los demás workloads.

Criterio de cierre:

- inventario previo y posterior de bindings y accesos efectivos;
- pruebas negativas sobre cada servicio, job y secreto v3;
- pruebas de no regresión sobre los recursos existentes ajenos a v3;
- gates `prepare` y `finalize` de autoridad verdes.

## 2. Identidad de release y controles GitHub

- crear y verificar el WIF exclusivo de Pymes v3;
- limitar repo, workflow, branch y environment;
- completar canary STG con la identidad nueva;
- retirar la identidad legacy sólo después de dos canaries y evidencia limpia;
- conservar el required check de `main` y las protecciones de `stg`/`prd`.

## 3. Secretos e integraciones reales

- reemplazar el bootstrap temporal del webhook Clerk STG;
- cargar credenciales reales de PerGo;
- crear clientes OAuth Google separados para STG y PRD y cargar sus secretos;
- mantener las credenciales ARCA por organización dentro del Fiscal Adapter;
- verificar IAM mínimo y ausencia de secretos, tokens o PII en logs.

Los valores sensibles se cargan mediante Secret Manager o el producto; nunca
por chat, commits ni artefactos de CI.

## 4. Evidencia e imágenes de release

- crear los buckets de evidencia por ambiente;
- aplicar retención, versionado y Bucket Lock después del readback;
- construir y publicar imágenes inmutables por digest;
- publicar el manifiesto create-only y verificar su receipt.

## 5. Despliegue STG

Orden obligatorio:

1. migraciones Pymes;
2. migraciones Fiscal;
3. migraciones Accounting;
4. Accounting;
5. Fiscal;
6. API;
7. Worker;
8. Web;
9. reconciliadores;
10. Monitoring.

Después se deben validar health/readiness, IAM, KMS, RLS, outbox, Agenda,
PerGo fake/real controlado, Google y Fiscal mock antes de promover tráfico.

## 6. Despliegue PRD

- promover únicamente artefactos ya acreditados en STG;
- ejecutar migraciones y verificaciones con las identidades PRD;
- mantener Fiscal real, Agenda, WhatsApp y Google detrás de flags por
  organización;
- no usar simultáneamente el mismo punto de venta ARCA desde v2 y v3.

## 7. Recuperación y operación

- ejecutar backup y restore administrado de las tres bases;
- probar reconciliadores de outbox, Fiscal incierto y Accounting sin acuse;
- crear uptime checks cuando existan URLs definitivas;
- guardar evidencia redactada de rollback, restore y recuperación.

## 8. Pilotos

- Agenda con dos organizaciones, dos sucursales, profesionales y recurso
  compartido;
- PerGo con workspace y número controlados;
- Google Calendar/Meet con una cuenta controlada;
- ARCA con una organización cliente, POS dedicado vacío y homologación propia.

El plan sólo podrá declararse completo al 100% después de cerrar y acreditar
estos ocho bloques operativos.
