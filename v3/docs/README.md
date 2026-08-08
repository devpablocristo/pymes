# Pymes v3: diseño e implementación

Este directorio conserva el dossier y la evidencia de la implementación activa
de Pymes v3. v1 y v2 siguen siendo referencias inmutables. La arquitectura
implementada es:

- **Pymes v3** es el producto, BFF, IAM, dueño de documentos comerciales y
  orquestador de procesos.
- **Open Accounting** se adapta como servicio contable privado y *headless*.
- **arca-facturacion** queda detrás de un adaptador fiscal privado; se usa su
  API de bajo nivel con numeración proporcionada por Pymes.
- **Pymes v2**, **pyafipws** y **LedgerSMB** son referencias de comportamiento,
  no dependencias de ejecución ni fuentes de código copiadas.

## Estado de entrega

<!-- drift:bind v3/scripts/deploy/retain-release-manifest.sh -->
<!-- drift:bind v3/scripts/deploy/cloud-restore-drill.sh -->
<!-- drift:bind v3/scripts/deploy/collect-pilot-evidence.sh -->

El código y los gates locales de H8 ya cubren releases inmutables por SHA y
digest, identidades WIF separadas por ambiente, Web same-origin con marcador
verificable, publicación create-only del manifiesto en almacenamiento con
Bucket Lock, restore coordinado de Pymes/Fiscal/Accounting y collectors
fail-closed para los cuatro pilotos. Estos mecanismos están implementados y
probados con adapters locales; los buckets, restores y pilotos reales siguen
pendientes. La automatización también valida imágenes, labels, service
accounts, invokers, secretos versionados, conectividad privada y jobs después
de cada despliegue. El alta inerte queda ligada por Audit Logs a los FQN exactos
de proyecto, región y tipo, admitiendo sólo el `actAs` inevitable sobre las
identidades runtime allowlisted y una única mutación inicial por recurso. La
evidencia se valida sólo después de diez minutos y dos lecturas estables, con
un margen superior de dos minutos para timestamps de auditoría. Además, cada
release verifica la autoridad completa y efectiva de builder y deployer antes
de usar el builder y nuevamente antes del deploy. Para el alta inicial,
`initial-seed-build` exige que los once recursos Cloud Run estén ausentes,
publica imágenes y evidencia desde GitHub y termina con Deploy omitido; así el
seed humano sólo crea recursos inertes desde el manifiesto retenido.
El builder valida en vivo el IAM de ese bucket mediante el custom role
`pymesV3ReleaseEvidenceIamRead`, que contiene sólo
`storage.buckets.getIamPolicy` y está ligado al bucket Pymes objetivo, no al
proyecto.

El bootstrap IAM sólo puede ejecutarse desde un checkout limpio cuyo HEAD y
árbol coincidan con `main` remoto, por el operador directo revisado y sin
impersonación de `gcloud`. Los gates fijan proyecto, número, región y recursos
Pymes; exigen las org policies efectivas que bloquean claves y adjunción
cross-project; rechazan bindings directos del pool WIF; inventarían adjunciones
de las identidades de release; y comparan mediante Policy Analyzer
project-scoped y auditoría inversa la autoridad definida en el proyecto Pymes o
sus recursos contra allowlists por componente. Release, `prepare` y `finalize`
no leen ni modifican folder, organización u otros proyectos. La única excepción
transitoria es la auditoría humana read-only de ancestros dentro del retiro
one-time del WIF legado; no crea grants ni forma parte del workflow normal.
`gcp-target-policy.sh` actúa como fusible común antes de las escrituras normales:
fija `pymes-dev-352318`, `us-central1`, Artifact Registry `pymes`, la conexión
Cloud SQL `pymes-dev-db` y el target de red
`default`/`pymes-v3-serverless`. Los migradores verifican además base y roles
PostgreSQL efectivos antes de ejecutar DDL. Tanto
`retire-legacy-pymes-wif.sh` como
`migrate-project-secret-access.sh` quedan fuera del release v3 y no se ejecutan
mientras v2 siga activo.

Las reejecuciones de GitHub quedan prohibidas antes de autenticar al builder:
un fallo se continúa con un nuevo dispatch. Cloud Asset y Policy Analyzer son
eventualmente consistentes, por lo que una lectura `fullyExplored` no sustituye
la ventana sin cambios ni la repetición estable exigidas antes de un cutover.

El primer alta de STG se divide deliberadamente en tres pasos:
`initial-seed-build` construye sin desplegar, el seed humano crea los recursos
inertes y `bootstrap` crea candidatos privados con tráfico cero, Fiscal mock,
worker detenido e integraciones externas deshabilitadas. Esto permite conocer
la URL estable sin exponer el producto. Después de configurar Clerk y
reemplazar el secreto temporal del webhook, una ejecución `operational` vuelve
a verificar todo y recién entonces promueve las revisiones. El worker permanece
en escalado manual cero, sin health check de despliegue, hasta que su candidato
sea el último en recibir tráfico; recién allí se activa una instancia.
`initial-seed-build` no cuenta como release ni canary; `bootstrap` no está
permitido en PRD ni cuenta como canary operativo.

Open Accounting quedó fusionado en `main` mediante el PR #2 y Pymes fija el
commit canónico `6647992c75bee76bb70a6baafdb6b0d94fc0acab`. En OA quedaron
verdes su CI remota y las suites locales unitarias/race/vet/lint, build,
integración PostgreSQL y los tres targets Docker. Pymes pasó
`make accounting-test`, `make accounting-e2e` y el `make ci` integral del
2026-08-08 contra ese checkout exacto. El pin definitivo está en cierre remoto
en el PR #52 de Pymes. PerGo quedó fusionado con su automatización
de despliegue segura
en `622296b8fd52ffb84b0e2dae1b81d0926af4675b`, con el run final de `master`
`30746001931` verde; la extensión
`arca-facturacion` `2.6.0` quedó fusionada, publicada y fijada por Pymes, con
CI remoto y suites integradas verdes. Nada de esto constituye
evidencia operativa: no se publicaron las imágenes de esta revisión, no se
desplegó Pymes v3 en STG ni PRD y no se realizaron los pilotos de Agenda,
PerGo, Google o ARCA. El plan sólo podrá declararse completo después de ejecutar
y verificar esas operaciones.

## Lectura sugerida

1. [Evidencia e inventario](01-evidencia.md)
2. [Arquitectura y contratos de comportamiento](02-arquitectura.md)
3. [Destino de cada módulo](03-disposicion-modulos.md)
4. [Validación y spikes descartables](04-validacion.md)
5. [Roadmap](05-roadmap.md)
6. [Plan de ejecución](06-plan-ejecucion.md)
7. [Estado verificable de implementación](07-estado-implementacion.md)
8. [Auditoría de cierre](08-auditoria-cierre.md)
9. [Seguridad GCP STG/PRD](09-gcp-stg-prd.md)
10. [Runbook de operación y recuperación](10-runbook-operacion.md)
11. [Arquitectura vertical Go](go-architecture.md)
12. [Agenda multi-tenant](scheduling.md)
13. [Web de Pymes v3](web-v3.md)
14. [Notificaciones WhatsApp mediante PerGo](11-notifications-pergo.md)
15. [Google Calendar y Meet](11-google-calendar-meet.md)
16. [Rollout de capacidades por organización](12-feature-flags.md)
17. [ADRs](adr/)
18. [Contratos OpenAPI internos](../contracts/)

Los diagramas Mermaid describen la arquitectura objetivo. Toda API marcada
`internal` sólo admite credenciales de servicio: ningún navegador llega a los
servicios contable o fiscal.
