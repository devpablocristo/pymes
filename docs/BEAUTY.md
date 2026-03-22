# Beauty (salón / belleza)

Vertical `beauty` para salones, barberías y estética en LATAM. El dominio propio es **equipo** y **menú de servicios** (duración, precio); la **agenda** sigue en `pymes-core` vía HTTP.

## Ownership

- vertical: `beauty`
- tablas: `beauty.staff_members`, `beauty.salon_services`
- integraciones: citas públicas y creación autenticada contra `appointments` del core (`pymescore`)

## Backend

### Local (Docker Compose)

- Servicio: **`beauty-backend`**, puerto host **8383** (mapa completo: [docs/README.md](./README.md)).
- Binario local sin contenedor: `cmd/local` escucha **8083** por defecto dentro del proceso.

- `beauty/backend/cmd/local`
- `beauty/backend/wire/bootstrap.go`

Rutas autenticadas (`/v1/beauty/...`):

- `GET/POST/PUT /staff`
- `GET/POST/PUT /salon-services`
- `POST /salon-appointments`

Público:

- `GET /v1/public/:org_slug/beauty/services`
- `POST /v1/public/:org_slug/beauty/appointments`

## Infraestructura (AWS)

- Terraform: `beauty/infra/` (Lambda + API Gateway HTTP + SSM + IAM + alarmas).
- Ver `beauty/infra/README.md` para `terraform init/apply` y variables.
- El frontend unificado no se empaqueta aquí; solo se configura `VITE_BEAUTY_API_URL` contra la URL del API Gateway (o dominio custom).

## Frontend

- Onboarding: vertical `beauty`
- Rutas: `/beauty/salon/staff`, `/beauty/salon/services`
- Variable Vite: `VITE_BEAUTY_API_URL` (Docker: `http://localhost:8383`)

## Reutiliza desde pymes-core

Clientes/parties, productos (vinculo opcional), citas (`CreateAppointment`, `BookAppointment`), cobros futuros como el resto de verticales.

## Crea nuevo en la vertical

Staff del salón, catálogo de servicios con duración en minutos y precio, superficie pública de servicios.
