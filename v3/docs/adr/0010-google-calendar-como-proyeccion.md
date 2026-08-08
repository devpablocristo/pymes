# ADR 0010: Google Calendar como proyección externa

## Estado

Parcialmente reemplazada por ADR 0014. La proyección sigue vigente, pero
Calendar y Meet pasan a ser adapters de Scheduling.

## Contexto

Scheduling necesita integrar Calendar y Meet sin delegar a Google la
disponibilidad, integridad de recursos, estados o propiedad del turno. Las APIs
del proveedor pueden contestar tarde, perder una respuesta ya procesada,
rechazar un ETag o revocar un refresh token.

## Decisión

Pymes conserva OAuth, conexiones, mappings, cifrado, outbox y reconciliación en
el contexto vertical `calendars`. Google es una proyección unidireccional y
eventualmente consistente:

- calendario secundario por conexión;
- IDs de evento determinísticos;
- `requestId` de Meet independiente;
- snapshot digest externo;
- ETag/`If-Match`;
- consulta exacta ante `409`, `412` o resultado incierto;
- tokens cifrados mediante envelope encryption y AAD tenant;
- SDK Platform publicado detrás de un adapter local consumer-owned.

La reserva local termina aun si Google está caído. El worker reintenta y
reconcilia. La sincronización inversa no forma parte del MVP.

## Consecuencias

- No se duplica un evento por repetir una entrega.
- Google no puede cambiar silenciosamente la fuente de verdad.
- La revocación degrada sólo esa conexión.
- Se necesita un KMS y un OAuth client distintos en STG y PRD.
- Los cambios externos concurrentes producen reconciliación explícita, no
  overwrite ciego.
- La futura sincronización bidireccional requerirá otro ADR y un modelo de
  conflicto adicional.
