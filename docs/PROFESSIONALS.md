# Professionals

`professionals` es una vertical umbrella con schema y backend propios. Hoy el unico modulo activo es `teachers`.

## Dominio

- umbrella vertical: `professionals`
- modulo activo hoy: `teachers`
- dominio actual del modulo:
  - professional profiles
  - specialties
  - intakes
  - sessions
  - service links
  - flujos publicos de agenda y atencion

## Piezas vigentes

- backend: `professionals/backend`
- infra: `professionals/infra`

La consola web y el AI especializado viven dentro de los deployables unificados:

- `frontend`
- `ai`

## Integracion con pymes-core

`professionals/teachers` consume capacidades transversales via HTTP:

- bootstrap y settings de organizacion
- customers y parties
- products
- appointments
- quotes, sales y payment links

Regla de borde:

- no importa dominio interno de `pymes-core`
- integra por clientes HTTP y contratos internos
- reutiliza solo runtime tecnico compartido desde `pymes-core/shared/` y `pkgs/`

## Superficie local

- backend: `http://localhost:8181`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`

Rutas frontend canonicas:

- `/professionals/teachers`
- `/professionals/teachers/specialties`
- `/professionals/teachers/intakes`
- `/professionals/teachers/sessions`
- `/professionals/teachers/public`

Compatibilidad:

- `/professionals`
- `/specialties`
- `/intakes`
- `/sessions`
- `/public`

redireccionan al modulo canonico `teachers`.

API canonica:

- `GET/POST/PUT /v1/teachers/professionals`
- `GET/POST/PUT /v1/teachers/specialties`
- `GET/POST/PUT /v1/teachers/intakes`
- `GET/POST /v1/teachers/sessions`
- `POST /v1/teachers/sessions/:id/complete`
- `POST /v1/teachers/sessions/:id/notes`
- `GET /v1/teachers/public-preview/bootstrap`

Compatibilidad:

- las rutas legacy `/v1/professionals`, `/v1/specialties`, `/v1/intakes`, `/v1/sessions` y asociadas siguen vivas como alias

AI canonico:

- `POST /v1/professionals/teachers/chat`
- `POST /v1/professionals/teachers/public/:org_slug/chat`

Compatibilidad AI:

- `/v1/professionals/chat`
- `/v1/professionals/public/:org_slug/chat`

siguen existiendo como alias.

## Estructura interna estandar

Cada modulo interno del vertical sigue esta forma:

- raiz del modulo: `handler.go`, `repository.go`, `usecases.go`
- DTOs HTTP en `handler/dto`
- modelos persistentes en `repository/models`
- entidades de dominio en `usecases/domain`
- helpers transversales en `internal/shared/handlers` y `internal/shared/values`

`teachers` ya usa este estandar completo y funciona como blueprint para futuros modulos dentro de `professionals`.

Comandos:

```bash
make prof-run
make frontend-dev
make ai-dev
```

## Validacion

```bash
go test ./professionals/backend/...
make ai-test
make frontend-test
```
