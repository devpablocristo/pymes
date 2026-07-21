# Estándar CRUD Pymes

Este documento fija el contrato objetivo para los CRUDs operativos del panel. Los módulos existentes pueden estar en transición, pero todo CRUD nuevo debe nacer con este contrato.

## Rutas

- `POST /v1/{entities}` crea un registro y devuelve `201`.
- `GET /v1/{entities}` lista registros activos.
- `GET /v1/{entities}/{id}` obtiene detalle.
- `PATCH /v1/{entities}/{id}` actualiza parcialmente.
- `DELETE /v1/{entities}/{id}` archiva o soft-deletea de forma idempotente.
- `POST /v1/{entities}/{id}/archive` es alias explícito del soft delete.
- `POST /v1/{entities}/{id}/restore` restaura.
- `DELETE /v1/{entities}/{id}/hard` borra físicamente solo si el dominio lo permite.
- `GET /v1/{entities}/archived` lista archivados con la misma forma de respuesta que la lista activa.

Las acciones de dominio que no son CRUD puro usan `POST /v1/{entities}/{id}/{action}` y deben documentar si requieren `Idempotency-Key`.

## Lista

Toda lista visible debe responder:

```json
{
  "items": [],
  "total": 0,
  "has_more": false,
  "next_cursor": ""
}
```

Parámetros comunes:

- `limit`: límite normalizado por helper compartido.
- `after`: cursor UUID.
- `search`: texto libre si el dominio lo soporta.
- `sort` y `order`: solo si el dominio tiene ordenamiento estable.
- Filtros de dominio: explícitos y tipados.

## Errores

Formato único:

```json
{"code":"VALIDATION","message":"mensaje humano"}
```

Códigos base:

- `VALIDATION`
- `NOT_FOUND`
- `CONFLICT`
- `UNAUTHORIZED`
- `FORBIDDEN`
- `ARCHIVED`
- `UPSTREAM_UNAVAILABLE`
- `INTERNAL`

No se expone `err.Error()` al cliente HTTP. El detalle técnico va a logs estructurados.

## Arquitectura Por Módulo

Cada módulo Go sigue la arquitectura hexagonal documentada:

- `handler.go`: adapter HTTP, parsing y mapeo DTO/dominio.
- `handler/dto/dto.go`: todos los DTOs HTTP.
- `usecases.go`: lógica de negocio y ports.
- `usecases/domain/entities.go`: entidades de dominio.
- `repository.go`: port e implementación GORM.
- `repository/models/models.go`: modelos DB si difieren del dominio.

Los usecases no importan DTOs ni modelos DB. Los handlers no contienen DTOs inline.

## Compatibilidad

Toda excepción al contrato debe estar registrada en `docs/TECH_DEBT_LEDGER.md` con:

- motivo,
- dueño,
- riesgo,
- criterio de retiro,
- test o métrica que habilita eliminarla.
