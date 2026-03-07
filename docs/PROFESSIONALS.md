# Professionals

Documentacion operativa y arquitectonica de `professionals`, la vertical especializada del repo.

## Rol en el repo

`professionals` implementa el delta de dominio especifico de atencion profesional y sesiones.

No duplica capacidades transversales que ya pertenecen a `control-plane`. Cuando necesita esas capacidades, las consume por HTTP mediante contratos internos.

En esta documentacion:

- `professionals` es una vertical
- `backend`, `frontend` y `AI` son piezas desplegables de la vertical
- `modulo` se usa solo para agrupaciones internas dentro de esos backends

## Componentes

- `professionals/backend`: backend Go de la vertical
- `professionals/frontend`: frontend React de la vertical
- `professionals/ai`: servicio AI especializado de la vertical
- `professionals/infra`: infraestructura Terraform de la vertical

## Superficie local

En Docker:

- backend: `http://localhost:8181`
- frontend: `http://localhost:5181`
- AI: `http://localhost:8201`

Fuera de Docker:

- backend: `make prof-run`
- frontend: `make prof-frontend-dev`
- AI: `make prof-ai-dev`

## Dominio propio

La vertical hoy modela principalmente:

- perfiles profesionales
- especialidades
- intake inicial
- sesiones
- servicios asociados al perfil
- flujos publicos de agenda y atencion sobre ese dominio

## Integracion con control-plane

La vertical usa `control-plane` como owner de capacidades transversales.

Ejemplos de consumo por HTTP:

- bootstrap organizacional
- configuracion de organizacion
- clientes
- productos
- presupuestos
- ventas
- pagos y links de pago
- turnos y disponibilidad publica

Regla de borde:

- `professionals` no importa dominio interno de `control-plane`
- la integracion entre verticales se hace por clientes HTTP
- `control-plane/shared/` y `pkgs/` no reemplazan contratos de ownership funcional

## Validacion

```bash
make prof-test
make prof-vet
make prof-ai-test
cd professionals/frontend && npm run build
```

Chequeos rapidos:

```bash
curl http://localhost:8181/healthz
curl http://localhost:8181/readyz
curl http://localhost:8201/healthz
curl http://localhost:8201/readyz
```

## Documentacion relacionada

- [`README.md`](../README.md)
- [`README de docs`](./README.md)
- [`CONTROL_PLANE.md`](./CONTROL_PLANE.md)
- `prompts/06-professionals.md`
