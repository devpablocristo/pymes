# ADR 0013: rollout de capacidades por organización

**Estado:** aceptada.
**Fecha:** 2026-08-01.

## Contexto

Agenda, WhatsApp, Google Calendar/Meet y ARCA real se incorporan gradualmente.
Una variable global de despliegue no permite pilotear una organización ni
detener una integración problemática sin afectar a todos los clientes. A la
vez, mantener switches dentro de cada contexto crea fuentes de verdad
contradictorias y hace difícil auditar quién habilitó una capacidad.

## Decisión

`organization` es dueño de una configuración versionada con cuatro flags
cerrados por default: `scheduling_enabled`, `whatsapp_enabled`,
`google_calendar_enabled` y `fiscal_real_enabled`.

La actualización reemplaza el snapshot completo con control optimista y agrega
una fila inmutable de auditoría en la misma transacción. Todas las tablas usan
RLS tenant. Cada contexto define su propio puerto mínimo `Enabled`; la
implementación concreta se conecta sólo en `wire`.

La autorización, los estados de organización y la configuración del proveedor
siguen siendo controles independientes. Un flag habilitado no concede un
permiso ni convierte una integración incompleta en lista.

Las fronteras HTTP y los workers comprueban el flag en servidor. Los eventos
durables que no deben perderse permanecen pendientes o se reconocen sin efecto,
según la semántica documentada de cada integración. Los webhooks firmados
continúan convergiendo estados emitidos antes de una desactivación.

## Alternativas descartadas

- Variables de entorno solamente: no permiten rollout ni rollback tenant.
- Un servicio remoto de feature flags en el camino crítico: agrega
  disponibilidad y una nueva fuente de identidad tenant.
- Un booleano en cada contexto: duplica ownership y auditoría.
- Cache obligatoria: introduce una ventana en la que un rollback de seguridad
  puede seguir activo.
- Flags enviados por el navegador: permiten manipulación del cliente.

## Consecuencias

Cada comprobación consulta PostgreSQL y falla cerrada. Esto agrega una lectura
pequeña en los límites, aceptable durante el piloto; una cache sólo se
incorporará con invalidación explícita y métricas que demuestren necesidad.

Desactivar no elimina datos ni revoca automáticamente OAuth, certificados o
mensajes ya aceptados. Los runbooks deben distinguir el flag de producto del
estado operativo del proveedor. La API usa versión optimista para evitar que
dos administradores sobrescriban cambios concurrentes.
