# Pymes v2

Runtime técnico de la nueva generación de Pymes. Parte de una base vacía y no
lee, ejecuta ni importa código de `v1/`.

## Componentes

- [`backend/`](backend/) es la API Go y su composition root.
- [`db/`](db/) contiene el migrador y las migraciones PostgreSQL nuevas.
- [`web/`](web/) es la aplicación React responsive.
- [`compose.yaml`](compose.yaml) levanta exclusivamente PostgreSQL 16 para
  desarrollo local.

Cada componente mantiene dependencias y checks propios. Los módulos de
`platform` se consumen exclusivamente mediante versiones publicadas.

## Desarrollo local con Docker

Requiere Docker con Compose. No requiere Go ni Node instalados en el host:

```bash
make up
make smoke
```

La web queda disponible en `http://localhost:5173`, la API en
`http://localhost:8080` y PostgreSQL en `127.0.0.1:55433`. El job `migrate`
termina antes de que se inicie el backend.

`make down` detiene los servicios sin borrar el volumen. `make ps` muestra el
estado y `make logs` sigue los logs del stack.

## Desarrollo nativo

Para ejecutar backend y web fuera de contenedores se requieren Go 1.26.5,
Node 20.19 o posterior y npm 10:

```bash
cp .env.example .env
make db-up
make db-migrate
make backend-run
make web-dev
```

La API escucha en `http://localhost:8080` y expone:

- `GET /healthz`: liveness del proceso;
- `GET /readyz`: disponibilidad de PostgreSQL.

Para ejecutar los mismos checks de cada componente:

```bash
make ci
make db-integration
```

`make db-down` detiene el stack sin borrar su volumen.

Principios iniciales:

- núcleo horizontal y web responsive;
- monolito modular con API Go y PostgreSQL nuevos;
- OpenAPI como contrato público;
- aislamiento tenant fail-closed;
- Money decimal exacto;
- consumo directo de releases publicadas de `platform`;
- cero lectura, escritura o fallback hacia v1.
