# ADR 0012: PerGo como servicio de entrega de WhatsApp

**Estado:** aceptada.  
**Fecha:** 2026-08-01.

## Contexto

Pymes necesita confirmaciones, recordatorios, reprogramaciones, cancelaciones y
ofertas de waitlist sin incorporar credenciales ni detalles de cada proveedor
WhatsApp al dominio de Agenda. PerGo ya resuelve workspace, canales, cola y
webhooks y expone un contrato HTTP estable.

El turno no puede depender de la disponibilidad de WhatsApp. A la vez, una
respuesta perdida no debe producir dos mensajes y un webhook público no puede
permitir que el remitente elija libremente otro tenant.

## Decisión

Pymes Notifications es un contexto del monolito modular. Es dueño de la
intención, template versionado, momento, idempotencia y proyección de estado.
PerGo se usa como servicio privado y es dueño de credenciales, proveedor,
transporte y entrega.

La integración vive exclusivamente en `notifications/pergo.go`, con modelos y
helpers propios. Scheduling comunica la intención mediante el outbox; no llama
al adapter. El relay conserva leases, backoff, DLQ y replay.

Pymes entrega una identidad tenant-aware y determinística en `X-Trace-ID`.
PerGo la devuelve sin cambios en el webhook firmado. El BFF obtiene
organización e intención desde esa identidad sólo después de verificar HMAC,
timestamp y workspace. Se usa un callback global y no un `org_id` suministrado
en el path.

## Alternativas descartadas

- Enviar desde la transacción del turno: introduce fallo parcial y latencia.
- Llamar PerGo desde el navegador: expone credenciales y rompe ownership.
- Incorporar el runtime de PerGo al monolito: acopla proveedores y operación.
- Aceptar organización en la URL del webhook: el evento real de PerGo no trae
  metadata de Pymes y el path sería una entrada tenant no autenticada.
- Crear un microservicio Go de Notifications: agrega operación sin una frontera
  de datos o escalado que lo justifique.

## Consecuencias

Una caída de PerGo aumenta el backlog, pero no bloquea reservas. Pymes debe
operar la clave de API y la firma del webhook por entorno, conservar el inbox y
monitorear incertidumbres. El contenido sigue persistido para poder reintentar,
por lo que backups y acceso a PostgreSQL se tratan como PII.

La sincronización es eventualmente consistente. El estado comercial permanece
en Pymes y el external message ID es sólo una proyección; ningún estado de
entrega modifica por sí solo el turno.

