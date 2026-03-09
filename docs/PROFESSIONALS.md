# Professionals

`professionals` es una vertical delgada con schema y backend propios.

## Dominio

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

## Integracion con control-plane

`professionals` consume capacidades transversales via HTTP:

- bootstrap y settings de organizacion
- customers y parties
- products
- appointments
- quotes, sales y payment links

Regla de borde:

- no importa dominio interno de `control-plane`
- integra por clientes HTTP y contratos internos
- reutiliza solo runtime tecnico compartido desde `control-plane/shared/` y `pkgs/`

## Superficie local

- backend: `http://localhost:8181`
- frontend unificado: `http://localhost:5180`
- AI unificado: `http://localhost:8200`

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
