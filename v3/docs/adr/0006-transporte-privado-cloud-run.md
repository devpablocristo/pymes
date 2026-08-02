# ADR 0006: transporte privado autenticado en Cloud Run

**Estado:** aceptada.

## Contexto

El diseño exige autenticación mutua entre workloads y que ningún navegador
acceda directamente a Fiscal o Accounting. En Cloud Run la aplicación no
termina ni administra certificados de cliente: Google termina HTTPS y Cloud
Run autentica la identidad del workload mediante un ID token e IAM. Forzar
mTLS dentro de cada contenedor duplicaría esa capa administrada y no
representaría el modelo real del runtime.

`stg` y `prd` comparten el proyecto GCP para reducir costos, pero mantienen
nombres, service accounts, secretos, claves y políticas IAM separados por
entorno.

## Decisión

Dentro de Cloud Run se adopta este control compuesto:

1. En una entrega `operational`, el BFF/API es la única API pública y el único
   servicio que el navegador invoca para leer o mutar datos. La Web estática
   también es pública, pero sólo entrega artefactos inmutables y el
   `index.html`: no tiene secretos, conexión SQL, URLs privadas ni permisos
   sobre servicios internos. Durante pretraffic, los hosts taggeados de API y
   Web exigen una capability efímera común; sin el header exacto responden
   `404`. El gate sólo aplica al host candidato, no al origen estable.
2. Fiscal, Accounting y Accounting Admin se despliegan con
   `ingress=internal` y `--no-allow-unauthenticated`.
3. `roles/run.invoker` se limita a:
   - API y worker → Fiscal;
   - worker → Accounting;
   - provisioner → Accounting Admin.
4. El proyecto compartido no concede `roles/run.invoker` a nivel proyecto; las
   políticas heredadas de organización/carpeta deben respetar la misma regla.
   Cada permiso se asigna en el servicio destino.
5. El despliegue agrega el invoker requerido y falla cerrado si la auditoría
   encuentra otro principal. No revoca automáticamente identidades
   desconocidas en un servicio preexistente: su propietario debe revisarlas y
   retirarlas explícitamente. Cada creación/actualización usa el chequeo IAM de
   invocación de Cloud Run y el readback exige ingress e invokers exactos. Un
   primer despliegue fallido sí se vuelve inerte retirando invokers y cambiando
   ingress a interno antes de eliminarlo.
6. Cada llamada lleva dos credenciales distintas:
   - ID token de Cloud Run en `X-Serverless-Authorization`, con la URL destino
     como audience, para autenticar el workload ante la plataforma;
   - JWT Ed25519 corto de Pymes en `Authorization`, con audience del servicio,
     organización, actor, roles, request/correlation ID y `jti`, para
     autorización tenant y auditoría de aplicación.
7. La URL de Fiscal se inyecta en API y worker porque el onboarding fiscal
   administrativo entra por el BFF; la de Accounting sólo en worker y la de
   Accounting Admin sólo en provisioner. La Web y el navegador nunca reciben
   ninguna URL privada. API, worker, Fiscal y provisioner usan Direct VPC Egress
   sobre una subred con Private Google Access y Public NAT verificables.
8. La Web usa una service account propia sin `roles/cloudsql.client`, se
   despliega con escala a cero y publica `/healthz` y `/readyz`. El endpoint de
   API no se fija en el bundle: Nginx recibe el upstream del BFF en runtime y
   sirve `/api/` en el mismo origen. La publishable key de Clerk sí es pública y
   se fija al construir la imagen; ningún secreto se incorpora al bundle. Por
   ello STG y PRD se reconstruyen desde el mismo SHA, pin, receta y materiales,
   pero sus imágenes Web pueden tener digests distintos.
9. Google OAuth inicia y termina en el BFF mediante un callback global,
   `/api/v1/calendars/google/oauth/callback`. El callback no contiene el tenant:
   organización y actor se recuperan de un `state` de un solo uso. El client
   secret se monta sólo en API y worker; nunca en la Web.
10. Antes del primer workflow, un seed manual cerrado crea los seis servicios
    con escalado manual cero, tráfico e IAM vacíos y los cinco jobs sin
    ejecuciones. No adjunta secretos, SQL, Direct VPC ni Serverless VPC
    Connector. Usa la autoridad preexistente del Owner revisado, no un grant
    temporal, y `finalize` concede Run Admin sólo por recurso después de validar
    manifiesto, identidad, Audit Logs y estado inerte. Esa auditoría liga cada
    escritura Cloud Run al proyecto, región, tipo y nombre exactos y sólo admite
    el evento `actAs` inevitable sobre la identidad runtime allowlisted; una
    segunda mutación del mismo recurso o `SetIamPolicy` invalida el seed. La
    privacidad inicial se prueba con ingress interno y policy IAM vacía, sin
    flags `--[no-]allow-unauthenticated` que puedan escribirla. La
    finalización espera diez minutos y exige dos lecturas estables de Admin
    Activity, usando dos minutos de margen superior para timestamps.
11. El alta inicial de STG usa una etapa `bootstrap` cerrada: los seis servicios
    se crean como candidatos con tráfico cero; API y Web permanecen con
    `ingress=internal` y sin invocación anónima; el worker queda con escala
    mínima cero; Fiscal usa el mock; PerGo y Google están deshabilitados. La
    etapa rechaza cualquier servicio que ya tenga tráfico, nunca promueve
    revisiones y elimina todos los tags antes de terminar. Sólo se permite el
    secreto Clerk rotulado
    `lifecycle=bootstrap-temporary`.
12. Antes de ejecutar `operational`, el operador configura Clerk con la URL
    estable, reemplaza el secreto temporal por el secreto real del endpoint y
    elimina ese rótulo. La etapa operacional rechaza el rótulo temporal,
    revalida los candidatos y aplica las reglas públicas de los puntos
    anteriores. `bootstrap` está prohibido en PRD.

HTTPS administrado por Cloud Run + ingress interno + IAM mínimo + JWT interno
es el equivalente operativo del requisito de mTLS para este runtime. No se
afirma que exista mTLS terminado por la aplicación. Si cualquiera de estos
servicios se ejecuta fuera de Cloud Run, el entorno debe imponer mTLS mediante
service mesh, proxy o identidad de workload equivalente; el JWT interno sigue
siendo obligatorio y no sustituye esa capa.

La implementación vive en
[`cloud-run.sh`](../../scripts/deploy/cloud-run.sh). La obtención y caché del
ID token está en
[`cloud_run_tokens.go`](../../backend/internal/identity/cloud_run_tokens.go).

## Observabilidad

El despliegue no inventa un collector. Si no existe
`OTEL_EXPORTER_OTLP_ENDPOINT` explícito, registra
`TRACING status=pending exporter=none endpoint=unset` y no inyecta variables de
tracing. Con un endpoint explícito sin credenciales embebidas, configura
`PYMES_TRACING_EXPORTER=otlp`, el endpoint y
`PYMES_TRACE_SAMPLE_RATIO` solamente en API y worker.

## Verificación

[`cloud-run-security-check.sh`](../../scripts/deploy/cloud-run-security-check.sh)
ejecuta el despliegue localmente en modo dry-run para `stg` y `prd`, con y sin
endpoint OTLP. El gate verifica ingress privado, ausencia de acceso anónimo,
invoker esperado, aislamiento de URLs internas, Direct VPC, release labels, Web
pública sin SQL/secretos y configuración condicional de tracing. El dry-run no
consulta GCP, no resuelve secretos y no crea recursos.

El mismo gate ejecuta también el bootstrap de STG y comprueba tráfico cero,
ingress interno para API/Web, worker detenido, ausencia de promoción, tag no
público y secreto Clerk temporal. Casos negativos prueban que bootstrap no
pueda habilitar Fiscal real, PerGo, Google, PRD ni reutilizar servicios con
tráfico.

El mismo gate ejecuta Calendar deshabilitado y habilitado para STG/PRD. En el
segundo caso exige callback global HTTPS, cliente y CryptoKey por entorno,
comprueba que API y worker reciban la misma configuración y que Web no reciba
ninguna credencial Google.

En un despliegue real, el script crea cada revisión candidata con tráfico cero
y vuelve a consultar ingress y la política IAM de cada servicio, además de
rechazar permisos `roles/run.invoker` asignados directamente al proyecto;
cualquier principal adicional bloquea la promoción. API y Web candidatas
conservan la misma capability de 64 caracteres hexadecimales. El verificador
prueba `404` sin ella y los probes exactos con ella; Nginx sólo la agrega al
upstream candidato y no la registra.

La identidad de transporte también se audita fuera de la policy del servicio.
Cada runtime SA debe estar habilitada, sin claves administradas por usuarios y
con una policy entrante que contenga sólo el `roles/iam.serviceAccountUser` del
deployer del mismo entorno. IAM Policy Analyzer expande roles, grupos e
impersonación y compara su autoridad efectiva con una allowlist por componente:
únicamente SQL, Secrets, KMS e invocaciones privadas que ese workload necesita.
Los deployers keyless no pueden estar adjuntos a un workload y la org policy
`iam.disableCrossProjectServiceAccountUsage` debe estar efectivamente forzada.
Así, el ID token aceptado por Cloud Run proviene de una identidad runtime
revisada y no de una ruta alternativa del pool de release.

El verificador pretraffic compara servicios y jobs contra los digests del
manifiesto, la revisión candidata lista, service account, ingress, escala,
CPU, SQL, subred, secretos con versión numérica, URLs internas, KMS/JWKS y el
label del SHA. Antes de la primera mutación de tráfico, el despliegue resuelve
exactamente el baseline y demuestra que el Web activo usa el tag, revisión y
capability de la API activa; errores de lectura no equivalen a ausencia. Sólo
después promueve el 100% del tráfico y repite las verificaciones sobre la
revisión activa dentro de la transacción que conserva el rollback. El
asentamiento elimina todos los
tags de servicios salvo uno: el tag del release activo de API que usa el proxy
interno del Web activo. El verificador exige exactamente esa forma y bloquea
tags residuales que harían alcanzable otra revisión; además prueba que cada URL
retirada responda `404`. Ante un fallo revierte a la revisión anterior,
restaura y prueba primero el tag API que necesita el Web anterior y elimina los
tags candidatos, o falla cerrado. La recuperación manual durable usa
`rollback-cloud-run.sh` con el SHA exacto y revalida el par Web/API desde
metadata de revisión antes de mutar. También prueba el
borde público por TLS y exige que
`/readyz` atraviese Nginx, responda por TLS y lleve el header de release exacto,
no una respuesta estática de otro artefacto. Además consulta
`/api/v1/session` sin credenciales: debe atravesar el mismo proxy, conservar el
header de release y responder `403` con el error JSON estable del BFF.

Para el worker, “escala” incluye dos invariantes adicionales: toda revisión
candidata conserva mínimo de revisión `0` y el servicio permanece en escalado
manual `0` durante pretraffic. No se ejecuta el deployment health check porque
iniciaría el `Runner` aun sin tráfico. La activación manual `1` ocurre sólo
después de enrutar al candidato. La release espera un
`worker_release_ready` de la revisión y SHA exactos, emitido únicamente después
de una primera lectura durable desde PostgreSQL. Rollback vuelve a `0` antes de
tocar el tráfico.

## Consecuencias

- La plataforma y la aplicación conservan responsabilidades separadas:
  identidad de workload en IAM, autorización tenant en el JWT.
- Un permiso residual no se oculta ni se elimina sin revisión; bloquea el
  rollout y deja evidencia accionable.
- Mover el sistema a otro runtime exige aprovisionar mTLS/mesh antes de
  habilitar tráfico productivo.
- La ausencia actual de un endpoint OTLP queda visible como pendiente, sin
  introducir costos ni dependencias ficticias.
