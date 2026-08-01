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

1. El BFF/API es la única API pública y el único servicio que el navegador
   invoca para leer o mutar datos. La Web estática también es pública, pero
   sólo entrega artefactos inmutables y el `index.html`: no tiene secretos,
   conexión SQL, URLs privadas ni permisos sobre servicios internos.
2. Fiscal, Accounting y Accounting Admin se despliegan con
   `ingress=internal` y `--no-allow-unauthenticated`.
3. `roles/run.invoker` se limita a:
   - worker → Fiscal;
   - worker → Accounting;
   - provisioner → Accounting Admin.
4. El proyecto compartido no concede `roles/run.invoker` a nivel proyecto; las
   políticas heredadas de organización/carpeta deben respetar la misma regla.
   Cada permiso se asigna en el servicio destino.
5. El despliegue agrega el invoker requerido y falla cerrado si la auditoría
   encuentra otro principal. No revoca automáticamente identidades
   desconocidas: su propietario debe revisarlas y retirarlas explícitamente.
6. Cada llamada lleva dos credenciales distintas:
   - ID token de Cloud Run en `X-Serverless-Authorization`, con la URL destino
     como audience, para autenticar el workload ante la plataforma;
   - JWT Ed25519 corto de Pymes en `Authorization`, con audience del servicio,
     organización, actor, roles, request/correlation ID y `jti`, para
     autorización tenant y auditoría de aplicación.
7. Las URLs privadas sólo se inyectan en worker y provisioner; nunca en el BFF,
   la Web ni el navegador.
8. La Web usa una service account propia sin `roles/cloudsql.client`, se
   despliega con escala a cero y publica `/healthz` y `/readyz`. El endpoint de
   API y la publishable key de Clerk se fijan al construir la imagen de cada
   entorno; ningún secreto se incorpora al bundle.

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
invoker esperado, aislamiento de URLs internas, Web pública sin SQL/secretos y
configuración condicional de tracing. El dry-run no consulta GCP, no resuelve
secretos y no crea recursos.

En un despliegue real, el script vuelve a consultar ingress y la política IAM
de cada servicio privado, además de rechazar permisos `roles/run.invoker`
asignados directamente al proyecto; cualquier principal adicional hace fallar
el despliegue antes de continuar con API y worker.

## Consecuencias

- La plataforma y la aplicación conservan responsabilidades separadas:
  identidad de workload en IAM, autorización tenant en el JWT.
- Un permiso residual no se oculta ni se elimina sin revisión; bloquea el
  rollout y deja evidencia accionable.
- Mover el sistema a otro runtime exige aprovisionar mTLS/mesh antes de
  habilitar tráfico productivo.
- La ausencia actual de un endpoint OTLP queda visible como pendiente, sin
  introducir costos ni dependencias ficticias.
