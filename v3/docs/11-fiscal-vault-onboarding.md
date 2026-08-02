# Vault fiscal y onboarding ARCA por organización

## Propiedad y límites

Pymes es un SaaS y no posee un CUIT, certificado ni punto de venta global.
Cada organización administra su identidad fiscal; Fiscal Adapter es el único
workload que puede crear claves privadas, descifrarlas para firmar WSAA y
acceder a certificados, tickets y artefactos ARCA. El BFF expone una fachada
autorizada, pero nunca recibe ni devuelve una clave privada.

En builds integrados, `arca-facturacion` se consume exclusivamente como paquete
npm publicado. Fiscal usa su entrada explícita para autorizar y consultar un
punto de venta, tipo y número ya reservados por Pymes y para leer el último
autorizado de una secuencia. Los métodos de autonumeración no forman parte del
adapter. La extensión `2.6.0` está publicada y fijada en el lockfile; su
implementación corresponde al merge `69a0d4cf5110aa280fa50420dc0d13f8010115d0`.

## Flujo de onboarding

1. Un owner/admin habilita el rollout `fiscal_real_enabled` para iniciar el
   onboarding. El flag habilita la capacidad, pero no acredita que Fiscal esté
   listo para emitir.
2. Un administrador tenant solicita un CSR con CUIT, razón social, nombre común
   y ambiente.
3. Fiscal genera RSA de 2048 bits y CSR SHA-256 con `C=AR` y
   `serialNumber=CUIT <cuit>`.
4. Fiscal cifra inmediatamente la clave privada con una DEK aleatoria
   AES-256-GCM; Cloud KMS cifra la DEK con la misma AAD.
5. La respuesta contiene sólo el CSR y metadatos sin material criptográfico.
6. El cliente registra el CSR en ARCA y carga el certificado emitido.
7. Fiscal verifica formato PEM, vigencia, issuer del ambiente, CUIT y que la
   clave pública coincida con la clave privada almacenada.
8. El cliente configura puntos de venta. Antes de aceptar un certificado de
   producción debe existir un certificado vigente y al menos un POS de
   homologación del mismo CUIT validado y habilitado.
9. Fiscal valida WSAA/WSFE en homologación y habilita el punto de venta sólo
   después de una respuesta correcta. Para el MVP, consulta
   `FECompUltimoAutorizado` para FA/NDA/NCA, FB/NDB/NCB y FC/NDC/NCC y exige
   cero en las nueve secuencias: el POS debe ser dedicado, nuevo y vacío.
   `POINT_OF_SALE_NOT_EMPTY` impide usar uno con numeración previa. STG y PRD
   usan credenciales y KMS distintos.

La solicitud de CSR es idempotente por organización. Reusar la clave con otro
payload devuelve `IDEMPOTENCY_KEY_REUSED`; las cargas de certificado usan
versión optimista.

## Fachada pública del BFF

El navegador autenticado con Clerk llama únicamente a Pymes:

- `POST /api/v1/organizations/{organizationId}/fiscal/credentials/csr`;
- `GET|PUT /api/v1/organizations/{organizationId}/fiscal/credentials/{credentialId}`;
- `PUT /api/v1/organizations/{organizationId}/fiscal/credentials/{credentialId}/points-of-sale/{pointOfSale}`;
- `POST .../points-of-sale/{pointOfSale}/validate`.

Lectura requiere membresía activa; cualquier mutación requiere `owner` o
`admin` y una organización lista. El BFF traduce DTOs, propaga identidad
interna firmada y no persiste CSR ni certificado. El certificado PEM es
`writeOnly`; ninguna respuesta pública contiene PEM, claves, tickets, XML,
envelopes KMS ni artefactos privados. Fiscal sigue siendo la única fuente de
verdad de las credenciales.

El identificador público de credencial es el valor opaco que emite Fiscal,
con formato `fcred_<base64url>`; no es un UUID y el BFF no lo reescribe. El
mismo valor se usa en respuestas, paths, configuración de POS, validaciones y
evidencia operacional. Tanto la fachada pública como el contrato interno
rechazan cualquier identificador fuera de `^fcred_[A-Za-z0-9_-]{8,80}$`.

## Cifrado

El envelope tiene formato `aes-256-gcm+kms-v1`. Cada operación genera DEK e IV
nuevos. La AAD incluye versión, organización, credential ID, ambiente y
propósito:

```text
pymes-fiscal-v1\0<org>\0<credential>\0<environment>\0<private-key|certificate>
```

Tickets WSAA agregan servicio; artefactos WSFE agregan request, artifact ID y
tipo. Cambiar organización, credencial, ambiente o propósito hace fallar la
autenticación. Claves privadas sólo se descifran en memoria durante la firma.
No se registran PEM, XML, tickets, CUIT, destinatarios ni payloads.

Cada entorno usa una clave KMS regional independiente. La service account de
Fiscal necesita sólo `cloudkms.cryptoKeyEncrypterDecrypter` sobre su clave.
API, worker, navegador y Accounting no reciben ese permiso.

## Persistencia y tenancy

Fiscal posee una base lógica independiente con:

- `fiscal.credentials`;
- `fiscal.points_of_sale`;
- `fiscal.wsaa_tickets`;
- `fiscal.encrypted_artifacts`;
- `fiscal.requests`;
- `fiscal.mock_authorizations`.

Todas las tablas tenant tienen clave compuesta con `organization_id`, RLS
habilitado y forzado. Cada operación abre una transacción y fija
`app.organization_id` localmente; una conexión sin contexto ve cero filas.
Tests PostgreSQL demuestran aislamiento con IDs iguales en dos organizaciones.

## Consistencia fiscal

Pymes reserva número y congela snapshot antes del outbox. Fiscal consulta
primero exactamente ese comprobante. La reconciliación compara todos los
escalares que devuelve el SDK publicado —referencia, concepto, fecha, receptor,
totales, no gravado, tributos, período de servicios, moneda, cotización,
condición IVA cuando está presente y modo CAE—, además del desglose de IVA y
los comprobantes asociados que WSFE devuelve dentro de `FECompConsultar`. Si
existe y coincide, recupera el CAE sin emitir; si difiere, devuelve
`VOUCHER_MISMATCH`. Si se pierde la conexión
después de despachar `FECAESolicitar`, responde `uncertain`; Pymes conserva el
número y sólo vuelve a consultar esa referencia. Un CAE conocido prevalece
sobre un fallo al persistir el artefacto operacional.

La integración de runtime importa exclusivamente la superficie versionada
`@devpablocristo/arca-facturacion/explicit`; no admite el cliente raíz ni
compatibilidad con métodos legacy.

El chequeo de baseline no calcula el siguiente número y no llama ningún método
de autonumeración. La operación tipada `lastAuthorizedVoucher` es de sólo
lectura y conserva la referencia POS/tipo devuelta. Deshabilitar un POS no
habilita después una vía rápida: para volver a activarlo hay que ejecutar de
nuevo el endpoint de validación, que repite las nueve consultas.

## Operación

El modo `mock` permanece para CI y desarrollo determinístico. `arca` requiere:

- `FISCAL_KMS_KEY_NAME`;
- `FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN`;
- `FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN`;
- `PYMES_INTERNAL_ISSUER`;
- `PYMES_INTERNAL_JWKS_JSON`;
- `FISCAL_DATABASE_URL`.

La clave KMS local está prohibida fuera de `development|test`.
`.github/workflows/v3-arca-homologation.yml` ejecuta la homologación real como
job manual fijado a STG: exige `main`, CI verde para el SHA exacto, confirmación
`VALIDATE_ARCA_HOMOLOGATION_STG`, primer intento y auditoría de controles antes
de leer la sesión Clerk temporal. Rechaza credenciales de producción, vencidas
o no listas y llama exclusivamente
`POST .../points-of-sale/{pointOfSale}/validate`. Un éxito prueba WSAA/WSFE,
registra `validated_at` y habilita el punto; no reserva número, crea venta ni
invoca autorización.

`make protected-live-validation-test` cubre ese flujo con transporte falso y
demuestra que certificados, tokens y cuerpos de error no llegan a argumentos,
logs o artifacts. La suite determinística nunca llama ARCA y la existencia del
workflow no sustituye un run verde del piloto. No se debe emitir desde v2 y v3
con el mismo punto de venta.

`make fiscal-e2e` agrega una barrera distinta y sin secretos: levanta
el proceso real de Fiscal Adapter contra PostgreSQL, mantiene autoridad y KMS
externos simulados, y recorre por HTTP la fachada BFF completa —CSR, certificado
de prueba firmado para la clave del CSR, configuración y validación de POS—.
El gate exige que `fcred_*` conserve identidad byte a byte en cada salto.
