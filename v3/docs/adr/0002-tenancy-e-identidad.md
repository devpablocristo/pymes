# ADR 0002: tenant explícito y credenciales cortas

**Estado:** aceptada.  
**Decisión:** cada organización tiene schema contable provisionado
explícitamente; ausencia significa `ORG_NOT_PROVISIONED`, nunca se deriva un
schema por concatenación. Workers usan JWT interno firmado, audience específica,
`org_id` obligatorio. Compose valida una clave pública Ed25519 fija de
desarrollo. Producción prohíbe semillas locales y firma bytes JWT crudos con una
versión numérica explícita de una clave Cloud KMS `EC_SIGN_ED25519`. Al arrancar
se valida algoritmo, nombre, clave pública, CRC32C y una firma de desafío; si
falla cualquier comprobación el worker no queda ready. El `kid` deriva de la
clave pública, por lo que es estable para esa versión, y JWKS publica la versión
activa junto con versiones anteriores durante una rotación.

**Consecuencia:** se elimina el fallback de tenancy observado en OA y se añade
un plano de provisionamiento/auditoría antes de crear documentos.

Clerk verifica la sesión, pero sus claims no autorizan directamente comandos
de negocio. El BFF resuelve un principal local con organización, actor, rol,
permisos, estado de membresía y estado de organización. Una membresía ausente o
inactiva falla cerrada; `owner` y `admin` pueden mutar, mientras `member` y
`viewer` son de sólo lectura. Toda mutación exige que la organización esté
exactamente en `ready`; `pending`, `failed`, `suspended` o un estado desconocido
devuelven `ORG_NOT_PROVISIONED` antes de escribir. PostgreSQL repite esa regla
dentro de la transacción tenant para que no dependa únicamente del BFF.

El subject de la credencial siempre identifica el workload. `request_id`,
`correlation_id`, actor y actor delegado se copian sólo desde contexto validado;
un principal de otra organización nunca se propaga. Cloud Run IAM autentica el
transporte privado y el JWT conserva autorización y trazabilidad por
organización; ninguna de las dos capas sustituye a la otra.
