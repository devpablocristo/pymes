# Baseline Auditoria CRUD

Este documento resume la primera version ejecutable de la auditoria CRUD usada para la Fase 3 del saneamiento.

## Comandos

- `make audit-crud`: reporte Markdown informativo.
- `make audit-crud-json`: reporte JSON para automatizar o guardar snapshots.
- `make audit-crud-strict`: falla si un recurso esperado no registra handlers/rutas canonicas.

## Que Verifica

- Rutas registradas en handlers Go visibles.
- Metodos canonicos del handler: list, archived, create, get, update, delete, archive, restore y hard delete.
- IDs de recursos frontend declarados en `frontend/src/crud/resourceConfigs*.tsx`.
- Drift de errores por respuestas `gin.H{"error": ...}`.
- Shape de listas con `items`, `total`, `has_more` y `next_cursor`.

## Lectura Del Resultado Actual

- Core comercial tiene la mayoria de rutas CRUD canonicas presentes.
- `products` y `services` ya no muestran drift de error shape en el handler principal.
- Persisten respuestas de error no canónicas en gran parte de core y verticales.
- `purchases`, `returns`, `recurring`, `payments`, credit notes e intakes todavia tienen listas parciales o shape no canonico detectado.
- `inventory` es una vista operativa derivada de productos: no debe normalizarse como CRUD puro sin decidir primero su contrato.
- Restaurants y professionals tienen recursos visibles sin archive/restore/hard delete completo.
- `restaurant-table-sessions` es lifecycle open/close, no CRUD puro; debe quedar explicitamente clasificado como no-CRUD o recibir contrato propio.

## Criterio De Uso

La auditoria no cambia comportamiento. Se usa para ordenar el refactor:

1. Normalizar primero recursos con rutas completas pero error/list shape inconsistente.
2. Despues agregar rutas faltantes de forma aditiva en recursos visibles.
3. Por ultimo activar `audit-crud-strict` en CI cuando la allowlist de excepciones este documentada y estable.
