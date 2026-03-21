# Infraestructura – vertical `beauty`

Terraform para desplegar el **backend Go** de belleza/salón en AWS:

- **Lambda** (imagen container ARM64) – entrypoint `beauty/backend/cmd/lambda`
- **API Gateway HTTP API** – ruta catch-all `ANY /{proxy+}` → Lambda
- **SSM Parameter Store** – `DATABASE_URL`, `PYMES_CORE_URL`, `INTERNAL_SERVICE_TOKEN`, Clerk (`JWT_ISSUER`, `JWKS_URL`), `FRONTEND_URL`
- **IAM** – rol de ejecución + lectura SSM bajo `/${prefix}/`
- **CloudWatch** – logs, alarmas de errores, dashboard mínimo

No incluye:

- Frontend SPA (la consola es única; despliegue global en `pymes-core` / pipeline del monorepo)
- Servicio AI (sigue en `ai/` unificado)

La referencia de diseño completo (Lambda + AI + S3 + CloudFront) es `professionals/infra/`; aquí se recorta a lo necesario para un **solo** backend vertical.

## Prerrequisitos

- Terraform >= 1.6
- Imagen publicada en ECR (build desde `beauty/backend` con Dockerfile de producción cuando exista; hoy el dev usa `Dockerfile.dev`)

## Uso

```bash
cd beauty/infra
cp terraform.tfvars.example terraform.tfvars
# Editar tfvars y valores iniciales en SSM (o aplicar y luego actualizar SSM en consola)

terraform init
terraform plan
terraform apply
```

Tras el primer `apply`, sustituir en SSM los placeholders `CHANGE_ME` (o usar `aws ssm put-parameter`) para URLs y secretos reales. Los parámetros tienen `lifecycle.ignore_changes` en el valor para no pisar rotaciones manuales en siguientes applies.

## Outputs

- `api_gateway_url` – URL base; el frontend debe usar `VITE_BEAUTY_API_URL` apuntando aquí (o al dominio custom).
- `backend_lambda_arn` – para CI/CD o alarmas adicionales.

## Estado remoto

Descomentar y configurar el bloque `backend "s3"` en `main.tf` para equipo (misma convención que `professionals/infra`).
