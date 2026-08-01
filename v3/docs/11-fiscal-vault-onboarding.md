# Vault fiscal y onboarding ARCA por organización

## Propiedad y límites

Pymes es un SaaS y no posee un CUIT, certificado ni punto de venta global.
Cada organización administra su identidad fiscal; Fiscal Adapter es el único
workload que puede crear claves privadas, descifrarlas para firmar WSAA y
acceder a certificados, tickets y artefactos ARCA. El BFF expone una fachada
autorizada, pero nunca recibe ni devuelve una clave privada.

`arca-facturacion` se consume exclusivamente como paquete npm publicado. Fiscal
usa su entrada explícita para autorizar y consultar un punto de venta, tipo y
número ya reservados por Pymes. Los métodos de autonumeración no forman parte
del adapter.

## Flujo de onboarding

1. Un administrador tenant solicita un CSR con CUIT, razón social, nombre común
   y ambiente.
2. Fiscal genera RSA de 2048 bits y CSR SHA-256 con `C=AR` y
   `serialNumber=CUIT <cuit>`.
3. Fiscal cifra inmediatamente la clave privada con una DEK aleatoria
   AES-256-GCM; Cloud KMS cifra la DEK con la misma AAD.
4. La respuesta contiene sólo el CSR y metadatos sin material criptográfico.
5. El cliente registra el CSR en ARCA y carga el certificado emitido.
6. Fiscal verifica formato PEM, vigencia, issuer del ambiente, CUIT y que la
   clave pública coincida con la clave privada almacenada.
7. El cliente configura puntos de venta. Homologación debe estar activa antes
   de aceptar un certificado de producción.
8. La organización habilita `fiscal_real_enabled` sólo después de validar
   WSAA/WSFE en homologación. STG y PRD usan credenciales y KMS distintos.

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
primero exactamente ese comprobante. Si existe y coincide, recupera el CAE sin
emitir; si difiere, devuelve `VOUCHER_MISMATCH`. Si se pierde la conexión
después de despachar `FECAESolicitar`, responde `uncertain`; Pymes conserva el
número y sólo vuelve a consultar esa referencia. Un CAE conocido prevalece
sobre un fallo al persistir el artefacto operacional.

## Operación

El modo `mock` permanece para CI y desarrollo determinístico. `arca` requiere:

- `FISCAL_KMS_KEY_NAME`;
- `FISCAL_ARCA_HOMOLOGATION_ISSUER_PATTERN`;
- `FISCAL_ARCA_PRODUCTION_ISSUER_PATTERN`;
- `PYMES_INTERNAL_ISSUER`;
- `PYMES_INTERNAL_JWKS_JSON`;
- `FISCAL_DATABASE_URL`.

La clave KMS local está prohibida fuera de `development|test`. La homologación
real se ejecuta como job protegido; la suite determinística nunca llama ARCA.
No se debe emitir desde v2 y v3 con el mismo punto de venta.
