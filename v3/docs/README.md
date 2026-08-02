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
de usar el builder y nuevamente antes del deploy.

El bootstrap IAM sólo puede ejecutarse desde un checkout limpio cuyo HEAD y
árbol coincidan con `main` remoto, por el operador directo revisado y sin
impersonación de `gcloud`. Los gates fijan la cadena proyecto/folder/organización
y los roles lectores exactos de ancestros; exigen las org policies que bloquean
claves y adjunción cross-project; rechazan bindings directos del pool WIF;
inventarían adjunciones de las identidades de release; y comparan la autoridad
efectiva, incluida impersonación, de las diez identidades runtime contra
allowlists por componente. La creación inicial de los dos custom roles de
organización y sus bindings requiere administración humana explícita y
`orgpolicy.googleapis.com`; esa capacidad no pertenece al workflow.

Las reejecuciones de GitHub quedan prohibidas antes de autenticar al builder:
un fallo se continúa con un nuevo dispatch. Cloud Asset y Policy Analyzer son
eventualmente consistentes, por lo que una lectura `fullyExplored` no sustituye
la ventana sin cambios ni la repetición estable exigidas antes de un cutover.

El primer alta de STG se divide deliberadamente en dos ejecuciones. La etapa
`bootstrap` crea candidatos privados con tráfico cero, Fiscal mock, worker
detenido e integraciones externas deshabilitadas; esto permite conocer la URL
estable sin exponer el producto. Después de configurar Clerk y reemplazar el
secreto temporal del webhook, una ejecución `operational` vuelve a verificar
todo y recién entonces promueve las revisiones. El worker permanece en
escalado manual cero, sin health check de despliegue, hasta que su candidato
sea el último en recibir tráfico; recién allí se activa una instancia.
`bootstrap` no está permitido en PRD ni cuenta como canary operativo.

Open Accounting está fusionado y su CI remoto es verde en
`ad1c182093986279aac7fb6582f7788202112a78`. Para el baseline integrado, Pymes
PR #47 está fusionado en `ccff2c106da92f3bfc74b2d12b5f4409aa743050`,
`make ci` pasó contra ese pin y su workflow remoto de `main` `30744384829`
quedó verde. PerGo quedó fusionado con su automatización de despliegue segura
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
