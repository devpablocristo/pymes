# Web de Pymes v3

La Web es un artefacto React 19 + TypeScript + Vite desplegado como servicio
estático. Sólo habla con el BFF público de Pymes; nunca recibe credenciales de
base de datos, URLs privadas, secretos de Clerk, PerGo, Google, Fiscal ni
Accounting.

## Rutas

| Ruta | Acceso | Responsabilidad |
|---|---|---|
| `/app/agenda` | Sesión Clerk | Operación interna de Agenda. |
| `/reservar/{organizationSlug}` | Público | Reserva y lista de espera. |
| `/agenda/accion/{token}` | Token opaco | Confirmación, cancelación, reprogramación o aceptación de waitlist. |
| `/healthz`, `/readyz` | Público | Probes del contenedor estático. |

## Identidad y tenant

El identificador de organización de Clerk no es un `org_id` de Pymes. Después
de seleccionar una organización, la Web obtiene un JWT de sesión y llama:

```text
GET /api/v1/session
Authorization: Bearer <Clerk session>
```

El BFF verifica Clerk, resuelve la membresía proyectada y devuelve la
organización local. Sólo ese `organization.id` se utiliza en rutas tenant y
claves de caché. El `organization.slug` canónico construye el enlace de booking
público. La respuesta lleva `Cache-Control: no-store`; un tenant pendiente,
fallido o suspendido no abre Agenda.

Los permisos `scheduling:read`, `scheduling:operate` y `scheduling:manage`
también provienen de esa sesión canónica. La Web oculta selección, drag/resize,
transiciones, bloqueos, waitlist y cola cuando falta el permiso correspondiente.
Esto mejora la experiencia, pero no reemplaza la autorización obligatoria del
BFF en cada request.

`VITE_ALLOW_INSECURE_LOCAL_AUTH`, `VITE_USE_FAKE_API` y
`VITE_PYMES_ORGANIZATION_ID` existen únicamente para desarrollo. Los bundles
productivos abortan si se intenta habilitar autenticación insegura o fakes. El
modo de build `e2e` activa un tenant y fake determinísticos sin incorporarlos al
build de producción.

## Capacidades

- día, semana, mes y lista mediante el Calendar Board 0.2 publicado;
- filtro por sucursal, servicio, profesional o recurso;
- alta, selección, reprogramación, drag-and-drop y resize;
- rollback visual ante conflicto de versión, slot, recurso o capacidad;
- confirmación, cancelación con motivo, check-in, completar y no-show;
- disponibilidad habitual, bloqueos, waitlist y cola;
- booking público por sucursal, servicio, profesional opcional, slot y cliente;
- acciones públicas por token de propósito único.

Los formularios de creación conservan un command ID mientras su snapshot no
cambie. Un reenvío idéntico después de perder la respuesta reutiliza la misma
`Idempotency-Key`; editar el formulario genera otro command ID. Las
reprogramaciones, transiciones y avances de cola ligan la clave a la versión
optimista que intentan modificar.

FullCalendar permanece en Standard `6.1.21`, con Luxon 3 y `luxon3`. No se
incluyen Premium, RRule ni FullCalendar 7.

## Build y despliegue

La imagen productiva se construye por entorno:

```bash
docker build \
  --target web \
  --build-arg VITE_API_BASE_URL=https://api.example.com \
  --build-arg VITE_CLERK_PUBLISHABLE_KEY=pk_example \
  --build-arg VITE_PYMES_ORGANIZATION_SLUG=demo \
  -t IMAGE .
```

La publishable key no es un secreto; la secret key de Clerk nunca entra al
bundle. `cloud-run.sh` despliega Web con `min=0`, CPU throttling, acceso público,
sin Cloud SQL y sin secretos. Nginx sirve assets versionados como inmutables,
`index.html` sin caché, fallback SPA y headers CSP, anti-framing y
`nosniff`.

Google OAuth tampoco termina en Web. Existe un único callback global del BFF:

```text
PYMES_GOOGLE_REDIRECT_URL=https://API_ORIGIN/api/v1/calendars/google/oauth/callback
```

No contiene organización en el path; el BFF recupera organización, actor y
expiración desde el `state` de un solo uso. Al habilitar Calendar, el deploy
exige cliente OAuth y CryptoKey diferentes para STG/PRD, monta el client secret
solamente en API y worker, y comprueba que nada de esa configuración llegue a
Web.

## Gates

`make web-ci` ejecuta:

1. drift del cliente TypeScript contra OpenAPI;
2. política de versiones, paquetes Standard y ausencia de rutas locales;
3. typecheck;
4. unitarias;
5. build productivo;
6. ausencia de fakes y source maps públicos en el artefacto;
7. auditoría de dependencias;
8. Playwright en Chromium desktop y mobile.

Los E2E cubren operación de Agenda, filtros accesibles, disponibilidad,
bloqueos, cola, reserva pública y acción opaca. `cloud-run-security-check.sh`
prueba en seco STG/PRD, escala a cero, ausencia de secretos/SQL en Web y el
callback Google global.
