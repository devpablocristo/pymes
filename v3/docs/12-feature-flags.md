# Rollout de capacidades por organización

Pymes controla la activación de capacidades mediante una única configuración
tenant en `app.organization_feature_flags`. Los flags no reemplazan permisos,
estado de organización ni configuración de proveedores: una operación sólo
puede ejecutarse cuando todas esas condiciones son válidas.

## Contrato

| Flag | Frontera protegida | Default |
|---|---|---|
| `scheduling_enabled` | API administrativa, booking público y acciones por token de Agenda | `false` |
| `whatsapp_enabled` | creación/consulta de intenciones y dispatcher de PerGo | `false` |
| `google_calendar_enabled` | OAuth, conexiones, proyección y reconciliación Google | `false` |
| `fiscal_real_enabled` | onboarding fiscal, leasing y reconciliación ARCA | `false` |

El default es siempre cerrado. Una fila ausente, un repositorio no configurado
o un error al consultar la configuración nunca habilita una capacidad.

La configuración se consulta y reemplaza mediante:

```text
GET /api/v1/organizations/{organizationId}/features
PUT /api/v1/organizations/{organizationId}/features
```

Lectura requiere una membresía activa de la misma organización. Escritura
requiere `owner` o `admin`, organización `ready` y `expected_version`. `PUT`
reemplaza los cuatro valores en una transacción; una versión vieja devuelve
`FEATURE_VERSION_CONFLICT` y no aplica cambios parciales.

## Persistencia y auditoría

`organization_feature_flags` tiene una sola fila por organización, RLS
habilitada y forzada, versión optimista, actor y timestamp. Al provisionar una
organización se crea la versión 1 con todos los valores en `false`.

Cada actualización agrega el snapshot completo a
`organization_feature_flag_audit` dentro de la misma transacción. La auditoría
no tiene FK deliberadamente: debe sobrevivir a limpiezas de lifecycle y no
aceptar borrados en cascada. `UPDATE`, `DELETE` y `TRUNCATE` están bloqueados
por trigger. Ningún contexto mantiene un flag alternativo; el valor histórico
de `notification_settings.whatsapp_enabled` se migra a la configuración
central.

## Aplicación en runtime

Las interfaces `FeatureGate` pertenecen a cada consumidor. `organization`
implementa esas interfaces sin exponer su dominio a Scheduling, Notifications,
Calendars o Commerce, y `wire` es el único lugar que conecta la implementación.

- Scheduling verifica el flag después de resolver de forma segura el tenant,
  incluso en rutas públicas y tokens de acción.
- Notifications vuelve a comprobarlo antes de PerGo. Si se desactiva con una
  intención ya encolada, el evento se reconoce sin enviar; reactivar después
  no produce un mensaje sorpresivo.
- Calendars reconoce sin proyectar eventos pendientes de una organización
  desactivada y omite sus reconciliaciones. Google nunca bloquea el turno.
- Commerce no alquila `FiscalAuthorizationRequested` ni reconcilia resultados
  inciertos mientras Fiscal real está desactivado. El número reservado no se
  reutiliza; al activar la capacidad el mismo evento durable continúa.

Los webhooks de PerGo permanecen activos aunque el flag se desactive: aceptar
un estado firmado y ya emitido es necesario para converger. Desactivar una
capacidad tampoco borra configuración, OAuth, turnos, notificaciones,
credenciales ni auditoría.

## Flag global y flag tenant

Las variables de workload como `PYMES_PERGO_ENABLED` y
`PYMES_GOOGLE_CALENDAR_ENABLED` deciden si el adapter existe en ese entorno.
Los flags tenant deciden qué organización puede utilizarlo. Ambas condiciones
son obligatorias:

```text
workload configurado AND organización habilitada AND permiso válido
```

`fiscal_real_enabled` habilita el onboarding y el relay ARCA para la
organización; no acredita por sí solo que una credencial o punto de venta esté
listo. Fiscal conserva sus validaciones de ambiente, certificado, CUIT y punto
de venta.

## Operación

1. verificar que el proveedor y su configuración estén listos en el entorno;
2. leer la versión actual;
3. cambiar un tenant piloto mediante `PUT`;
4. observar backlog y errores estables sin inspeccionar PII;
5. ampliar el rollout sólo después de validar el piloto;
6. para rollback, volver el flag a `false`; no borrar datos ni eventos.

Los códigos públicos son `FEATURE_DISABLED` cuando la capacidad está apagada y
`FEATURE_VERSION_CONFLICT` ante una escritura concurrente. Un fallo técnico al
leer la configuración se devuelve como indisponibilidad del contexto y se
reintenta en workers; nunca se convierte silenciosamente en `true`.
