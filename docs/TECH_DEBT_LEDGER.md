# Ledger De Deuda Técnica

Este ledger convierte compatibilidades, fallbacks y parches en deuda explícita. Nada se elimina sin evidencia de reemplazo, tests y criterio de retiro cumplido.

## Estados

- `active`: deuda conocida y todavía necesaria.
- `migrating`: existe reemplazo y se está moviendo consumo.
- `ready_to_remove`: sin uso observado y con tests de reemplazo.
- `removed`: eliminado.

## Entradas Iniciales

| id | área | estado | problema | riesgo | criterio de retiro |
|---|---|---|---|---|---|
| DEBT-001 | Backend CRUD | active | Handlers mezclan `{code,message}` con `gin.H{"error": ...}`. | Frontend e integraciones reciben errores inconsistentes. | Todos los handlers CRUD usan helper estándar y contract tests de error pasan. |
| DEBT-002 | Backend CRUD | active | Listas mezclan `{items,total,has_more,next_cursor}` con solo `{items}`. | UI y AI necesitan lógica especial por recurso. | Todas las listas visibles pasan contract test de lista canónica. |
| DEBT-003 | Frontend CRUD | active | `restCrudDataSource` contiene excepciones para archive/delete por recurso. | El frontend conoce diferencias internas del backend. | Backend expone archive/delete canónico en todos los CRUDs y la allowlist queda vacía. |
| DEBT-004 | AI | active | `internal_chat` todavía depende de piezas de `ai/src/agents/service.py`. | El chat interno hereda acoplamiento y fallback legacy. | Dossier/config compartidos viven en módulo pequeño independiente y tests internos siguen pasando. |
| DEBT-005 | AI | active | `agents.service` concentra routing, tools, prompts, persistencia y fallback. | Alto costo de cambio y riesgo de respuestas no confiables. | Servicio legacy dividido por responsabilidades o aislado detrás de adapters probados. |
| DEBT-006 | Workshops | in_progress | Vehículos/bicicletas persisten en `workshops.customer_assets`; backend y frontend de OTs ya usan `asset_type`, `asset_id`, `asset_label` como contrato principal. `target_*` queda como alias legacy de lectura/escritura. | Queda compatibilidad de nombres/rutas por vertical y clientes externos que todavía envían `target_*`. | Próximo paso: medir uso de `target_*`, anunciar deprecación y retirar columnas/DTO legacy en una ventana controlada. |
| DEBT-007 | Productos | active | Soporte/rechazo especial de campo legacy `type` y fallback de imágenes. | API conserva reglas históricas fuera del DTO estándar. | Clientes migrados a campos actuales y tests de rechazo documentados. |
| DEBT-008 | Seeds | migrating | Datos demo históricos dejaban módulos con un solo registro visible. | Auditorías y E2E daban falsa confianza. | `make seed-reset` y `make seed-verify` pasan por DB y API para todos los módulos del contrato. |
| DEBT-009 | Verticales | active | Parsers HTTP y errores repetidos entre core, workshops, restaurants y professionals. | Cambios de contrato deben hacerse varias veces. | Verticales usan helpers compartidos para org, id, cursor, límite y errores. |
| DEBT-010 | Frontend | active | `ModulePage`, `CrudEntityEditorModal` y helpers de billing son hotspots grandes. | Modificaciones pequeñas tienen alto riesgo de regresión. | Componentes divididos por responsabilidad con tests equivalentes. |
| DEBT-011 | Payments | active | Listado global de pagos mantiene compatibilidad con respuesta vacía si falta `sale_id`. | Pantalla/API no expresa si pagos es recurso global o sale-scoped. | Decisión de producto tomada y contrato actualizado con test. |
| DEBT-012 | Medical | active | Posible duplicación de módulos core/verticales. | Se puede refactorizar algo que quizá es producto activo. | Dueño decide si medical queda como vertical activa o adaptador fino. |

## Cómo Agregar Una Entrada

1. Crear id `DEBT-###`.
2. Registrar área, estado, problema concreto y riesgo.
3. Definir un criterio de retiro verificable.
4. Vincular PRs o tests cuando avance de estado.
